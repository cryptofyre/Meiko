package processor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
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

	// Calculate audio duration
	if duration, err := cp.getAudioDuration(event.Path); err == nil {
		callRecord.Duration = int(duration.Seconds())
	} else {
		cp.logger.Warn("Failed to calculate audio duration", "error", err, "file", filepath.Base(event.Path))
		callRecord.Duration = 0
	}

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

	// Send Discord notification for new call
	if cp.discord != nil && cp.discord.IsConnected() {
		if err := cp.discord.SendCallNotification(callRecord); err != nil {
			cp.logger.Error("Failed to send Discord notification", "error", err, "call_id", callRecord.ID)
		}
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

	record := &database.CallRecord{
		Filename:  filename,
		Filepath:  filePath,
		Timestamp: time.Now(), // Default to current time
		Duration:  0,          // Will be determined from audio file later
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// SDRTrunk filename format analysis:
	// 20250607_203346Heart_of_Texas_Regional_Radio_System_(HOTRRS)_McLennan_T-Control__TO_198_FROM_3071.mp3
	// Parts: [timestamp][system_name][site][talkgroup][TO_xxx_FROM_yyy]

	parts := strings.Split(name, "_")

	// Extract timestamp from first part if present (YYYYMMDD_HHMMSS format)
	if len(parts) >= 2 && len(parts[0]) == 8 && len(parts[1]) >= 6 {
		dateStr := parts[0] + parts[1][:6] // YYYYMMDDHHMMSS
		if timestamp, err := time.Parse("20060102150405", dateStr); err == nil {
			record.Timestamp = timestamp
		}
	}

	// Find system name (usually after timestamp, before site info)
	systemName := ""
	for i := 2; i < len(parts) && i < 8; i++ {
		part := parts[i]
		// Skip short parts, T-Control, TO/FROM parts
		if len(part) > 3 && !strings.HasPrefix(part, "T-") &&
			!strings.HasPrefix(part, "TO") && !strings.HasPrefix(part, "FROM") &&
			!strings.Contains(part, "(") {
			if systemName == "" {
				systemName = part
			} else {
				systemName += " " + part
			}
		}
		// Stop if we hit a parenthetical or T-Control
		if strings.Contains(part, "(") || strings.HasPrefix(part, "T-") {
			break
		}
	}

	// Extract TO and FROM values for actual talkgroup identification
	var toValue, fromValue string
	for i, part := range parts {
		if strings.HasPrefix(part, "TO") && i+1 < len(parts) {
			toValue = parts[i+1]
		}
		if strings.HasPrefix(part, "FROM") && i+1 < len(parts) {
			fromValue = parts[i+1]
		}
	}

	// Determine primary talkgroup (usually the FROM value is the calling unit)
	talkgroupID := ""
	talkgroupAlias := ""
	if fromValue != "" {
		talkgroupID = fromValue
		talkgroupAlias = "TG " + fromValue
		if toValue != "" && toValue != fromValue {
			talkgroupAlias += " → TG " + toValue
		}
	} else if toValue != "" {
		talkgroupID = toValue
		talkgroupAlias = "TG " + toValue
	}

	// If no TO/FROM found, look for T-Control or other patterns
	if talkgroupID == "" {
		for _, part := range parts {
			if strings.HasPrefix(part, "T-") {
				talkgroupID = part
				talkgroupAlias = part
				break
			}
		}
	}

	// Set default if still empty
	if talkgroupID == "" {
		talkgroupID = "Unknown"
		talkgroupAlias = "Unknown Talkgroup"
	}

	record.TalkgroupID = talkgroupID
	record.TalkgroupAlias = talkgroupAlias
	record.TalkgroupGroup = systemName

	// Try to extract frequency if present in filename
	for _, part := range parts {
		// Look for frequency patterns (numbers with MHz or decimal points)
		if strings.Contains(strings.ToLower(part), "mhz") ||
			(strings.Contains(part, ".") && len(part) > 3 && len(part) < 10) {
			record.Frequency = part
			break
		}
	}

	cp.logger.Debug("Parsed filename",
		"file", filename,
		"talkgroup_id", record.TalkgroupID,
		"talkgroup_alias", record.TalkgroupAlias,
		"system", record.TalkgroupGroup,
		"timestamp", record.Timestamp.Format("2006-01-02 15:04:05"))

	return record
}

// getAudioDuration calculates the duration of an audio file using ffprobe
func (cp *CallProcessor) getAudioDuration(filePath string) (time.Duration, error) {
	// Try ffprobe first (most reliable)
	cmd := exec.Command("ffprobe", "-v", "quiet", "-show_entries", "format=duration", "-of", "csv=p=0", filePath)
	output, err := cmd.Output()
	if err == nil {
		durationStr := strings.TrimSpace(string(output))
		if duration, err := strconv.ParseFloat(durationStr, 64); err == nil {
			return time.Duration(duration * float64(time.Second)), nil
		}
	}

	// Fallback: try using file stat for a rough estimate (not accurate but better than 0)
	if info, err := os.Stat(filePath); err == nil {
		// Very rough estimate: assume 128kbps audio, this is not accurate but better than nothing
		sizeBytes := info.Size()
		estimatedDuration := time.Duration(sizeBytes/(128*1024/8)) * time.Second
		return estimatedDuration, nil
	}

	return 0, fmt.Errorf("unable to determine audio duration")
}
