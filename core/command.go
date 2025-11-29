package core

import (
	"errors"
	"sync"
)

// CommandHandler is a function that handles a command with raw frame data
// The handler is responsible for decoding its own arguments from the data pointer
type CommandHandler func(data *[]byte) error

// Command represents a Klipper command
type Command struct {
	ID      uint16
	Name    string
	Format  string // Format string for dictionary (e.g., "oid=%c pin=%u")
	Handler CommandHandler
}

// CommandRegistry holds all registered commands
type CommandRegistry struct {
	mu         sync.RWMutex
	commands   map[uint16]*Command
	nameToID   map[string]uint16
	nextID     uint16
	dictionary string // Serialized dictionary for host
}

var globalRegistry = NewCommandRegistry()

// NewCommandRegistry creates a new command registry
func NewCommandRegistry() *CommandRegistry {
	return &CommandRegistry{
		commands: make(map[uint16]*Command),
		nameToID: make(map[string]uint16),
		nextID:   0,
	}
}

// RegisterCommand registers a command handler
// This is similar to DECL_COMMAND in C Klipper
func RegisterCommand(name string, format string, handler CommandHandler) uint16 {
	return globalRegistry.Register(name, format, handler)
}

// Register adds a command to the registry
func (r *CommandRegistry) Register(name string, format string, handler CommandHandler) uint16 {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if already registered
	if id, exists := r.nameToID[name]; exists {
		return id
	}

	id := r.nextID
	r.nextID++

	cmd := &Command{
		ID:      id,
		Name:    name,
		Format:  format,
		Handler: handler,
	}

	r.commands[id] = cmd
	r.nameToID[name] = id

	// NOTE: Dictionary is built once at the end via BuildDictionary()
	// Don't rebuild on every registration - causes memory allocation issues with cores scheduler

	return id
}

// GetCommand retrieves a command by ID
func (r *CommandRegistry) GetCommand(id uint16) (*Command, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	cmd, ok := r.commands[id]
	return cmd, ok
}

// Count returns the number of registered commands
func (r *CommandRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.commands)
}

// Dispatch calls the appropriate command handler
func (r *CommandRegistry) Dispatch(cmdID uint16, data *[]byte) error {
	cmd, ok := r.GetCommand(cmdID)
	if !ok {
		DebugPrintln("[CMD] Unknown command ID: " + itoa(int(cmdID)))
		return errors.New("unknown command ID: " + itoa(int(cmdID)))
	}
	id := itoa(int(cmdID))
	if id == "1" || id == "3" {
		//nop
	} else {
		DebugPrintln("[CMD] Dispatch: ID=" + itoa(int(cmdID)) + " name=" + cmd.Name)
	}

	return cmd.Handler(data)
}

// GetDictionary returns the command dictionary string
func (r *CommandRegistry) GetDictionary() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.dictionary
}

// GetCommandsAndResponses returns commands and responses for JSON dictionary
// Commands have handlers (host->MCU), responses have nil handlers (MCU->host)
func (r *CommandRegistry) GetCommandsAndResponses() (map[string]int, map[string]int) {
	DebugPrintln("[CmdReg] GetCommandsAndResponses: Acquiring read lock...")
	r.mu.RLock()
	DebugPrintln("[CmdReg] GetCommandsAndResponses: Read lock acquired")
	defer r.mu.RUnlock()

	commands := make(map[string]int)
	responses := make(map[string]int)

	DebugPrintln("[CmdReg] GetCommandsAndResponses: Processing " + itoa(int(r.nextID)) + " registered items")
	for i := uint16(0); i < r.nextID; i++ {
		if cmd, ok := r.commands[i]; ok {
			// Build format string: "name param1=%type param2=%type"
			formatStr := cmd.Name
			if cmd.Format != "" {
				formatStr = cmd.Name + " " + cmd.Format
			}

			// Commands have handlers, responses don't
			if cmd.Handler != nil {
				commands[formatStr] = int(cmd.ID)
			} else {
				responses[formatStr] = int(cmd.ID)
			}
		}
	}

	DebugPrintln("[CmdReg] GetCommandsAndResponses: Done, returning " + itoa(len(commands)) + " commands, " + itoa(len(responses)) + " responses")
	return commands, responses
}

// rebuildDictionary rebuilds the dictionary string
// Must be called with lock held
func (r *CommandRegistry) rebuildDictionary() {
	dict := ""
	for i := uint16(0); i < r.nextID; i++ {
		if cmd, ok := r.commands[i]; ok {
			// dict += fmt.Sprintf("%s %s\n", cmd.Name, cmd.Format)
			if cmd.Format != "" {
				dict += cmd.Name + " " + cmd.Format + "\n"
			} else {
				dict += cmd.Name + "\n"
			}
		}
	}
	r.dictionary = dict
}

// DispatchCommand is a convenience function using the global registry
func DispatchCommand(cmdID uint16, data *[]byte) error {
	return globalRegistry.Dispatch(cmdID, data)
}

// GetGlobalRegistry returns the global command registry
func GetGlobalRegistry() *CommandRegistry {
	return globalRegistry
}

// GetCommandCount returns the number of registered commands
func GetCommandCount() int {
	return globalRegistry.Count()
}

// RegisterResponse registers a response message (MCU -> Host)
// This is a convenience wrapper around RegisterCommand with a nil handler
func RegisterResponse(name string, format string) uint16 {
	return globalRegistry.Register(name, format, nil)
}

// LogRegisteredCommands logs all registered commands for debugging
func LogRegisteredCommands() {
	globalRegistry.mu.RLock()
	defer globalRegistry.mu.RUnlock()

	DebugPrintln("[CMD] === Registered Commands ===")
	for i := uint16(0); i < globalRegistry.nextID; i++ {
		if cmd, ok := globalRegistry.commands[i]; ok {
			handlerType := "cmd"
			if cmd.Handler == nil {
				handlerType = "rsp"
			}
			DebugPrintln("[CMD] ID=" + itoa(int(cmd.ID)) + " " + handlerType + ": " + cmd.Name)
		}
	}
	DebugPrintln("[CMD] === End Commands ===")
}
