package homeassistant

// Device groups multiple HA entities under a single physical device in the
// UI. Every entity that shares Identifiers shows up under the same tile.
type Device struct {
	Identifiers  []string `json:"identifiers"`
	Name         string   `json:"name,omitempty"`
	Manufacturer string   `json:"manufacturer,omitempty"`
	Model        string   `json:"model,omitempty"`
	SWVersion    string   `json:"sw_version,omitempty"`
}

// AutoConfiguration is the MQTT discovery payload for a device_tracker entity.
type AutoConfiguration struct {
	StateTopic          string  `json:"state_topic"`
	Name                string  `json:"name"`
	AvailabilityTopic   string  `json:"availability_topic"`
	JSONAttributesTopic string  `json:"json_attributes_topic"`
	Icon                string  `json:"icon,omitempty"`
	SourceType          string  `json:"source_type"`
	UniqueID            string  `json:"unique_id"`
	Device              *Device `json:"device,omitempty"`
}

// Sensor is the MQTT discovery payload for a sensor entity that pulls its
// state from a JSON attributes topic via value_template.
type Sensor struct {
	Name              string  `json:"name"`
	StateTopic        string  `json:"state_topic"`
	AvailabilityTopic string  `json:"availability_topic,omitempty"`
	ValueTemplate     string  `json:"value_template,omitempty"`
	UnitOfMeasurement string  `json:"unit_of_measurement,omitempty"`
	DeviceClass       string  `json:"device_class,omitempty"`
	StateClass        string  `json:"state_class,omitempty"`
	EntityCategory    string  `json:"entity_category,omitempty"`
	Icon              string  `json:"icon,omitempty"`
	UniqueID          string  `json:"unique_id"`
	ObjectID          string  `json:"object_id,omitempty"`
	Device            *Device `json:"device,omitempty"`
}

// BinarySensor is the MQTT discovery payload for a binary_sensor entity.
type BinarySensor struct {
	Name              string  `json:"name"`
	StateTopic        string  `json:"state_topic"`
	AvailabilityTopic string  `json:"availability_topic,omitempty"`
	ValueTemplate     string  `json:"value_template,omitempty"`
	PayloadOn         string  `json:"payload_on,omitempty"`
	PayloadOff        string  `json:"payload_off,omitempty"`
	DeviceClass       string  `json:"device_class,omitempty"`
	EntityCategory    string  `json:"entity_category,omitempty"`
	Icon              string  `json:"icon,omitempty"`
	UniqueID          string  `json:"unique_id"`
	ObjectID          string  `json:"object_id,omitempty"`
	Device            *Device `json:"device,omitempty"`
}
