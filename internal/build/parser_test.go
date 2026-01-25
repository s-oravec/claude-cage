package build

import (
	"strings"
	"testing"
)

func TestParseFrom(t *testing.T) {
	input := "FROM ubuntu:22.04"
	instructions, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(instructions) != 1 {
		t.Fatalf("expected 1 instruction, got %d", len(instructions))
	}
	if instructions[0].Type != "FROM" {
		t.Errorf("expected FROM, got %s", instructions[0].Type)
	}
	if instructions[0].Value != "ubuntu:22.04" {
		t.Errorf("expected ubuntu:22.04, got %s", instructions[0].Value)
	}
}

func TestParseRun(t *testing.T) {
	input := "RUN apt-get update && apt-get install -y curl"
	instructions, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if instructions[0].Type != "RUN" {
		t.Errorf("expected RUN, got %s", instructions[0].Type)
	}
	if instructions[0].Value != "apt-get update && apt-get install -y curl" {
		t.Errorf("unexpected value: %s", instructions[0].Value)
	}
}

func TestParseCopy(t *testing.T) {
	input := "COPY ./src /app/src"
	instructions, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if instructions[0].Type != "COPY" {
		t.Errorf("expected COPY, got %s", instructions[0].Type)
	}
	if len(instructions[0].Args) != 2 {
		t.Fatalf("expected 2 args, got %d", len(instructions[0].Args))
	}
	if instructions[0].Args[0] != "./src" || instructions[0].Args[1] != "/app/src" {
		t.Errorf("unexpected args: %v", instructions[0].Args)
	}
}

func TestParseEnv(t *testing.T) {
	input := "ENV NODE_ENV=production"
	instructions, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if instructions[0].Type != "ENV" {
		t.Errorf("expected ENV, got %s", instructions[0].Type)
	}
}

func TestParseArg(t *testing.T) {
	input := "ARG VERSION=1.0"
	instructions, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if instructions[0].Type != "ARG" {
		t.Errorf("expected ARG, got %s", instructions[0].Type)
	}
}

func TestParseWorkdir(t *testing.T) {
	input := "WORKDIR /app"
	instructions, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if instructions[0].Type != "WORKDIR" {
		t.Errorf("expected WORKDIR, got %s", instructions[0].Type)
	}
}

func TestParseMultipleInstructions(t *testing.T) {
	input := `FROM ubuntu:22.04
ARG NODE_VERSION=18
ENV NODE_ENV=production
WORKDIR /app
RUN apt-get update
COPY ./src /app/src`

	instructions, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(instructions) != 6 {
		t.Fatalf("expected 6 instructions, got %d", len(instructions))
	}
}

func TestParseCommentsAndEmptyLines(t *testing.T) {
	input := `# This is a comment
FROM ubuntu:22.04

# Another comment
RUN echo hello`

	instructions, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(instructions) != 2 {
		t.Fatalf("expected 2 instructions, got %d", len(instructions))
	}
}

func TestParseErrors(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"unknown instruction", "UNKNOWN value"},
		{"FROM without arg", "FROM"},
		{"RUN without arg", "RUN"},
		{"COPY without dest", "COPY ./src"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse(strings.NewReader(tt.input))
			if err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}

func TestValidateCagefile(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "valid cagefile",
			input:   "FROM ubuntu:22.04\nRUN echo hello",
			wantErr: false,
		},
		{
			name:    "missing FROM",
			input:   "RUN echo hello",
			wantErr: true,
		},
		{
			name:    "FROM not first",
			input:   "RUN echo hello\nFROM ubuntu:22.04",
			wantErr: true,
		},
		{
			name:    "multiple FROM",
			input:   "FROM ubuntu:22.04\nFROM alpine:3.18",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseAndValidate(strings.NewReader(tt.input))
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseAndValidate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestArgSubstitution(t *testing.T) {
	input := `FROM ubuntu:22.04
ARG VERSION=1.0
RUN echo ${VERSION}
ENV APP_VERSION=${VERSION}`

	buildArgs := map[string]string{}
	cf, err := ParseAndValidate(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resolved := cf.ResolveArgs(buildArgs)

	// RUN should have ${VERSION} replaced with 1.0
	if resolved.Instructions[1].Value != "echo 1.0" {
		t.Errorf("expected 'echo 1.0', got '%s'", resolved.Instructions[1].Value)
	}

	// ENV should have ${VERSION} replaced with 1.0
	if resolved.Instructions[2].Value != "APP_VERSION=1.0" {
		t.Errorf("expected 'APP_VERSION=1.0', got '%s'", resolved.Instructions[2].Value)
	}
}

func TestArgOverride(t *testing.T) {
	input := `FROM ubuntu:22.04
ARG VERSION=1.0
RUN echo ${VERSION}`

	buildArgs := map[string]string{"VERSION": "2.0"}
	cf, err := ParseAndValidate(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resolved := cf.ResolveArgs(buildArgs)

	if resolved.Instructions[1].Value != "echo 2.0" {
		t.Errorf("expected 'echo 2.0', got '%s'", resolved.Instructions[1].Value)
	}
}
