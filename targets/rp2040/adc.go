//go:build rp2040 || rp2350

package main

import (
	"device/rp"
	"errors"
	"gopper/core"
	"machine"
	"sync"
)

// RpAdcDriver implements core.ADCDriver using TinyGo's machine.ADC.
type RpAdcDriver struct {
	mu            sync.Mutex // serialize sampling if needed
	arefMilliVolt uint32

	// Per-channel TinyGo ADC handles.
	// Adjust size/mapping to match your hardware channels.
	channels map[core.ADCChannelID]*machine.ADC
}

// NewRPAdcDriver constructs the driver but does not Init() it yet.
func NewRPAdcDriver() *RpAdcDriver {
	return &RpAdcDriver{
		arefMilliVolt: 3300,
		channels:      make(map[core.ADCChannelID]*machine.ADC),
	}
}

// rawInternalTemp returns the 12-bit raw ADC value from the internal temp sensor (0–4095).
func rawInternalTemp() uint16 {
	// Ensure ADC is initialized
	if rp.ADC.CS.Get()&rp.ADC_CS_EN == 0 {
		machine.InitADC()
	}

	// Enable temperature sensor
	rp.ADC.CS.SetBits(rp.ADC_CS_TS_EN)

	// Select ADC channel 4 (internal temperature sensor)
	const tempChannel = 4
	rp.ADC.CS.ReplaceBits(
		uint32(tempChannel)<<rp.ADC_CS_AINSEL_Pos,
		rp.ADC_CS_AINSEL_Msk,
		0,
	)

	// Start a single conversion
	rp.ADC.CS.SetBits(rp.ADC_CS_START_ONCE)

	// Wait until conversion is ready
	for !rp.ADC.CS.HasBits(rp.ADC_CS_READY) {
	}

	// Read and return raw 12-bit result (0-4095)
	// NOTE: We return the raw 12-bit value to match ADC_MAX=4095
	// Klipper expects values in the range 0-ADC_MAX for temperature conversion
	return uint16(rp.ADC.RESULT.Get())
}

func (d *RpAdcDriver) Init(cfg core.ADCConfig) error {
	if cfg.Reference != 0 {
		d.arefMilliVolt = cfg.Reference
	}

	// Use TinyGo's global ADC init, if available.
	machine.InitADC()

	// Pin enumeration is registered centrally in main.go (registerRP2040Pins)
	// to avoid conflicts between GPIO and ADC pin names

	return nil
}

// ConfigureChannel sets up a specific ADC channel (pin mux, etc.).
func (d *RpAdcDriver) ConfigureChannel(ch core.ADCChannelID) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Translate pin enumeration index to ADC channel:
	// Pin indices 30-34 (ADC0-ADC3, ADC_TEMPERATURE) → channels 0-4
	if ch >= 30 && ch <= 34 {
		ch = ch - 30
	}

	// Internal temperature sensor (channel 4) is handled via rawInternalTempXX
	// and does not need a machine.ADC instance.
	if ch == 4 {
		// Nothing to configure in TinyGo's high-level API; rawInternalTemp12/16
		// directly manipulate the ADC peripheral.
		return nil
	}

	if _, ok := d.channels[ch]; ok {
		// already configured
		return nil
	}

	// Map core.ADCChannelID -> TinyGo ADC for external channels 0–3.
	var adc machine.ADC

	switch ch {
	case 0:
		adc = machine.ADC{Pin: machine.ADC0}
	case 1:
		adc = machine.ADC{Pin: machine.ADC1}
	case 2:
		adc = machine.ADC{Pin: machine.ADC2}
	case 3:
		adc = machine.ADC{Pin: machine.ADC3}
	default:
		// Unknown channel
		return errors.New("unsupported ADC channel")
	}

	if err := adc.Configure(machine.ADCConfig{}); err != nil {
		return err
	}

	d.channels[ch] = &adc
	return nil
}

// ReadRaw returns a raw 12-bit ADC value (0-4095) from a channel.
// This matches ADC_MAX=4095 that is reported to Klipper.
func (d *RpAdcDriver) ReadRaw(ch core.ADCChannelID) (core.ADCValue, error) {
	//d.mu.Lock()
	//defer d.mu.Unlock()

	// Translate pin enumeration index to ADC channel:
	// Pin indices 30-34 (ADC0-ADC3, ADC_TEMPERATURE) → channels 0-4
	if ch >= 30 && ch <= 34 {
		ch = ch - 30
	}

	// Internal temperature sensor: read raw 12-bit value
	if ch == 4 {
		raw12 := rawInternalTemp()
		return core.ADCValue(raw12), nil
	}

	adc, ok := d.channels[ch]
	if !ok {
		if err := d.ConfigureChannel(ch); err != nil {
			return 0, err
		}
		adc = d.channels[ch]
	}

	// TinyGo rp2040 ADC returns 12-bit value (0..4095)
	// Return it directly without scaling to match ADC_MAX=4095
	raw12 := adc.Get()
	return core.ADCValue(raw12), nil
}
