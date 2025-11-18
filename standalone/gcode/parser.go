package gcode

import (
	"gopper/standalone"
)

// Parser handles G-code parsing
type Parser struct {
	lineBuffer []byte
	pos        int
}

// NewParser creates a new G-code parser
func NewParser() *Parser {
	return &Parser{
		lineBuffer: make([]byte, 0, 256),
	}
}

// ParseLine parses a single line of G-code
func (p *Parser) ParseLine(line string) (*standalone.GCodeCommand, error) {
	if len(line) == 0 {
		return nil, nil
	}

	cmd := &standalone.GCodeCommand{
		Parameters: make(map[byte]float64),
	}

	i := 0
	// Skip whitespace
	for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
		i++
	}

	if i >= len(line) {
		return nil, nil
	}

	// Check for comment
	if line[i] == ';' || line[i] == '(' {
		cmd.Comment = line[i:]
		return cmd, nil
	}

	// Parse command type (G, M, T)
	if i < len(line) && (line[i] == 'G' || line[i] == 'M' || line[i] == 'T' ||
		line[i] == 'g' || line[i] == 'm' || line[i] == 't') {
		cmd.Type = toUpper(line[i])
		i++

		// Parse command number
		num, newPos := parseInt(line, i)
		if newPos > i {
			cmd.Number = num
			i = newPos
		}
	}

	// Parse parameters
	for i < len(line) {
		// Skip whitespace
		for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
			i++
		}

		if i >= len(line) {
			break
		}

		// Check for comment
		if line[i] == ';' || line[i] == '(' {
			cmd.Comment = line[i:]
			break
		}

		// Parse parameter letter
		if i < len(line) && isLetter(line[i]) {
			letter := toUpper(line[i])
			i++

			// Parse parameter value
			value, newPos := parseFloat(line, i)
			if newPos > i {
				cmd.Parameters[letter] = value
				i = newPos
			}
		} else {
			i++
		}
	}

	return cmd, nil
}

// parseInt parses an integer from the string starting at pos
func parseInt(s string, pos int) (int, int) {
	if pos >= len(s) {
		return 0, pos
	}

	negative := false
	if s[pos] == '-' {
		negative = true
		pos++
	} else if s[pos] == '+' {
		pos++
	}

	start := pos
	value := 0

	for pos < len(s) && s[pos] >= '0' && s[pos] <= '9' {
		value = value*10 + int(s[pos]-'0')
		pos++
	}

	if pos == start {
		return 0, start - 1 // No digits found
	}

	if negative {
		value = -value
	}

	return value, pos
}

// parseFloat parses a floating-point number from the string starting at pos
func parseFloat(s string, pos int) (float64, int) {
	if pos >= len(s) {
		return 0, pos
	}

	negative := false
	if s[pos] == '-' {
		negative = true
		pos++
	} else if s[pos] == '+' {
		pos++
	}

	start := pos
	intPart := 0
	fracPart := 0.0
	fracDigits := 0

	// Parse integer part
	for pos < len(s) && s[pos] >= '0' && s[pos] <= '9' {
		intPart = intPart*10 + int(s[pos]-'0')
		pos++
	}

	// Parse fractional part
	if pos < len(s) && s[pos] == '.' {
		pos++
		fracStart := pos
		for pos < len(s) && s[pos] >= '0' && s[pos] <= '9' {
			fracPart = fracPart*10.0 + float64(s[pos]-'0')
			pos++
		}
		fracDigits = pos - fracStart
	}

	if pos == start || (pos == start+1 && s[start] == '.') {
		return 0, start - 1 // No valid number found
	}

	// Combine integer and fractional parts
	value := float64(intPart)
	if fracDigits > 0 {
		divisor := 1.0
		for i := 0; i < fracDigits; i++ {
			divisor *= 10.0
		}
		value += fracPart / divisor
	}

	if negative {
		value = -value
	}

	return value, pos
}

// isLetter checks if a byte is a letter
func isLetter(c byte) bool {
	return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z')
}

// toUpper converts a byte to uppercase
func toUpper(c byte) byte {
	if c >= 'a' && c <= 'z' {
		return c - ('a' - 'A')
	}
	return c
}

// HasParameter checks if a parameter exists in the command
func (cmd *standalone.GCodeCommand) HasParameter(param byte) bool {
	_, ok := cmd.Parameters[param]
	return ok
}

// GetParameter gets a parameter value, or returns the default if not present
func (cmd *standalone.GCodeCommand) GetParameter(param byte, defaultValue float64) float64 {
	if val, ok := cmd.Parameters[param]; ok {
		return val
	}
	return defaultValue
}
