package watcher

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"

	"Meiko/internal/config"
	"Meiko/internal/logger"
)

// FileEvent represents a new file event
type FileEvent struct {
	Path      string
	Size      int64
	ModTime   time.Time
	EventType string
}

// FileWatcher monitors a directory for new audio files
type FileWatcher struct {
	directory string
	config    config.FileMonitorConfig
	logger    *logger.Logger
	watcher   *fsnotify.Watcher
	events    chan FileEvent
	errors    chan error
	running   bool
	mutex     sync.RWMutex
	ctx       context.Context
	cancel    context.CancelFunc
}

// New creates a new file watcher
func New(directory string, config config.FileMonitorConfig, logger *logger.Logger) (*FileWatcher, error) {
	// Validate directory exists
	if _, err := os.Stat(directory); os.IsNotExist(err) {
		return nil, fmt.Errorf("directory does not exist: %s", directory)
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create filesystem watcher: %w", err)
	}

	return &FileWatcher{
		directory: directory,
		config:    config,
		logger:    logger,
		watcher:   watcher,
		events:    make(chan FileEvent, 100), // Buffered channel for events
		errors:    make(chan error, 10),
	}, nil
}

// Start begins monitoring the directory
func (fw *FileWatcher) Start(ctx context.Context) error {
	fw.mutex.Lock()
	defer fw.mutex.Unlock()

	if fw.running {
		return fmt.Errorf("file watcher is already running")
	}

	fw.ctx, fw.cancel = context.WithCancel(ctx)

	// Add the directory to the watcher
	if err := fw.watcher.Add(fw.directory); err != nil {
		return fmt.Errorf("failed to add directory to watcher: %w", err)
	}

	fw.running = true
	fw.logger.Info("File watcher started", "directory", fw.directory)

	// Start the monitoring goroutine
	go fw.monitor()

	return nil
}

// Stop stops monitoring the directory
func (fw *FileWatcher) Stop() error {
	fw.mutex.Lock()
	defer fw.mutex.Unlock()

	if !fw.running {
		return nil
	}

	fw.logger.Info("Stopping file watcher...")

	// Cancel the context
	if fw.cancel != nil {
		fw.cancel()
	}

	// Close the filesystem watcher
	if fw.watcher != nil {
		fw.watcher.Close()
	}

	fw.running = false
	fw.logger.Info("File watcher stopped")

	return nil
}

// Events returns the channel for file events
func (fw *FileWatcher) Events() <-chan FileEvent {
	return fw.events
}

// Errors returns the channel for errors
func (fw *FileWatcher) Errors() <-chan error {
	return fw.errors
}

// IsWatching returns whether the watcher is currently active
func (fw *FileWatcher) IsWatching() bool {
	fw.mutex.RLock()
	defer fw.mutex.RUnlock()
	return fw.running
}

// monitor runs in a separate goroutine to handle filesystem events
func (fw *FileWatcher) monitor() {
	defer func() {
		fw.mutex.Lock()
		fw.running = false
		fw.mutex.Unlock()
		close(fw.events)
		close(fw.errors)
	}()

	// Keep track of files that are being written to
	pendingFiles := make(map[string]time.Time)
	ticker := time.NewTicker(time.Duration(fw.config.PollInterval) * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-fw.ctx.Done():
			fw.logger.Debug("FileWatcher", "Context cancelled, stopping monitor")
			return

		case event, ok := <-fw.watcher.Events:
			if !ok {
				fw.logger.Debug("FileWatcher", "Watcher events channel closed")
				return
			}

			if err := fw.handleEvent(event, pendingFiles); err != nil {
				fw.logger.Error("Error handling file event", "error", err, "file", event.Name)
			}

		case err, ok := <-fw.watcher.Errors:
			if !ok {
				fw.logger.Debug("FileWatcher", "Watcher errors channel closed")
				return
			}

			fw.logger.Error("File watcher error", "error", err)
			fw.errors <- err

		case <-ticker.C:
			// Check pending files to see if they're ready for processing
			fw.checkPendingFiles(pendingFiles)
		}
	}
}

// handleEvent processes a filesystem event
func (fw *FileWatcher) handleEvent(event fsnotify.Event, pendingFiles map[string]time.Time) error {
	// Only handle write and create events
	if !event.Has(fsnotify.Write) && !event.Has(fsnotify.Create) {
		return nil
	}

	// Check if the file matches our patterns
	if !fw.matchesPattern(event.Name) {
		return nil
	}

	fw.logger.Debug("FileWatcher", "File event detected", "file", event.Name, "op", event.Op.String())

	// Add to pending files to wait for file to be completely written
	pendingFiles[event.Name] = time.Now()

	return nil
}

// checkPendingFiles checks if pending files are ready for processing
func (fw *FileWatcher) checkPendingFiles(pendingFiles map[string]time.Time) {
	now := time.Now()
	minAge := time.Duration(fw.config.MinFileAge) * time.Second

	for filename, addedTime := range pendingFiles {
		// Check if enough time has passed
		if now.Sub(addedTime) < minAge {
			continue
		}

		// Check if file exists and get info
		fileInfo, err := os.Stat(filename)
		if err != nil {
			if os.IsNotExist(err) {
				// File was deleted, remove from pending
				delete(pendingFiles, filename)
			} else {
				fw.logger.Error("Error checking file info", "error", err, "file", filename)
			}
			continue
		}

		// Check if file size is reasonable (not empty, not too small)
		if fileInfo.Size() < 1024 { // Less than 1KB
			fw.logger.Debug("FileWatcher", "File too small, skipping", "file", filename, "size", fileInfo.Size())
			delete(pendingFiles, filename)
			continue
		}

		// File is ready, emit event
		event := FileEvent{
			Path:      filename,
			Size:      fileInfo.Size(),
			ModTime:   fileInfo.ModTime(),
			EventType: "new_file",
		}

		select {
		case fw.events <- event:
			fw.logger.Debug("FileWatcher", "New file detected", "file", filepath.Base(filename), "size", fileInfo.Size())
		case <-fw.ctx.Done():
			return
		default:
			fw.logger.Warn("File events channel full, dropping event", "file", filename)
		}

		// Remove from pending
		delete(pendingFiles, filename)
	}
}

// matchesPattern checks if a filename matches any of the configured patterns
func (fw *FileWatcher) matchesPattern(filename string) bool {
	basename := filepath.Base(filename)

	for _, pattern := range fw.config.Patterns {
		matched, err := filepath.Match(pattern, basename)
		if err != nil {
			fw.logger.Debug("FileWatcher", "Pattern match error", "pattern", pattern, "file", basename, "error", err)
			continue
		}
		if matched {
			return true
		}
	}

	return false
}

// ScanExisting scans for existing files in the directory that haven't been processed
func (fw *FileWatcher) ScanExisting() ([]FileEvent, error) {
	var events []FileEvent

	err := filepath.Walk(fw.directory, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Check if file matches patterns
		if !fw.matchesPattern(path) {
			return nil
		}

		// Check file age
		minAge := time.Duration(fw.config.MinFileAge) * time.Second
		if time.Since(info.ModTime()) < minAge {
			return nil
		}

		// Check file size
		if info.Size() < 1024 {
			return nil
		}

		event := FileEvent{
			Path:      path,
			Size:      info.Size(),
			ModTime:   info.ModTime(),
			EventType: "existing_file",
		}

		events = append(events, event)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to scan directory: %w", err)
	}

	fw.logger.Info("Scanned existing files", "directory", fw.directory, "found", len(events))
	return events, nil
}

// GetStats returns statistics about the file watcher
func (fw *FileWatcher) GetStats() map[string]interface{} {
	fw.mutex.RLock()
	defer fw.mutex.RUnlock()

	return map[string]interface{}{
		"running":         fw.running,
		"directory":       fw.directory,
		"patterns":        fw.config.Patterns,
		"poll_interval":   fw.config.PollInterval,
		"min_file_age":    fw.config.MinFileAge,
		"events_buffered": len(fw.events),
		"errors_buffered": len(fw.errors),
	}
}

// UpdatePatterns updates the file patterns to watch for
func (fw *FileWatcher) UpdatePatterns(patterns []string) {
	fw.mutex.Lock()
	defer fw.mutex.Unlock()

	fw.config.Patterns = patterns
	fw.logger.Info("Updated file patterns", "patterns", patterns)
}

// MatchesAnyPattern checks if a filename matches any configured pattern
func (fw *FileWatcher) MatchesAnyPattern(filename string) bool {
	return fw.matchesPattern(filename)
}

// GetDirectory returns the monitored directory
func (fw *FileWatcher) GetDirectory() string {
	return fw.directory
}

// ValidateFile performs additional validation on a file
func (fw *FileWatcher) ValidateFile(path string) error {
	// Check if file exists
	fileInfo, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("file does not exist or is not accessible: %w", err)
	}

	// Check if it's a regular file
	if !fileInfo.Mode().IsRegular() {
		return fmt.Errorf("path is not a regular file: %s", path)
	}

	// Check file size
	if fileInfo.Size() == 0 {
		return fmt.Errorf("file is empty: %s", path)
	}

	// Check if file is recent enough (not too old)
	maxAge := 24 * time.Hour // Don't process files older than 24 hours
	if time.Since(fileInfo.ModTime()) > maxAge {
		return fmt.Errorf("file is too old: %s", path)
	}

	// Check file extension
	ext := strings.ToLower(filepath.Ext(path))
	validExts := []string{".mp3", ".wav", ".m4a", ".ogg", ".flac"}
	valid := false
	for _, validExt := range validExts {
		if ext == validExt {
			valid = true
			break
		}
	}

	if !valid {
		return fmt.Errorf("invalid file extension: %s", ext)
	}

	return nil
}
