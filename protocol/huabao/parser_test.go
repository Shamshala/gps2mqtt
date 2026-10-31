package huabao

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRegister(t *testing.T) {
	fromGPS := []byte{0x7e,
		0x01, 0x00,
		0x00, 0x2d,
		0x01, 0x91, 0x75, 0x69, 0x02, 0x32,
		0x00, 0x07,
		0x00, 0x22, 0x04, 0x4e, 0x37, 0x30, 0x34, 0x34, 0x34, 0x4d, 0x4c, 0x35, 0x30, 0x30, 0x5f, 0x45, 0x44, 0x5f, 0x47, 0x54, 0x32, 0x35, 0x48, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x35, 0x36, 0x39, 0x30, 0x32, 0x33, 0x32, 0x02, 0xd4, 0xc1, 0x42, 0x35, 0x37, 0x31, 0x39, 0x31,
		0xf2, // checksum
		0x7e,
	}

	toGPS := []byte{0x7e,
		0x81, 0x00, // message id
		0x00, 0x0f, // properties
		0x01, 0x91, 0x75, 0x69, 0x02, 0x32, // terminal
		0x00, 0x00, // sequence
		0x00, 0x07, // response sequence
		0x00, // status byte
		0x30, 0x31, 0x39, 0x31, 0x37, 0x35, 0x36, 0x39, 0x30, 0x32, 0x33, 0x32,
		0x32, // checksum
		0x7e,
	}

	p := &Parser{reader: bufio.NewReader(bytes.NewReader(fromGPS))}
	packet, err := p.ReadPacket()
	assert.NoError(t, err)

	var r bytes.Buffer
	assert.NoError(t, packet.Respond(&r))
	assert.Equal(t, toGPS, r.Bytes())

	assert.Equal(t, "019175690232", packet.DeviceID)
	assert.Equal(t, time.Time{}, packet.Timestamp)
	assert.Equal(t, 0.0, packet.Latitude)
	assert.Equal(t, 0.0, packet.Longitude)
	assert.Equal(t, 0.0, packet.Altitude)
	assert.Equal(t, 0.0, packet.Heading)
	assert.Equal(t, 0.0, packet.Speed)
	assert.Equal(t, false, packet.Position)
	assert.Equal(t, int64(0), packet.Satellites)
	assert.Equal(t, 0.0, packet.RSSI)
	assert.Equal(t, 0.0, packet.Battery)
	assert.Equal(t, "70444", packet.ManufacturerID)
	assert.Equal(t, "ML500_ED_GT25H", packet.TerminalModel)
	assert.Equal(t, "5690232", packet.TerminalID)
}

func TestAuthenticate(t *testing.T) {
	fromGPS := []byte{0x7e, 0x01, 0x02, 0x00, 0x0c, 0x01, 0x91, 0x75, 0x69, 0x02, 0x32, 0x00, 0x09, 0x30, 0x31, 0x39, 0x31, 0x37, 0x35, 0x36, 0x39, 0x30, 0x32, 0x33, 0x32, 0xbd, 0x7e}
	toGPS := []byte{0x7e,
		0x80, 0x01, //message id
		0x00, 0x05, //properties
		0x01, 0x91, 0x75, 0x69, 0x02, 0x32, //terminal
		0x00, 0x00, // sequence --
		0x00, 0x09, // response sequence
		0x01, 0x02, // original message id
		0x00, // code
		0x32, // crc
		0x7e,
	}

	p := &Parser{reader: bufio.NewReader(bytes.NewReader(fromGPS))}
	packet, err := p.ReadPacket()
	assert.NoError(t, err)

	var r bytes.Buffer
	assert.NoError(t, packet.Respond(&r))
	assert.Equal(t, toGPS, r.Bytes())

	assert.Equal(t, "019175690232", packet.DeviceID)
	assert.Equal(t, time.Time{}, packet.Timestamp)
	assert.Equal(t, 0.0, packet.Latitude)
	assert.Equal(t, 0.0, packet.Longitude)
	assert.Equal(t, 0.0, packet.Altitude)
	assert.Equal(t, 0.0, packet.Heading)
	assert.Equal(t, 0.0, packet.Speed)
	assert.Equal(t, false, packet.Position)
	assert.Equal(t, int64(0), packet.Satellites)
	assert.Equal(t, 0.0, packet.RSSI)
	assert.Equal(t, 0.0, packet.Battery)
	assert.Equal(t, "", packet.ManufacturerID)
	assert.Equal(t, "", packet.TerminalModel)
	assert.Equal(t, "", packet.TerminalID)
}

func TestLocationReport(t *testing.T) {
	fromGPS := []byte{0x7e, 0x02, 0x00, 0x00, 0x56, 0x01, 0x91, 0x75, 0x69, 0x02, 0x32, 0x00, 0xb9, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x04, 0x02, 0x9c, 0xfa, 0xee, 0x08, 0x1d, 0x81, 0x2c, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x24, 0x06, 0x04, 0x01, 0x28, 0x23, 0x01, 0x04, 0x00, 0x00, 0x00, 0x65, 0x30, 0x01, 0x0f, 0x31, 0x01, 0x00, 0x51, 0x02, 0x00, 0x00, 0x57, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x9f, 0x17, 0x35, 0x30, 0x35, 0x2c, 0x30, 0x31, 0x2c, 0x37, 0x30, 0x30, 0x64, 0x2c, 0x30, 0x38, 0x63, 0x62, 0x34, 0x61, 0x32, 0x39, 0x2c, 0x31, 0x35, 0xe1, 0x01, 0x64, 0xe2, 0x02, 0x00, 0x00, 0xf4, 0x7e}
	toGPS := []byte{0x7e, 0x80, 0x01, 0x00, 0x05, 0x01, 0x91, 0x75, 0x69, 0x02, 0x32, 0x00, 0x00, 0x00, 0xb9, 0x02, 0x00, 0x00, 0x83, 0x7e}

	p := &Parser{reader: bufio.NewReader(bytes.NewReader(fromGPS))}
	packet, err := p.ReadPacket()
	assert.NoError(t, err)

	var r bytes.Buffer
	assert.NoError(t, packet.Respond(&r))
	assert.Equal(t, toGPS, r.Bytes())

	assert.Equal(t, "019175690232", packet.DeviceID)
	assert.Equal(t, time.Date(2024, 06, 04, 01, 28, 23, 0, time.FixedZone("GMT+8", 8*60*60)), packet.Timestamp)
	assert.Equal(t, -43.842286, packet.Latitude)
	assert.Equal(t, 136.15134, packet.Longitude)
	assert.Equal(t, 0.0, packet.Altitude)
	assert.Equal(t, 0.0, packet.Heading)
	assert.Equal(t, 0.0, packet.Speed)
	assert.Equal(t, false, packet.Position)
	assert.Equal(t, int64(0), packet.Satellites)
	assert.Equal(t, 15.0, packet.RSSI)
	assert.Equal(t, 100.0, packet.Battery)
}

func Test7DDecode(t *testing.T) {
	fromGPS, _ := hex.DecodeString("7e02000056019175690232007d010000000000000000000000000000000000000000000024060316074101040000000030011931010051020000570800000000000000009f173530352c30312c373030642c30386338353030322c3235e10164e2020000287e")

	p := &Parser{reader: bufio.NewReader(bytes.NewReader(fromGPS))}
	packet, err := p.ReadPacket()
	assert.NoError(t, err)

	assert.Equal(t, "019175690232", packet.DeviceID)
	assert.Equal(t, time.Date(2024, 06, 03, 16, 07, 41, 0, time.FixedZone("GMT+8", 8*60*60)), packet.Timestamp)
	assert.Equal(t, 0.0, packet.Latitude)
	assert.Equal(t, 0.0, packet.Longitude)
	assert.Equal(t, 0.0, packet.Altitude)
	assert.Equal(t, 0.0, packet.Heading)
	assert.Equal(t, 0.0, packet.Speed)
	assert.Equal(t, false, packet.Position)
	assert.Equal(t, int64(0), packet.Satellites)
	assert.Equal(t, 25.0, packet.RSSI)
	assert.Equal(t, 100.0, packet.Battery)
	assert.Equal(t, "", packet.ManufacturerID)
	assert.Equal(t, "", packet.TerminalModel)
	assert.Equal(t, "", packet.TerminalID)
}

// Location report from an OBD GPS tracker installed in a BMW, taken from a
// Traccar log. Identifying fields (terminal ID, ICCID, VIN, cell, position)
// were replaced with fake values and the checksum recomputed; OBD values are
// as captured. Vehicle was driving at 11.1 km/h (GPS) / 19 km/h (OBD).
// RPM 1428, engine load 38 %.
func TestLocationReportOBD_Driving(t *testing.T) {
	fromGPS, err := hex.DecodeString(
		"7e0200008d012345678901007b000000000000000302fc445800dc0c080004006f00f5" +
			"240318142358010400042eb6cc14383930303030303030303030303030303030304630" +
			"011f9f133233302c30332c313233342c616263642c33318202008e8001138102059483" +
			"012684015d85021d1c86012d870209048801618901298b115742413054455354303030" +
			"3030303030318c0400042eb6a0005d7e")
	assert.NoError(t, err)

	p := &Parser{reader: bufio.NewReader(bytes.NewReader(fromGPS))}
	packet, err := p.ReadPacket()
	assert.NoError(t, err)

	// Location
	assert.Equal(t, "012345678901", packet.DeviceID)
	assert.Equal(t, time.Date(2024, 03, 18, 14, 23, 58, 0, time.FixedZone("GMT+8", 8*60*60)), packet.Timestamp)
	assert.InDelta(t, 50.087000, packet.Latitude, 1e-6)
	assert.InDelta(t, 14.421000, packet.Longitude, 1e-6)
	assert.Equal(t, 4.0, packet.Altitude)
	assert.InDelta(t, 11.1, packet.Speed, 1e-9)
	assert.Equal(t, 245.0, packet.Heading)
	assert.Equal(t, true, packet.Position)

	// Network / SIM
	assert.Equal(t, 31.0, packet.RSSI)
	assert.Equal(t, "8900000000000000000F", packet.ICCID)
	assert.Equal(t, "230,03,1234,abcd,31", packet.NetworkInfo)

	// OBD2 — moving
	assert.InDelta(t, 27410.2, packet.Odometer, 1e-9)
	assert.InDelta(t, 27410.2, packet.TotalMileage, 1e-9)
	assert.Equal(t, 19.0, packet.OBDSpeed)
	assert.Equal(t, 1428.0, packet.RPM)
	assert.InDelta(t, 14.2, packet.BatteryVoltage, 1e-9)
	assert.Equal(t, 38.0, packet.EngineLoad)
	assert.Equal(t, 53.0, packet.CoolantTemp)  // 93 - 40
	assert.Equal(t, 7452.0, packet.InstantFuel)
	assert.Equal(t, 5.0, packet.IntakeAirTemp) // 45 - 40
	assert.InDelta(t, 23.08, packet.MAF, 1e-9) // 2308 * 0.01
	assert.Equal(t, 97.0, packet.MAP)
	assert.Equal(t, 41.0, packet.ThrottlePos)
	assert.Equal(t, "WBA0TEST000000001", packet.VIN)
	assert.Equal(t, "", packet.FaultCodes)
}

// Same tracker, 30 seconds later, vehicle stopped at idle.
func TestLocationReportOBD_Idling(t *testing.T) {
	fromGPS, err := hex.DecodeString(
		"7e0200008a012345678901007d02000000000000000302fc440d00dc0a070004000000" +
			"fa240318142428010400042eb7cc143839303030303030303030303030303030303046" +
			"30011f9f133233302c30332c313233342c616263642c333182020093810202c683013e" +
			"84015d8502157586012d870204a288016389013e8b1157424130544553543030303030" +
			"303030318c0400042eb7a000097e")
	assert.NoError(t, err)

	p := &Parser{reader: bufio.NewReader(bytes.NewReader(fromGPS))}
	packet, err := p.ReadPacket()
	assert.NoError(t, err)

	assert.Equal(t, "012345678901", packet.DeviceID)
	assert.Equal(t, time.Date(2024, 03, 18, 14, 24, 28, 0, time.FixedZone("GMT+8", 8*60*60)), packet.Timestamp)
	assert.InDelta(t, 50.086925, packet.Latitude, 1e-6)
	assert.InDelta(t, 14.420487, packet.Longitude, 1e-6)
	assert.Equal(t, 0.0, packet.Speed)

	// Stopped — 0x80 OBD speed not transmitted, stays at zero-value.
	assert.Equal(t, 0.0, packet.OBDSpeed)

	// OBD2 — engine idling
	assert.InDelta(t, 27410.3, packet.Odometer, 1e-9)
	assert.Equal(t, 710.0, packet.RPM)
	assert.InDelta(t, 14.7, packet.BatteryVoltage, 1e-9) // alternator running
	assert.Equal(t, 62.0, packet.EngineLoad)
	assert.Equal(t, 53.0, packet.CoolantTemp)
	assert.Equal(t, 5493.0, packet.InstantFuel)
	assert.InDelta(t, 11.86, packet.MAF, 1e-9)
	assert.Equal(t, 99.0, packet.MAP) // ≈ atmospheric pressure at idle
	assert.Equal(t, 62.0, packet.ThrottlePos)
	assert.Equal(t, "WBA0TEST000000001", packet.VIN)
}

func TestFromTraccarLog(t *testing.T) {
	f, err := os.Open("test_data/tracker-server.log")
	if err != nil {
		t.Skip("Unable to open tracker-server.log")
	}

	defer f.Close()

	lineIndex := 0
	re := regexp.MustCompile(`INFO: \[\w+: huabao ([<>]) .+?\] ([0-9a-f]+)`)

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()

		lineIndex++

		matches := re.FindAllStringSubmatch(line, -1)
		if len(matches) == 0 || matches[0][1] != "<" {
			continue
		}

		fromGPSString := matches[0][2]
		var toGPSString string

		for scanner.Scan() {
			line := scanner.Text()

			lineIndex++

			matches := re.FindAllStringSubmatch(line, -1)
			if len(matches) == 0 {
				continue
			}

			if matches[0][1] != ">" {
				if matches[0][2][0:2] != "7e" && !strings.HasSuffix(fromGPSString, "7e") {
					t.Logf("Packet split across multiple lines")
					fromGPSString += matches[0][2]

					continue
				}

				t.Logf("Didn't find reply packet: %d", lineIndex)
				fromGPSString = matches[0][2]

				continue
			}

			toGPSString = matches[0][2]
			break
		}

		fromGPS, err := hex.DecodeString(fromGPSString)
		if !assert.NoError(t, err) {
			t.FailNow()
		}

		toGPS, err := hex.DecodeString(toGPSString)
		if !assert.NoError(t, err) {
			t.FailNow()
		}

		p := &Parser{reader: bufio.NewReader(bytes.NewReader(fromGPS))}
		packet, err := p.ReadPacket()
		assert.NoError(t, err, lineIndex)

		var r bytes.Buffer
		assert.NoError(t, packet.Respond(&r))
		assert.Equal(t, toGPS, r.Bytes())

		if packet.Valid() {
			t.Logf("%v: Altitude: %0.2f, Latitude: %0.5f Longitude: %0.5f, Heading: %0.2f, Speed: %0.2f, Battery: %0.2f", packet.Timestamp, packet.Altitude, packet.Latitude, packet.Longitude, packet.Heading, packet.Speed, packet.Battery)
		}
	}
}
