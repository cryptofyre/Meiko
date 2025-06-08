package logger

import (
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"Meiko/internal/config"
)

// LogLevel represents the logging level
type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
)

// LogEntry represents a single log entry
type LogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"`
	Component string    `json:"component"`
	Message   string    `json:"message"`
}

// Logger provides structured logging with colors and levels
type Logger struct {
	level      LogLevel
	colors     bool
	timestamps bool
	fileLogger *log.Logger
	buffer     []LogEntry
	bufferMu   sync.RWMutex
	maxBuffer  int
}

// Color constants for terminal output
const (
	Reset  = "\033[0m"
	Red    = "\033[31m"
	Green  = "\033[32m"
	Yellow = "\033[33m"
	Blue   = "\033[34m"
	Purple = "\033[35m"
	Cyan   = "\033[36m"
	Gray   = "\033[37m"
	White  = "\033[97m"
	Bold   = "\033[1m"
)

// Spinner characters for animated status
var spinnerChars = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// New creates a new logger instance
func New(config config.LoggingConfig) *Logger {
	logger := &Logger{
		level:      parseLogLevel(config.Level),
		colors:     config.Colors,
		timestamps: config.Timestamps,
		buffer:     make([]LogEntry, 0),
		maxBuffer:  100, // Keep last 100 log entries
	}

	// Setup file logging if enabled
	if config.FileLogging.Enabled {
		file, err := os.OpenFile(config.FileLogging.Path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			log.Printf("Failed to open log file: %v", err)
		} else {
			logger.fileLogger = log.New(file, "", log.LstdFlags)
		}
	}

	return logger
}

// parseLogLevel converts string to LogLevel
func parseLogLevel(level string) LogLevel {
	switch strings.ToUpper(level) {
	case "DEBUG":
		return DEBUG
	case "INFO":
		return INFO
	case "WARN", "WARNING":
		return WARN
	case "ERROR":
		return ERROR
	default:
		return INFO
	}
}

// Debug logs a debug message
func (l *Logger) Debug(component string, message string, args ...interface{}) {
	if l.level <= DEBUG {
		l.log(DEBUG, component, message, args...)
	}
}

// Info logs an info message
func (l *Logger) Info(message string, args ...interface{}) {
	if l.level <= INFO {
		l.log(INFO, "SYSTEM", message, args...)
	}
}

// Warn logs a warning message
func (l *Logger) Warn(message string, args ...interface{}) {
	if l.level <= WARN {
		l.log(WARN, "SYSTEM", message, args...)
	}
}

// Error logs an error message
func (l *Logger) Error(message string, args ...interface{}) {
	if l.level <= ERROR {
		l.log(ERROR, "SYSTEM", message, args...)
	}
}

// Success logs a success message (special case of Info)
func (l *Logger) Success(message string, args ...interface{}) {
	if l.level <= INFO {
		l.logSuccess("SUCCESS", message, args...)
	}
}

// addToBuffer adds a log entry to the internal buffer
func (l *Logger) addToBuffer(level LogLevel, component, message string) {
	l.bufferMu.Lock()
	defer l.bufferMu.Unlock()

	entry := LogEntry{
		Timestamp: time.Now(),
		Level:     levelToString(level),
		Component: component,
		Message:   message,
	}

	l.buffer = append(l.buffer, entry)

	// Keep only the last maxBuffer entries
	if len(l.buffer) > l.maxBuffer {
		l.buffer = l.buffer[len(l.buffer)-l.maxBuffer:]
	}
}

// GetRecentLogs returns recent log entries
func (l *Logger) GetRecentLogs(limit int) []LogEntry {
	l.bufferMu.RLock()
	defer l.bufferMu.RUnlock()

	if limit <= 0 || limit > len(l.buffer) {
		limit = len(l.buffer)
	}

	// Return the last 'limit' entries
	start := len(l.buffer) - limit
	if start < 0 {
		start = 0
	}

	result := make([]LogEntry, limit)
	copy(result, l.buffer[start:])
	return result
}

// log formats and outputs a log message
func (l *Logger) log(level LogLevel, component, message string, args ...interface{}) {
	timestamp := ""
	if l.timestamps {
		timestamp = time.Now().Format("03:04:05 PM")
	}

	// Format the message
	formattedMessage := message
	if len(args) > 0 {
		// Handle key-value pairs
		for i := 0; i < len(args); i += 2 {
			if i+1 < len(args) {
				formattedMessage += fmt.Sprintf(" %s=%v", args[i], args[i+1])
			}
		}
	}

	// Add to buffer
	l.addToBuffer(level, component, formattedMessage)

	// Build the log entry
	var logEntry string
	var coloredEntry string

	if l.colors {
		switch level {
		case DEBUG:
			coloredEntry = l.buildColoredEntry(timestamp, Gray+"[DEBUG]"+Reset, component, formattedMessage)
		case INFO:
			coloredEntry = l.buildColoredEntry(timestamp, Blue+"[INFO]"+Reset, component, formattedMessage)
		case WARN:
			coloredEntry = l.buildColoredEntry(timestamp, Yellow+"[WARN]"+Reset, component, formattedMessage)
		case ERROR:
			coloredEntry = l.buildColoredEntry(timestamp, Red+"[ERROR]"+Reset, component, formattedMessage)
		}
		fmt.Println(coloredEntry)
	} else {
		logEntry = l.buildPlainEntry(timestamp, "["+levelToString(level)+"]", component, formattedMessage)
		fmt.Println(logEntry)
	}

	// Also log to file if configured
	if l.fileLogger != nil {
		plainEntry := l.buildPlainEntry(timestamp, "["+levelToString(level)+"]", component, formattedMessage)
		l.fileLogger.Println(plainEntry)
	}
}

// logSuccess formats and outputs a success message with special formatting
func (l *Logger) logSuccess(component, message string, args ...interface{}) {
	timestamp := ""
	if l.timestamps {
		timestamp = time.Now().Format("03:04:05 PM")
	}

	// Format the message
	formattedMessage := message
	if len(args) > 0 {
		for i := 0; i < len(args); i += 2 {
			if i+1 < len(args) {
				formattedMessage += fmt.Sprintf(" %s=%v", args[i], args[i+1])
			}
		}
	}

	// Add to buffer as SUCCESS level (using INFO level enum but SUCCESS string)
	entry := LogEntry{
		Timestamp: time.Now(),
		Level:     "SUCCESS",
		Component: component,
		Message:   formattedMessage,
	}

	l.bufferMu.Lock()
	l.buffer = append(l.buffer, entry)
	if len(l.buffer) > l.maxBuffer {
		l.buffer = l.buffer[len(l.buffer)-l.maxBuffer:]
	}
	l.bufferMu.Unlock()

	var logEntry string
	var coloredEntry string

	if l.colors {
		coloredEntry = l.buildColoredEntry(timestamp, Green+"[SUCCESS]"+Reset, component, formattedMessage)
		fmt.Println(coloredEntry)
	} else {
		logEntry = l.buildPlainEntry(timestamp, "[SUCCESS]", component, formattedMessage)
		fmt.Println(logEntry)
	}

	// Also log to file if configured
	if l.fileLogger != nil {
		plainEntry := l.buildPlainEntry(timestamp, "[SUCCESS]", component, formattedMessage)
		l.fileLogger.Println(plainEntry)
	}
}

// buildColoredEntry builds a colored log entry
func (l *Logger) buildColoredEntry(timestamp, levelStr, component, message string) string {
	parts := []string{}

	if timestamp != "" {
		parts = append(parts, Gray+timestamp+Reset)
	}

	parts = append(parts, levelStr)

	if component != "" && component != "INFO" && component != "WARN" && component != "ERROR" && component != "SUCCESS" {
		parts = append(parts, Cyan+component+Reset)
	}

	parts = append(parts, message)

	return strings.Join(parts, " ")
}

// buildPlainEntry builds a plain text log entry
func (l *Logger) buildPlainEntry(timestamp, levelStr, component, message string) string {
	parts := []string{}

	if timestamp != "" {
		parts = append(parts, timestamp)
	}

	parts = append(parts, levelStr)

	if component != "" && component != "INFO" && component != "WARN" && component != "ERROR" && component != "SUCCESS" {
		parts = append(parts, component)
	}

	parts = append(parts, message)

	return strings.Join(parts, " ")
}

// levelToString converts LogLevel to string
func levelToString(level LogLevel) string {
	switch level {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARN:
		return "WARN"
	case ERROR:
		return "ERROR"
	default:
		return "INFO"
	}
}

// SpinnerLogger provides animated status updates
type SpinnerLogger struct {
	logger  *Logger
	message string
	active  bool
	stopCh  chan bool
}

// NewSpinner creates a new spinner logger
func (l *Logger) NewSpinner(message string) *SpinnerLogger {
	return &SpinnerLogger{
		logger:  l,
		message: message,
		stopCh:  make(chan bool),
	}
}

// Start begins the spinner animation
func (s *SpinnerLogger) Start() {
	if !s.logger.colors {
		s.logger.Info(s.message + "...")
		return
	}

	s.active = true
	go func() {
		i := 0
		for s.active {
			select {
			case <-s.stopCh:
				return
			default:
				fmt.Printf("\r%s %s %s", s.logger.getTimestamp(), spinnerChars[i%len(spinnerChars)], s.message)
				i++
				time.Sleep(100 * time.Millisecond)
			}
		}
	}()
}

// Stop ends the spinner animation
func (s *SpinnerLogger) Stop() {
	if !s.logger.colors {
		return
	}

	s.active = false
	s.stopCh <- true
	fmt.Print("\r" + strings.Repeat(" ", len(s.message)+20) + "\r") // Clear the line
}

// getTimestamp returns a formatted timestamp if enabled
func (l *Logger) getTimestamp() string {
	if l.timestamps {
		return Gray + time.Now().Format("03:04:05 PM") + Reset
	}
	return ""
}

// InfoWithSpinner logs a message with a spinner
func (l *Logger) InfoWithSpinner(message string) *SpinnerLogger {
	spinner := l.NewSpinner(message)
	spinner.Start()
	return spinner
}

// SuccessAfterSpinner stops a spinner and logs a success message
func (l *Logger) SuccessAfterSpinner(spinner *SpinnerLogger, message string) {
	spinner.Stop()
	l.Success(message)
}

// ErrorAfterSpinner stops a spinner and logs an error message
func (l *Logger) ErrorAfterSpinner(spinner *SpinnerLogger, message string) {
	spinner.Stop()
	l.Error(message)
}
