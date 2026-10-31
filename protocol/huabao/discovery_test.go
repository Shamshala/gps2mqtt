package huabao

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/freman/gps2mqtt/mqtt"
	"github.com/stretchr/testify/assert"
)

func TestHomeAssistantDiscovery(t *testing.T) {
	pkt := &Packet{
		DeviceID:       "012345678901",
		TerminalModel:  "ML500",
		ManufacturerID: "70444",
	}

	msgs, err := pkt.HomeAssistantDiscovery(mqtt.DeviceMeta{
		Name: "BMW 320i",
		Icon: "mdi:car",
	})
	assert.NoError(t, err)
	assert.NotEmpty(t, msgs)

	// Index messages by topic for easy assertion.
	byTopic := make(map[string]map[string]interface{})
	for _, m := range msgs {
		var v map[string]interface{}
		if err := json.Unmarshal(m.Payload, &v); err != nil {
			t.Fatalf("payload for %s is not valid JSON: %v", m.Topic, err)
		}
		byTopic[m.Topic] = v
	}

	// device_tracker
	tracker, ok := byTopic["homeassistant/device_tracker/012345678901/config"]
	if assert.True(t, ok, "device_tracker discovery missing") {
		assert.Equal(t, "gps2mqtt/device/012345678901", tracker["state_topic"])
		assert.Equal(t, "gps2mqtt/device/012345678901/attributes", tracker["json_attributes_topic"])
		assert.Equal(t, "gps2mqtt_012345678901", tracker["unique_id"])
		assert.Equal(t, "gps", tracker["source_type"])
		assert.Contains(t, tracker, "device")
	}

	// Spot-check each kind of sensor we care about
	type expect struct {
		topic     string
		key       string // value_json field referenced in value_template
		unit      string
		devClass  string
		entityCat string
	}
	cases := []expect{
		{"homeassistant/sensor/gps2mqtt_012345678901_rpm/config", "rpm", "rpm", "", ""},
		{"homeassistant/sensor/gps2mqtt_012345678901_coolant_temp/config", "coolant_temp", "°C", "temperature", ""},
		{"homeassistant/sensor/gps2mqtt_012345678901_battery_voltage/config", "battery_voltage", "V", "voltage", ""},
		{"homeassistant/sensor/gps2mqtt_012345678901_odometer/config", "odometer", "km", "distance", ""},
		{"homeassistant/sensor/gps2mqtt_012345678901_maf/config", "maf", "g/s", "", ""},
		{"homeassistant/sensor/gps2mqtt_012345678901_map/config", "map", "kPa", "pressure", ""},
		{"homeassistant/sensor/gps2mqtt_012345678901_vin/config", "vin", "", "", "diagnostic"},
		{"homeassistant/sensor/gps2mqtt_012345678901_iccid/config", "iccid", "", "", "diagnostic"},
		{"homeassistant/sensor/gps2mqtt_012345678901_fault_codes/config", "fault_codes", "", "", "diagnostic"},
		{"homeassistant/sensor/gps2mqtt_012345678901_rssi/config", "rssi", "dB", "signal_strength", "diagnostic"},
	}
	for _, c := range cases {
		s, ok := byTopic[c.topic]
		if !assert.Truef(t, ok, "sensor %s missing", c.topic) {
			continue
		}
		assert.Contains(t, s["value_template"], "value_json."+c.key, c.topic)
		if c.unit != "" {
			assert.Equal(t, c.unit, s["unit_of_measurement"], c.topic)
		}
		if c.devClass != "" {
			assert.Equal(t, c.devClass, s["device_class"], c.topic)
		}
		if c.entityCat != "" {
			assert.Equal(t, c.entityCat, s["entity_category"], c.topic)
		}
		// All entities share the same Device block (same identifiers) so HA
		// groups them under one device tile.
		if dev, ok := s["device"].(map[string]interface{}); assert.True(t, ok, c.topic) {
			ids := dev["identifiers"].([]interface{})
			assert.Equal(t, []interface{}{"gps2mqtt_012345678901"}, ids)
			assert.Equal(t, "BMW 320i", dev["name"])
		}
	}

	// Binary sensors
	binCases := []struct {
		topic    string
		devClass string
	}{
		{"homeassistant/binary_sensor/gps2mqtt_012345678901_engine_running/config", "running"},
		{"homeassistant/binary_sensor/gps2mqtt_012345678901_dtc_present/config", "problem"},
		{"homeassistant/binary_sensor/gps2mqtt_012345678901_position_fix/config", "connectivity"},
	}
	for _, c := range binCases {
		b, ok := byTopic[c.topic]
		if !assert.Truef(t, ok, "binary_sensor %s missing", c.topic) {
			continue
		}
		assert.Equal(t, c.devClass, b["device_class"], c.topic)
		assert.Equal(t, "ON", b["payload_on"], c.topic)
		assert.Equal(t, "OFF", b["payload_off"], c.topic)
	}

	// Fault codes sensor falls back to "OK" when no DTCs reported.
	fc := byTopic["homeassistant/sensor/gps2mqtt_012345678901_fault_codes/config"]
	if assert.NotNil(t, fc) {
		tpl, _ := fc["value_template"].(string)
		assert.True(t, strings.Contains(tpl, "'OK'"), "fault_codes template should fall back to 'OK': %q", tpl)
	}
}

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"BMW 320i":          "bmw_320i",
		"Motorbike Tracker": "motorbike_tracker",
		"Auto-Mobil":        "auto_mobil",
		"  spaces  ":        "spaces",
		"":                  "",
		"...":               "",
	}
	for in, want := range cases {
		assert.Equal(t, want, slugify(in), "slugify(%q)", in)
	}
}
