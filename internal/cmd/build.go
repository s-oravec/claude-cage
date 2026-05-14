// internal/cmd/build.go
package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/s-oravec/claude-cage/internal/build"
	"github.com/s-oravec/claude-cage/internal/images"
	"github.com/spf13/cobra"
)

// NewBuildCmd creates the build command
func NewBuildCmd() *cobra.Command {
	var tag string
	var cagefilePath string
	var buildArgs []string
	var keepOnError bool
	var interactive bool
	var forwardAgent bool

	cmd := &cobra.Command{
		Use:   "build <context>",
		Short: "Build an image from a Cagefile",
		Long: `Build a custom image by executing Cagefile instructions.

The Cagefile uses Dockerfile-compatible syntax:
  FROM <base-image>    - Base image (required, must be first)
  ARG <name>=<value>   - Build argument
  ENV <key>=<value>    - Environment variable
  WORKDIR <path>       - Set working directory
  COPY <src> <dest>    - Copy files from context
  RUN <command>        - Execute shell command

Example Cagefile:
  FROM ubuntu:22.04
  ARG VERSION=1.0
  RUN apt-get update && apt-get install -y curl
  COPY ./app /app
  WORKDIR /app
  RUN ./setup.sh

Usage:
  cage build -t my-image .
  cage build -t my-image -f ./custom/Cagefile ./project
  cage build -t my-image --build-arg VERSION=2.0 .`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBuild(cmd, args[0], tag, cagefilePath, buildArgs, keepOnError, interactive, forwardAgent)
		},
	}

	cmd.Flags().StringVarP(&tag, "tag", "t", "", "Name for the built image (required)")
	cmd.Flags().StringVarP(&cagefilePath, "file", "f", "", "Path to Cagefile (default: <context>/Cagefile)")
	cmd.Flags().StringArrayVar(&buildArgs, "build-arg", nil, "Build argument (KEY=VALUE)")
	cmd.Flags().BoolVar(&keepOnError, "keep-on-error", false, "Keep temporary cage defined on build failure")
	cmd.Flags().BoolVar(&interactive, "interactive", false, "On failure, leave the temp cage running and print SSH instructions for debugging")
	cmd.Flags().BoolVarP(&forwardAgent, "forward-agent", "A", false, "Forward the local ssh-agent into RUN steps (for git clone over SSH from inside the build)")

	cmd.MarkFlagRequired("tag")

	return cmd
}

func runBuild(cmd *cobra.Command, context, tag, cagefilePath string, buildArgsList []string, keepOnError, interactive, forwardAgent bool) error {
	// Validate tag
	if tag == "" {
		return fmt.Errorf("--tag is required")
	}

	// Check if image already exists
	if images.Exists(tag) {
		return fmt.Errorf("image '%s' already exists", tag)
	}

	// Resolve context path
	contextDir, err := filepath.Abs(context)
	if err != nil {
		return fmt.Errorf("invalid context path: %w", err)
	}

	// Check context exists
	info, err := os.Stat(contextDir)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("context directory not found: %s", context)
	}

	// Resolve Cagefile path
	if cagefilePath == "" {
		cagefilePath = filepath.Join(contextDir, "Cagefile")
	} else {
		cagefilePath, err = filepath.Abs(cagefilePath)
		if err != nil {
			return fmt.Errorf("invalid Cagefile path: %w", err)
		}
	}

	// Check Cagefile exists
	if _, err := os.Stat(cagefilePath); err != nil {
		return fmt.Errorf("Cagefile not found: %s", cagefilePath)
	}

	// Parse build args
	buildArgs := make(map[string]string)
	for _, arg := range buildArgsList {
		parts := strings.SplitN(arg, "=", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid build arg format: %s (expected KEY=VALUE)", arg)
		}
		buildArgs[parts[0]] = parts[1]
	}

	// Create and run executor
	executor := build.NewExecutor(&build.BuildConfig{
		Tag:          tag,
		ContextDir:   contextDir,
		CagefilePath: cagefilePath,
		BuildArgs:    buildArgs,
		KeepOnError:  keepOnError,
		Interactive:  interactive,
		ForwardAgent: forwardAgent,
		Output:       cmd.OutOrStdout(),
	})

	return executor.Build()
}
