package transcription

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"Meiko/internal/config"
	"Meiko/internal/logger"
)

// TranscriptionResult represents the result of transcription
type TranscriptionResult struct {
	Text      string    `json:"text"`
	Language  string    `json:"language,omitempty"`
	Duration  float64   `json:"duration,omitempty"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
	FilePath  string    `json:"file_path"`
	Error     error     `json:"error,omitempty"`
}

// Service handles audio transcription using local or remote methods
type Service struct {
	config config.TranscriptionConfig
	logger *logger.Logger
	client *http.Client
}

// New creates a new transcription service
func New(config config.TranscriptionConfig, logger *logger.Logger) (*Service, error) {
	service := &Service{
		config: config,
		logger: logger,
		client: &http.Client{
			Timeout: time.Duration(config.Remote.Timeout) * time.Second,
		},
	}

	// Validate configuration based on mode
	if err := service.validate(); err != nil {
		return nil, fmt.Errorf("transcription service validation failed: %w", err)
	}

	logger.Info("Transcription service initialized", "mode", config.Mode)
	return service, nil
}

// validate checks the transcription configuration
func (s *Service) validate() error {
	switch s.config.Mode {
	case "local":
		return s.validateLocal()
	case "remote":
		return s.validateRemote()
	default:
		return fmt.Errorf("invalid transcription mode: %s", s.config.Mode)
	}
}

// validateLocal validates local transcription configuration
func (s *Service) validateLocal() error {
	// Check if whisper script exists
	if _, err := os.Stat(s.config.Local.WhisperScript); os.IsNotExist(err) {
		return fmt.Errorf("whisper script not found: %s", s.config.Local.WhisperScript)
	}

	// Check if Python is available
	if _, err := exec.LookPath(s.config.Local.PythonPath); err != nil {
		return fmt.Errorf("Python executable not found: %s", s.config.Local.PythonPath)
	}

	return nil
}

// validateRemote validates remote transcription configuration
func (s *Service) validateRemote() error {
	if s.config.Remote.Endpoint == "" {
		return fmt.Errorf("remote endpoint is required for remote mode")
	}

	return nil
}

// TranscribeFile transcribes an audio file and returns the result
func (s *Service) TranscribeFile(ctx context.Context, filePath string) (*TranscriptionResult, error) {
	startTime := time.Now()

	// Validate file exists and is accessible
	if err := s.validateFile(filePath); err != nil {
		return nil, fmt.Errorf("file validation failed: %w", err)
	}

	result := &TranscriptionResult{
		FilePath:  filePath,
		StartTime: startTime,
	}

	var err error
	switch s.config.Mode {
	case "local":
		result.Text, err = s.transcribeLocal(ctx, filePath)
	case "remote":
		result.Text, err = s.transcribeRemote(ctx, filePath)
	default:
		err = fmt.Errorf("unknown transcription mode: %s", s.config.Mode)
	}

	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime).Seconds()

	if err != nil {
		result.Error = err
		s.logger.Error("Transcription failed", "file", filepath.Base(filePath), "error", err)
		return result, err
	}

	s.logger.Success("Transcription completed",
		"file", filepath.Base(filePath),
		"duration", fmt.Sprintf("%.2fs", result.Duration),
		"length", len(result.Text))

	return result, nil
}

// transcribeLocal performs local transcription using faster-whisper
func (s *Service) transcribeLocal(ctx context.Context, filePath string) (string, error) {
	s.logger.Debug("Transcription", "Starting local transcription", "file", filepath.Base(filePath))

	// Build the command
	args := []string{s.config.Local.WhisperScript, filePath}
	cmd := exec.CommandContext(ctx, s.config.Local.PythonPath, args...)

	// Capture output
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Run the command
	if err := cmd.Run(); err != nil {
		stderrStr := stderr.String()
		if stderrStr != "" {
			s.logger.Error("Whisper script stderr", "output", stderrStr)
		}
		return "", fmt.Errorf("whisper script failed: %w", err)
	}

	// Parse the JSON output
	output := stdout.String()
	if output == "" {
		return "", fmt.Errorf("no output from whisper script")
	}

	var result struct {
		Text string `json:"text"`
	}

	if err := json.Unmarshal([]byte(output), &result); err != nil {
		s.logger.Error("Failed to parse whisper output", "output", output, "error", err)
		return "", fmt.Errorf("failed to parse whisper output: %w", err)
	}

	return strings.TrimSpace(result.Text), nil
}

// transcribeRemote performs remote transcription via API
func (s *Service) transcribeRemote(ctx context.Context, filePath string) (string, error) {
	s.logger.Debug("Transcription", "Starting remote transcription", "file", filepath.Base(filePath))

	// Open the file
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Create a buffer to hold the multipart form data
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Add the file
	part, err := writer.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return "", fmt.Errorf("failed to create form file: %w", err)
	}

	if _, err := io.Copy(part, file); err != nil {
		return "", fmt.Errorf("failed to copy file data: %w", err)
	}

	writer.Close()

	// Create the request
	req, err := http.NewRequestWithContext(ctx, "POST", s.config.Remote.Endpoint, &buf)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if s.config.Remote.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+s.config.Remote.APIKey)
	}

	// Send the request
	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var result struct {
		Text string `json:"text"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	return strings.TrimSpace(result.Text), nil
}

// validateFile validates that the audio file is suitable for transcription
func (s *Service) validateFile(filePath string) error {
	// Check if file exists
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("file does not exist: %w", err)
	}

	// Check file size
	if fileInfo.Size() == 0 {
		return fmt.Errorf("file is empty")
	}

	// Check minimum size (should be at least a few KB for valid audio)
	if fileInfo.Size() < 1024 {
		return fmt.Errorf("file too small: %d bytes", fileInfo.Size())
	}

	// Check file extension
	ext := strings.ToLower(filepath.Ext(filePath))
	validExts := []string{".mp3", ".wav", ".m4a", ".ogg", ".flac"}
	for _, validExt := range validExts {
		if ext == validExt {
			return nil
		}
	}

	return fmt.Errorf("unsupported file extension: %s", ext)
}

// TranscribeBatch transcribes multiple files in batch
func (s *Service) TranscribeBatch(ctx context.Context, filePaths []string) ([]*TranscriptionResult, error) {
	if len(filePaths) == 0 {
		return []*TranscriptionResult{}, nil
	}

	s.logger.Info("Starting batch transcription", "files", len(filePaths))

	results := make([]*TranscriptionResult, len(filePaths))

	for i, filePath := range filePaths {
		select {
		case <-ctx.Done():
			return results, ctx.Err()
		default:
			result, err := s.TranscribeFile(ctx, filePath)
			if result != nil {
				results[i] = result
			} else {
				results[i] = &TranscriptionResult{
					FilePath:  filePath,
					Error:     err,
					StartTime: time.Now(),
					EndTime:   time.Now(),
				}
			}
		}
	}

	return results, nil
}
