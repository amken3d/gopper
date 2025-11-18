package gcode

import (
	"testing"
)

func TestParseBasicCommands(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		input    string
		cmdType  byte
		cmdNum   int
		params   map[byte]float64
	}{
		{
			input:   "G0 X10 Y20",
			cmdType: 'G',
			cmdNum:  0,
			params:  map[byte]float64{'X': 10, 'Y': 20},
		},
		{
			input:   "G1 X100.5 Y200.25 F3000",
			cmdType: 'G',
			cmdNum:  1,
			params:  map[byte]float64{'X': 100.5, 'Y': 200.25, 'F': 3000},
		},
		{
			input:   "G28",
			cmdType: 'G',
			cmdNum:  28,
			params:  map[byte]float64{},
		},
		{
			input:   "M104 S200",
			cmdType: 'M',
			cmdNum:  104,
			params:  map[byte]float64{'S': 200},
		},
		{
			input:   "G92 X0 Y0 Z0",
			cmdType: 'G',
			cmdNum:  92,
			params:  map[byte]float64{'X': 0, 'Y': 0, 'Z': 0},
		},
	}

	for _, test := range tests {
		cmd, err := parser.ParseLine(test.input)
		if err != nil {
			t.Errorf("Failed to parse '%s': %v", test.input, err)
			continue
		}

		if cmd == nil {
			t.Errorf("Got nil command for '%s'", test.input)
			continue
		}

		if cmd.Type != test.cmdType {
			t.Errorf("Expected type %c, got %c for '%s'", test.cmdType, cmd.Type, test.input)
		}

		if cmd.Number != test.cmdNum {
			t.Errorf("Expected number %d, got %d for '%s'", test.cmdNum, cmd.Number, test.input)
		}

		for param, value := range test.params {
			if !cmd.HasParameter(param) {
				t.Errorf("Missing parameter %c in '%s'", param, test.input)
			} else if cmd.GetParameter(param, 0) != value {
				t.Errorf("Expected %c=%f, got %c=%f in '%s'",
					param, value, param, cmd.GetParameter(param, 0), test.input)
			}
		}
	}
}

func TestParseNegativeNumbers(t *testing.T) {
	parser := NewParser()

	cmd, err := parser.ParseLine("G1 X-10.5 Y-20")
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	if cmd.GetParameter('X', 0) != -10.5 {
		t.Errorf("Expected X=-10.5, got X=%f", cmd.GetParameter('X', 0))
	}

	if cmd.GetParameter('Y', 0) != -20 {
		t.Errorf("Expected Y=-20, got Y=%f", cmd.GetParameter('Y', 0))
	}
}

func TestParseComments(t *testing.T) {
	parser := NewParser()

	tests := []string{
		"; This is a comment",
		"G0 X10 ; Move to X10",
		"(This is a comment)",
	}

	for _, test := range tests {
		cmd, err := parser.ParseLine(test)
		if err != nil {
			t.Errorf("Failed to parse '%s': %v", test, err)
		}

		if cmd == nil {
			t.Errorf("Got nil command for '%s'", test)
		}
	}
}

func TestParseLowercase(t *testing.T) {
	parser := NewParser()

	cmd, err := parser.ParseLine("g1 x10 y20")
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	if cmd.Type != 'G' {
		t.Errorf("Expected type G, got %c", cmd.Type)
	}

	if cmd.Number != 1 {
		t.Errorf("Expected number 1, got %d", cmd.Number)
	}

	if cmd.GetParameter('X', 0) != 10 {
		t.Errorf("Expected X=10, got X=%f", cmd.GetParameter('X', 0))
	}
}

func TestParseEmptyLine(t *testing.T) {
	parser := NewParser()

	cmd, err := parser.ParseLine("")
	if err != nil {
		t.Errorf("Empty line should not error: %v", err)
	}

	if cmd != nil {
		t.Errorf("Empty line should return nil command")
	}
}
