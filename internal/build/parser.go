package build

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
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

// ParseAndValidate parses a Cagefile and validates its structure
func ParseAndValidate(r io.Reader) (*Cagefile, error) {
	instructions, err := Parse(r)
	if err != nil {
		return nil, err
	}

	if len(instructions) == 0 {
		return nil, fmt.Errorf("Cagefile is empty")
	}

	// FROM must be first instruction
	if instructions[0].Type != "FROM" {
		return nil, fmt.Errorf("first instruction must be FROM")
	}

	// Only one FROM allowed
	fromCount := 0
	for _, inst := range instructions {
		if inst.Type == "FROM" {
			fromCount++
		}
	}
	if fromCount > 1 {
		return nil, fmt.Errorf("multiple FROM instructions not supported")
	}

	return &Cagefile{
		BaseImage:    instructions[0].Value,
		Instructions: instructions[1:], // Skip FROM
	}, nil
}

// ResolveArgs resolves ARG values in instructions
// buildArgs overrides default values from ARG instructions
func (cf *Cagefile) ResolveArgs(buildArgs map[string]string) *Cagefile {
	// Collect ARG definitions with defaults
	args := make(map[string]string)
	for _, inst := range cf.Instructions {
		if inst.Type == "ARG" {
			name, defaultVal := parseArgValue(inst.Value)
			args[name] = defaultVal
		}
	}

	// Override with build args
	for k, v := range buildArgs {
		args[k] = v
	}

	// Create new instructions with substituted values
	resolved := &Cagefile{
		BaseImage:    cf.BaseImage,
		Instructions: make([]Instruction, len(cf.Instructions)),
	}

	for i, inst := range cf.Instructions {
		resolved.Instructions[i] = Instruction{
			Type:  inst.Type,
			Value: substituteArgs(inst.Value, args),
			Args:  inst.Args,
		}
		// Also substitute in Args for COPY
		if len(inst.Args) > 0 {
			resolved.Instructions[i].Args = make([]string, len(inst.Args))
			for j, arg := range inst.Args {
				resolved.Instructions[i].Args[j] = substituteArgs(arg, args)
			}
		}
	}

	return resolved
}

// parseArgValue parses "NAME=default" or "NAME" format
func parseArgValue(value string) (name, defaultVal string) {
	parts := strings.SplitN(value, "=", 2)
	name = parts[0]
	if len(parts) > 1 {
		defaultVal = parts[1]
	}
	return
}

// substituteArgs replaces ${VAR} and $VAR with values from args map
func substituteArgs(s string, args map[string]string) string {
	// Match ${VAR} pattern
	re := regexp.MustCompile(`\$\{(\w+)\}`)
	result := re.ReplaceAllStringFunc(s, func(match string) string {
		varName := match[2 : len(match)-1] // Remove ${ and }
		if val, ok := args[varName]; ok {
			return val
		}
		return match // Keep original if not found
	})

	// Match $VAR pattern (word boundary)
	re2 := regexp.MustCompile(`\$(\w+)`)
	result = re2.ReplaceAllStringFunc(result, func(match string) string {
		varName := match[1:] // Remove $
		if val, ok := args[varName]; ok {
			return val
		}
		return match
	})

	return result
}
