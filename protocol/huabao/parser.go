package huabao

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"strings"
)

type Parser struct {
	reader *bufio.Reader

	terminalInfo struct {
		ManufacturerID [5]byte
		Model          [20]byte
		ID             [7]byte
	}
}

func (p *Parser) read(d []byte) (err error) {
	sz, err := reader{p.reader}.Read(d)
	if err != nil {
		return err
	}

	if len(d) != sz {
		return errors.New("unexpected number of bytes read")
	}

	return nil
}

func (p *Parser) ReadPacket() (packet *Packet, err error) {
	preamble, err := p.reader.ReadByte()
	if err != nil {
		return nil, err
	}

	if preamble != 0x7e {
		return nil, errors.New("bad preamble")
	}

	var head header
	if err := binary.Read(reader{p.reader}, binary.BigEndian, &head); err != nil {
		return nil, fmt.Errorf("failed to read header: %w", err)
	}

	if head.Properties.Encrypted() {
		return nil, errors.New("encryption not supported")
	}

	if head.Properties.SubPackage() {
		return nil, errors.New("subpackage not supported")
	}

	body := make([]byte, head.Properties.Length())
	p.read(body)

	checksum := make([]byte, 1)
	if err := p.read(checksum); err != nil {
		return nil, errors.New("unable to read checksum")
	}
	_ = checksum //todo check checksum

	tail, err := p.reader.ReadByte()
	if err != nil {
		return nil, err
	}

	if tail != 0x7e {
		return nil, errors.New("invalid tail")
	}

	bodyBuf := bytes.NewReader(body)

	packet = &Packet{
		header:   head,
		DeviceID: head.Terminal.String(),
	}

	switch head.MessageType {
	case protoRegister:
		bodyBuf.Seek(4, io.SeekCurrent)
		if err := binary.Read(bodyBuf, binary.BigEndian, &p.terminalInfo); err != nil {
			return nil, err
		}
	case protoLocationReport:
		var rep basicLocationInformation
		binary.Read(bodyBuf, binary.BigEndian, &rep)
		packet.importBasicInformation(rep)

		if err := p.parseAdditionalInformation(bodyBuf, packet); err != nil {
			return nil, err
		}
	}

	packet.ManufacturerID = strings.TrimRight(string(p.terminalInfo.ManufacturerID[:]), "\x00")
	packet.TerminalID = strings.TrimRight(string(p.terminalInfo.ID[:]), "\x00")
	packet.TerminalModel = strings.TrimRight(string(p.terminalInfo.Model[:]), "\x00")

	return packet, nil
}

func (p *Parser) parseAdditionalInformation(buf *bytes.Reader, packet *Packet) (err error) {
	for buf.Len() > 0 {
		addInfo, err := buf.ReadByte()
		if err != nil {
			return fmt.Errorf("unable to read additinal information type: %w", err)
		}
		addLen, err := buf.ReadByte()
		if err != nil {
			return fmt.Errorf("unable to read additinal information len: %w", err)
		}

		// Snapshot starting position so we can robustly skip the whole TLV value
		// after parsing — handles length mismatches without desyncing the stream.
		start, _ := buf.Seek(0, io.SeekCurrent)

		switch addInfo {
		case 0x01: // Mileage, DWORD, 0.1 km (odometer reading)
			var v uint32
			if err := binary.Read(buf, binary.BigEndian, &v); err != nil {
				return fmt.Errorf("unable to read mileage: %w", err)
			}
			packet.Odometer = float64(v) * 0.1
		case 0x30: // Wireless network signal strength, BYTE
			tmp, err := buf.ReadByte()
			if err != nil {
				return fmt.Errorf("unable to read RSSI from additional information: %w", err)
			}
			packet.RSSI = float64(tmp)
		case 0x31: // GNSS positioning satellites, BYTE
			tmp, err := buf.ReadByte()
			if err != nil {
				return fmt.Errorf("unable to read Satellites from additional information: %w", err)
			}
			packet.Satellites = int64(tmp)

		// --- JT/T 808 OBD2 extension ---
		case 0x80: // Instantaneous speed, BYTE km/h
			tmp, err := buf.ReadByte()
			if err != nil {
				return fmt.Errorf("unable to read OBD speed: %w", err)
			}
			packet.OBDSpeed = float64(tmp)
		case 0x81: // Engine RPM, WORD
			var v uint16
			if err := binary.Read(buf, binary.BigEndian, &v); err != nil {
				return fmt.Errorf("unable to read RPM: %w", err)
			}
			packet.RPM = float64(v)
		case 0x82: // Battery voltage, WORD * 0.1 V
			var v uint16
			if err := binary.Read(buf, binary.BigEndian, &v); err != nil {
				return fmt.Errorf("unable to read battery voltage: %w", err)
			}
			packet.BatteryVoltage = float64(v) * 0.1
		case 0x83: // Engine load, BYTE %
			tmp, err := buf.ReadByte()
			if err != nil {
				return fmt.Errorf("unable to read engine load: %w", err)
			}
			packet.EngineLoad = float64(tmp)
		case 0x84: // Coolant temperature, BYTE − 40 °C
			tmp, err := buf.ReadByte()
			if err != nil {
				return fmt.Errorf("unable to read coolant temp: %w", err)
			}
			packet.CoolantTemp = float64(tmp) - 40
		case 0x85: // Instant fuel consumption, WORD mL/h
			var v uint16
			if err := binary.Read(buf, binary.BigEndian, &v); err != nil {
				return fmt.Errorf("unable to read instant fuel: %w", err)
			}
			packet.InstantFuel = float64(v)
		case 0x86: // Intake air temperature, BYTE − 40 °C
			tmp, err := buf.ReadByte()
			if err != nil {
				return fmt.Errorf("unable to read intake air temp: %w", err)
			}
			packet.IntakeAirTemp = float64(tmp) - 40
		case 0x87: // Air flow (MAF), WORD; raw / 100 = g/s (OBD-II PID 0x10 convention)
			var v uint16
			if err := binary.Read(buf, binary.BigEndian, &v); err != nil {
				return fmt.Errorf("unable to read MAF: %w", err)
			}
			packet.MAF = float64(v) * 0.01
		case 0x88: // Intake manifold absolute pressure, BYTE kPa
			tmp, err := buf.ReadByte()
			if err != nil {
				return fmt.Errorf("unable to read MAP: %w", err)
			}
			packet.MAP = float64(tmp)
		case 0x89: // Throttle position, BYTE %
			tmp, err := buf.ReadByte()
			if err != nil {
				return fmt.Errorf("unable to read throttle position: %w", err)
			}
			packet.ThrottlePos = float64(tmp)
		case 0x8a: // Fuel pressure, WORD kPa
			var v uint16
			if err := binary.Read(buf, binary.BigEndian, &v); err != nil {
				return fmt.Errorf("unable to read fuel pressure: %w", err)
			}
			packet.FuelPressure = float64(v)
		case 0x8b: // VIN, 17 byte ASCII
			vin := make([]byte, addLen)
			if _, err := io.ReadFull(buf, vin); err != nil {
				return fmt.Errorf("unable to read VIN: %w", err)
			}
			packet.VIN = strings.TrimRight(string(vin), "\x00 ")
		case 0x8c: // Total mileage, DWORD 0.1 km
			var v uint32
			if err := binary.Read(buf, binary.BigEndian, &v); err != nil {
				return fmt.Errorf("unable to read total mileage: %w", err)
			}
			packet.TotalMileage = float64(v) * 0.1
			// Prefer total mileage as Odometer if no 0x01 was sent.
			if packet.Odometer == 0 {
				packet.Odometer = packet.TotalMileage
			}
		case 0x8e: // Fuel level, BYTE %
			tmp, err := buf.ReadByte()
			if err != nil {
				return fmt.Errorf("unable to read fuel level: %w", err)
			}
			packet.FuelLevel = float64(tmp)
		case 0x9f: // Base station info, variable-length CSV string
			b := make([]byte, addLen)
			if _, err := io.ReadFull(buf, b); err != nil {
				return fmt.Errorf("unable to read network info: %w", err)
			}
			packet.NetworkInfo = string(b)
		case 0xa0: // Fault code info (DTCs), variable-length CSV string
			b := make([]byte, addLen)
			if _, err := io.ReadFull(buf, b); err != nil {
				return fmt.Errorf("unable to read fault codes: %w", err)
			}
			packet.FaultCodes = string(b)
		case 0xcc: // ICCID, 20 byte ASCII
			b := make([]byte, addLen)
			if _, err := io.ReadFull(buf, b); err != nil {
				return fmt.Errorf("unable to read ICCID: %w", err)
			}
			packet.ICCID = strings.TrimRight(string(b), "\x00 ")

		// --- Vendor-specific battery percentage (non-standard) ---
		case 0xd4: // LT-160
			tmp, err := buf.ReadByte()
			if err != nil {
				return fmt.Errorf("unable to read battery percentage from additional information: %w", err)
			}
			packet.Battery = float64(tmp)
		case 0xe1: // ML100G
			tmp, err := buf.ReadByte()
			if err != nil {
				return fmt.Errorf("unable to read battery percentage from additional information: %w", err)
			}
			packet.Battery = float64(tmp)
		default:
			buf.Seek(int64(addLen), io.SeekCurrent) // skip unknown TLV
			continue
		}

		// Ensure we land exactly at start+addLen even if our typed read consumed
		// a different number of bytes (e.g. device sent a longer 0xCC than 20 B).
		if _, err := buf.Seek(start+int64(addLen), io.SeekStart); err != nil {
			return fmt.Errorf("unable to realign after TLV 0x%02x: %w", addInfo, err)
		}
	}

	return err
}
