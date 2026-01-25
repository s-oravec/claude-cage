package build

import (
	"fmt"
	"io"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/s-oravec/claude-cage/internal/cage"
	"github.com/s-oravec/claude-cage/internal/cloudinit"
	"github.com/s-oravec/claude-cage/internal/config"
	"github.com/s-oravec/claude-cage/internal/images"
	"github.com/s-oravec/claude-cage/internal/libvirt"
	"github.com/s-oravec/claude-cage/internal/network"
	"github.com/s-oravec/claude-cage/internal/runtime"
	"github.com/s-oravec/claude-cage/internal/ssh"
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

// Stub methods - to be implemented in Tasks 6-8
func (e *Executor) parseCagefile() error {
	e.log("Step 1: Parsing Cagefile...")

	f, err := os.Open(e.config.CagefilePath)
	if err != nil {
		return fmt.Errorf("failed to open Cagefile: %w", err)
	}
	defer f.Close()

	cf, err := ParseAndValidate(f)
	if err != nil {
		return fmt.Errorf("failed to parse Cagefile: %w", err)
	}

	// Resolve ARG substitutions
	e.cagefile = cf.ResolveArgs(e.config.BuildArgs)

	e.log(" ---> Base image: %s", e.cagefile.BaseImage)
	e.log(" ---> %d instruction(s) to execute", len(e.cagefile.Instructions))

	return nil
}

func (e *Executor) createTempCage() error {
	e.log("Step 2: Creating temporary cage...")

	// Generate unique temp cage name
	e.tempCage = fmt.Sprintf("cage-build-%d", rand.Int()%100000)

	// Resolve image alias
	imageName := images.ResolveAlias(e.cagefile.BaseImage)

	// Check image exists
	if !images.IsDownloaded(imageName) {
		return fmt.Errorf("base image '%s' not found, run 'cage setup --base %s' first", imageName, imageName)
	}

	e.log(" ---> Using base image: %s", imageName)

	// Create cage directory
	cageDir := cage.Dir(e.tempCage)
	if err := cage.EnsureDir(e.tempCage); err != nil {
		return fmt.Errorf("failed to create cage directory: %w", err)
	}

	// Find free SSH port
	sshPort, err := network.FindFreePort()
	if err != nil {
		cage.DeleteState(e.tempCage)
		return fmt.Errorf("failed to find free port: %w", err)
	}
	e.sshPort = sshPort

	// Create qcow2 overlay (10G default for build)
	baseImage := images.ImagePath(imageName)
	overlayPath := filepath.Join(cageDir, "disk.qcow2")

	createCmd := exec.Command("qemu-img", "create", "-f", "qcow2",
		"-b", baseImage, "-F", "qcow2", overlayPath, "10G")
	if out, err := createCmd.CombinedOutput(); err != nil {
		cage.DeleteState(e.tempCage)
		return fmt.Errorf("failed to create overlay: %s", string(out))
	}

	// Generate SSH keys
	if err := ssh.GenerateKeyPair(e.tempCage); err != nil {
		cage.DeleteState(e.tempCage)
		return fmt.Errorf("failed to generate SSH keys: %w", err)
	}

	pubKey, err := ssh.GetPublicKey(e.tempCage)
	if err != nil {
		cage.DeleteState(e.tempCage)
		return fmt.Errorf("failed to read public key: %w", err)
	}

	// Create runtime directory
	if err := runtime.EnsureRuntimeDir(cageDir); err != nil {
		cage.DeleteState(e.tempCage)
		return fmt.Errorf("failed to create runtime directory: %w", err)
	}

	// Create cloud-init ISO
	cloudInitPath, err := cloudinit.GenerateISOWithConfig(cageDir, &cloudinit.CloudInitConfig{
		CageName:      e.tempCage,
		PubKey:        pubKey,
		MountVirtiofs: false,
		UseRuntimeEnv: false,
		InstallSSH:    true,
	})
	if err != nil {
		cage.DeleteState(e.tempCage)
		return fmt.Errorf("failed to create cloud-init: %w", err)
	}

	// Load default profile for VM resources
	globalCfg, _ := config.Load()
	memoryMB := 2048
	vcpu := 2
	if globalCfg != nil {
		if profile, _ := globalCfg.GetProfile("default"); profile != nil {
			memoryMB = profile.MemoryMB
			vcpu = profile.VCPU
		}
	}

	// Generate domain XML
	runtimeDir := runtime.RuntimeDir(cageDir)
	domainCfg := &libvirt.DomainConfig{
		Name:         e.tempCage,
		MemoryMB:     memoryMB,
		VCPU:         vcpu,
		DiskPath:     overlayPath,
		CloudInitISO: cloudInitPath,
		NetworkName:  "",
		RuntimeDir:   runtimeDir,
		SSHPort:      sshPort,
	}

	xml, err := libvirt.GenerateDomainXML(domainCfg)
	if err != nil {
		cage.DeleteState(e.tempCage)
		return fmt.Errorf("failed to generate domain XML: %w", err)
	}

	// Define domain
	client := libvirt.NewClient()
	if err := client.DefineDomain(xml); err != nil {
		ssh.DeleteKeys(e.tempCage)
		cage.DeleteState(e.tempCage)
		return fmt.Errorf("failed to define domain: %w", err)
	}

	// Save state
	state := &cage.State{
		Name:        e.tempCage,
		Status:      cage.StatusStopped,
		Image:       imageName,
		Profile:     "default",
		NetworkMode: cage.NetworkAuto,
		SSHPort:     sshPort,
	}

	if err := cage.SaveState(state); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	e.log(" ---> Temporary cage: %s (SSH port %d)", e.tempCage, sshPort)

	return nil
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
