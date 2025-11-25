package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"gopper/host/mcu"
	"gopper/protocol"
)

var (
	device  = flag.String("device", "/dev/ttyACM0", "Serial device path")
	baud    = flag.Int("baud", 250000, "Baud rate (ignored for USB CDC)")
	verbose = flag.Bool("verbose", false, "Enable verbose output")
)

func main() {
	flag.Parse()

	fmt.Println("Gopper Host - Klipper Protocol Host Implementation")
	fmt.Println("===================================================\n")

	// Create MCU instance
	mcuConn := mcu.NewMCU()

	// Connect to MCU
	fmt.Printf("Connecting to MCU on %s...\n", *device)
	if err := mcuConn.Connect(*device); err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to connect: %v\n", err)
		os.Exit(1)
	}
	defer mcuConn.Close()

	fmt.Println("Connected successfully!")

	// Retrieve dictionary
	if err := mcuConn.RetrieveDictionary(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to retrieve dictionary: %v\n", err)
		os.Exit(1)
	}

	// Print dictionary summary
	mcuConn.PrintDictionary()

	// Interactive command loop
	fmt.Println("Enter commands (type 'help' for available commands, 'quit' to exit):")
	scanner := bufio.NewScanner(os.Stdin)

	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		cmd := parts[0]

		switch cmd {
		case "quit", "exit", "q":
			fmt.Println("Goodbye!")
			return

		case "help", "?":
			printHelp()

		case "dict":
			mcuConn.PrintDictionary()

		case "raw":
			// Print raw dictionary data
			raw := mcuConn.GetDictionaryRaw()
			fmt.Printf("Raw dictionary data (%d bytes):\n%s\n", len(raw), string(raw))

		case "get_uptime":
			if err := sendGetUptime(mcuConn); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			}

		case "get_clock":
			if err := sendGetClock(mcuConn); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			}

		case "get_config":
			if err := sendGetConfig(mcuConn); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			}

		default:
			fmt.Printf("Unknown command: %s (type 'help' for available commands)\n", cmd)
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Error reading input: %v\n", err)
		os.Exit(1)
	}
}

func printHelp() {
	fmt.Println("\nAvailable commands:")
	fmt.Println("  help           - Show this help message")
	fmt.Println("  dict           - Print dictionary summary")
	fmt.Println("  raw            - Print raw dictionary data")
	fmt.Println("  get_uptime     - Get MCU uptime")
	fmt.Println("  get_clock      - Get MCU clock")
	fmt.Println("  get_config     - Get MCU configuration")
	fmt.Println("  quit/exit/q    - Exit the program")
	fmt.Println()
}

func sendGetUptime(mcuConn *mcu.MCU) error {
	fmt.Println("Sending get_uptime command...")

	// get_uptime has no arguments, format: ""
	if err := mcuConn.SendCommand("get_uptime", nil); err != nil {
		return fmt.Errorf("failed to send get_uptime: %w", err)
	}

	fmt.Println("Command sent successfully!")
	fmt.Println("(Note: Response handling not yet implemented - check MCU logs)")

	return nil
}

func sendGetClock(mcuConn *mcu.MCU) error {
	fmt.Println("Sending get_clock command...")

	// get_clock has no arguments, format: ""
	if err := mcuConn.SendCommand("get_clock", nil); err != nil {
		return fmt.Errorf("failed to send get_clock: %w", err)
	}

	fmt.Println("Command sent successfully!")
	fmt.Println("Waiting for response...")

	// Wait a bit for response to arrive
	time.Sleep(100 * time.Millisecond)

	// TODO: Implement proper response handling
	fmt.Println("(Note: Response handling not yet implemented - check MCU logs)")

	return nil
}

func sendGetConfig(mcuConn *mcu.MCU) error {
	fmt.Println("Sending get_config command...")

	// get_config has no arguments, format: ""
	if err := mcuConn.SendCommand("get_config", nil); err != nil {
		return fmt.Errorf("failed to send get_config: %w", err)
	}

	fmt.Println("Command sent successfully!")
	fmt.Println("(Note: Response handling not yet implemented - check MCU logs)")

	return nil
}

// DecodeResponse decodes a response message payload
func DecodeResponse(payload []byte) (cmdID uint16, data []byte, err error) {
	// Decode command ID
	cmdIDUint, err := protocol.DecodeVLQUint(&payload)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to decode command ID: %w", err)
	}

	return uint16(cmdIDUint), payload, nil
}
