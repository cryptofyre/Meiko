package preflight

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"Meiko/internal/config"
	"Meiko/internal/logger"
)

// Checker performs system validation checks
type Checker struct {
	config *config.Config
	logger *logger.Logger
}

// New creates a new preflight checker
func New(config *config.Config, logger *logger.Logger) *Checker {
	return &Checker{
		config: config,
		logger: logger,
	}
}

// RunAll runs all preflight checks
func (c *Checker) RunAll() error {
	checks := []struct {
		name string
		fn   func() error
	}{
		{"SDRTrunk Path", c.checkSDRTrunkPath},
		{"Java Runtime", c.checkJavaRuntime},
		{"Audio Output Directory", c.checkAudioOutputDir},
		{"Transcription Config", c.checkTranscriptionConfig},
		{"Database Path", c.checkDatabasePath},
	}

	for _, check := range checks {
		c.logger.Info(fmt.Sprintf("Checking %s...", check.name))
		if err := check.fn(); err != nil {
			return fmt.Errorf("%s check failed: %w", check.name, err)
		}
		c.logger.Success(fmt.Sprintf("%s ✓", check.name))
	}

	return nil
}

// checkSDRTrunkPath validates the SDRTrunk executable path
func (c *Checker) checkSDRTrunkPath() error {
	path := c.config.SDRTrunk.Path
	if path == "" {
		return fmt.Errorf("SDRTrunk path is not configured")
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("SDRTrunk file does not exist: %s", path)
	}

	return nil
}

// checkJavaRuntime validates Java is available
func (c *Checker) checkJavaRuntime() error {
	javaPath := c.config.SDRTrunk.JavaPath
	if javaPath == "" {
		javaPath = "java"
	}

	if _, err := exec.LookPath(javaPath); err != nil {
		return fmt.Errorf("Java runtime not found: %s", javaPath)
	}

	// Test Java version
	cmd := exec.Command(javaPath, "-version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("Java runtime test failed: %w", err)
	}

	return nil
}

// checkAudioOutputDir validates the audio output directory
func (c *Checker) checkAudioOutputDir() error {
	dir := c.config.SDRTrunk.AudioOutputDir
	if dir == "" {
		return fmt.Errorf("audio output directory is not configured")
	}

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return fmt.Errorf("audio output directory does not exist: %s", dir)
	}

	// Check if directory is writable
	testFile := dir + "/.meiko_test"
	file, err := os.Create(testFile)
	if err != nil {
		return fmt.Errorf("audio output directory is not writable: %w", err)
	}
	file.Close()
	os.Remove(testFile)

	return nil
}

// checkTranscriptionConfig validates transcription configuration
func (c *Checker) checkTranscriptionConfig() error {
	switch c.config.Transcription.Mode {
	case "local":
		return c.checkLocalTranscription()
	case "remote":
		return c.checkRemoteTranscription()
	default:
		return fmt.Errorf("invalid transcription mode: %s", c.config.Transcription.Mode)
	}
}

// checkLocalTranscription validates local transcription setup
func (c *Checker) checkLocalTranscription() error {
	// Check Python
	pythonPath := c.config.Transcription.Local.PythonPath
	if pythonPath == "" {
		pythonPath = "python"
	}

	if _, err := exec.LookPath(pythonPath); err != nil {
		return fmt.Errorf("Python not found: %s", pythonPath)
	}

	// Check whisper script
	scriptPath := c.config.Transcription.Local.WhisperScript
	if scriptPath == "" {
		return fmt.Errorf("whisper script path not configured")
	}

	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		return fmt.Errorf("whisper script not found: %s", scriptPath)
	}

	return nil
}

// checkRemoteTranscription validates remote transcription setup
func (c *Checker) checkRemoteTranscription() error {
	if c.config.Transcription.Remote.Endpoint == "" {
		return fmt.Errorf("remote transcription endpoint not configured")
	}

	// TODO: Add network connectivity check
	return nil
}

// checkDatabasePath validates database configuration and creates directory if needed
func (c *Checker) checkDatabasePath() error {
	dbPath := c.config.Database.Path
	if dbPath == "" {
		return fmt.Errorf("database path not configured")
	}

	// Extract directory from database file path
	dir := filepath.Dir(dbPath)
	if dir == "" {
		dir = "."
	}

	// Create directory if it doesn't exist
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		c.logger.Info(fmt.Sprintf("Creating database directory: %s", dir))
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create database directory: %w", err)
		}
	}

	// Test if directory is writable by creating a temporary file
	testFile := filepath.Join(dir, ".meiko_test")
	file, err := os.Create(testFile)
	if err != nil {
		return fmt.Errorf("database directory is not writable: %w", err)
	}
	file.Close()
	os.Remove(testFile)

	return nil
}
