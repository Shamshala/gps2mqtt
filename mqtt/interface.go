package mqtt

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
