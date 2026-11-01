package filter

import (
	"math"
	"testing"
	"time"

	"github.com/freman/gps2mqtt/mqtt"
	"github.com/stretchr/testify/assert"
)

type fakePacket struct {
	fix    mqtt.Fix
	reason string
}

func (p *fakePacket) GPSFix() mqtt.Fix { return p.fix }

func (p *fakePacket) HoldPosition(lat, lon float64, reason string) {
	p.fix.Latitude, p.fix.Longitude, p.reason = lat, lon, reason
}

var t0 = time.Date(2024, 3, 18, 14, 0, 0, 0, time.UTC)

func moving(sec int, lat, lon float64) *fakePacket {
	return &fakePacket{fix: mqtt.Fix{
		Time:      t0.Add(time.Duration(sec) * time.Second),
		Latitude:  lat,
		Longitude: lon,
		Speed:     50,
		Valid:     true,
	}}
}

func TestAcceptsPlausibleTrack(t *testing.T) {
	f := New(DefaultConfig())

	for i, p := range []*fakePacket{
		moving(0, 50.0870, 14.4210),
		moving(10, 50.0871, 14.4215), // ~40 m in 10 s
		moving(20, 50.0872, 14.4220),
	} {
		reason, publish := f.Apply("dev", p)
		assert.Equal(t, "", reason, i)
		assert.True(t, publish, i)
		assert.Equal(t, "", p.reason, i)
	}
}

func TestRejectsJump(t *testing.T) {
	f := New(DefaultConfig())
	f.Apply("dev", moving(0, 50.0870, 14.4210))

	// ~11 km in 10 s
	p := moving(10, 50.1870, 14.4210)
	reason, publish := f.Apply("dev", p)
	assert.Equal(t, "speed", reason)
	assert.True(t, publish)
	assert.Equal(t, 50.0870, p.fix.Latitude)
	assert.Equal(t, 14.4210, p.fix.Longitude)
	assert.Equal(t, "speed", p.reason)

	// The track continues from the original position.
	reason, _ = f.Apply("dev", moving(20, 50.0871, 14.4215))
	assert.Equal(t, "", reason)
}

func TestRecoversFromBadReference(t *testing.T) {
	f := New(DefaultConfig())
	f.Apply("dev", moving(0, 50.1870, 14.4210)) // outlier accepted as the first fix

	for i := 1; i < maxConsecutiveRejects; i++ {
		reason, _ := f.Apply("dev", moving(i*10, 50.0870, 14.4210))
		assert.Equal(t, "speed", reason, i)
	}

	reason, _ := f.Apply("dev", moving(maxConsecutiveRejects*10, 50.0870, 14.4210))
	assert.Equal(t, "", reason)
}

func TestAcceptsCatchUpAfterFrozenPosition(t *testing.T) {
	f := New(DefaultConfig())
	f.Apply("dev", moving(0, 50.0870, 14.4210))

	// Tracker repeats the same coordinates for a minute while driving at 50 km/h.
	for sec := 10; sec <= 60; sec += 10 {
		reason, _ := f.Apply("dev", moving(sec, 50.0870, 14.4210))
		assert.Equal(t, "", reason, sec)
	}

	// ~970 m catch-up: 350 km/h over the last 10 s, but 50 km/h since the
	// position froze.
	reason, _ := f.Apply("dev", moving(70, 50.0957, 14.4210))
	assert.Equal(t, "", reason)
}

func withSpeed(p *fakePacket, gps, obd float64) *fakePacket {
	p.fix.Speed, p.fix.VehicleSpeed = gps, obd
	return p
}

func TestRejectsJumpBeyondAcceleration(t *testing.T) {
	f := New(DefaultConfig())
	f.Apply("dev", withSpeed(moving(0, 50.0870, 14.4210), 30, 0))

	// ~417 m in 10 s = 150 km/h on average, under MaxSpeed, but at 30 km/h
	// on both ends the car can cover at most ~217 m.
	p := withSpeed(moving(10, 50.09075, 14.4210), 30, 0)
	reason, _ := f.Apply("dev", p)
	assert.Equal(t, "jump", reason)
	assert.Equal(t, 50.0870, p.fix.Latitude)
}

func TestAcceptsHardAcceleration(t *testing.T) {
	f := New(DefaultConfig())
	f.Apply("dev", withSpeed(moving(0, 50.0870, 14.4210), 5, 0))

	// 5 -> 80 km/h in 10 s, ~120 m.
	reason, _ := f.Apply("dev", withSpeed(moving(10, 50.08808, 14.4210), 80, 0))
	assert.Equal(t, "", reason)
}

func TestPrefersVehicleSpeed(t *testing.T) {
	f := New(DefaultConfig())

	// GPS says standing, OBD says 30 km/h: not static.
	reason, _ := f.Apply("dev", withSpeed(moving(0, 50.0870, 14.4210), 0, 30))
	assert.Equal(t, "", reason)
	reason, _ = f.Apply("dev", withSpeed(moving(10, 50.0871, 14.4214), 0, 30))
	assert.Equal(t, "", reason)

	// GPS claims 150 km/h along with the jump, OBD says 30 km/h.
	reason, _ = f.Apply("dev", withSpeed(moving(20, 50.09085, 14.4214), 150, 30))
	assert.Equal(t, "jump", reason)
}

func TestDryRun(t *testing.T) {
	cfg := DefaultConfig()
	cfg.DryRun = true
	f := New(cfg)

	zero := moving(0, 0, 0)
	reason, publish := f.Apply("dev", zero)
	assert.Equal(t, "zero", reason)
	assert.True(t, publish)
	assert.Equal(t, "zero (dry-run)", zero.reason)

	f.Apply("dev", moving(10, 50.0870, 14.4210))

	p := moving(20, 50.1870, 14.4210)
	reason, publish = f.Apply("dev", p)
	assert.Equal(t, "speed", reason)
	assert.True(t, publish)
	assert.Equal(t, 50.1870, p.fix.Latitude, "coordinates untouched")
	assert.Equal(t, "speed (dry-run)", p.reason)
}

func TestMaxDistance(t *testing.T) {
	v := 30 / 3.6

	// Accelerate 30 -> 126 km/h for 6.7 s, brake back for 3.3 s.
	assert.InDelta(t, 216.7, maxDistance(v, v, 10, 4, 8, math.Inf(1)), 0.5)

	// Capped by vmax: 10 s at 30 km/h.
	assert.InDelta(t, 83.3, maxDistance(v, v, 10, 4, 8, v), 0.5)
}

func TestHoldsWhileStanding(t *testing.T) {
	f := New(DefaultConfig())
	f.Apply("dev", moving(0, 50.0870, 14.4210))

	p := moving(10, 50.0875, 14.4218) // drift while parked
	p.fix.Speed = 0
	reason, publish := f.Apply("dev", p)
	assert.Equal(t, "static", reason)
	assert.True(t, publish)
	assert.Equal(t, 50.0870, p.fix.Latitude)
}

func TestRejectsInvalidAndZero(t *testing.T) {
	f := New(DefaultConfig())

	zero := moving(0, 0, 0)
	reason, publish := f.Apply("dev", zero)
	assert.Equal(t, "zero", reason)
	assert.False(t, publish, "nothing to fall back on yet")

	f.Apply("dev", moving(10, 50.0870, 14.4210))

	invalid := moving(20, 50.2000, 14.6000)
	invalid.fix.Valid = false
	reason, publish = f.Apply("dev", invalid)
	assert.Equal(t, "invalid", reason)
	assert.True(t, publish)
	assert.Equal(t, 50.0870, invalid.fix.Latitude)
}

func TestDevicesAreIndependent(t *testing.T) {
	f := New(DefaultConfig())
	f.Apply("a", moving(0, 50.0870, 14.4210))

	reason, _ := f.Apply("b", moving(10, 49.1950, 16.6080))
	assert.Equal(t, "", reason)
}

func TestDisabled(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Enabled = false
	f := New(cfg)

	p := moving(0, 0, 0)
	reason, publish := f.Apply("dev", p)
	assert.Equal(t, "", reason)
	assert.True(t, publish)
}

func TestDistanceKm(t *testing.T) {
	// Prague -> Brno, roughly 185 km.
	d := distanceKm(mqtt.Fix{Latitude: 50.0870, Longitude: 14.4210}, mqtt.Fix{Latitude: 49.1950, Longitude: 16.6080})
	assert.InDelta(t, 185, d, 5)
}
