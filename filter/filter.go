// Package filter holds back implausible GPS fixes before they reach Home
// Assistant, similar to Traccar's position filters. A rejected fix keeps the
// last accepted coordinates; the packet itself is still published so
// non-location data (OBD values etc.) isn't lost.
package filter

import (
	"math"

	"github.com/freman/gps2mqtt/mqtt"
)

type Config struct {
	Enabled       bool
	Invalid       bool    // reject fixes the device flags as not valid
	Zero          bool    // reject 0,0 coordinates
	MinSatellites int64   // reject fixes with fewer satellites, 0 disables
	MaxSpeed      float64 // km/h, reject jumps implying a higher speed, 0 disables
	StaticSpeed   float64 // km/h, below this the vehicle is treated as standing and the position is held, 0 disables

	// Jumps longer than JumpDistance metres are rejected when they imply a
	// speed above reported speed * SpeedFactor + SpeedMargin (km/h).
	// JumpDistance 0 disables this check.
	JumpDistance float64
	SpeedFactor  float64
	SpeedMargin  float64
}

func DefaultConfig() Config {
	return Config{
		Enabled:      true,
		Invalid:      true,
		Zero:         true,
		MaxSpeed:     250,
		StaticSpeed:  1,
		JumpDistance: 100,
		SpeedFactor:  1.5,
		SpeedMargin:  20,
	}
}

// After this many consecutive speed rejections the reference fix is assumed
// to be the outlier and the new fix is accepted.
const maxConsecutiveRejects = 3

type deviceState struct {
	last     mqtt.Fix
	rejected int
}

// Filter keeps per-device state. It is not safe for concurrent use.
type Filter struct {
	cfg   Config
	state map[string]*deviceState
}

func New(cfg Config) *Filter {
	return &Filter{cfg: cfg, state: make(map[string]*deviceState)}
}

// Apply checks the fix of p against the last accepted one for device and
// holds the position if it is rejected. It returns the rejection reason
// ("" when accepted) and false when the message should not be published at
// all because there is no earlier position to fall back on.
func (f *Filter) Apply(device string, p mqtt.Positioner) (string, bool) {
	if !f.cfg.Enabled {
		return "", true
	}

	fix := p.GPSFix()
	st := f.state[device]

	reason := f.check(st, fix)
	if reason == "" {
		// Trackers sometimes repeat a frozen position while driving and then
		// catch up. Keep the time the position was first reported so the
		// catch-up jump is judged over the whole interval.
		if st != nil && fix.Latitude == st.last.Latitude && fix.Longitude == st.last.Longitude {
			return "", true
		}

		f.state[device] = &deviceState{last: fix}
		return "", true
	}

	if st == nil {
		return reason, false
	}

	p.HoldPosition(st.last.Latitude, st.last.Longitude, reason)
	return reason, true
}

func (f *Filter) check(st *deviceState, fix mqtt.Fix) string {
	switch {
	case f.cfg.Zero && fix.Latitude == 0 && fix.Longitude == 0:
		return "zero"
	case f.cfg.Invalid && !fix.Valid:
		return "invalid"
	case f.cfg.MinSatellites > 0 && fix.Satellites < f.cfg.MinSatellites:
		return "satellites"
	}

	if st == nil {
		return ""
	}

	if f.cfg.StaticSpeed > 0 && fix.Speed < f.cfg.StaticSpeed {
		return "static"
	}

	// Skip the speed checks when the timestamps can't be compared sensibly
	// (same second, clock reset, long gap).
	dt := fix.Time.Sub(st.last.Time).Hours()
	if dt > 0 && dt <= 1 && f.tooFast(st.last, fix, distanceKm(st.last, fix), dt) {
		st.rejected++
		if st.rejected < maxConsecutiveRejects {
			return "speed"
		}
	}

	return ""
}

func (f *Filter) tooFast(last, fix mqtt.Fix, distKm, hours float64) bool {
	implied := distKm / hours

	if f.cfg.MaxSpeed > 0 && implied > f.cfg.MaxSpeed {
		return true
	}

	if f.cfg.JumpDistance > 0 && distKm*1000 > f.cfg.JumpDistance {
		reported := math.Max(last.Speed, fix.Speed)
		return implied > reported*f.cfg.SpeedFactor+f.cfg.SpeedMargin
	}

	return false
}

// distanceKm returns the great-circle distance between two fixes.
func distanceKm(a, b mqtt.Fix) float64 {
	const earthRadiusKm = 6371.0
	rad := math.Pi / 180

	dLat := (b.Latitude - a.Latitude) * rad
	dLon := (b.Longitude - a.Longitude) * rad
	h := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(a.Latitude*rad)*math.Cos(b.Latitude*rad)*math.Sin(dLon/2)*math.Sin(dLon/2)

	return 2 * earthRadiusKm * math.Asin(math.Sqrt(h))
}
