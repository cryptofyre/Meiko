package processor

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"Meiko/internal/config"
	"Meiko/internal/database"
	"Meiko/internal/discord"
	"Meiko/internal/logger"
	"Meiko/internal/transcription"
	"Meiko/internal/watcher"
)

// CallProcessor handles the processing pipeline for audio files
type CallProcessor struct {
	db          *database.Database
	transcriber *transcription.Service
	discord     *discord.Client
	config      *config.Config
	logger      *logger.Logger
	webServer   WebServer
}

// WebServer interface for broadcasting new calls
type WebServer interface {
	BroadcastNewCall(call *database.CallRecord)
}

// New creates a new call processor
func New(db *database.Database, transcriber *transcription.Service, discord *discord.Client, config *config.Config, logger *logger.Logger) *CallProcessor {
	return &CallProcessor{
		db:          db,
		transcriber: transcriber,
		discord:     discord,
		config:      config,
		logger:      logger,
	}
}

// SetWebServer sets the web server for broadcasting new calls
func (cp *CallProcessor) SetWebServer(webServer WebServer) {
	cp.webServer = webServer
}

// Start begins processing file events
func (cp *CallProcessor) Start(ctx context.Context, events <-chan watcher.FileEvent) {
	go cp.processEvents(ctx, events)
}

// processEvents processes incoming file events
func (cp *CallProcessor) processEvents(ctx context.Context, events <-chan watcher.FileEvent) {
	for {
		select {
		case <-ctx.Done():
			cp.logger.Info("Call processor stopping...")
			return
		case event, ok := <-events:
			if !ok {
				cp.logger.Info("File events channel closed, stopping processor")
				return
			}
			cp.processFileEvent(ctx, event)
		}
	}
}

// processFileEvent processes a single file event
func (cp *CallProcessor) processFileEvent(ctx context.Context, event watcher.FileEvent) {
	cp.logger.Info("Processing new audio file", "file", filepath.Base(event.Path))

	// Check if file already exists in database
	exists, err := cp.db.FileExists(event.Path)
	if err != nil {
		cp.logger.Error("Error checking if file exists", "error", err, "file", event.Path)
		return
	}

	if exists {
		cp.logger.Debug("Processor", "File already processed, skipping", "file", filepath.Base(event.Path))
		return
	}

	// Parse filename to extract metadata
	callRecord := cp.parseFilename(event.Path)
	callRecord.Filepath = event.Path

	// Insert into database
	if err := cp.db.InsertCall(callRecord); err != nil {
		cp.logger.Error("Failed to insert call record", "error", err, "file", event.Path)
		return
	}

	// Transcribe the audio file
	result, err := cp.transcriber.TranscribeFile(ctx, event.Path)
	if err != nil {
		cp.logger.Error("Transcription failed", "error", err, "file", filepath.Base(event.Path))
		return
	}

	// Update database with transcription
	if err := cp.db.UpdateTranscription(callRecord.ID, result.Text); err != nil {
		cp.logger.Error("Failed to update transcription", "error", err, "id", callRecord.ID)
		return
	}

	// Update the call record with transcription
	callRecord.Transcription = result.Text

	// Mark as processed
	if err := cp.db.MarkAsProcessed(callRecord.ID); err != nil {
		cp.logger.Error("Failed to mark as processed", "error", err, "id", callRecord.ID)
		return
	}

	// Broadcast to web clients
	if cp.webServer != nil {
		cp.webServer.BroadcastNewCall(callRecord)
	}

	cp.logger.Success("Successfully processed audio file",
		"file", filepath.Base(event.Path),
		"transcription_length", len(result.Text))
}

// parseFilename extracts metadata from SDRTrunk filename format
func (cp *CallProcessor) parseFilename(filePath string) *database.CallRecord {
	filename := filepath.Base(filePath)

	// Remove extension
	name := strings.TrimSuffix(filename, filepath.Ext(filename))

	// SDRTrunk typically uses format like:
	// 20250103_174503Heart_of_Texas_Regional_Radio_System_(HOTRRS)_McLennan_T-Control__TO_33_FROM_8.mp3

	parts := strings.Split(name, "_")

	record := &database.CallRecord{
		Filename:  filename,
		Filepath:  filePath,
		Timestamp: time.Now(), // Default to current time, could be parsed from filename
		Duration:  0,          // Will be determined from audio file
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Try to parse timestamp if present
	if len(parts) > 0 && len(parts[0]) >= 8 {
		// Parse YYYYMMDD format
		dateStr := parts[0]
		if len(dateStr) == 8 {
			// This is a basic parse - in a real implementation you'd want more robust parsing
			// For now, just use current time
			record.Timestamp = time.Now()
		}
	}

	// Look for talkgroup and frequency patterns
	for i, part := range parts {
		part = strings.ToUpper(part)

		// Look for talkgroup patterns
		if i > 0 && (strings.Contains(part, "T-") || strings.Contains(part, "TG")) {
			record.TalkgroupID = part
			record.TalkgroupAlias = part // Use ID as alias for now
		}

		// Look for frequency patterns (MHz)
		if strings.Contains(part, "MHZ") || strings.Contains(part, ".") {
			record.Frequency = part
		}
	}

	// Extract system name for talkgroup group
	if len(parts) > 2 {
		// Try to find system name in the filename
		for _, part := range parts[2:6] { // Look in middle parts
			if len(part) > 3 && !strings.Contains(part, "T-") && !strings.Contains(part, "TO") && !strings.Contains(part, "FROM") {
				record.TalkgroupGroup = part
				break
			}
		}
	}

	return record
}
