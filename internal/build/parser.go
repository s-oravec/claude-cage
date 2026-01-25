package build

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// Instruction represents a single Cagefile instruction
type Instruction struct {
	Type  string   // FROM, RUN, COPY, ENV, ARG, WORKDIR
	Value string   // The argument(s) to the instruction
	Args  []string // Parsed arguments for COPY (src, dest)
}

// Cagefile represents a parsed Cagefile
type Cagefile struct {
	BaseImage    string
	Instructions []Instruction
}

// Parse reads a Cagefile and returns parsed instructions
func Parse(r io.Reader) ([]Instruction, error) {
	var instructions []Instruction
	scanner := bufio.NewScanner(r)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		instruction, err := parseLine(line, lineNum)
		if err != nil {
			return nil, err
		}
		instructions = append(instructions, instruction)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading Cagefile: %w", err)
	}

	return instructions, nil
}

func parseLine(line string, lineNum int) (Instruction, error) {
	// Split into instruction and arguments
	parts := strings.SplitN(line, " ", 2)
	if len(parts) < 1 {
		return Instruction{}, fmt.Errorf("line %d: empty instruction", lineNum)
	}

	instType := strings.ToUpper(parts[0])
	value := ""
	if len(parts) > 1 {
		value = strings.TrimSpace(parts[1])
	}

	switch instType {
	case "FROM", "RUN", "ENV", "ARG", "WORKDIR":
		if value == "" {
			return Instruction{}, fmt.Errorf("line %d: %s requires an argument", lineNum, instType)
		}
		return Instruction{Type: instType, Value: value}, nil

	case "COPY":
		if value == "" {
			return Instruction{}, fmt.Errorf("line %d: COPY requires source and destination", lineNum)
		}
		args := strings.Fields(value)
		if len(args) < 2 {
			return Instruction{}, fmt.Errorf("line %d: COPY requires source and destination", lineNum)
		}
		return Instruction{Type: instType, Value: value, Args: args}, nil

	default:
		return Instruction{}, fmt.Errorf("line %d: unknown instruction %s", lineNum, instType)
	}
}
