package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
	"tinygo.org/x/bluetooth"
)

// GATT characteristic UUIDs exposed by the Wave
var (
	datetimeUUID    = bluetooth.New16BitUUID(0x2A08)
	humidityUUID    = bluetooth.New16BitUUID(0x2A6F)
	temperatureUUID = bluetooth.New16BitUUID(0x2A6E)
	radonSTAUUID    = mustParseUUID("b42e01aa-ade7-11e4-89d3-123b93f75cba")
	radonLTAUUID    = mustParseUUID("b42e0a4c-ade7-11e4-89d3-123b93f75cba")
)

// Airthings' Bluetooth SIG manufacturer ID, used both to recognise the
// advertisement and to pull the serial number out of it. Check for ID == 0x0334.
const airthingsManufacturerID = 0x0334

// mustParseUUID reads the UUID as a string and parses it. Will panic on errors.
// This is meant to be used for global variables.
func mustParseUUID(s string) bluetooth.UUID {
	uuid, err := bluetooth.ParseUUID(s)
	if err != nil {
		panic(err)
	}
	return uuid
}

type Wave struct {
	adapter *bluetooth.Adapter

	SerialNumber uint32 // The Wave serial number

	address     bluetooth.Address
	haveAddress bool
	device      *bluetooth.Device

	datetimeChar    bluetooth.DeviceCharacteristic
	humidityChar    bluetooth.DeviceCharacteristic
	temperatureChar bluetooth.DeviceCharacteristic
	radonSTAChar    bluetooth.DeviceCharacteristic
	radonLTAChar    bluetooth.DeviceCharacteristic
}

func NewWave(adapter *bluetooth.Adapter, serialNumber uint32) *Wave {
	return &Wave{
		adapter:      adapter,
		SerialNumber: serialNumber,
	}
}

// IsConnected shows if the Wave is connected. Set on connect, cleared on Disconnect.
func (w *Wave) IsConnected() bool {
	return w.device != nil
}

// Discover scans for a Wave advertising the configured serial number in its
// manufacturer data
func (w *Wave) Discover(timeout time.Duration) error {
	found := make(chan bluetooth.Address, 1)

	timer := time.AfterFunc(timeout, func() {
		_ = w.adapter.StopScan()
	})
	defer timer.Stop()

	err := w.adapter.Scan(func(a *bluetooth.Adapter, result bluetooth.ScanResult) {
		for _, m := range result.AdvertisementPayload.ManufacturerData() {
			if m.CompanyID != airthingsManufacturerID || len(m.Data) < 4 {
				continue
			}
			// Data is the manufacturer field with the 2-byte company ID
			// already stripped off, so Data[0:4] is the little-endian
			// serial number
			if binary.LittleEndian.Uint32(m.Data[0:4]) == w.SerialNumber {
				_ = a.StopScan()
				found <- result.Address
				return
			}
		}
	})
	if err != nil {
		return fmt.Errorf("scan: %w", err)
	}

	select {
	case addr := <-found:
		w.address = addr
		w.haveAddress = true
		return nil
	default:
		return errors.New("wave not found within timeout")
	}
}

// Connect discover the address if we don't already have one,
// then connect and resolve the characteristics we need, retrying on failure.
func (w *Wave) Connect(retries int) error {
	var lastErr error
	for tries := 0; tries < retries && !w.IsConnected(); tries++ {
		if !w.haveAddress {
			if err := w.Discover(3 * time.Second); err != nil {
				lastErr = err
				continue
			}
		}
		if err := w.connectAndResolve(); err != nil {
			lastErr = err
			continue
		}
		return nil
	}
	if !w.IsConnected() && lastErr != nil {
		return lastErr
	}
	return nil
}

// connectAndResolve connects to Wave and gather the metrics by UUIDS
func (w *Wave) connectAndResolve() error {
	device, err := w.adapter.Connect(w.address, bluetooth.ConnectionParams{})
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}

	services, err := device.DiscoverServices(nil)
	if err != nil {
		_ = device.Disconnect()
		return fmt.Errorf("discover services: %w", err)
	}

	want := map[bluetooth.UUID]*bluetooth.DeviceCharacteristic{
		datetimeUUID:    &w.datetimeChar,
		humidityUUID:    &w.humidityChar,
		temperatureUUID: &w.temperatureChar,
		radonSTAUUID:    &w.radonSTAChar,
		radonLTAUUID:    &w.radonLTAChar,
	}

	for _, svc := range services {
		chars, err := svc.DiscoverCharacteristics(nil)
		if err != nil {
			continue
		}
		for _, c := range chars {
			if dst, ok := want[c.UUID()]; ok {
				*dst = c
				delete(want, c.UUID())
			}
		}
	}
	if len(want) > 0 {
		_ = device.Disconnect()
		return fmt.Errorf("could not find %d expected characteristic(s)", len(want))
	}

	w.device = &device
	return nil
}

// Disconnect mirrors bluepy's disconnect().
func (w *Wave) Disconnect() error {
	if w.device == nil {
		return nil
	}
	err := w.device.Disconnect()
	w.device = nil

	return err
}

type CurrentValues struct {
	Timestamp   time.Time
	Humidity    float64 // %rH
	Temperature float64 // °C
	RadonSTA    uint16  // short-term average, Bq/m3
	RadonLTA    uint16  // long-term average, Bq/m3
}

func (c CurrentValues) String() string {
	return fmt.Sprintf(
		"Timestamp: %s, Humidity: %.2f %%rH, Temperature: %.2f *C, Radon STA: %d Bq/m3, Radon LTA: %d Bq/m3",
		c.Timestamp, c.Humidity, c.Temperature, c.RadonSTA, c.RadonLTA,
	)
}

// Read read each characteristic in turn and unpack
// the concatenated bytes
func (w *Wave) Read() (*CurrentValues, error) {
	if !w.IsConnected() {
		return nil, errors.New("not connected")
	}

	log.Debug("Starting read")

	raw := make([]byte, 0, 15)

	dt := make([]byte, 7) // year(2) + month/day/hour/min/sec (1 each)
	if _, err := w.datetimeChar.Read(dt); err != nil {
		return nil, fmt.Errorf("read datetime characteristic: %w", err)
	}
	raw = append(raw, dt...)

	for _, c := range []*bluetooth.DeviceCharacteristic{
		&w.humidityChar, &w.temperatureChar, &w.radonSTAChar, &w.radonLTAChar,
	} {
		buf := make([]byte, 2)
		if _, err := c.Read(buf); err != nil {
			return nil, fmt.Errorf("read characteristic %s: %w", c.UUID().String(), err)
		}
		raw = append(raw, buf...)
	}

	log.Debug("Read completed")

	return parseCurrentValues(raw)
}

func parseCurrentValues(raw []byte) (*CurrentValues, error) {
	if len(raw) != 15 {
		return nil, fmt.Errorf("unexpected payload length: got %d, want 15", len(raw))
	}

	year := binary.LittleEndian.Uint16(raw[0:2])
	month, day, hour, minute, second := raw[2], raw[3], raw[4], raw[5], raw[6]
	humidity := binary.LittleEndian.Uint16(raw[7:9])
	temperature := binary.LittleEndian.Uint16(raw[9:11])
	radonSTA := binary.LittleEndian.Uint16(raw[11:13])
	radonLTA := binary.LittleEndian.Uint16(raw[13:15])

	return &CurrentValues{
		Timestamp: time.Date(
			int(year), time.Month(month), int(day),
			int(hour), int(minute), int(second), 0, time.UTC,
		),
		Humidity:    float64(humidity) / 100.0,
		Temperature: float64(temperature) / 100.0,
		RadonSTA:    radonSTA,
		RadonLTA:    radonLTA,
	}, nil
}
