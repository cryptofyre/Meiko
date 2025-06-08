package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"Meiko/internal/config"
	"Meiko/internal/logger"
)

// Database handles SQLite database operations
type Database struct {
	db     *sql.DB
	logger *logger.Logger
}

// CallRecord represents a call record in the database
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
	Processed       bool      `json:"processed"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// New creates a new database connection
func New(config config.DatabaseConfig, logger *logger.Logger) (*Database, error) {
	// Ensure database directory exists
	dir := filepath.Dir(config.Path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create database directory: %w", err)
		}
	}

	db, err := sql.Open("sqlite3", config.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(config.MaxOpenConns)
	db.SetMaxIdleConns(config.MaxIdleConns)

	database := &Database{
		db:     db,
		logger: logger,
	}

	// Initialize database schema
	if err := database.initSchema(); err != nil {
		return nil, fmt.Errorf("failed to initialize database schema: %w", err)
	}

	logger.Info("Database initialized successfully", "path", config.Path)
	return database, nil
}

// initSchema creates the necessary database tables
func (d *Database) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS calls (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		filename TEXT NOT NULL,
		filepath TEXT NOT NULL UNIQUE,
		timestamp DATETIME,
		duration INTEGER,
		frequency TEXT,
		talkgroup_id TEXT,
		talkgroup_alias TEXT,
		talkgroup_group TEXT,
		transcription_id INTEGER,
		transcription TEXT,
		processed BOOLEAN DEFAULT FALSE,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_calls_timestamp ON calls(timestamp);
	CREATE INDEX IF NOT EXISTS idx_calls_talkgroup_id ON calls(talkgroup_id);
	CREATE INDEX IF NOT EXISTS idx_calls_processed ON calls(processed);
	CREATE INDEX IF NOT EXISTS idx_calls_created_at ON calls(created_at);
	CREATE INDEX IF NOT EXISTS idx_calls_frequency ON calls(frequency);

	CREATE TRIGGER IF NOT EXISTS update_calls_updated_at 
		AFTER UPDATE ON calls
		BEGIN
			UPDATE calls SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
		END;
	`

	if _, err := d.db.Exec(schema); err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}

	return nil
}

// InsertCall inserts a new call record
func (d *Database) InsertCall(call *CallRecord) error {
	query := `
		INSERT INTO calls (filename, filepath, timestamp, duration, frequency, talkgroup_id, talkgroup_alias, talkgroup_group, transcription)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	result, err := d.db.Exec(query,
		call.Filename, call.Filepath, call.Timestamp, call.Duration, call.Frequency,
		call.TalkgroupID, call.TalkgroupAlias, call.TalkgroupGroup, call.Transcription)

	if err != nil {
		return fmt.Errorf("failed to insert call: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert ID: %w", err)
	}

	call.ID = int(id)
	d.logger.Debug("Database", "Inserted call record", "id", id, "file", call.Filename)
	return nil
}

// UpdateTranscription updates the transcription for a call
func (d *Database) UpdateTranscription(id int, transcription string) error {
	query := `UPDATE calls SET transcription = ? WHERE id = ?`

	result, err := d.db.Exec(query, transcription, id)
	if err != nil {
		return fmt.Errorf("failed to update transcription: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("no call found with ID %d", id)
	}

	d.logger.Debug("Database", "Updated transcription", "id", id)
	return nil
}

// MarkAsProcessed marks a call as processed
func (d *Database) MarkAsProcessed(id int) error {
	query := `UPDATE calls SET processed = TRUE WHERE id = ?`

	result, err := d.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to mark call as processed: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("no call found with ID %d", id)
	}

	d.logger.Debug("Database", "Marked call as processed", "id", id)
	return nil
}

// GetUnprocessedCalls returns calls that haven't been processed yet
func (d *Database) GetUnprocessedCalls(limit int) ([]*CallRecord, error) {
	query := `
		SELECT id, filename, filepath, timestamp, duration, frequency, talkgroup_id, 
		       talkgroup_alias, talkgroup_group, transcription_id, transcription, 
		       processed, created_at, updated_at
		FROM calls 
		WHERE processed = FALSE 
		ORDER BY created_at ASC 
		LIMIT ?
	`

	rows, err := d.db.Query(query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query unprocessed calls: %w", err)
	}
	defer rows.Close()

	var calls []*CallRecord
	for rows.Next() {
		call := &CallRecord{}
		err := rows.Scan(
			&call.ID, &call.Filename, &call.Filepath, &call.Timestamp,
			&call.Duration, &call.Frequency, &call.TalkgroupID,
			&call.TalkgroupAlias, &call.TalkgroupGroup, &call.TranscriptionID,
			&call.Transcription, &call.Processed, &call.CreatedAt, &call.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan call record: %w", err)
		}
		calls = append(calls, call)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error: %w", err)
	}

	return calls, nil
}

// GetCallByFilepath returns a call record by its filepath
func (d *Database) GetCallByFilepath(filepath string) (*CallRecord, error) {
	query := `
		SELECT id, filename, filepath, timestamp, duration, frequency, talkgroup_id, 
		       talkgroup_alias, talkgroup_group, transcription_id, transcription, 
		       processed, created_at, updated_at
		FROM calls 
		WHERE filepath = ?
	`

	call := &CallRecord{}
	err := d.db.QueryRow(query, filepath).Scan(
		&call.ID, &call.Filename, &call.Filepath, &call.Timestamp,
		&call.Duration, &call.Frequency, &call.TalkgroupID,
		&call.TalkgroupAlias, &call.TalkgroupGroup, &call.TranscriptionID,
		&call.Transcription, &call.Processed, &call.CreatedAt, &call.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("call not found")
		}
		return nil, fmt.Errorf("failed to get call by filepath: %w", err)
	}

	return call, nil
}

// GetRecentCalls returns the most recent calls
func (d *Database) GetRecentCalls(limit int) ([]*CallRecord, error) {
	query := `
		SELECT id, filename, filepath, timestamp, duration, frequency, talkgroup_id, 
		       talkgroup_alias, talkgroup_group, transcription_id, transcription, 
		       processed, created_at, updated_at
		FROM calls 
		ORDER BY timestamp DESC 
		LIMIT ?
	`

	rows, err := d.db.Query(query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query recent calls: %w", err)
	}
	defer rows.Close()

	var calls []*CallRecord
	for rows.Next() {
		call := &CallRecord{}
		err := rows.Scan(
			&call.ID, &call.Filename, &call.Filepath, &call.Timestamp,
			&call.Duration, &call.Frequency, &call.TalkgroupID,
			&call.TalkgroupAlias, &call.TalkgroupGroup, &call.TranscriptionID,
			&call.Transcription, &call.Processed, &call.CreatedAt, &call.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan call record: %w", err)
		}
		calls = append(calls, call)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error: %w", err)
	}

	return calls, nil
}

// GetCallRecords returns call records with optional filtering
func (d *Database) GetCallRecords(start, end *time.Time, talkgroupID string, limit, offset int) ([]*CallRecord, error) {
	query := `
		SELECT id, filename, filepath, timestamp, duration, frequency, talkgroup_id, 
		       talkgroup_alias, talkgroup_group, transcription_id, transcription, 
		       processed, created_at, updated_at
		FROM calls 
		WHERE 1=1
	`
	args := []interface{}{}

	if start != nil {
		query += " AND timestamp >= ?"
		args = append(args, start)
	}
	if end != nil {
		query += " AND timestamp <= ?"
		args = append(args, end)
	}
	if talkgroupID != "" {
		query += " AND talkgroup_id = ?"
		args = append(args, talkgroupID)
	}

	query += " ORDER BY timestamp DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := d.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query call records: %w", err)
	}
	defer rows.Close()

	var calls []*CallRecord
	for rows.Next() {
		call := &CallRecord{}
		err := rows.Scan(
			&call.ID, &call.Filename, &call.Filepath, &call.Timestamp,
			&call.Duration, &call.Frequency, &call.TalkgroupID,
			&call.TalkgroupAlias, &call.TalkgroupGroup, &call.TranscriptionID,
			&call.Transcription, &call.Processed, &call.CreatedAt, &call.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan call record: %w", err)
		}
		calls = append(calls, call)
	}

	return calls, nil
}

// GetCallRecord returns a single call record by ID
func (d *Database) GetCallRecord(id int) (*CallRecord, error) {
	query := `
		SELECT id, filename, filepath, timestamp, duration, frequency, talkgroup_id, 
		       talkgroup_alias, talkgroup_group, transcription_id, transcription, 
		       processed, created_at, updated_at
		FROM calls 
		WHERE id = ?
	`

	row := d.db.QueryRow(query, id)
	call := &CallRecord{}

	err := row.Scan(
		&call.ID, &call.Filename, &call.Filepath, &call.Timestamp,
		&call.Duration, &call.Frequency, &call.TalkgroupID,
		&call.TalkgroupAlias, &call.TalkgroupGroup, &call.TranscriptionID,
		&call.Transcription, &call.Processed, &call.CreatedAt, &call.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("call record with ID %d not found", id)
		}
		return nil, fmt.Errorf("failed to get call record: %w", err)
	}

	return call, nil
}

// GetMostRecentCall returns the most recent call record
func (d *Database) GetMostRecentCall() (*CallRecord, error) {
	query := `
		SELECT id, filename, filepath, timestamp, duration, frequency, talkgroup_id, 
		       talkgroup_alias, talkgroup_group, transcription_id, transcription, 
		       processed, created_at, updated_at
		FROM calls 
		ORDER BY timestamp DESC 
		LIMIT 1
	`

	row := d.db.QueryRow(query)
	call := &CallRecord{}

	err := row.Scan(
		&call.ID, &call.Filename, &call.Filepath, &call.Timestamp,
		&call.Duration, &call.Frequency, &call.TalkgroupID,
		&call.TalkgroupAlias, &call.TalkgroupGroup, &call.TranscriptionID,
		&call.Transcription, &call.Processed, &call.CreatedAt, &call.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("no call records found")
		}
		return nil, fmt.Errorf("failed to get most recent call: %w", err)
	}

	return call, nil
}

// GetCallStats returns aggregated call statistics for a time range
func (d *Database) GetCallStats(start, end *time.Time) (map[string]interface{}, error) {
	query := `
		SELECT 
			COUNT(*) as total_calls,
			AVG(duration) as avg_duration,
			SUM(duration) as total_duration,
			COUNT(DISTINCT talkgroup_id) as unique_talkgroups,
			COUNT(DISTINCT frequency) as unique_frequencies
		FROM calls 
		WHERE 1=1
	`
	args := []interface{}{}

	if start != nil {
		query += " AND timestamp >= ?"
		args = append(args, start)
	}
	if end != nil {
		query += " AND timestamp <= ?"
		args = append(args, end)
	}

	var totalCalls int64
	var avgDuration, totalDuration sql.NullFloat64
	var uniqueTalkgroups, uniqueFrequencies int64

	err := d.db.QueryRow(query, args...).Scan(
		&totalCalls, &avgDuration, &totalDuration, &uniqueTalkgroups, &uniqueFrequencies,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get call stats: %w", err)
	}

	stats := map[string]interface{}{
		"total_calls":        totalCalls,
		"avg_duration":       avgDuration.Float64,
		"total_duration":     totalDuration.Float64,
		"unique_talkgroups":  uniqueTalkgroups,
		"unique_frequencies": uniqueFrequencies,
	}

	return stats, nil
}

// GetTotalCallCount returns the total number of calls
func (d *Database) GetTotalCallCount() (int64, error) {
	var count int64
	err := d.db.QueryRow("SELECT COUNT(*) FROM calls").Scan(&count)
	return count, err
}

// GetLastCallTime returns the timestamp of the most recent call
func (d *Database) GetLastCallTime() (*time.Time, error) {
	var timestamp *time.Time
	err := d.db.QueryRow("SELECT MAX(timestamp) FROM calls").Scan(&timestamp)
	if err != nil {
		return nil, err
	}
	return timestamp, nil
}

// GetCallsToday returns the number of calls today
func (d *Database) GetCallsToday() (int64, error) {
	today := time.Now().Format("2006-01-02")
	var count int64
	err := d.db.QueryRow("SELECT COUNT(*) FROM calls WHERE DATE(timestamp) = ?", today).Scan(&count)
	return count, err
}

// GetFrequencyStats returns frequency usage statistics
func (d *Database) GetFrequencyStats() (map[string]int64, error) {
	query := "SELECT frequency, COUNT(*) FROM calls WHERE frequency IS NOT NULL GROUP BY frequency"
	rows, err := d.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stats := make(map[string]int64)
	for rows.Next() {
		var frequency string
		var count int64
		if err := rows.Scan(&frequency, &count); err != nil {
			return nil, err
		}
		stats[frequency] = count
	}

	return stats, nil
}

// GetTalkgroupStats returns talkgroup usage statistics
func (d *Database) GetTalkgroupStats() (map[string]int64, error) {
	query := "SELECT talkgroup_alias, COUNT(*) FROM calls WHERE talkgroup_alias IS NOT NULL GROUP BY talkgroup_alias"
	rows, err := d.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stats := make(map[string]int64)
	for rows.Next() {
		var talkgroup string
		var count int64
		if err := rows.Scan(&talkgroup, &count); err != nil {
			return nil, err
		}
		stats[talkgroup] = count
	}

	return stats, nil
}

// GetLifetimeStats returns comprehensive lifetime statistics
func (d *Database) GetLifetimeStats() (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Total calls
	totalCalls, _ := d.GetTotalCallCount()
	stats["total_calls"] = totalCalls

	// Total duration
	var totalDuration sql.NullFloat64
	d.db.QueryRow("SELECT SUM(duration) FROM calls").Scan(&totalDuration)
	stats["total_duration"] = totalDuration.Float64

	// Average duration
	var avgDuration sql.NullFloat64
	d.db.QueryRow("SELECT AVG(duration) FROM calls").Scan(&avgDuration)
	stats["avg_duration"] = avgDuration.Float64

	// First and last call
	var firstCall, lastCall *time.Time
	d.db.QueryRow("SELECT MIN(timestamp) FROM calls").Scan(&firstCall)
	d.db.QueryRow("SELECT MAX(timestamp) FROM calls").Scan(&lastCall)
	stats["first_call"] = firstCall
	stats["last_call"] = lastCall

	// Unique talkgroups and frequencies
	var uniqueTalkgroups, uniqueFrequencies int64
	d.db.QueryRow("SELECT COUNT(DISTINCT talkgroup_id) FROM calls").Scan(&uniqueTalkgroups)
	d.db.QueryRow("SELECT COUNT(DISTINCT frequency) FROM calls").Scan(&uniqueFrequencies)
	stats["unique_talkgroups"] = uniqueTalkgroups
	stats["unique_frequencies"] = uniqueFrequencies

	return stats, nil
}

// GetStats returns general database statistics (legacy method)
func (d *Database) GetStats() (map[string]interface{}, error) {
	return d.GetLifetimeStats()
}

// DeleteOldCalls deletes calls older than specified days
func (d *Database) DeleteOldCalls(daysOld int) (int, error) {
	cutoff := time.Now().AddDate(0, 0, -daysOld)

	result, err := d.db.Exec("DELETE FROM calls WHERE timestamp < ?", cutoff)
	if err != nil {
		return 0, fmt.Errorf("failed to delete old calls: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get affected rows: %w", err)
	}

	d.logger.Info("Database", "Deleted old calls", "count", rows, "cutoff", cutoff)
	return int(rows), nil
}

// Close closes the database connection
func (d *Database) Close() error {
	if d.db != nil {
		d.logger.Info("Database", "Closing database connection")
		return d.db.Close()
	}
	return nil
}

// Ping checks if the database connection is alive
func (d *Database) Ping() error {
	return d.db.Ping()
}

// BeginTransaction starts a new transaction
func (d *Database) BeginTransaction() (*sql.Tx, error) {
	return d.db.Begin()
}

// FileExists checks if a file has already been processed
func (d *Database) FileExists(filepath string) (bool, error) {
	var count int
	err := d.db.QueryRow("SELECT COUNT(*) FROM calls WHERE filepath = ?", filepath).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check file existence: %w", err)
	}
	return count > 0, nil
}
