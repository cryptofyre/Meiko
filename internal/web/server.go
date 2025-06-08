package web

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/websocket/v2"
	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"

	"Meiko/internal/config"
	"Meiko/internal/database"
	meikoLogger "Meiko/internal/logger"
	"Meiko/internal/monitoring"
	"Meiko/internal/talkgroups"
)

// AutoSummary represents an automatically generated summary
type AutoSummary struct {
	Summary     string    `json:"summary"`
	GeneratedAt time.Time `json:"generated_at"`
	TimeRange   string    `json:"time_range"`
	CallCount   int       `json:"call_count"`
}

// Server represents the web server instance
type Server struct {
	app             *fiber.App
	config          *config.Config
	db              *database.Database
	monitor         *monitoring.Monitor
	talkgroups      *talkgroups.Service
	logger          *meikoLogger.Logger
	clients         map[*websocket.Conn]bool
	broadcast       chan []byte
	gemini          *genai.Client
	lastAutoSummary *AutoSummary
	summaryMu       sync.RWMutex
}

// CallRecord represents a call record for API responses
type CallRecord struct {
	ID              int       `json:"id"`
	Filename        string    `json:"filename"`
	Filepath        string    `json:"filepath"`
	Timestamp       time.Time `json:"timestamp"`
	Duration        int       `json:"duration"`
	Frequency       string    `json:"frequency"`
	TalkgroupID     string    `json:"talkgroup_id"`
	TalkgroupAlias  string    `json:"talkgroup_alias"`
	TalkgroupGroup  string    `json:"talkgroup_group"`
	TranscriptionID *int      `json:"transcription_id,omitempty"`
	Transcription   string    `json:"transcription"`
	CreatedAt       time.Time `json:"created_at"`
}

// TimelineEvent represents an event in the timeline
type TimelineEvent struct {
	ID          string                 `json:"id"`
	Type        string                 `json:"type"`
	Timestamp   time.Time              `json:"timestamp"`
	Title       string                 `json:"title"`
	Description string                 `json:"description"`
	Data        map[string]interface{} `json:"data,omitempty"`
	Icon        string                 `json:"icon"`
	Color       string                 `json:"color"`
}

// TimelineResponse represents the timeline API response
type TimelineResponse struct {
	Events     []TimelineEvent `json:"events"`
	HasMore    bool            `json:"has_more"`
	NextCursor string          `json:"next_cursor,omitempty"`
}

// SystemStats represents system statistics for API responses
type SystemStats struct {
	CPU         float64          `json:"cpu"`
	Memory      float64          `json:"memory"`
	Disk        float64          `json:"disk"`
	Temperature float64          `json:"temperature"`
	Uptime      time.Duration    `json:"uptime"`
	TotalCalls  int64            `json:"total_calls"`
	LastCall    *time.Time       `json:"last_call,omitempty"`
	CallsToday  int64            `json:"calls_today"`
	Frequencies map[string]int64 `json:"frequencies"`
	Talkgroups  map[string]int64 `json:"talkgroups"`
}

// TimeRange represents a time range filter
type TimeRange struct {
	Start time.Time
	End   time.Time
}

// New creates a new web server instance
func New(cfg *config.Config, db *database.Database, monitor *monitoring.Monitor, talkgroups *talkgroups.Service, logger *meikoLogger.Logger) (*Server, error) {
	server := &Server{
		config:     cfg,
		db:         db,
		monitor:    monitor,
		talkgroups: talkgroups,
		logger:     logger,
		clients:    make(map[*websocket.Conn]bool),
		broadcast:  make(chan []byte),
	}

	// Initialize Fiber app
	server.app = fiber.New(fiber.Config{
		AppName:      "Meiko Web Dashboard",
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	})

	// Add middleware
	server.app.Use(recover.New())
	server.app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowMethods: "GET,POST,HEAD,PUT,DELETE,PATCH,OPTIONS",
		AllowHeaders: "Origin,Content-Type,Accept,Authorization",
	}))

	// Initialize Gemini client if enabled
	if cfg.Web.Gemini.Enabled && cfg.Web.Gemini.APIKey != "" {
		ctx := context.Background()
		client, err := genai.NewClient(ctx, option.WithAPIKey(cfg.Web.Gemini.APIKey))
		if err != nil {
			log.Printf("Failed to initialize Gemini client: %v", err)
		} else {
			server.gemini = client
		}
	}

	// Setup routes
	server.setupRoutes()

	// Start WebSocket broadcast goroutine
	go server.handleBroadcast()

	// Start auto-summary generation routine
	go server.autoSummaryRoutine()

	return server, nil
}

// setupRoutes configures all the API routes
func (s *Server) setupRoutes() {
	// Serve static files
	s.app.Static("/", "./web/static")
	s.app.Static("/static", "./web/static")

	// API routes
	api := s.app.Group("/api")

	// Timeline endpoints
	api.Get("/timeline", s.getTimeline)
	api.Get("/timeline/:date", s.getTimelineForDate)

	// Call records endpoints
	api.Get("/calls", s.getCalls)
	api.Get("/calls/:id", s.getCall)
	api.Get("/calls/:id/audio", s.getCallAudio)
	api.Get("/calls/summary/:range", s.getCallsSummary)

	// Statistics endpoints
	api.Get("/stats", s.getStats)
	api.Get("/stats/lifetime", s.getLifetimeStats)

	// Auto-summary endpoints
	api.Get("/summary/auto", s.getAutoSummary)

	// System endpoints
	api.Get("/system", s.getSystemInfo)
	api.Get("/logs", s.getLogs)

	// AI Summary endpoints (requires Gemini)
	api.Post("/summary/generate", s.generateSummary)

	// WebSocket endpoint
	s.app.Use("/ws", func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			c.Locals("allowed", true)
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})
	s.app.Get("/ws", websocket.New(s.handleWebSocket))
}

// getTimeline returns timeline events for today
func (s *Server) getTimeline(c *fiber.Ctx) error {
	limit := c.QueryInt("limit", 50)

	// Get today's date
	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	endOfDay := startOfDay.Add(24 * time.Hour)

	events, err := s.buildTimelineEvents(&startOfDay, &endOfDay, limit)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error":   "Failed to fetch timeline events",
			"details": err.Error(),
		})
	}

	response := TimelineResponse{
		Events:  events,
		HasMore: len(events) >= limit,
	}

	return c.JSON(response)
}

// getTimelineForDate returns timeline events for a specific date
func (s *Server) getTimelineForDate(c *fiber.Ctx) error {
	dateParam := c.Params("date")
	limit := c.QueryInt("limit", 50)

	// Parse date (expected format: YYYY-MM-DD)
	date, err := time.Parse("2006-01-02", dateParam)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "Invalid date format. Use YYYY-MM-DD",
		})
	}

	startOfDay := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	endOfDay := startOfDay.Add(24 * time.Hour)

	events, err := s.buildTimelineEvents(&startOfDay, &endOfDay, limit)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error":   "Failed to fetch timeline events",
			"details": err.Error(),
		})
	}

	response := TimelineResponse{
		Events:  events,
		HasMore: len(events) >= limit,
	}

	return c.JSON(response)
}

// buildTimelineEvents creates timeline events from various data sources
func (s *Server) buildTimelineEvents(start, end *time.Time, limit int) ([]TimelineEvent, error) {
	var events []TimelineEvent

	// Get call records for the time period
	calls, err := s.db.GetCallRecords(start, end, "", limit*2, 0) // Get more calls to mix with other events
	if err != nil {
		return nil, err
	}

	// Convert calls to timeline events
	for _, call := range calls {
		// Get enhanced talkgroup information
		var eventColor string
		var eventIcon string
		var eventTitle string

		if s.talkgroups != nil {
			talkgroupInfo := s.talkgroups.GetTalkgroupInfo(call.TalkgroupID)
			deptInfo := s.talkgroups.GetDepartmentInfo(call.TalkgroupID)

			// Use department-specific colors and icons
			eventColor = deptInfo.Color
			eventTitle = fmt.Sprintf("Call from %s %s", deptInfo.Emoji, talkgroupInfo.Group)

			// Set icon based on service type
			switch deptInfo.Type {
			case talkgroups.ServicePolice:
				eventIcon = "shield-alt"
			case talkgroups.ServiceFire:
				eventIcon = "fire"
			case talkgroups.ServiceEMS:
				eventIcon = "ambulance"
			case talkgroups.ServiceEmergency:
				eventIcon = "exclamation-triangle"
			case talkgroups.ServicePublicWorks:
				eventIcon = "tools"
			case talkgroups.ServiceEducation:
				eventIcon = "graduation-cap"
			case talkgroups.ServiceAirport:
				eventIcon = "plane"
			case talkgroups.ServiceEvents:
				eventIcon = "broadcast-tower"
			default:
				eventIcon = "phone"
			}
		} else {
			// Fallback to basic formatting
			eventColor = "#3b82f6"
			eventIcon = "phone"
			eventTitle = fmt.Sprintf("Call on %s", call.TalkgroupAlias)
		}

		event := TimelineEvent{
			ID:        fmt.Sprintf("call_%d", call.ID),
			Type:      "call",
			Timestamp: call.Timestamp,
			Title:     eventTitle,
			Icon:      eventIcon,
			Color:     eventColor,
			Data: map[string]interface{}{
				"talkgroup":    call.TalkgroupAlias,
				"frequency":    call.Frequency,
				"duration":     call.Duration,
				"call_id":      call.ID,
				"service_type": "",
			},
		}

		// Add service type if available
		if s.talkgroups != nil {
			talkgroupInfo := s.talkgroups.GetTalkgroupInfo(call.TalkgroupID)
			event.Data["service_type"] = string(talkgroupInfo.ServiceType)
		}

		// Create description based on transcription
		if call.Transcription != "" {
			if len(call.Transcription) > 100 {
				event.Description = call.Transcription[:100] + "..."
			} else {
				event.Description = call.Transcription
			}
		} else {
			event.Description = fmt.Sprintf("Duration: %ds on %s", call.Duration, call.Frequency)
		}

		events = append(events, event)
	}

	// Add system events (you can expand this based on your logging/event system)
	systemInfo := s.monitor.GetSystemInfo()
	var startupTime time.Time

	// Get the actual startup time from monitor uptime
	if uptime, ok := systemInfo["uptime"].(float64); ok {
		startupTime = time.Now().Add(-time.Duration(uptime) * time.Second)
	} else {
		// Fallback to a reasonable time
		startupTime = time.Now().Add(-30 * time.Minute)
	}

	// Only add system startup event if it falls within the requested time range
	if (start == nil || startupTime.After(*start)) && (end == nil || startupTime.Before(*end)) {
		events = append(events, TimelineEvent{
			ID:          "system_start",
			Type:        "system",
			Timestamp:   startupTime,
			Title:       "Meiko System Started",
			Description: "SDR monitoring and transcription system came online",
			Icon:        "power-off",
			Color:       "#22c55e",
		})
	}

	// Sort events by timestamp (newest first)
	for i := 0; i < len(events)-1; i++ {
		for j := i + 1; j < len(events); j++ {
			if events[i].Timestamp.Before(events[j].Timestamp) {
				events[i], events[j] = events[j], events[i]
			}
		}
	}

	// Limit results
	if len(events) > limit {
		events = events[:limit]
	}

	return events, nil
}

// getCalls returns call records with optional filtering
func (s *Server) getCalls(c *fiber.Ctx) error {
	// Parse query parameters
	limit := c.QueryInt("limit", 50)
	offset := c.QueryInt("offset", 0)
	timeRange := c.Query("range", "")
	talkgroupID := c.Query("talkgroup", "")

	// Build time filter
	var start, end *time.Time
	if timeRange != "" {
		tr, err := s.parseTimeRange(timeRange)
		if err == nil {
			start = &tr.Start
			end = &tr.End
		}
	}

	// Get calls from database
	calls, err := s.db.GetCallRecords(start, end, talkgroupID, limit, offset)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error":   "Failed to fetch call records",
			"details": err.Error(),
		})
	}

	// Convert to API format
	apiCalls := make([]CallRecord, len(calls))
	for i, call := range calls {
		apiCalls[i] = CallRecord{
			ID:              call.ID,
			Filename:        call.Filename,
			Filepath:        call.Filepath,
			Timestamp:       call.Timestamp,
			Duration:        call.Duration,
			Frequency:       call.Frequency,
			TalkgroupID:     call.TalkgroupID,
			TalkgroupAlias:  call.TalkgroupAlias,
			TalkgroupGroup:  call.TalkgroupGroup,
			TranscriptionID: call.TranscriptionID,
			Transcription:   call.Transcription,
			CreatedAt:       call.CreatedAt,
		}
	}

	return c.JSON(fiber.Map{
		"calls": apiCalls,
		"pagination": fiber.Map{
			"limit":  limit,
			"offset": offset,
			"total":  len(apiCalls),
		},
	})
}

// getCall returns a specific call record
func (s *Server) getCall(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "Invalid call ID",
		})
	}

	call, err := s.db.GetCallRecord(id)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{
			"error": "Call record not found",
		})
	}

	apiCall := CallRecord{
		ID:              call.ID,
		Filename:        call.Filename,
		Filepath:        call.Filepath,
		Timestamp:       call.Timestamp,
		Duration:        call.Duration,
		Frequency:       call.Frequency,
		TalkgroupID:     call.TalkgroupID,
		TalkgroupAlias:  call.TalkgroupAlias,
		TalkgroupGroup:  call.TalkgroupGroup,
		TranscriptionID: call.TranscriptionID,
		Transcription:   call.Transcription,
		CreatedAt:       call.CreatedAt,
	}

	return c.JSON(apiCall)
}

// getCallAudio serves the audio file for a specific call
func (s *Server) getCallAudio(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "Invalid call ID",
		})
	}

	call, err := s.db.GetCallRecord(id)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{
			"error": "Call record not found",
		})
	}

	// Check if audio file exists
	if _, err := os.Stat(call.Filepath); os.IsNotExist(err) {
		return c.Status(404).JSON(fiber.Map{
			"error": "Audio file not found",
		})
	}

	// Set proper headers for audio streaming
	c.Set("Content-Type", "audio/mpeg")
	c.Set("Content-Disposition", fmt.Sprintf("inline; filename=\"%s\"", call.Filename))
	c.Set("Accept-Ranges", "bytes")

	// Stream the audio file
	return c.SendFile(call.Filepath)
}

// getCallsSummary returns aggregated call statistics
func (s *Server) getCallsSummary(c *fiber.Ctx) error {
	rangeParam := c.Params("range")

	tr, err := s.parseTimeRange(rangeParam)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error":   "Invalid time range",
			"details": err.Error(),
		})
	}

	stats, err := s.db.GetCallStats(&tr.Start, &tr.End)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error":   "Failed to fetch call statistics",
			"details": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"range": rangeParam,
		"start": tr.Start,
		"end":   tr.End,
		"stats": stats,
	})
}

// getStats returns current system statistics
func (s *Server) getStats(c *fiber.Ctx) error {
	stats := s.monitor.GetCurrentStats()
	systemInfo := s.monitor.GetSystemInfo()

	// Get call statistics
	totalCalls, _ := s.db.GetTotalCallCount()
	lastCall, _ := s.db.GetLastCallTime()
	callsToday, _ := s.db.GetCallsToday()
	frequencies, _ := s.db.GetFrequencyStats()
	talkgroups, _ := s.db.GetTalkgroupStats()

	// Convert uptime from seconds to duration
	var uptime time.Duration
	if uptimeSeconds, ok := systemInfo["uptime"].(float64); ok {
		uptime = time.Duration(uptimeSeconds) * time.Second
	}

	systemStats := SystemStats{
		CPU:         stats.CPU,
		Memory:      stats.Memory,
		Disk:        stats.Disk,
		Temperature: stats.Temperature,
		Uptime:      uptime,
		TotalCalls:  totalCalls,
		LastCall:    lastCall,
		CallsToday:  callsToday,
		Frequencies: frequencies,
		Talkgroups:  talkgroups,
	}

	return c.JSON(systemStats)
}

// getLifetimeStats returns lifetime system statistics
// getAutoSummary returns the latest auto-generated summary
func (s *Server) getAutoSummary(c *fiber.Ctx) error {
	s.summaryMu.RLock()
	defer s.summaryMu.RUnlock()

	if s.lastAutoSummary == nil {
		return c.Status(404).JSON(fiber.Map{
			"error": "No auto-generated summary available yet",
		})
	}

	// Check if summary is still fresh (within last 2 hours)
	if time.Since(s.lastAutoSummary.GeneratedAt) > 2*time.Hour {
		return c.Status(404).JSON(fiber.Map{
			"error": "Auto-generated summary is stale, generating new one...",
		})
	}

	return c.JSON(s.lastAutoSummary)
}

func (s *Server) getLifetimeStats(c *fiber.Ctx) error {
	stats, err := s.db.GetLifetimeStats()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error":   "Failed to fetch lifetime statistics",
			"details": err.Error(),
		})
	}

	return c.JSON(stats)
}

// getSystemInfo returns system information
func (s *Server) getSystemInfo(c *fiber.Ctx) error {
	info := s.monitor.GetSystemInfo()
	return c.JSON(info)
}

// getLogs returns recent log entries
func (s *Server) getLogs(c *fiber.Ctx) error {
	limit := c.QueryInt("limit", 50)
	level := c.Query("level", "")

	// Get recent logs from logger buffer
	logs := s.logger.GetRecentLogs(limit)

	// Filter by level if specified
	if level != "" {
		filteredLogs := make([]meikoLogger.LogEntry, 0)
		for _, log := range logs {
			if strings.EqualFold(log.Level, level) {
				filteredLogs = append(filteredLogs, log)
			}
		}
		logs = filteredLogs
	}

	return c.JSON(fiber.Map{
		"logs":  logs,
		"limit": limit,
		"level": level,
		"total": len(logs),
	})
}

// generateSummary generates an AI summary using Gemini
func (s *Server) generateSummary(c *fiber.Ctx) error {
	if s.gemini == nil {
		return c.Status(503).JSON(fiber.Map{
			"error": "Gemini AI is not configured",
		})
	}

	var req struct {
		TimeRange string `json:"time_range"`
		Prompt    string `json:"prompt,omitempty"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Parse time range
	tr, err := s.parseTimeRange(req.TimeRange)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "Invalid time range",
		})
	}

	// Get call records for the time range
	calls, err := s.db.GetCallRecords(&tr.Start, &tr.End, "", 100, 0)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "Failed to fetch call records",
		})
	}

	// Build prompt for Gemini
	prompt := s.buildSummaryPrompt(calls, req.Prompt)

	// Generate summary using Gemini
	ctx := context.Background()
	model := s.gemini.GenerativeModel(s.config.Web.Gemini.Model)

	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error":   "Failed to generate summary",
			"details": err.Error(),
		})
	}

	var summary string
	if len(resp.Candidates) > 0 && len(resp.Candidates[0].Content.Parts) > 0 {
		summary = fmt.Sprintf("%v", resp.Candidates[0].Content.Parts[0])
	}

	return c.JSON(fiber.Map{
		"summary":      summary,
		"time_range":   req.TimeRange,
		"call_count":   len(calls),
		"generated_at": time.Now(),
	})
}

// handleWebSocket manages WebSocket connections
func (s *Server) handleWebSocket(c *websocket.Conn) {
	defer func() {
		delete(s.clients, c)
		c.Close()
	}()

	s.clients[c] = true

	for {
		_, _, err := c.ReadMessage()
		if err != nil {
			break
		}
	}
}

// handleBroadcast manages broadcasting to WebSocket clients
func (s *Server) handleBroadcast() {
	ticker := time.NewTicker(time.Duration(s.config.Web.Realtime.UpdateInterval) * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if s.config.Web.Realtime.Enabled {
				s.broadcastStats()
			}
		case message := <-s.broadcast:
			s.sendToClients(message)
		}
	}
}

// broadcastStats sends current statistics to all WebSocket clients
func (s *Server) broadcastStats() {
	stats := s.monitor.GetCurrentStats()
	data, err := json.Marshal(fiber.Map{
		"type":      "stats_update",
		"data":      stats,
		"timestamp": time.Now(),
	})
	if err != nil {
		return
	}

	s.sendToClients(data)
}

// sendToClients sends data to all connected WebSocket clients
func (s *Server) sendToClients(data []byte) {
	for client := range s.clients {
		if err := client.WriteMessage(websocket.TextMessage, data); err != nil {
			delete(s.clients, client)
			client.Close()
		}
	}
}

// BroadcastNewCall sends a new call notification to all clients
func (s *Server) BroadcastNewCall(call *database.CallRecord) {
	apiCall := CallRecord{
		ID:              call.ID,
		Filename:        call.Filename,
		Filepath:        call.Filepath,
		Timestamp:       call.Timestamp,
		Duration:        call.Duration,
		Frequency:       call.Frequency,
		TalkgroupID:     call.TalkgroupID,
		TalkgroupAlias:  call.TalkgroupAlias,
		TalkgroupGroup:  call.TalkgroupGroup,
		TranscriptionID: call.TranscriptionID,
		Transcription:   call.Transcription,
		CreatedAt:       call.CreatedAt,
	}

	data, err := json.Marshal(fiber.Map{
		"type":      "new_call",
		"data":      apiCall,
		"timestamp": time.Now(),
	})
	if err != nil {
		return
	}

	select {
	case s.broadcast <- data:
	default:
		// Channel is full, skip this broadcast
	}
}

// parseTimeRange parses a time range string into start and end times
func (s *Server) parseTimeRange(rangeStr string) (TimeRange, error) {
	now := time.Now()

	switch rangeStr {
	case "30min", "30m":
		return TimeRange{
			Start: now.Add(-30 * time.Minute),
			End:   now,
		}, nil
	case "1hour", "1h":
		return TimeRange{
			Start: now.Add(-1 * time.Hour),
			End:   now,
		}, nil
	case "today", "1day", "1d":
		start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		return TimeRange{
			Start: start,
			End:   now,
		}, nil
	case "week", "1week", "1w":
		return TimeRange{
			Start: now.AddDate(0, 0, -7),
			End:   now,
		}, nil
	case "month", "1month", "1M":
		return TimeRange{
			Start: now.AddDate(0, -1, 0),
			End:   now,
		}, nil
	default:
		return TimeRange{}, fmt.Errorf("unsupported time range: %s", rangeStr)
	}
}

// buildSummaryPrompt builds a prompt for Gemini based on call data
func (s *Server) buildSummaryPrompt(calls []*database.CallRecord, customPrompt string) string {
	prompt := "Please analyze the following radio communication data and provide a summary:\n\n"

	if customPrompt != "" {
		prompt += customPrompt + "\n\n"
	}

	prompt += "Call Records:\n"
	for _, call := range calls {
		prompt += fmt.Sprintf("- %s: %s (Talkgroup: %s, Duration: %ds)\n",
			call.Timestamp.Format("15:04:05"),
			call.Transcription,
			call.TalkgroupAlias,
			call.Duration)
	}

	prompt += "\nPlease provide insights about patterns, significant events, and overall activity levels."

	return prompt
}

// Start starts the web server
func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.config.Web.Host, s.config.Web.Port)

	if s.config.Web.TLS.Enabled {
		return s.app.ListenTLS(addr, s.config.Web.TLS.CertFile, s.config.Web.TLS.KeyFile)
	}

	return s.app.Listen(addr)
}

// autoSummaryRoutine generates summaries automatically in the background
func (s *Server) autoSummaryRoutine() {
	ticker := time.NewTicker(30 * time.Minute) // Generate summary every 30 minutes
	defer ticker.Stop()

	// Generate initial summary after 2 minutes
	time.Sleep(2 * time.Minute)
	s.generateAutoSummary()

	for {
		select {
		case <-ticker.C:
			s.generateAutoSummary()
		}
	}
}

// generateAutoSummary creates a new auto-generated summary
func (s *Server) generateAutoSummary() {
	// Only generate if Gemini is configured
	if s.gemini == nil {
		return
	}

	// Get today's calls
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	tomorrow := today.Add(24 * time.Hour)

	calls, err := s.db.GetCallRecords(&today, &tomorrow, "", 100, 0)
	if err != nil {
		log.Printf("Failed to get calls for auto summary: %v", err)
		return
	}

	if len(calls) == 0 {
		// No calls today, skip summary
		return
	}

	// Generate summary using Gemini
	prompt := s.buildSummaryPrompt(calls, "Provide a concise daily summary of radio communication activity")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	model := s.gemini.GenerativeModel("gemini-pro")
	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		log.Printf("Failed to generate auto summary: %v", err)
		return
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		log.Printf("No summary content generated")
		return
	}

	summaryText := fmt.Sprintf("%v", resp.Candidates[0].Content.Parts[0])

	// Store the summary
	s.summaryMu.Lock()
	s.lastAutoSummary = &AutoSummary{
		Summary:     summaryText,
		GeneratedAt: time.Now(),
		TimeRange:   "today",
		CallCount:   len(calls),
	}
	s.summaryMu.Unlock()

	log.Printf("Auto-generated summary for %d calls", len(calls))
}

// Stop gracefully stops the web server
func (s *Server) Stop() error {
	if s.gemini != nil {
		s.gemini.Close()
	}
	return s.app.Shutdown()
}

// GetPort returns the configured port
func (s *Server) GetPort() int {
	return s.config.Web.Port
}
