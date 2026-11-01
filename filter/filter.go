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
	DryRun        bool    // only mark rejected fixes, keep their coordinates
	Invalid       bool    // reject fixes the device flags as not valid
	Zero          bool    // reject 0,0 coordinates
	MinSatellites int64   // reject fixes with fewer satellites, 0 disables
	MaxSpeed      float64 // km/h, reject jumps implying a higher speed, 0 disables
	StaticSpeed   float64 // km/h, below this the vehicle is treated as standing and the position is held, 0 disables

	// Jumps are rejected when they are longer than the vehicle could have
	// driven between the two fixes, given the speeds at both ends and the
	// vehicle's MaxAcceleration and MaxDeceleration (m/s²), plus JumpDistance
	// metres of GPS error tolerance. MaxAcceleration 0 disables this check.
	MaxAcceleration float64
	MaxDeceleration float64
	JumpDistance    float64
}

func DefaultConfig() Config {
	return Config{
		Enabled:         true,
		Invalid:         true,
		Zero:            true,
		MaxSpeed:        250,
		StaticSpeed:     1,
		MaxAcceleration: 4, // 0-100 km/h in 7 s
		MaxDeceleration: 8,
		JumpDistance:    50,
	}
}

// After this many consecutive jump rejections the reference fix is assumed
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

	if f.cfg.DryRun {
		p.HoldPosition(fix.Latitude, fix.Longitude, reason+" (dry-run)")
		return reason, true
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

	if f.cfg.StaticSpeed > 0 && speed(fix) < f.cfg.StaticSpeed {
		return "static"
	}

	// Skip the jump checks when the timestamps can't be compared sensibly
	// (same second, clock reset, long gap).
	dt := fix.Time.Sub(st.last.Time).Seconds()
	if dt <= 0 || dt > 3600 {
		return ""
	}

	if reason := f.jump(st.last, fix, distanceKm(st.last, fix)*1000, dt); reason != "" {
		st.rejected++
		if st.rejected < maxConsecutiveRejects {
			return reason
		}
	}

	return ""
}

// jump reports why moving distM metres in dt seconds from last to fix is
// implausible, or "" if it isn't.
func (f *Filter) jump(last, fix mqtt.Fix, distM, dt float64) string {
	if f.cfg.MaxSpeed > 0 && distM/dt*3.6 > f.cfg.MaxSpeed {
		return "speed"
	}

	if f.cfg.MaxAcceleration > 0 {
		vmax := math.Inf(1)
		if f.cfg.MaxSpeed > 0 {
			vmax = f.cfg.MaxSpeed / 3.6
		}

		limit := maxDistance(speed(last)/3.6, speed(fix)/3.6, dt, f.cfg.MaxAcceleration, f.cfg.MaxDeceleration, vmax)
		if distM > limit+f.cfg.JumpDistance {
			return "jump"
		}
	}

	return ""
}

// speed prefers the vehicle's own (OBD) speed over the GPS one. Trackers
// omit the OBD speed when it is zero, so 0 means unknown.
func speed(fix mqtt.Fix) float64 {
	if fix.VehicleSpeed > 0 {
		return fix.VehicleSpeed
	}

	return fix.Speed
}

// maxDistance returns the longest distance in metres a vehicle can cover in
// dt seconds starting at v0 and ending at v1 (m/s): accelerating at a, then
// cruising at vmax if reached, then braking at b (m/s²).
func maxDistance(v0, v1, dt, a, b, vmax float64) float64 {
	if b <= 0 {
		b = a
	}
	vmax = math.Max(vmax, math.Max(v0, v1))

	// Peak speed where accelerating from v0 and braking to v1 take dt.
	vp := math.Min((dt+v0/a+v1/b)/(1/a+1/b), vmax)

	t1 := math.Max((vp-v0)/a, 0)
	t2 := math.Max((vp-v1)/b, 0)
	cruise := math.Max(dt-t1-t2, 0)

	return (v0+vp)/2*t1 + (vp+v1)/2*t2 + vp*cruise
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
