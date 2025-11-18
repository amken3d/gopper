package standalone

import (
	"errors"
	"gopper/core"
	"gopper/standalone/config"
	"gopper/standalone/gcode"
	"gopper/standalone/kinematics"
	"gopper/standalone/planner"
)

// Manager coordinates all standalone mode components
type Manager struct {
	config      *MachineConfig
	parser      *gcode.Parser
	interpreter *gcode.Interpreter
	planner     *planner.Planner
	kinematics  kinematics.Kinematics

	// Serial interface
	inputBuffer  []byte
	outputBuffer []byte

	// Status
	initialized bool
	running     bool
}

// NewManager creates a new standalone mode manager
func NewManager(configData []byte) (*Manager, error) {
	// Load configuration
	cfg, err := config.LoadConfig(configData)
	if err != nil {
		return nil, err
	}

	return NewManagerWithConfig(cfg)
}

// NewManagerWithConfig creates a manager with an existing config
func NewManagerWithConfig(cfg *MachineConfig) (*Manager, error) {
	mgr := &Manager{
		config:       cfg,
		parser:       gcode.NewParser(),
		inputBuffer:  make([]byte, 0, 256),
		outputBuffer: make([]byte, 0, 256),
		initialized:  false,
		running:      false,
	}

	return mgr, nil
}

// Initialize sets up all components
func (m *Manager) Initialize(gpioDriver core.GPIODriver) error {
	if m.initialized {
		return errors.New("already initialized")
	}

	// Create kinematics based on config
	var kin kinematics.Kinematics
	var err error

	switch m.config.Kinematics {
	case "cartesian":
		kin, err = kinematics.NewCartesian(m.config)
	default:
		return errors.New("unsupported kinematics: " + m.config.Kinematics)
	}

	if err != nil {
		return err
	}

	m.kinematics = kin

	// Create planner
	m.planner = planner.NewPlanner(m.config, kin)

	// Initialize steppers
	err = m.planner.InitSteppers(gpioDriver)
	if err != nil {
		return err
	}

	// Create interpreter
	m.interpreter = gcode.NewInterpreter(m.config, m.planner)

	m.initialized = true
	return nil
}

// ProcessLine processes a line of G-code
func (m *Manager) ProcessLine(line string) error {
	if !m.initialized {
		return errors.New("manager not initialized")
	}

	// Parse G-code
	cmd, err := m.parser.ParseLine(line)
	if err != nil {
		return err
	}

	// Execute command
	if cmd != nil {
		err = m.interpreter.Execute(cmd)
		if err != nil {
			return err
		}
	}

	return nil
}

// ProcessByte processes a single byte of input (for serial streaming)
func (m *Manager) ProcessByte(b byte) error {
	// Add to buffer
	m.inputBuffer = append(m.inputBuffer, b)

	// Check for line terminator
	if b == '\n' || b == '\r' {
		// Process line
		line := string(m.inputBuffer)
		m.inputBuffer = m.inputBuffer[:0] // Clear buffer

		// Remove trailing whitespace
		for len(line) > 0 && (line[len(line)-1] == '\n' || line[len(line)-1] == '\r' || line[len(line)-1] == ' ') {
			line = line[:len(line)-1]
		}

		if len(line) > 0 {
			err := m.ProcessLine(line)
			if err != nil {
				return err
			}

			// Send "ok" response
			m.SendResponse("ok\n")
		}
	}

	return nil
}

// SendResponse queues a response to be sent to the host
func (m *Manager) SendResponse(response string) {
	m.outputBuffer = append(m.outputBuffer, []byte(response)...)
}

// GetOutput returns any pending output and clears the buffer
func (m *Manager) GetOutput() []byte {
	if len(m.outputBuffer) == 0 {
		return nil
	}

	output := make([]byte, len(m.outputBuffer))
	copy(output, m.outputBuffer)
	m.outputBuffer = m.outputBuffer[:0]
	return output
}

// Start begins standalone operation
func (m *Manager) Start() error {
	if !m.initialized {
		return errors.New("manager not initialized")
	}

	m.running = true
	m.SendResponse("Gopper Standalone Mode Ready\n")
	return nil
}

// Stop halts all operation
func (m *Manager) Stop() {
	m.running = false
	if m.planner != nil {
		m.planner.ClearQueue()
	}
}

// IsRunning returns whether the manager is running
func (m *Manager) IsRunning() bool {
	return m.running
}

// GetState returns the current machine state
func (m *Manager) GetState() *MachineState {
	if m.interpreter != nil {
		return m.interpreter.GetState()
	}
	return nil
}

// Emergency stop
func (m *Manager) EmergencyStop() {
	m.Stop()
	// TODO: Disable all heaters
	// TODO: Trigger alarm state
}
