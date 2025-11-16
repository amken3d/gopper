package core

//
//import (
//	"testing"
//)
//
//// MockGPIODriver is a test implementation of GPIODriver
//type MockGPIODriver struct {
//	pins map[GPIOPin]bool
//}
//
//func NewMockGPIODriver() *MockGPIODriver {
//	return &MockGPIODriver{
//		pins: make(map[GPIOPin]bool),
//	}
//}
//
//func (m *MockGPIODriver) ConfigureOutput(pin GPIOPin) error {
//	m.pins[pin] = false
//	return nil
//}
//
//func (m *MockGPIODriver) SetPin(pin GPIOPin, value bool) error {
//	m.pins[pin] = value
//	return nil
//}
//
//func (m *MockGPIODriver) GetPin(pin GPIOPin) (bool, error) {
//	return m.pins[pin], nil
//}
//func TestDigitalOutBasic(t *testing.T) {
//	// Setup mock GPIO driver
//	mockDriver := NewMockGPIODriver()
//	SetGPIODriver(mockDriver)
//
//	// Initialize commands
//	InitGPIOCommands()
//
//	// Test config_digital_out
//	// Format: oid=%c pin=%u value=%c default_value=%c max_duration=%u
//	// Configure pin 25 (LED on Pico), initial=1, default=0, max_duration=0
//	testOID := uint8(1)
//	testPin := GPIOPin(25)
//
//	// Get configured digital output
//	if dout, exists := digitalOutputs[testOID]; exists {
//		if dout.Pin != testPin {
//			t.Errorf("Expected pin %d, got %d", testPin, dout.Pin)
//		}
//	}
//}
//
//func TestGPIODriverBasic(t *testing.T) {
//	mockDriver := NewMockGPIODriver()
//	SetGPIODriver(mockDriver)
//
//	// Test configure
//	pin := GPIOPin(25)
//	err := mockDriver.ConfigureOutput(pin)
//	if err != nil {
//		t.Fatalf("ConfigureOutput failed: %v", err)
//	}
//
//	// Test set high
//	err = mockDriver.SetPin(pin, true)
//	if err != nil {
//		t.Fatalf("SetPin(true) failed: %v", err)
//	}
//
//	// Verify state
//	state, err := mockDriver.GetPin(pin)
//	if err != nil {
//		t.Fatalf("GetPin failed: %v", err)
//	}
//	if !state {
//		t.Errorf("Expected pin to be high, got low")
//	}
//
//	// Test set low
//	err = mockDriver.SetPin(pin, false)
//	if err != nil {
//		t.Fatalf("SetPin(false) failed: %v", err)
//	}
//
//	// Verify state
//	state, err = mockDriver.GetPin(pin)
//	if err != nil {
//		t.Fatalf("GetPin failed: %v", err)
//	}
//	if state {
//		t.Errorf("Expected pin to be low, got high")
//	}
//}
