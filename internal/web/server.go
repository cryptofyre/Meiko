package web

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os"
	"sort"
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
	mu              sync.RWMutex

	// Timeline caching
	timelineCache    map[string]*TimelineCacheEntry
	timelineCacheMu  sync.RWMutex
	talkgroupCache   map[string]*TalkgroupCacheEntry
	talkgroupCacheMu sync.RWMutex

	// AI Summary caching
	aiSummaryCache   map[string]*AISummaryCacheEntry
	aiSummaryCacheMu sync.RWMutex

	// Rate limiting for AI API calls
	lastAICall     time.Time
	aiCallMu       sync.Mutex
	aiRequestCount int
	aiErrorCount   int
}

// TimelineCacheEntry represents a cached timeline response
type TimelineCacheEntry struct {
	Events    []TimelineEvent `json:"events"`
	CachedAt  time.Time       `json:"cached_at"`
	ExpiresAt time.Time       `json:"expires_at"`
}

// TalkgroupCacheEntry represents cached talkgroup information
type TalkgroupCacheEntry struct {
	Info      *TalkgroupInfo `json:"info"`
	CachedAt  time.Time      `json:"cached_at"`
	ExpiresAt time.Time      `json:"expires_at"`
}

// TalkgroupInfo represents processed talkgroup information
type TalkgroupInfo struct {
	ServiceType talkgroups.ServiceType `json:"service_type"`
	Color       string                 `json:"color"`
	Icon        string                 `json:"icon"`
	Title       string                 `json:"title"`
	Emoji       string                 `json:"emoji"`
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

// AISummaryCacheEntry represents a cached AI summary
type AISummaryCacheEntry struct {
	Summary   string    `json:"summary"`
	CachedAt  time.Time `json:"cached_at"`
	ExpiresAt time.Time `json:"expires_at"`
	CallCount int       `json:"call_count"`
}

// New creates a new web server instance
func New(cfg *config.Config, db *database.Database, monitor *monitoring.Monitor, talkgroups *talkgroups.Service, logger *meikoLogger.Logger) (*Server, error) {
	server := &Server{
		config:         cfg,
		db:             db,
		monitor:        monitor,
		talkgroups:     talkgroups,
		logger:         logger,
		clients:        make(map[*websocket.Conn]bool),
		broadcast:      make(chan []byte),
		timelineCache:  make(map[string]*TimelineCacheEntry),
		talkgroupCache: make(map[string]*TalkgroupCacheEntry),
		aiSummaryCache: make(map[string]*AISummaryCacheEntry),
	}

	// Initialize Fiber app
	server.app = fiber.New(fiber.Config{
		AppName:                   "Meiko Web Dashboard",
		ReadTimeout:               30 * time.Second,
		WriteTimeout:              30 * time.Second,
		IdleTimeout:               60 * time.Second, // Close idle connections
		Prefork:                   false,            // Disabled: Causes multiple SDRTrunk instances
		CaseSensitive:             false,            // URLs are case-insensitive
		StrictRouting:             false,            // Trailing slashes are ignored
		DisableKeepalive:          false,            // Keep connections alive for better performance
		DisableDefaultDate:        false,            // Include Date header
		DisableDefaultContentType: false,            // Include Content-Type header
		DisableHeaderNormalizing:  false,            // Normalize headers
		ServerHeader:              "Meiko",          // Custom server header
		CompressedFileSuffix:      ".meiko.gz",      // Custom compressed file suffix
		ReduceMemoryUsage:         true,             // Optimize memory usage
		Concurrency:               256 * 1024,       // Max concurrent connections
		BodyLimit:                 4 * 1024 * 1024,  // 4MB body limit
		EnableTrustedProxyCheck:   false,            // Skip proxy checks for performance
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

	// Start cache cleanup routine
	go server.cacheCleanupRoutine()

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

	// Live streaming endpoints
	api.Get("/live/stream", s.getLiveStream)
	api.Get("/live/status", s.getLiveStatus)

	// Debug endpoints (for development)
	api.Post("/debug/broadcast-latest", s.debugBroadcastLatest)

	// AI Summary endpoints (requires Gemini)
	api.Post("/summary/generate", s.generateSummary)

	// Timeline-specific summary endpoints
	api.Get("/timeline/summaries/:date", s.getTimelineSummaries)
	api.Get("/timeline/summary/:date/:hour", s.getHourlySummary)
	api.Post("/timeline/summary/generate", s.generateTimelineSummary)

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

	// Create cache key
	cacheKey := fmt.Sprintf("timeline_%s_%d", startOfDay.Format("2006-01-02"), limit)

	// Check cache first
	s.timelineCacheMu.RLock()
	if cached, exists := s.timelineCache[cacheKey]; exists && time.Now().Before(cached.ExpiresAt) {
		s.timelineCacheMu.RUnlock()
		response := TimelineResponse{
			Events:  cached.Events,
			HasMore: len(cached.Events) >= limit,
		}
		return c.JSON(response)
	}
	s.timelineCacheMu.RUnlock()

	events, err := s.buildTimelineEvents(&startOfDay, &endOfDay, limit)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error":   "Failed to fetch timeline events",
			"details": err.Error(),
		})
	}

	// Cache the results (5 minute cache for today, 1 hour for past days)
	cacheExpiry := 5 * time.Minute
	if startOfDay.Before(time.Now().Truncate(24 * time.Hour)) {
		cacheExpiry = 1 * time.Hour // Past days can be cached longer
	}

	s.timelineCacheMu.Lock()
	s.timelineCache[cacheKey] = &TimelineCacheEntry{
		Events:    events,
		CachedAt:  time.Now(),
		ExpiresAt: time.Now().Add(cacheExpiry),
	}
	s.timelineCacheMu.Unlock()

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

	// Create cache key
	cacheKey := fmt.Sprintf("timeline_%s_%d", dateParam, limit)

	// Check cache first
	s.timelineCacheMu.RLock()
	if cached, exists := s.timelineCache[cacheKey]; exists && time.Now().Before(cached.ExpiresAt) {
		s.timelineCacheMu.RUnlock()
		s.logger.Debug("Timeline cache hit", "date", dateParam, "events", len(cached.Events))
		response := TimelineResponse{
			Events:  cached.Events,
			HasMore: len(cached.Events) >= limit,
		}
		return c.JSON(response)
	}
	s.timelineCacheMu.RUnlock()

	log.Printf("Timeline request for %s (from %s to %s) with limit %d", dateParam, startOfDay.Format("2006-01-02 15:04:05"), endOfDay.Format("2006-01-02 15:04:05"), limit)

	events, err := s.buildTimelineEvents(&startOfDay, &endOfDay, limit)
	if err != nil {
		log.Printf("Failed to build timeline events for %s: %v", dateParam, err)
		return c.Status(500).JSON(fiber.Map{
			"error":   "Failed to fetch timeline events",
			"details": err.Error(),
		})
	}

	log.Printf("Timeline response for %s: %d events", dateParam, len(events))

	// Cache the results (longer cache for past dates, shorter for today/future)
	cacheExpiry := 1 * time.Hour // Default 1 hour
	now := time.Now()
	if startOfDay.Before(now.Truncate(24 * time.Hour)) {
		// Past days can be cached longer since they won't change
		cacheExpiry = 4 * time.Hour
	} else if startOfDay.Equal(now.Truncate(24 * time.Hour)) {
		// Today gets shorter cache since it's actively changing
		cacheExpiry = 5 * time.Minute
	}

	s.timelineCacheMu.Lock()
	s.timelineCache[cacheKey] = &TimelineCacheEntry{
		Events:    events,
		CachedAt:  time.Now(),
		ExpiresAt: time.Now().Add(cacheExpiry),
	}
	s.timelineCacheMu.Unlock()

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
	// Use a higher multiplier to ensure we get enough calls for the full day
	callLimit := limit * 3
	if callLimit < 500 {
		callLimit = 500 // Ensure we get a good amount of data for a full day
	}

	calls, err := s.db.GetCallRecords(start, end, "", callLimit, 0)
	if err != nil {
		return nil, err
	}

	log.Printf("Retrieved %d calls for timeline between %s and %s", len(calls), start.Format("2006-01-02 15:04:05"), end.Format("2006-01-02 15:04:05"))

	// Convert calls to timeline events using cached talkgroup processing
	for _, call := range calls {
		// Use cached talkgroup information for better performance
		talkgroupInfo := s.getCachedTalkgroupInfo(call.TalkgroupID, call.TalkgroupGroup)

		event := TimelineEvent{
			ID:        fmt.Sprintf("call_%d", call.ID),
			Type:      "call",
			Timestamp: call.Timestamp,
			Title:     talkgroupInfo.Title,
			Icon:      talkgroupInfo.Icon,
			Color:     talkgroupInfo.Color,
			Data: map[string]interface{}{
				"talkgroup":    call.TalkgroupAlias,
				"frequency":    call.Frequency,
				"duration":     call.Duration,
				"call_id":      call.ID,
				"service_type": string(talkgroupInfo.ServiceType),
			},
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

	// Sort events by timestamp (newest first) using efficient built-in sort
	sort.Slice(events, func(i, j int) bool {
		return events[i].Timestamp.After(events[j].Timestamp)
	})

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

	s.mu.Lock()
	s.clients[c] = true
	clientCount := len(s.clients)
	s.mu.Unlock()

	s.logger.Info("WebSocket client connected", "total_clients", clientCount)

	// Send initial status
	status := fiber.Map{
		"type":      "status",
		"connected": true,
		"timestamp": time.Now(),
	}
	if err := c.WriteJSON(status); err != nil {
		s.logger.Error("Failed to send initial status", "error", err)
	}

	// Read messages from client (though we don't expect many)
	go func() {
		defer func() {
			s.mu.Lock()
			delete(s.clients, c)
			clientCount := len(s.clients)
			s.mu.Unlock()
			s.logger.Info("WebSocket client disconnected", "total_clients", clientCount)
			c.Close()
		}()

		for {
			_, _, err := c.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					s.logger.Warn("WebSocket read error", "error", err)
				}
				break
			}
		}
	}()

	// Listen for broadcast messages
	for {
		select {
		case message := <-s.broadcast:
			s.logger.Debug("Broadcasting message to WebSocket clients", "message_size", len(message), "total_clients", clientCount)

			s.mu.Lock()
			activeClients := len(s.clients)
			sentCount := 0
			for client := range s.clients {
				if err := client.WriteMessage(websocket.TextMessage, message); err != nil {
					s.logger.Warn("Failed to send message to WebSocket client", "error", err)
					delete(s.clients, client)
					client.Close()
				} else {
					sentCount++
				}
			}
			s.mu.Unlock()

			s.logger.Debug("WebSocket broadcast completed", "sent_to", sentCount, "total_clients", activeClients)

		case <-time.After(30 * time.Second):
			// Ping to keep connection alive
			if err := c.WriteMessage(websocket.PingMessage, nil); err != nil {
				s.logger.Warn("Failed to send ping", "error", err)
				return
			}
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
	// Invalidate timeline cache to ensure fresh data
	s.InvalidateTimelineCache()

	s.logger.Info("Broadcasting new call via WebSocket",
		"call_id", call.ID,
		"filename", call.Filename,
		"talkgroup", call.TalkgroupAlias,
		"connected_clients", len(s.clients))

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

	// Enhanced data for live scanner
	enhancedData := fiber.Map{
		"type":      "new_call",
		"data":      apiCall,
		"timestamp": time.Now(),
		"live_scanner": fiber.Map{
			"should_auto_play": true,
			"waveform_data":    generateSampleWaveformData(call.Duration),
			"frequency_info":   s.getFrequencyInfo(call.Frequency),
		},
	}

	data, err := json.Marshal(enhancedData)
	if err != nil {
		s.logger.Error("Failed to marshal new call data for WebSocket", "error", err)
		return
	}

	s.logger.Debug("WebSocket message prepared", "data_size", len(data), "message_type", "new_call")

	select {
	case s.broadcast <- data:
		s.logger.Debug("New call message sent to broadcast channel", "call_id", call.ID)
	default:
		s.logger.Warn("Broadcast channel full, skipping new call message", "call_id", call.ID)
	}
}

// BroadcastLiveScannerEvent sends live scanner specific events
func (s *Server) BroadcastLiveScannerEvent(eventType string, eventData interface{}) {
	data, err := json.Marshal(fiber.Map{
		"type":      "live_scanner_event",
		"event":     eventType,
		"data":      eventData,
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

// generateSampleWaveformData creates sample waveform data for visualization
func generateSampleWaveformData(duration int) []float64 {
	// Generate realistic-looking waveform data
	dataPoints := 100
	waveform := make([]float64, dataPoints)

	for i := 0; i < dataPoints; i++ {
		// Create varying amplitude based on position
		progress := float64(i) / float64(dataPoints)

		// Peak in the middle, tapering off at ends
		envelope := 1.0 - (2.0*progress-1.0)*(2.0*progress-1.0)

		// Add some randomness
		randomComponent := (rand.Float64() - 0.5) * 0.3

		// Combine components
		waveform[i] = envelope * (0.7 + randomComponent)
	}

	return waveform
}

// getFrequencyInfo returns information about a frequency
func (s *Server) getFrequencyInfo(frequency string) fiber.Map {
	// Basic frequency info - could be enhanced with frequency database
	return fiber.Map{
		"frequency":       frequency,
		"description":     "Emergency Services",
		"is_encrypted":    false,
		"signal_strength": 85 + int(rand.Float64()*15), // Simulated signal strength
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
	// Use the enhanced timeline summary prompt for better results
	return s.buildTimelineSummaryPrompt(calls, customPrompt)
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

// getLiveStream provides real-time audio streaming status
func (s *Server) getLiveStream(c *fiber.Ctx) error {
	// Get recent calls for live streaming
	now := time.Now()
	since := now.Add(-5 * time.Minute) // Last 5 minutes

	calls, err := s.db.GetCallRecords(&since, &now, "", 10, 0)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "Failed to fetch recent calls",
		})
	}

	// Convert to API format
	recentCalls := make([]CallRecord, len(calls))
	for i, call := range calls {
		recentCalls[i] = CallRecord{
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
		"status":             "active",
		"recent_calls":       recentCalls,
		"timestamp":          now,
		"active_frequencies": s.getActiveFrequencies(),
	})
}

// getLiveStatus provides current live streaming status
func (s *Server) getLiveStatus(c *fiber.Ctx) error {
	stats := s.monitor.GetCurrentStats()

	// Get latest call
	now := time.Now()
	since := now.Add(-1 * time.Hour)

	calls, err := s.db.GetCallRecords(&since, &now, "", 1, 0)
	var lastCall *CallRecord
	if err == nil && len(calls) > 0 {
		call := calls[0]
		lastCall = &CallRecord{
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
		"is_active":         true,
		"connected_clients": len(s.clients),
		"system_stats":      stats,
		"last_call":         lastCall,
		"timestamp":         now,
	})
}

// getActiveFrequencies returns currently active frequencies
func (s *Server) getActiveFrequencies() []string {
	// Get frequencies from recent calls (last hour)
	now := time.Now()
	since := now.Add(-1 * time.Hour)

	calls, err := s.db.GetCallRecords(&since, &now, "", 100, 0)
	if err != nil {
		return []string{}
	}

	frequencyMap := make(map[string]bool)
	for _, call := range calls {
		if call.Frequency != "" {
			frequencyMap[call.Frequency] = true
		}
	}

	frequencies := make([]string, 0, len(frequencyMap))
	for freq := range frequencyMap {
		frequencies = append(frequencies, freq)
	}

	return frequencies
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

	// Generate summary using cached AI system
	cacheKey := fmt.Sprintf("auto_summary_%s", today.Format("2006-01-02"))
	summaryText := s.getCachedAISummary(cacheKey, calls, "Provide a concise daily summary of radio communication activity")

	if summaryText == "" {
		log.Printf("Failed to generate auto summary")
		return
	}

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

// debugBroadcastLatest handles the debug endpoint to manually test WebSocket broadcasting with the most recent call
func (s *Server) debugBroadcastLatest(c *fiber.Ctx) error {
	// Get the most recent call
	call, err := s.db.GetMostRecentCall()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "Failed to fetch the most recent call",
		})
	}

	// Broadcast the new call
	s.BroadcastNewCall(call)

	return c.JSON(fiber.Map{
		"message": "Most recent call broadcasted successfully",
	})
}

// getTimelineSummaries returns AI summaries for timeline integration
func (s *Server) getTimelineSummaries(c *fiber.Ctx) error {
	dateStr := c.Params("date")
	if dateStr == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Date parameter required"})
	}

	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid date format"})
	}

	// Get summaries for each hour of the day
	startOfDay := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	summaries := make(map[int]fiber.Map)

	for hour := 0; hour < 24; hour++ {
		hourStart := startOfDay.Add(time.Duration(hour) * time.Hour)
		hourEnd := hourStart.Add(time.Hour)

		calls, err := s.db.GetCallRecords(&hourStart, &hourEnd, "", 50, 0)
		if err != nil || len(calls) == 0 {
			continue
		}

		// Generate summary for this hour if we have enough activity
		if len(calls) >= 3 { // Only generate summaries for hours with significant activity
			summary := s.generateHourSummary(calls, date, hour)
			if summary != "" {
				summaries[hour] = fiber.Map{
					"hour":         hour,
					"summary":      summary,
					"call_count":   len(calls),
					"time_range":   fmt.Sprintf("%02d:00-%02d:59", hour, hour),
					"generated_at": time.Now(),
					"categories":   s.categorizeHourActivity(calls),
				}
			}
		}
	}

	return c.JSON(fiber.Map{
		"date":         dateStr,
		"summaries":    summaries,
		"generated_at": time.Now(),
	})
}

// getHourlySummary returns a specific hour's summary
func (s *Server) getHourlySummary(c *fiber.Ctx) error {
	dateStr := c.Params("date")
	hourStr := c.Params("hour")

	if dateStr == "" || hourStr == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Date and hour parameters required"})
	}

	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid date format"})
	}

	hour, err := strconv.Atoi(hourStr)
	if err != nil || hour < 0 || hour > 23 {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid hour (0-23)"})
	}

	startOfDay := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	hourStart := startOfDay.Add(time.Duration(hour) * time.Hour)
	hourEnd := hourStart.Add(time.Hour)

	calls, err := s.db.GetCallRecords(&hourStart, &hourEnd, "", 100, 0)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch calls"})
	}

	if len(calls) == 0 {
		return c.JSON(fiber.Map{
			"hour":       hour,
			"summary":    "No activity during this hour",
			"call_count": 0,
			"time_range": fmt.Sprintf("%02d:00-%02d:59", hour, hour),
			"categories": []string{},
		})
	}

	summary := s.generateHourSummary(calls, date, hour)
	categories := s.categorizeHourActivity(calls)

	return c.JSON(fiber.Map{
		"hour":         hour,
		"summary":      summary,
		"call_count":   len(calls),
		"time_range":   fmt.Sprintf("%02d:00-%02d:59", hour, hour),
		"generated_at": time.Now(),
		"categories":   categories,
		"calls":        s.buildCallSummaries(calls),
	})
}

// generateTimelineSummary generates a custom summary for timeline
func (s *Server) generateTimelineSummary(c *fiber.Ctx) error {
	var req struct {
		StartTime string `json:"start_time"`
		EndTime   string `json:"end_time"`
		Prompt    string `json:"prompt,omitempty"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	startTime, err := time.Parse(time.RFC3339, req.StartTime)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid start_time format"})
	}

	endTime, err := time.Parse(time.RFC3339, req.EndTime)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid end_time format"})
	}

	calls, err := s.db.GetCallRecords(&startTime, &endTime, "", 100, 0)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch calls"})
	}

	if len(calls) == 0 {
		return c.JSON(fiber.Map{
			"summary":    "No radio activity detected during this time period",
			"call_count": 0,
			"time_range": fmt.Sprintf("%s to %s", startTime.Format("15:04"), endTime.Format("15:04")),
			"categories": []string{},
		})
	}

	prompt := req.Prompt
	if prompt == "" {
		prompt = "Provide a detailed timeline summary of radio communication activity, highlighting significant events and patterns"
	}

	summary := s.generateCustomSummary(calls, prompt)
	categories := s.categorizeHourActivity(calls)

	return c.JSON(fiber.Map{
		"summary":      summary,
		"call_count":   len(calls),
		"time_range":   fmt.Sprintf("%s to %s", startTime.Format("15:04"), endTime.Format("15:04")),
		"generated_at": time.Now(),
		"categories":   categories,
	})
}

// generateHourSummary generates a cached summary for a specific hour
func (s *Server) generateHourSummary(calls []*database.CallRecord, date time.Time, hour int) string {
	if s.gemini == nil || len(calls) == 0 {
		return ""
	}

	// Create cache key based on date and hour
	cacheKey := fmt.Sprintf("hour_summary_%s_%02d", date.Format("2006-01-02"), hour)
	promptSuffix := fmt.Sprintf("Analyze radio communications for hour %02d:00-%02d:59", hour, hour)

	return s.getCachedAISummary(cacheKey, calls, promptSuffix)
}

// generateCustomSummary generates a cached custom summary with specific prompt
func (s *Server) generateCustomSummary(calls []*database.CallRecord, customPrompt string) string {
	if s.gemini == nil || len(calls) == 0 {
		return ""
	}

	// Create cache key based on time range and prompt hash
	startTime := calls[0].Timestamp
	endTime := calls[len(calls)-1].Timestamp
	cacheKey := fmt.Sprintf("custom_summary_%s_%s_%d",
		startTime.Format("2006-01-02-15"),
		endTime.Format("2006-01-02-15"),
		len(customPrompt)) // Simple hash based on prompt length

	return s.getCachedAISummary(cacheKey, calls, customPrompt)
}

// categorizeHourActivity categorizes the types of activity in an hour
func (s *Server) categorizeHourActivity(calls []*database.CallRecord) []string {
	categories := make(map[string]bool)

	for _, call := range calls {
		// Categorize by talkgroup
		if call.TalkgroupGroup != "" {
			switch strings.ToUpper(call.TalkgroupGroup) {
			case "POLICE", "LAW ENFORCEMENT":
				categories["POLICE"] = true
			case "FIRE", "FIRE DEPARTMENT":
				categories["FIRE"] = true
			case "EMS", "MEDICAL", "AMBULANCE":
				categories["EMS"] = true
			case "PUBLIC WORKS", "UTILITIES":
				categories["PUBLIC_WORKS"] = true
			case "EMERGENCY":
				categories["EMERGENCY"] = true
			default:
				categories["OTHER"] = true
			}
		}

		// Categorize by transcription content
		if call.Transcription != "" {
			text := strings.ToUpper(call.Transcription)
			if strings.Contains(text, "EMERGENCY") || strings.Contains(text, "CODE") {
				categories["EMERGENCY"] = true
			}
			if strings.Contains(text, "TRAFFIC") || strings.Contains(text, "ACCIDENT") {
				categories["TRAFFIC"] = true
			}
			if strings.Contains(text, "MEDICAL") || strings.Contains(text, "AMBULANCE") {
				categories["MEDICAL"] = true
			}
		}
	}

	result := make([]string, 0, len(categories))
	for category := range categories {
		result = append(result, category)
	}

	return result
}

// buildCallSummaries creates brief summaries of individual calls
func (s *Server) buildCallSummaries(calls []*database.CallRecord) []fiber.Map {
	summaries := make([]fiber.Map, 0, len(calls))

	for _, call := range calls {
		summary := fiber.Map{
			"id":        call.ID,
			"timestamp": call.Timestamp,
			"duration":  call.Duration,
			"talkgroup": call.TalkgroupAlias,
			"frequency": call.Frequency,
		}

		// Add brief transcription
		if call.Transcription != "" {
			if len(call.Transcription) > 100 {
				summary["brief"] = call.Transcription[:100] + "..."
			} else {
				summary["brief"] = call.Transcription
			}
		}

		summaries = append(summaries, summary)
	}

	return summaries
}

// buildTimelineSummaryPrompt builds an enhanced prompt for timeline summaries
func (s *Server) buildTimelineSummaryPrompt(calls []*database.CallRecord, customPrompt string) string {
	prompt := `You are analyzing radio communications from an emergency services scanner system. The data contains police, fire, EMS, and other emergency service communications.

Context: This is real radio traffic data with timestamps, talkgroups (radio channels), frequencies, and transcriptions of audio communications.

Instructions:
- Provide a concise but informative summary
- Identify patterns, significant events, and activity levels
- Categorize types of incidents (police, fire, medical, traffic, etc.)
- Note any unusual or noteworthy communications
- Keep the summary focused and professional
- Use clear, accessible language for emergency service communications

`

	if customPrompt != "" {
		prompt += customPrompt + "\n\n"
	}

	prompt += "Radio Communications Data:\n"
	for _, call := range calls {
		timeStr := call.Timestamp.Format("15:04:05")
		talkgroup := call.TalkgroupAlias
		if talkgroup == "" {
			talkgroup = call.TalkgroupID
		}

		transcription := call.Transcription
		if transcription == "" {
			transcription = "[No transcription available]"
		}

		prompt += fmt.Sprintf("• %s [%s] %s (%ds): %s\n",
			timeStr,
			talkgroup,
			call.Frequency,
			call.Duration,
			transcription)
	}

	prompt += "\nAnalysis Requirements:\n"
	prompt += "- Summarize key activities and patterns\n"
	prompt += "- Identify service types involved (police, fire, EMS, utilities, etc.)\n"
	prompt += "- Note significant incidents or unusual activity\n"
	prompt += "- Provide context about communication volume and timing\n"
	prompt += "- Keep response concise but informative (2-4 sentences)\n"

	return prompt
}

// getCachedTalkgroupInfo returns cached talkgroup information or processes and caches it
func (s *Server) getCachedTalkgroupInfo(talkgroupID, talkgroupGroup string) *TalkgroupInfo {
	cacheKey := fmt.Sprintf("%s_%s", talkgroupID, talkgroupGroup)

	// Check cache first
	s.talkgroupCacheMu.RLock()
	if cached, exists := s.talkgroupCache[cacheKey]; exists && time.Now().Before(cached.ExpiresAt) {
		s.talkgroupCacheMu.RUnlock()
		return cached.Info
	}
	s.talkgroupCacheMu.RUnlock()

	// Process talkgroup information
	info := &TalkgroupInfo{
		ServiceType: talkgroups.ServiceOther,
		Color:       "#3b82f6",
		Icon:        "phone",
		Title:       fmt.Sprintf("Call on %s", talkgroupID),
	}

	if s.talkgroups != nil {
		deptInfo := s.talkgroups.GetDepartmentInfo(talkgroupID)
		info.ServiceType = deptInfo.Type
		info.Color = deptInfo.Color
		info.Emoji = deptInfo.Emoji

		// Enhanced processing if TalkgroupGroup contains emoji
		if talkgroupGroup != "" && talkgroupGroup != "Unknown Department" {
			for st, dept := range s.talkgroups.GetServiceTypes() {
				if strings.Contains(talkgroupGroup, dept.Emoji) {
					deptInfo = dept
					info.ServiceType = st
					break
				}
			}
			info.Title = fmt.Sprintf("Call from %s", talkgroupGroup)
		} else {
			talkgroupInfo := s.talkgroups.GetTalkgroupInfo(talkgroupID)
			info.Title = fmt.Sprintf("Call from %s %s", deptInfo.Emoji, talkgroupInfo.Group)
		}

		// Set icon based on service type
		switch info.ServiceType {
		case talkgroups.ServicePolice:
			info.Icon = "shield-alt"
		case talkgroups.ServiceFire:
			info.Icon = "fire"
		case talkgroups.ServiceEMS:
			info.Icon = "ambulance"
		case talkgroups.ServiceEmergency:
			info.Icon = "exclamation-triangle"
		case talkgroups.ServicePublicWorks:
			info.Icon = "tools"
		case talkgroups.ServiceEducation:
			info.Icon = "graduation-cap"
		case talkgroups.ServiceAirport:
			info.Icon = "plane"
		case talkgroups.ServiceEvents:
			info.Icon = "broadcast-tower"
		default:
			info.Icon = "phone"
		}
	}

	// Cache the result (cache for 1 hour since talkgroup info doesn't change often)
	s.talkgroupCacheMu.Lock()
	s.talkgroupCache[cacheKey] = &TalkgroupCacheEntry{
		Info:      info,
		CachedAt:  time.Now(),
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}
	s.talkgroupCacheMu.Unlock()

	return info
}

// cacheCleanupRoutine cleans up expired cache entries
func (s *Server) cacheCleanupRoutine() {
	ticker := time.NewTicker(30 * time.Minute) // Clean up every 30 minutes
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.cleanUpExpiredCacheEntries()

			// Reset AI error count periodically to allow recovery
			s.aiCallMu.Lock()
			if s.aiErrorCount > 0 {
				s.logger.Debug("Resetting AI error count", "previous_count", s.aiErrorCount)
				s.aiErrorCount = 0
			}
			s.aiCallMu.Unlock()
		}
	}
}

// cleanUpExpiredCacheEntries removes expired cache entries
func (s *Server) cleanUpExpiredCacheEntries() {
	now := time.Now()

	cleanedTimeline := 0
	cleanedTalkgroup := 0
	cleanedAISummary := 0

	s.timelineCacheMu.Lock()
	for key, entry := range s.timelineCache {
		if now.After(entry.ExpiresAt) {
			delete(s.timelineCache, key)
			cleanedTimeline++
		}
	}
	s.timelineCacheMu.Unlock()

	s.talkgroupCacheMu.Lock()
	for key, entry := range s.talkgroupCache {
		if now.After(entry.ExpiresAt) {
			delete(s.talkgroupCache, key)
			cleanedTalkgroup++
		}
	}
	s.talkgroupCacheMu.Unlock()

	s.aiSummaryCacheMu.Lock()
	for key, entry := range s.aiSummaryCache {
		if now.After(entry.ExpiresAt) {
			delete(s.aiSummaryCache, key)
			cleanedAISummary++
		}
	}
	s.aiSummaryCacheMu.Unlock()

	if cleanedTimeline > 0 || cleanedTalkgroup > 0 || cleanedAISummary > 0 {
		s.logger.Debug("Cache cleanup completed",
			"timeline_cleaned", cleanedTimeline,
			"talkgroup_cleaned", cleanedTalkgroup,
			"ai_summary_cleaned", cleanedAISummary)
	}
}

// InvalidateTimelineCache invalidates timeline cache for today to ensure fresh data
func (s *Server) InvalidateTimelineCache() {
	today := time.Now().Format("2006-01-02")

	s.timelineCacheMu.Lock()
	for key := range s.timelineCache {
		if strings.Contains(key, today) {
			delete(s.timelineCache, key)
		}
	}
	s.timelineCacheMu.Unlock()

	// Also invalidate AI summaries for today
	s.aiSummaryCacheMu.Lock()
	for key := range s.aiSummaryCache {
		if strings.Contains(key, today) {
			delete(s.aiSummaryCache, key)
		}
	}
	s.aiSummaryCacheMu.Unlock()

	s.logger.Debug("Timeline and AI summary cache invalidated for today", "date", today)
}

// getCachedAISummary returns a cached AI summary or generates and caches a new one
func (s *Server) getCachedAISummary(cacheKey string, calls []*database.CallRecord, promptSuffix string) string {
	// Check cache first
	s.aiSummaryCacheMu.RLock()
	if cached, exists := s.aiSummaryCache[cacheKey]; exists && time.Now().Before(cached.ExpiresAt) {
		s.aiSummaryCacheMu.RUnlock()
		s.logger.Debug("AI summary cache hit", "key", cacheKey, "call_count", cached.CallCount)
		return cached.Summary
	}
	s.aiSummaryCacheMu.RUnlock()

	// Generate new summary if not cached or expired
	s.logger.Info("Generating new AI summary", "key", cacheKey, "call_count", len(calls))

	if s.gemini == nil || len(calls) == 0 {
		return ""
	}

	// Rate limiting check - prevent too many rapid API calls
	s.aiCallMu.Lock()
	timeSinceLastCall := time.Since(s.lastAICall)
	if timeSinceLastCall < 3*time.Second {
		s.aiCallMu.Unlock()
		s.logger.Warn("AI API rate limit - too many rapid calls", "key", cacheKey, "time_since_last", timeSinceLastCall)
		return ""
	}

	// Check error count - if too many recent errors, back off
	if s.aiErrorCount > 5 {
		s.aiCallMu.Unlock()
		s.logger.Warn("AI API error threshold exceeded - backing off", "key", cacheKey, "error_count", s.aiErrorCount)
		return ""
	}

	s.lastAICall = time.Now()
	s.aiRequestCount++
	s.aiCallMu.Unlock()

	prompt := s.buildTimelineSummaryPrompt(calls, promptSuffix)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	model := s.gemini.GenerativeModel(s.config.Web.Gemini.Model)
	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		s.aiCallMu.Lock()
		s.aiErrorCount++
		s.aiCallMu.Unlock()
		s.logger.Error("Failed to generate AI summary", "error", err, "key", cacheKey, "error_count", s.aiErrorCount)
		return ""
	}

	// Reset error count on success
	s.aiCallMu.Lock()
	s.aiErrorCount = 0
	s.aiCallMu.Unlock()

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		s.logger.Warn("Empty AI summary response", "key", cacheKey)
		return ""
	}

	summary := fmt.Sprintf("%v", resp.Candidates[0].Content.Parts[0])

	// Cache the result - longer cache for historical data, shorter for recent data
	cacheExpiry := 6 * time.Hour // Default 6 hours for historical summaries
	now := time.Now()

	// If this is for today, use shorter cache time
	if strings.Contains(cacheKey, now.Format("2006-01-02")) {
		cacheExpiry = 2 * time.Hour // 2 hours for today's summaries
	}

	// Cache the summary
	s.aiSummaryCacheMu.Lock()
	s.aiSummaryCache[cacheKey] = &AISummaryCacheEntry{
		Summary:   summary,
		CachedAt:  time.Now(),
		ExpiresAt: time.Now().Add(cacheExpiry),
		CallCount: len(calls),
	}
	s.aiSummaryCacheMu.Unlock()

	s.logger.Info("AI summary generated and cached", "key", cacheKey, "cache_expiry_hours", cacheExpiry.Hours())
	return summary
}
