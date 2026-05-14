package build

import (
	"fmt"
	"io"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/s-oravec/claude-cage/internal/cage"
	"github.com/s-oravec/claude-cage/internal/cloudinit"
	"github.com/s-oravec/claude-cage/internal/config"
	"github.com/s-oravec/claude-cage/internal/images"
	"github.com/s-oravec/claude-cage/internal/libvirt"
	"github.com/s-oravec/claude-cage/internal/logging"
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
	KeepOnError  bool              // Keep temp cage defined on error (don't undefine)
	Interactive  bool              // Like KeepOnError, but also keep the cage running and print SSH instructions for debugging
	ForwardAgent bool              // Forward host ssh-agent into RUN steps (for git clone over SSH etc.)
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
	user     string            // Current USER ("root" by default, or any user with sudo NOPASSWD via cage)
}

// NewExecutor creates a new build executor
func NewExecutor(config *BuildConfig) *Executor {
	return &Executor{
		config:  config,
		workdir: "/",
		env:     make(map[string]string),
		user:    "root", // Cagefile default: instructions run as root (matches Docker)
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

	// Ensure cleanup unless KeepOnError or Interactive.
	// Interactive additionally keeps the cage *running* so the user can SSH
	// in and inspect the failed state; that's handled in executeInstructions.
	defer func() {
		if e.tempCage != "" && !e.config.KeepOnError && !e.config.Interactive {
			e.cleanup()
		}
	}()

	// Step 3: Start cage and wait for SSH
	if err := e.startCage(); err != nil {
		return err
	}

	// Step 4: Execute instructions
	if err := e.executeInstructions(); err != nil {
		if e.config.Interactive {
			e.log("")
			e.log("=== Build failed — interactive debug mode ===")
			e.log("Temp cage '%s' is still running. Connect with:", e.tempCage)
			e.log("  cage ssh %s", e.tempCage)
			e.log("When you're done debugging, clean up with:")
			e.log("  cage remove %s --force", e.tempCage)
			e.log("")
		}
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
		return fmt.Errorf("base image '%s' not found, run 'cage pull --base %s' first", imageName, imageName)
	}

	e.log(" ---> Using base image: %s", imageName)

	// Create cage directories (metadata + VM artifacts; same in user mode)
	cageVMDir := cage.VMDir(e.tempCage)
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

	// Create qcow2 overlay (10G default for build) in VM artifacts dir
	baseImage := images.ImagePath(imageName)
	overlayPath := filepath.Join(cageVMDir, "disk.qcow2")

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

	// Create runtime directory in VM artifacts dir
	if err := runtime.EnsureRuntimeDir(cageVMDir); err != nil {
		cage.DeleteState(e.tempCage)
		return fmt.Errorf("failed to create runtime directory: %w", err)
	}

	// Create cloud-init ISO in VM artifacts dir
	cloudInitPath, err := cloudinit.GenerateISOWithConfig(cageVMDir, &cloudinit.CloudInitConfig{
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
	runtimeDir := runtime.RuntimeDir(cageVMDir)
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
	e.log("Step 3: Starting cage...")

	client := libvirt.NewClient()

	if err := client.StartDomain(e.tempCage); err != nil {
		return fmt.Errorf("failed to start VM: %w", err)
	}

	// Update state to running
	state, _ := cage.LoadState(e.tempCage)
	state.Status = cage.StatusRunning
	state.StartedAt = time.Now()
	cage.SaveState(state)

	// Wait for SSH with retry
	e.log(" ---> Waiting for SSH...")
	if err := ssh.WaitForSSHWithPort(e.tempCage, "127.0.0.1", e.sshPort, 120*time.Second); err != nil {
		return fmt.Errorf("SSH timeout: %w", err)
	}

	e.log(" ---> SSH ready")

	// Wait for cloud-init to complete (avoids apt lock issues)
	e.log(" ---> Waiting for cloud-init...")
	_, err := ssh.ExecCaptureWithPort(e.tempCage, "127.0.0.1", e.sshPort, "cloud-init status --wait 2>/dev/null || true")
	if err != nil {
		// cloud-init might not exist on all images, that's OK
		e.log(" ---> cloud-init not available, continuing")
	} else {
		e.log(" ---> cloud-init complete")
	}

	return nil
}

func (e *Executor) executeInstructions() error {
	total := len(e.cagefile.Instructions)

	for i, inst := range e.cagefile.Instructions {
		stepNum := i + 1 // +1 because FROM was step 1
		e.log("Step %d/%d : %s %s", stepNum+1, total+1, inst.Type, truncate(inst.Value, 50))

		var err error
		switch inst.Type {
		case "ARG":
			// ARG already processed during parsing
			e.log(" ---> Build arg set")
		case "ENV":
			err = e.executeEnv(inst)
		case "WORKDIR":
			err = e.executeWorkdir(inst)
		case "USER":
			err = e.executeUser(inst)
		case "RUN":
			err = e.executeRun(inst)
		case "COPY":
			err = e.executeCopy(inst)
		default:
			err = fmt.Errorf("unknown instruction: %s", inst.Type)
		}

		if err != nil {
			return fmt.Errorf("step %d failed: %w", stepNum+1, err)
		}
	}

	return nil
}

func (e *Executor) executeEnv(inst Instruction) error {
	// Parse KEY=VALUE
	parts := strings.SplitN(inst.Value, "=", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid ENV format: %s", inst.Value)
	}

	key := parts[0]
	value := parts[1]

	e.env[key] = value
	e.log(" ---> ENV %s=%s", key, value)

	return nil
}

func (e *Executor) executeWorkdir(inst Instruction) error {
	e.workdir = inst.Value

	// Create directory in cage
	cmd := fmt.Sprintf("mkdir -p %q", e.workdir)
	_, err := ssh.ExecCaptureWithPort(e.tempCage, "127.0.0.1", e.sshPort, cmd)
	if err != nil {
		return fmt.Errorf("failed to create workdir: %w", err)
	}

	e.log(" ---> WORKDIR %s", e.workdir)
	return nil
}

func (e *Executor) executeUser(inst Instruction) error {
	user := strings.TrimSpace(inst.Value)
	if user == "" {
		return fmt.Errorf("USER requires a username")
	}
	e.user = user
	e.log(" ---> USER %s", user)
	return nil
}

func (e *Executor) executeRun(inst Instruction) error {
	e.log(" ---> Running in %s (as %s)", e.tempCage, e.user)

	// Build command with ENV and WORKDIR
	var envExports string
	for k, v := range e.env {
		envExports += fmt.Sprintf("export %s=%q; ", k, v)
	}

	inner := fmt.Sprintf("cd %q && %s%s", e.workdir, envExports, inst.Value)
	cmd := e.wrapAsUser(inner)

	// Execute and stream output
	output, err := ssh.ExecCaptureWithOpts(e.tempCage, "127.0.0.1", e.sshPort, cmd, ssh.SSHOptions{
		ForwardAgent: e.config.ForwardAgent,
	})
	if output != "" {
		// Print output line by line
		for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
			e.log("%s", line)
		}
	}

	if err != nil {
		return fmt.Errorf("command failed: %w", err)
	}

	return nil
}

// wrapAsUser wraps a shell command so it runs as e.user inside the temp
// cage. The SSH connection itself is always the cage user (the only one
// with our key); USER changes are implemented via sudo NOPASSWD, which
// cloud-init grants cage on every image.
func (e *Executor) wrapAsUser(inner string) string {
	switch e.user {
	case "cage", "":
		return inner
	case "root":
		return "sudo bash -c " + shellQuote(inner)
	default:
		return "sudo -u " + e.user + " bash -c " + shellQuote(inner)
	}
}

// shellQuote returns s wrapped in single-quotes, with any embedded
// single-quote escaped.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func (e *Executor) executeCopy(inst Instruction) error {
	if len(inst.Args) < 2 {
		return fmt.Errorf("COPY requires source and destination")
	}

	src := inst.Args[0]
	dest := inst.Args[len(inst.Args)-1]

	// Resolve source relative to context directory
	srcPath := filepath.Join(e.config.ContextDir, src)

	// Validate source path doesn't escape build context (prevent path traversal)
	srcPath, _ = filepath.Abs(srcPath)
	ctxDir, _ := filepath.Abs(e.config.ContextDir)
	if !strings.HasPrefix(srcPath, ctxDir+string(filepath.Separator)) && srcPath != ctxDir {
		return fmt.Errorf("source path escapes build context: %s", src)
	}

	// Check source exists
	srcInfo, err := os.Stat(srcPath)
	if err != nil {
		return fmt.Errorf("source not found: %s: %w", src, err)
	}

	// Resolve destination relative to WORKDIR if not absolute
	if !filepath.IsAbs(dest) {
		dest = filepath.Join(e.workdir, dest)
	}

	e.log(" ---> Copying %s to %s (owner: %s)", src, dest, e.user)

	// Use SCP via a staging dir so we can land into root-owned destinations
	// and chown the result to the active USER (matches Docker COPY semantics).
	if srcInfo.IsDir() {
		return e.scpDir(srcPath, dest)
	}
	return e.scpFile(srcPath, dest)
}

// stagedDest returns a temp staging path inside the cage for the given dest.
func (e *Executor) stagedDest(dest string) string {
	return fmt.Sprintf("/tmp/cage-copy-%d-%s", time.Now().UnixNano(), filepath.Base(dest))
}

// moveStagedIntoPlace moves /tmp/staging → dest as root and chowns to the
// active USER. Always runs via sudo (cage user has NOPASSWD).
func (e *Executor) moveStagedIntoPlace(staged, dest string) error {
	parent := filepath.Dir(dest)
	owner := e.user
	if owner == "" {
		owner = "root"
	}
	// mkdir -p parent, mv staging → dest, chown owner.
	cmd := fmt.Sprintf(
		"sudo mkdir -p %s && sudo rm -rf %s && sudo mv %s %s && sudo chown -R %s:%s %s",
		shellQuote(parent), shellQuote(dest), shellQuote(staged), shellQuote(dest),
		owner, owner, shellQuote(dest),
	)
	_, err := ssh.ExecCaptureWithPort(e.tempCage, "127.0.0.1", e.sshPort, cmd)
	return err
}

func (e *Executor) scpFile(src, dest string) error {
	staged := e.stagedDest(dest)
	keyPath := ssh.KeyPath(e.tempCage)
	knownHostsPath := ssh.KnownHostsPath()

	args := []string{
		"-i", keyPath,
		"-o", "StrictHostKeyChecking=accept-new",
		"-o", fmt.Sprintf("UserKnownHostsFile=%s", knownHostsPath),
		"-o", fmt.Sprintf("LogLevel=%s", logging.SSHLogLevel()),
		"-P", fmt.Sprintf("%d", e.sshPort),
		src,
		fmt.Sprintf("cage@127.0.0.1:%s", staged),
	}

	cmd := exec.Command("scp", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("scp failed: %s", string(out))
	}

	return e.moveStagedIntoPlace(staged, dest)
}

func (e *Executor) scpDir(src, dest string) error {
	staged := e.stagedDest(dest)
	keyPath := ssh.KeyPath(e.tempCage)
	knownHostsPath := ssh.KnownHostsPath()

	// Create the staging directory (cage user owns /tmp). We copy contents
	// into it, then sudo mv it into dest.
	mkdirCmd := fmt.Sprintf("mkdir -p %s", shellQuote(staged))
	if _, err := ssh.ExecCaptureWithPort(e.tempCage, "127.0.0.1", e.sshPort, mkdirCmd); err != nil {
		return fmt.Errorf("failed to create staging directory: %w", err)
	}

	args := []string{
		"-i", keyPath,
		"-o", "StrictHostKeyChecking=accept-new",
		"-o", fmt.Sprintf("UserKnownHostsFile=%s", knownHostsPath),
		"-o", fmt.Sprintf("LogLevel=%s", logging.SSHLogLevel()),
		"-P", fmt.Sprintf("%d", e.sshPort),
		"-r",       // recursive
		src + "/.", // copy contents, not directory itself
		fmt.Sprintf("cage@127.0.0.1:%s", staged),
	}

	cmd := exec.Command("scp", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("scp failed: %s", string(out))
	}

	return e.moveStagedIntoPlace(staged, dest)
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func (e *Executor) stopCage() error {
	e.log("Stopping temporary cage...")

	client := libvirt.NewClient()

	if err := client.StopDomain(e.tempCage); err != nil {
		// Try force stop
		client.DestroyDomain(e.tempCage)
	}

	// Wait for domain to stop
	for i := 0; i < 30; i++ {
		active, _ := client.IsDomainActive(e.tempCage)
		if !active {
			break
		}
		time.Sleep(1 * time.Second)
	}

	// Update state
	state, _ := cage.LoadState(e.tempCage)
	state.Status = cage.StatusStopped
	cage.SaveState(state)

	return nil
}

func (e *Executor) saveImage() error {
	e.log("Saving image as '%s'...", e.config.Tag)

	result, err := images.Save(e.tempCage, e.config.Tag, fmt.Sprintf("Built from %s", e.cagefile.BaseImage))
	if err != nil {
		return fmt.Errorf("failed to save image: %w", err)
	}

	if result.VirtCustomizeError != "" {
		e.log(" ---> Warning: %s", result.VirtCustomizeError)
	}

	e.log("Successfully built image: %s", e.config.Tag)

	return nil
}

func (e *Executor) cleanup() {
	if e.tempCage == "" {
		return
	}

	client := libvirt.NewClient()

	// Force stop if still running
	if active, _ := client.IsDomainActive(e.tempCage); active {
		client.DestroyDomain(e.tempCage)
	}

	// Undefine domain
	client.UndefineDomain(e.tempCage)

	// Delete SSH keys
	ssh.DeleteKeys(e.tempCage)

	// Remove known hosts entry
	ssh.RemoveKnownHost(fmt.Sprintf("[127.0.0.1]:%d", e.sshPort))

	// Delete cage state and files
	cage.DeleteState(e.tempCage)

	e.tempCage = ""
}
