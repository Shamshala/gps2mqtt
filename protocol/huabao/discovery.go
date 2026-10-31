package huabao

import (
	"encoding/json"
	"fmt"

	"github.com/freman/gps2mqtt/homeassistant"
	"github.com/freman/gps2mqtt/mqtt"
)

const haDiscoveryPrefix = "homeassistant"

// HomeAssistantDiscovery returns the full set of MQTT discovery payloads for
// this device: one device_tracker plus per-OBD-field sensors and a few
// derived binary_sensors. All entities share one Device block so HA groups
// them under a single device tile in the UI.
//
// The caller is responsible for publishing each DiscoveryMessage with the
// retain flag set, so HA picks up the entities on restart.
func (p *Packet) HomeAssistantDiscovery(meta mqtt.DeviceMeta) ([]mqtt.DiscoveryMessage, error) {
	if p.DeviceID == "" {
		return nil, fmt.Errorf("packet has no device id")
	}

	devID := p.DeviceID
	base := "gps2mqtt/device/" + devID
	attrTopic := base + "/attributes"
	availTopic := "gps2mqtt/availability"
	uniq := "gps2mqtt_" + devID
	slug := slugify(meta.Name)
	if slug == "" {
		slug = "device_" + devID
	}

	model := p.TerminalModel
	if model == "" {
		model = "JT/T 808 OBD GPS tracker"
	}
	manufacturer := p.ManufacturerID
	if manufacturer == "" {
		manufacturer = "Huabao / JT/T 808"
	}

	dev := &homeassistant.Device{
		Identifiers:  []string{uniq},
		Name:         coalesce(meta.Name, devID),
		Manufacturer: manufacturer,
		Model:        model,
	}

	var msgs []mqtt.DiscoveryMessage
	add := func(m mqtt.DiscoveryMessage, err error) error {
		if err != nil {
			return err
		}
		msgs = append(msgs, m)
		return nil
	}

	// --- device_tracker (GPS location) -------------------------------------
	if err := add(jsonMsg(
		haDiscoveryPrefix+"/device_tracker/"+devID+"/config",
		homeassistant.AutoConfiguration{
			Name:                "Location",
			StateTopic:          base,
			AvailabilityTopic:   availTopic,
			JSONAttributesTopic: attrTopic,
			Icon:                meta.Icon,
			SourceType:          "gps",
			UniqueID:            uniq,
			Device:              dev,
		},
	)); err != nil {
		return nil, err
	}

	// --- sensors -----------------------------------------------------------
	type sensorSpec struct {
		suffix     string
		name       string
		field      string
		unit       string
		devClass   string
		stateClass string
		entCat     string
		icon       string
	}

	sensors := []sensorSpec{
		// Basic GPS-derived
		{"speed", "GPS Speed", "speed", "km/h", "speed", "measurement", "", "mdi:speedometer"},
		{"altitude", "Altitude", "altitude", "m", "distance", "measurement", "", "mdi:altimeter"},
		{"heading", "Heading", "heading", "°", "", "measurement", "", "mdi:compass"},
		{"satellites", "GNSS Satellites", "satellites", "", "", "measurement", "diagnostic", "mdi:satellite-variant"},
		{"rssi", "Signal Strength", "rssi", "dB", "signal_strength", "measurement", "diagnostic", "mdi:signal"},

		// OBD numeric
		{"odometer", "Odometer", "odometer", "km", "distance", "total_increasing", "", "mdi:counter"},
		{"total_mileage", "Total Mileage", "total_mileage", "km", "distance", "total_increasing", "diagnostic", "mdi:counter"},
		{"obd_speed", "OBD Speed", "obd_speed", "km/h", "speed", "measurement", "", "mdi:speedometer"},
		{"rpm", "RPM", "rpm", "rpm", "", "measurement", "", "mdi:engine"},
		{"battery_voltage", "Battery Voltage", "battery_voltage", "V", "voltage", "measurement", "", "mdi:car-battery"},
		{"engine_load", "Engine Load", "engine_load", "%", "", "measurement", "", "mdi:engine"},
		{"coolant_temp", "Coolant Temperature", "coolant_temp", "°C", "temperature", "measurement", "", "mdi:coolant-temperature"},
		{"intake_air_temp", "Intake Air Temperature", "intake_air_temp", "°C", "temperature", "measurement", "", "mdi:thermometer-lines"},
		{"instant_fuel", "Instant Fuel Consumption", "instant_fuel", "mL/h", "", "measurement", "", "mdi:fuel"},
		{"maf", "MAF", "maf", "g/s", "", "measurement", "", "mdi:weather-windy"},
		{"map", "MAP", "map", "kPa", "pressure", "measurement", "", "mdi:gauge"},
		{"throttle_pos", "Throttle Position", "throttle_pos", "%", "", "measurement", "", "mdi:car-cruise-control"},
		{"fuel_pressure", "Fuel Pressure", "fuel_pressure", "kPa", "pressure", "measurement", "", "mdi:gauge"},
		{"fuel_level", "Fuel Level", "fuel_level", "%", "", "measurement", "", "mdi:fuel"},
	}

	for _, s := range sensors {
		cfg := homeassistant.Sensor{
			Name:              s.name,
			StateTopic:        attrTopic,
			AvailabilityTopic: availTopic,
			ValueTemplate:     "{{ value_json." + s.field + " }}",
			UnitOfMeasurement: s.unit,
			DeviceClass:       s.devClass,
			StateClass:        s.stateClass,
			EntityCategory:    s.entCat,
			Icon:              s.icon,
			UniqueID:          uniq + "_" + s.suffix,
			ObjectID:          slug + "_" + s.suffix,
			Device:            dev,
		}
		if err := add(jsonMsg(
			haDiscoveryPrefix+"/sensor/"+uniq+"_"+s.suffix+"/config",
			cfg,
		)); err != nil {
			return nil, err
		}
	}

	// --- diagnostic text sensors -------------------------------------------
	textSensors := []struct {
		suffix, name, field, icon, valueTemplate string
	}{
		{"vin", "VIN", "vin", "mdi:identifier", "{{ value_json.vin }}"},
		{"iccid", "ICCID", "iccid", "mdi:sim", "{{ value_json.iccid }}"},
		{"network", "Network", "network", "mdi:radio-tower", "{{ value_json.network }}"},
		// Fault codes: show "OK" when no DTCs reported, otherwise the raw
		// comma-separated DTC list. Keeps the entity from displaying
		// "unknown" the 99 % of the time the car is healthy.
		{"fault_codes", "Fault Codes", "fault_codes", "mdi:car-wrench",
			"{{ value_json.fault_codes if value_json.fault_codes else 'OK' }}"},
	}
	for _, s := range textSensors {
		cfg := homeassistant.Sensor{
			Name:              s.name,
			StateTopic:        attrTopic,
			AvailabilityTopic: availTopic,
			ValueTemplate:     s.valueTemplate,
			EntityCategory:    "diagnostic",
			Icon:              s.icon,
			UniqueID:          uniq + "_" + s.suffix,
			ObjectID:          slug + "_" + s.suffix,
			Device:            dev,
		}
		if err := add(jsonMsg(
			haDiscoveryPrefix+"/sensor/"+uniq+"_"+s.suffix+"/config",
			cfg,
		)); err != nil {
			return nil, err
		}
	}

	// --- binary sensors (derived) ------------------------------------------
	binarySensors := []struct {
		suffix, name, devClass, entCat, icon, valueTemplate string
	}{
		{"engine_running", "Engine Running", "running", "", "mdi:engine",
			"{{ 'ON' if value_json.rpm and value_json.rpm > 0 else 'OFF' }}"},
		{"position_fix", "Position Fix", "connectivity", "diagnostic", "mdi:crosshairs-gps",
			"{{ 'ON' if value_json.position else 'OFF' }}"},
		// DTC present: trigger an automation/notification when this goes ON.
		{"dtc_present", "DTC Present", "problem", "diagnostic", "mdi:car-wrench",
			"{{ 'ON' if value_json.fault_codes else 'OFF' }}"},
	}
	for _, b := range binarySensors {
		cfg := homeassistant.BinarySensor{
			Name:              b.name,
			StateTopic:        attrTopic,
			AvailabilityTopic: availTopic,
			ValueTemplate:     b.valueTemplate,
			PayloadOn:         "ON",
			PayloadOff:        "OFF",
			DeviceClass:       b.devClass,
			EntityCategory:    b.entCat,
			Icon:              b.icon,
			UniqueID:          uniq + "_" + b.suffix,
			ObjectID:          slug + "_" + b.suffix,
			Device:            dev,
		}
		if err := add(jsonMsg(
			haDiscoveryPrefix+"/binary_sensor/"+uniq+"_"+b.suffix+"/config",
			cfg,
		)); err != nil {
			return nil, err
		}
	}

	return msgs, nil
}

func jsonMsg(topic string, v interface{}) (mqtt.DiscoveryMessage, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return mqtt.DiscoveryMessage{}, fmt.Errorf("marshal discovery for %s: %w", topic, err)
	}
	return mqtt.DiscoveryMessage{Topic: topic, Payload: b}, nil
}

func coalesce(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// slugify turns a human-readable name into a lowercase HA-safe object_id
// fragment. HA requires [a-z0-9_] for object_id; everything else is mapped
// to '_' or dropped.
func slugify(s string) string {
	out := make([]rune, 0, len(s))
	prevUnderscore := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			out = append(out, r)
			prevUnderscore = false
		case r >= 'A' && r <= 'Z':
			out = append(out, r+('a'-'A'))
			prevUnderscore = false
		case r == ' ', r == '-', r == '_', r == '.':
			if !prevUnderscore && len(out) > 0 {
				out = append(out, '_')
				prevUnderscore = true
			}
		}
	}
	// Trim trailing underscore
	for len(out) > 0 && out[len(out)-1] == '_' {
		out = out[:len(out)-1]
	}
	return string(out)
}
