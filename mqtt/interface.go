package mqtt

import "time"

type Identifier interface {
	MQTTID() string
	Device() string
	Valid() bool
}

// DeviceMeta carries user-supplied friendly metadata about a device, so a
// Discoverer can build human-readable HA discovery payloads.
type DeviceMeta struct {
	Name string
	Icon string
}

// DiscoveryMessage is one retained MQTT message that announces a HA entity
// (device_tracker, sensor, binary_sensor, …). The receiver publishes it
// verbatim with the retain flag set.
type DiscoveryMessage struct {
	Topic   string
	Payload []byte
}

// Discoverer is implemented by packet types that know how to describe
// themselves to Home Assistant via MQTT discovery. Protocols that don't
// implement this interface fall back to the legacy single-device_tracker
// configuration in main.
type Discoverer interface {
	HomeAssistantDiscovery(meta DeviceMeta) ([]DiscoveryMessage, error)
}

// Fix is the part of a location report the position filter reasons about.
type Fix struct {
	Time       time.Time
	Latitude   float64
	Longitude  float64
	Speed      float64 // km/h as reported by the device
	Valid      bool    // device reports a valid GNSS fix
	Satellites int64   // 0 when the device doesn't report it
}

// Positioner is implemented by packet types that carry a GPS position, so
// the position filter in main can hold back implausible fixes. When a fix is
// rejected HoldPosition replaces the coordinates with the last accepted ones;
// the rest of the packet (OBD data etc.) is still published.
type Positioner interface {
	GPSFix() Fix
	HoldPosition(lat, lon float64, reason string)
}
