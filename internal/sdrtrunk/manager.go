package sdrtrunk

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"Meiko/internal/config"
	"Meiko/internal/logger"
)

// Manager handles the SDRTrunk process lifecycle
type Manager struct {
	config  config.SDRTrunkConfig
	logger  *logger.Logger
	cmd     *exec.Cmd
	mutex   sync.RWMutex
	running bool
	ctx     context.Context
	cancel  context.CancelFunc
}

// ProcessStatus represents the status of the SDRTrunk process
type ProcessStatus struct {
	Running   bool
	PID       int
	StartTime time.Time
	Error     error
}

// New creates a new SDRTrunk manager
func New(config config.SDRTrunkConfig, logger *logger.Logger) *Manager {
	return &Manager{
		config: config,
		logger: logger,
	}
}

// Start launches the SDRTrunk process
func (m *Manager) Start(ctx context.Context) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.running {
		return fmt.Errorf("SDRTrunk is already running")
	}

	// Create a context for this process
	m.ctx, m.cancel = context.WithCancel(ctx)

	// Validate the SDRTrunk path
	if err := m.validateSDRTrunkPath(); err != nil {
		return fmt.Errorf("SDRTrunk validation failed: %w", err)
	}

	// Build the command
	cmd, err := m.buildCommand()
	if err != nil {
		return fmt.Errorf("failed to build command: %w", err)
	}

	m.cmd = cmd

	// Log detailed startup information
	fileName := strings.ToLower(filepath.Base(m.config.Path))
	isJarFile := strings.HasSuffix(fileName, ".jar")

	if isJarFile {
		m.logger.Info("Starting SDRTrunk JAR",
			"jar_path", m.config.Path,
			"java_path", m.config.JavaPath,
			"working_dir", cmd.Dir,
			"jvm_args", m.config.JVMArgs)
	} else {
		m.logger.Info("Starting SDRTrunk binary",
			"binary_path", m.config.Path,
			"working_dir", cmd.Dir,
			"args", m.config.Args)
	}

	// Start the process
	if err := m.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start SDRTrunk: %w", err)
	}

	m.running = true
	m.logger.Success("SDRTrunk process started successfully",
		"pid", m.cmd.Process.Pid,
		"type", map[bool]string{true: "JAR", false: "binary"}[isJarFile])

	m.logger.Info("SDRTrunk output directory", "path", m.config.AudioOutputDir)

	// Start monitoring in a separate goroutine
	go m.monitor()

	// Start periodic status reporting
	go m.statusReporter()

	return nil
}

// Stop gracefully stops the SDRTrunk process
func (m *Manager) Stop() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if !m.running || m.cmd == nil {
		m.logger.Debug("SDRTrunk", "Stop requested but process not running")
		return nil
	}

	pid := m.cmd.Process.Pid
	m.logger.Info("Stopping SDRTrunk process", "pid", pid)

	// Cancel the context
	if m.cancel != nil {
		m.cancel()
	}

	// Try graceful shutdown first
	m.logger.Debug("SDRTrunk", "Sending SIGTERM for graceful shutdown")
	if err := m.cmd.Process.Signal(syscall.SIGTERM); err != nil {
		m.logger.Warn("Failed to send SIGTERM to SDRTrunk", "pid", pid, "error", err)
	}

	// Wait for graceful shutdown
	done := make(chan error, 1)
	go func() {
		done <- m.cmd.Wait()
	}()

	select {
	case <-time.After(10 * time.Second):
		// Force kill if it doesn't shutdown gracefully
		m.logger.Warn("SDRTrunk did not shutdown gracefully, forcing termination", "pid", pid)
		if err := m.cmd.Process.Kill(); err != nil {
			m.logger.Error("Failed to kill SDRTrunk process", "pid", pid, "error", err)
		}
		<-done // Wait for the process to actually exit
		m.logger.Info("SDRTrunk process terminated forcefully", "pid", pid)
	case err := <-done:
		if err != nil {
			m.logger.Debug("SDRTrunk", "Process exited during shutdown", "pid", pid, "error", err)
		} else {
			m.logger.Info("SDRTrunk process shutdown cleanly", "pid", pid)
		}
	}

	m.running = false
	m.cmd = nil
	m.logger.Success("SDRTrunk process stopped successfully", "pid", pid)
	return nil
}

// IsRunning returns whether the SDRTrunk process is currently running
func (m *Manager) IsRunning() bool {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.running
}

// GetStatus returns the current status of the SDRTrunk process
func (m *Manager) GetStatus() ProcessStatus {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	status := ProcessStatus{
		Running: m.running,
	}

	if m.cmd != nil && m.cmd.Process != nil {
		status.PID = m.cmd.Process.Pid
		// Note: StartTime would need to be tracked separately
	}

	return status
}

// Restart stops and starts the SDRTrunk process
func (m *Manager) Restart() error {
	m.logger.Info("Restarting SDRTrunk process...")

	if err := m.Stop(); err != nil {
		return fmt.Errorf("failed to stop SDRTrunk: %w", err)
	}

	// Wait a moment before restarting
	time.Sleep(2 * time.Second)

	if err := m.Start(m.ctx); err != nil {
		return fmt.Errorf("failed to start SDRTrunk: %w", err)
	}

	return nil
}

// validateSDRTrunkPath validates that the SDRTrunk executable exists and is accessible
func (m *Manager) validateSDRTrunkPath() error {
	// Check if the path exists
	fileInfo, err := os.Stat(m.config.Path)
	if os.IsNotExist(err) {
		return fmt.Errorf("SDRTrunk path does not exist: %s", m.config.Path)
	}

	fileName := strings.ToLower(filepath.Base(m.config.Path))

	// Check if it's a JAR file or executable binary
	isJarFile := strings.HasSuffix(fileName, ".jar")
	isExecutable := false

	// Check if it's an executable file (common names for SDRTrunk binary)
	if strings.Contains(fileName, "sdr-trunk") || strings.Contains(fileName, "sdrtrunk") {
		// Check if file has execute permissions (Unix-like systems)
		if fileInfo.Mode()&0111 != 0 {
			isExecutable = true
		}
	}

	if !isJarFile && !isExecutable {
		return fmt.Errorf("SDRTrunk path must be a JAR file or executable binary: %s", m.config.Path)
	}

	// Only check for Java if we're using a JAR file
	if isJarFile {
		if _, err := exec.LookPath(m.config.JavaPath); err != nil {
			return fmt.Errorf("Java executable not found (required for JAR): %s", m.config.JavaPath)
		}
	}

	return nil
}

// buildCommand constructs the command to run SDRTrunk
func (m *Manager) buildCommand() (*exec.Cmd, error) {
	var cmd *exec.Cmd
	fileName := strings.ToLower(filepath.Base(m.config.Path))
	isJarFile := strings.HasSuffix(fileName, ".jar")

	if isJarFile {
		// Build the Java command for JAR file
		args := []string{}

		// Add JVM arguments
		args = append(args, m.config.JVMArgs...)

		// Add the JAR file
		args = append(args, "-jar", m.config.Path)

		// Add SDRTrunk arguments
		args = append(args, m.config.Args...)

		// Create the command using Java
		cmd = exec.CommandContext(m.ctx, m.config.JavaPath, args...)
	} else {
		// Build command for executable binary
		args := []string{}

		// Add SDRTrunk arguments (JVM args are not applicable for native binary)
		args = append(args, m.config.Args...)

		// Create the command using the binary directly
		cmd = exec.CommandContext(m.ctx, m.config.Path, args...)
	}

	// Set working directory if specified
	if m.config.WorkingDir != "" {
		cmd.Dir = m.config.WorkingDir
	} else {
		// Use the directory containing the SDRTrunk file as working directory
		cmd.Dir = filepath.Dir(m.config.Path)
	}

	// Set up environment
	cmd.Env = os.Environ()

	// Redirect stdout and stderr to our logger
	// Use configured log level for stdout, ERROR for stderr
	stdoutLevel := strings.ToUpper(m.config.LogLevel)
	cmd.Stdout = &logWriter{logger: m.logger, level: stdoutLevel}
	cmd.Stderr = &logWriter{logger: m.logger, level: "ERROR"}

	return cmd, nil
}

// monitor runs in a separate goroutine to monitor the SDRTrunk process
func (m *Manager) monitor() {
	defer func() {
		m.mutex.Lock()
		m.running = false
		m.mutex.Unlock()
		m.logger.Debug("SDRTrunk", "Monitor goroutine exiting")
	}()

	m.logger.Debug("SDRTrunk", "Starting process monitor")

	// Wait for the process to exit
	err := m.cmd.Wait()

	// Get exit information
	exitCode := -1
	if m.cmd.ProcessState != nil {
		exitCode = m.cmd.ProcessState.ExitCode()
	}

	// Check if this was an expected shutdown
	select {
	case <-m.ctx.Done():
		// Expected shutdown
		m.logger.Info("SDRTrunk process stopped gracefully",
			"exit_code", exitCode,
			"reason", "context_cancelled")
		return
	default:
		// Unexpected exit
		if err != nil {
			m.logger.Error("SDRTrunk process exited unexpectedly",
				"error", err,
				"exit_code", exitCode)
		} else {
			m.logger.Warn("SDRTrunk process exited without error",
				"exit_code", exitCode)
		}
	}

	// TODO: Implement restart logic or notification to main application
	// For now, just log the unexpected exit
}

// statusReporter periodically reports SDRTrunk status
func (m *Manager) statusReporter() {
	ticker := time.NewTicker(5 * time.Minute) // Report status every 5 minutes
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			m.logger.Debug("SDRTrunk", "Status reporter stopping")
			return
		case <-ticker.C:
			m.logStatus()
		}
	}
}

// logStatus logs current SDRTrunk status
func (m *Manager) logStatus() {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if !m.running {
		return
	}

	if m.cmd != nil && m.cmd.Process != nil {
		pid := m.cmd.Process.Pid
		m.logger.Info("SDRTrunk status check",
			"pid", pid,
			"running", m.running,
			"output_dir", m.config.AudioOutputDir)
	}
}

// logWriter implements io.Writer to redirect process output to our logger
type logWriter struct {
	logger *logger.Logger
	level  string
}

func (lw *logWriter) Write(p []byte) (n int, err error) {
	message := strings.TrimSpace(string(p))
	if message == "" {
		return len(p), nil
	}

	// Add prefix to distinguish SDRTrunk output
	prefixedMessage := "SDRTrunk: " + message

	switch lw.level {
	case "ERROR":
		lw.logger.Error(prefixedMessage)
	case "WARN":
		lw.logger.Warn(prefixedMessage)
	case "INFO":
		lw.logger.Info(prefixedMessage)
	case "DEBUG":
		lw.logger.Debug("SDRTrunk", message)
	default:
		// Default to INFO for unknown levels
		lw.logger.Info(prefixedMessage)
	}

	return len(p), nil
}

// GetOutputDirectory returns the configured audio output directory
func (m *Manager) GetOutputDirectory() string {
	return m.config.AudioOutputDir
}

// GetConfig returns the current configuration
func (m *Manager) GetConfig() config.SDRTrunkConfig {
	return m.config
}

// UpdateConfig updates the SDRTrunk configuration
func (m *Manager) UpdateConfig(newConfig config.SDRTrunkConfig) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.running {
		return fmt.Errorf("cannot update configuration while SDRTrunk is running")
	}

	m.config = newConfig
	return nil
}

// GetProcessInfo returns detailed information about the running process
func (m *Manager) GetProcessInfo() (map[string]interface{}, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	info := map[string]interface{}{
		"running": m.running,
		"config":  m.config,
	}

	if m.cmd != nil && m.cmd.Process != nil {
		info["pid"] = m.cmd.Process.Pid

		// Get process state if available
		if m.cmd.ProcessState != nil {
			info["exit_code"] = m.cmd.ProcessState.ExitCode()
			info["system_time"] = m.cmd.ProcessState.SystemTime()
			info["user_time"] = m.cmd.ProcessState.UserTime()
		}
	}

	return info, nil
}
