package build

import (
	"fmt"
	"io"
)

// BuildConfig contains configuration for the build
type BuildConfig struct {
	Tag          string            // Output image name
	ContextDir   string            // Build context directory
	CagefilePath string            // Path to Cagefile
	BuildArgs    map[string]string // Build arguments
	KeepOnError  bool              // Keep temp cage on error
	Output       io.Writer         // Output writer for progress
}

// Executor handles the build process
type Executor struct {
	config   *BuildConfig
	cagefile *Cagefile
	tempCage string            // Temporary cage name
	sshPort  int               // SSH port for temp cage
	workdir  string            // Current WORKDIR in cage
	env      map[string]string // Current ENV vars
}

// NewExecutor creates a new build executor
func NewExecutor(config *BuildConfig) *Executor {
	return &Executor{
		config:  config,
		workdir: "/",
		env:     make(map[string]string),
	}
}

// Build executes the full build process
func (e *Executor) Build() error {
	// Step 1: Parse Cagefile
	if err := e.parseCagefile(); err != nil {
		return err
	}

	// Step 2: Create temporary cage
	if err := e.createTempCage(); err != nil {
		return err
	}

	// Ensure cleanup unless KeepOnError
	defer func() {
		if e.tempCage != "" && !e.config.KeepOnError {
			e.cleanup()
		}
	}()

	// Step 3: Start cage and wait for SSH
	if err := e.startCage(); err != nil {
		return err
	}

	// Step 4: Execute instructions
	if err := e.executeInstructions(); err != nil {
		return err
	}

	// Step 5: Stop cage
	if err := e.stopCage(); err != nil {
		return err
	}

	// Step 6: Save as image
	if err := e.saveImage(); err != nil {
		return err
	}

	// Cleanup temp cage
	e.cleanup()

	return nil
}

func (e *Executor) log(format string, args ...interface{}) {
	if e.config.Output != nil {
		fmt.Fprintf(e.config.Output, format+"\n", args...)
	}
}

// Stub methods - to be implemented in Tasks 5-8
func (e *Executor) parseCagefile() error {
	return fmt.Errorf("not implemented")
}

func (e *Executor) createTempCage() error {
	return fmt.Errorf("not implemented")
}

func (e *Executor) startCage() error {
	return fmt.Errorf("not implemented")
}

func (e *Executor) executeInstructions() error {
	return fmt.Errorf("not implemented")
}

func (e *Executor) stopCage() error {
	return fmt.Errorf("not implemented")
}

func (e *Executor) saveImage() error {
	return fmt.Errorf("not implemented")
}

func (e *Executor) cleanup() {
	// Stub - to be implemented
}
