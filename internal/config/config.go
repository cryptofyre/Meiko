package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the main configuration structure
type Config struct {
	SDRTrunk      SDRTrunkConfig      `yaml:"sdrtrunk"`
	Transcription TranscriptionConfig `yaml:"transcription"`
	Discord       DiscordConfig       `yaml:"discord"`
	Database      DatabaseConfig      `yaml:"database"`
	Logging       LoggingConfig       `yaml:"logging"`
	Monitoring    MonitoringConfig    `yaml:"monitoring"`
	FileMonitor   FileMonitorConfig   `yaml:"file_monitor"`
	Talkgroups    TalkgroupConfig     `yaml:"talkgroups"`
	Preflight     PreflightConfig     `yaml:"preflight"`
	Web           WebConfig           `yaml:"web"`
}

// SDRTrunkConfig contains SDRTrunk process management settings
type SDRTrunkConfig struct {
	Path           string   `yaml:"path"`
	JavaPath       string   `yaml:"java_path"`
	JVMArgs        []string `yaml:"jvm_args"`
	Args           []string `yaml:"args"`
	WorkingDir     string   `yaml:"working_dir"`
	AudioOutputDir string   `yaml:"audio_output_dir"`
	LogLevel       string   `yaml:"log_level"` // Level for SDRTrunk output: DEBUG, INFO, WARN, ERROR
}

// TranscriptionConfig contains transcription service settings
type TranscriptionConfig struct {
	Mode            string                    `yaml:"mode"`
	Local           LocalTranscriptionConfig  `yaml:"local"`
	Remote          RemoteTranscriptionConfig `yaml:"remote"`
	MinDurationSecs int                       `yaml:"min_duration_seconds"`
	MaxRetries      int                       `yaml:"max_retries"`
	BatchSize       int                       `yaml:"batch_size"`
}

// LocalTranscriptionConfig contains local transcription settings
type LocalTranscriptionConfig struct {
	WhisperScript string `yaml:"whisper_script"`
	PythonPath    string `yaml:"python_path"`
	ModelSize     string `yaml:"model_size"`
	Device        string `yaml:"device"`
	Language      string `yaml:"language"`
}

// RemoteTranscriptionConfig contains remote transcription settings
type RemoteTranscriptionConfig struct {
	Endpoint   string `yaml:"endpoint"`
	APIKey     string `yaml:"api_key"`
	Timeout    int    `yaml:"timeout"`
	MaxRetries int    `yaml:"max_retries"`
}

// DiscordConfig contains Discord integration settings
type DiscordConfig struct {
	Token         string                    `yaml:"token"`
	ChannelID     string                    `yaml:"channel_id"`
	WebhookURL    string                    `yaml:"webhook_url"`
	Notifications DiscordNotificationConfig `yaml:"notifications"`
	Monitoring    DiscordMonitoringConfig   `yaml:"monitoring"`
}

// DiscordNotificationConfig defines which events to send to Discord
type DiscordNotificationConfig struct {
	Startup        bool `yaml:"startup"`
	Shutdown       bool `yaml:"shutdown"`
	Errors         bool `yaml:"errors"`
	Transcriptions bool `yaml:"transcriptions"`
	SystemHealth   bool `yaml:"system_health"`
}

// DiscordMonitoringConfig contains Discord monitoring settings
type DiscordMonitoringConfig struct {
	Enabled            bool `yaml:"enabled"`
	UpdateChannelTopic bool `yaml:"update_channel_topic"`
	UpdateInterval     int  `yaml:"update_interval"`
}

// DatabaseConfig contains database settings
type DatabaseConfig struct {
	Path         string `yaml:"path"`
	MaxOpenConns int    `yaml:"max_open_conns"`
	MaxIdleConns int    `yaml:"max_idle_conns"`
}

// LoggingConfig contains logging settings
type LoggingConfig struct {
	Level       string            `yaml:"level"`
	Colors      bool              `yaml:"colors"`
	Timestamps  bool              `yaml:"timestamps"`
	FileLogging FileLoggingConfig `yaml:"file_logging"`
}

// FileLoggingConfig contains file logging settings
type FileLoggingConfig struct {
	Enabled    bool   `yaml:"enabled"`
	Path       string `yaml:"path"`
	MaxSizeMB  int    `yaml:"max_size_mb"`
	MaxBackups int    `yaml:"max_backups"`
}

// MonitoringConfig contains system monitoring settings
type MonitoringConfig struct {
	Enabled       bool                      `yaml:"enabled"`
	CheckInterval int                       `yaml:"check_interval"`
	Thresholds    MonitoringThresholdConfig `yaml:"thresholds"`
}

// MonitoringThresholdConfig contains monitoring thresholds
type MonitoringThresholdConfig struct {
	CPUUsage    float64 `yaml:"cpu_usage"`
	MemoryUsage float64 `yaml:"memory_usage"`
	DiskUsage   float64 `yaml:"disk_usage"`
	Temperature float64 `yaml:"temperature"`
}

// FileMonitorConfig contains file monitoring settings
type FileMonitorConfig struct {
	PollInterval    int      `yaml:"poll_interval"`
	Patterns        []string `yaml:"patterns"`
	MinFileAge      int      `yaml:"min_file_age"`
	MinCallDuration int      `yaml:"min_call_duration"`
}

// TalkgroupConfig contains talkgroup-related settings
type TalkgroupConfig struct {
	PlaylistPath string                  `yaml:"playlist_path"`
	Glossaries   TalkgroupGlossaryConfig `yaml:"glossaries"`
}

// TalkgroupGlossaryConfig contains glossary settings
type TalkgroupGlossaryConfig struct {
	Global   []string            `yaml:"global"`
	Specific map[string][]string `yaml:"specific"`
}

// PreflightConfig contains pre-flight check settings
type PreflightConfig struct {
	Enabled         bool    `yaml:"enabled"`
	CheckUSBDevices bool    `yaml:"check_usb_devices"`
	MinDiskSpaceGB  float64 `yaml:"min_disk_space_gb"`
	CheckNetwork    bool    `yaml:"check_network"`
}

// WebConfig contains web dashboard settings
type WebConfig struct {
	Enabled  bool              `yaml:"enabled"`
	Port     int               `yaml:"port"`
	Host     string            `yaml:"host"`
	TLS      WebTLSConfig      `yaml:"tls"`
	Auth     WebAuthConfig     `yaml:"auth"`
	Gemini   WebGeminiConfig   `yaml:"gemini"`
	Realtime WebRealtimeConfig `yaml:"realtime"`
}

// WebTLSConfig contains TLS settings
type WebTLSConfig struct {
	Enabled  bool   `yaml:"enabled"`
	CertFile string `yaml:"cert_file"`
	KeyFile  string `yaml:"key_file"`
}

// WebAuthConfig contains authentication settings
type WebAuthConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

// WebGeminiConfig contains Google Gemini integration settings
type WebGeminiConfig struct {
	Enabled bool   `yaml:"enabled"`
	APIKey  string `yaml:"api_key"`
	Model   string `yaml:"model"`
}

// WebRealtimeConfig contains real-time update settings
type WebRealtimeConfig struct {
	Enabled        bool `yaml:"enabled"`
	UpdateInterval int  `yaml:"update_interval"`
}

// Load reads and parses the configuration file
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Set defaults
	config.setDefaults()

	// Validate configuration
	if err := config.validate(); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	return &config, nil
}

// setDefaults sets default values for configuration fields
func (c *Config) setDefaults() {
	// SDRTrunk defaults
	if c.SDRTrunk.JavaPath == "" {
		c.SDRTrunk.JavaPath = "java"
	}
	if c.SDRTrunk.LogLevel == "" {
		c.SDRTrunk.LogLevel = "INFO" // Default to INFO level for SDRTrunk output
	}

	// Transcription defaults
	if c.Transcription.Mode == "" {
		c.Transcription.Mode = "local"
	}
	if c.Transcription.MinDurationSecs == 0 {
		c.Transcription.MinDurationSecs = 3
	}
	if c.Transcription.MaxRetries == 0 {
		c.Transcription.MaxRetries = 3
	}
	if c.Transcription.BatchSize == 0 {
		c.Transcription.BatchSize = 5
	}

	// Local transcription defaults
	if c.Transcription.Local.PythonPath == "" {
		c.Transcription.Local.PythonPath = "python"
	}
	if c.Transcription.Local.ModelSize == "" {
		c.Transcription.Local.ModelSize = "tiny"
	}
	if c.Transcription.Local.Device == "" {
		c.Transcription.Local.Device = "cpu"
	}
	if c.Transcription.Local.Language == "" {
		c.Transcription.Local.Language = "en"
	}

	// Remote transcription defaults
	if c.Transcription.Remote.Timeout == 0 {
		c.Transcription.Remote.Timeout = 30
	}
	if c.Transcription.Remote.MaxRetries == 0 {
		c.Transcription.Remote.MaxRetries = 3
	}

	// Database defaults
	if c.Database.Path == "" {
		c.Database.Path = "./meiko.db"
	}
	if c.Database.MaxOpenConns == 0 {
		c.Database.MaxOpenConns = 10
	}
	if c.Database.MaxIdleConns == 0 {
		c.Database.MaxIdleConns = 5
	}

	// Logging defaults
	if c.Logging.Level == "" {
		c.Logging.Level = "INFO"
	}

	// Monitoring defaults
	if c.Monitoring.CheckInterval == 0 {
		c.Monitoring.CheckInterval = 60
	}
	if c.Monitoring.Thresholds.CPUUsage == 0 {
		c.Monitoring.Thresholds.CPUUsage = 80.0
	}
	if c.Monitoring.Thresholds.MemoryUsage == 0 {
		c.Monitoring.Thresholds.MemoryUsage = 85.0
	}
	if c.Monitoring.Thresholds.DiskUsage == 0 {
		c.Monitoring.Thresholds.DiskUsage = 90.0
	}
	if c.Monitoring.Thresholds.Temperature == 0 {
		c.Monitoring.Thresholds.Temperature = 70.0
	}

	// File monitor defaults
	if c.FileMonitor.PollInterval == 0 {
		c.FileMonitor.PollInterval = 1000
	}
	if len(c.FileMonitor.Patterns) == 0 {
		c.FileMonitor.Patterns = []string{"*.mp3", "*.wav"}
	}
	if c.FileMonitor.MinFileAge == 0 {
		c.FileMonitor.MinFileAge = 2
	}
	if c.FileMonitor.MinCallDuration == 0 {
		c.FileMonitor.MinCallDuration = 3
	}

	// Preflight defaults
	if c.Preflight.MinDiskSpaceGB == 0 {
		c.Preflight.MinDiskSpaceGB = 1.0
	}

	// Web defaults
	if c.Web.Port == 0 {
		c.Web.Port = 8080
	}
	if c.Web.Host == "" {
		c.Web.Host = "0.0.0.0"
	}
	if c.Web.Gemini.Model == "" {
		c.Web.Gemini.Model = "gemini-1.5-flash"
	}
	if c.Web.Realtime.UpdateInterval == 0 {
		c.Web.Realtime.UpdateInterval = 1000
	}
}

// validate checks the configuration for required fields and logical consistency
func (c *Config) validate() error {
	// Validate SDRTrunk configuration
	if c.SDRTrunk.Path == "" {
		return fmt.Errorf("sdrtrunk.path is required")
	}
	if c.SDRTrunk.AudioOutputDir == "" {
		return fmt.Errorf("sdrtrunk.audio_output_dir is required")
	}

	// Validate transcription mode
	if c.Transcription.Mode != "local" && c.Transcription.Mode != "remote" {
		return fmt.Errorf("transcription.mode must be 'local' or 'remote'")
	}

	// Validate transcription configuration based on mode
	if c.Transcription.Mode == "local" {
		if c.Transcription.Local.WhisperScript == "" {
			return fmt.Errorf("transcription.local.whisper_script is required for local mode")
		}
	} else if c.Transcription.Mode == "remote" {
		if c.Transcription.Remote.Endpoint == "" {
			return fmt.Errorf("transcription.remote.endpoint is required for remote mode")
		}
	}

	// Validate Discord configuration (if enabled)
	if c.Discord.Token != "" {
		if c.Discord.ChannelID == "" && c.Discord.WebhookURL == "" {
			return fmt.Errorf("discord.channel_id or discord.webhook_url is required when Discord is enabled")
		}
	}

	// Validate file paths exist
	if _, err := os.Stat(c.SDRTrunk.Path); os.IsNotExist(err) {
		return fmt.Errorf("sdrtrunk.path does not exist: %s", c.SDRTrunk.Path)
	}

	// Validate audio output directory exists
	if _, err := os.Stat(c.SDRTrunk.AudioOutputDir); os.IsNotExist(err) {
		return fmt.Errorf("sdrtrunk.audio_output_dir does not exist: %s", c.SDRTrunk.AudioOutputDir)
	}

	return nil
}

// GetPollInterval returns the file monitor poll interval as a time.Duration
func (c *Config) GetPollInterval() time.Duration {
	return time.Duration(c.FileMonitor.PollInterval) * time.Millisecond
}

// GetMinFileAge returns the minimum file age as a time.Duration
func (c *Config) GetMinFileAge() time.Duration {
	return time.Duration(c.FileMonitor.MinFileAge) * time.Second
}

// GetCheckInterval returns the monitoring check interval as a time.Duration
func (c *Config) GetCheckInterval() time.Duration {
	return time.Duration(c.Monitoring.CheckInterval) * time.Second
}

// GetDiscordUpdateInterval returns the Discord update interval as a time.Duration
func (c *Config) GetDiscordUpdateInterval() time.Duration {
	return time.Duration(c.Discord.Monitoring.UpdateInterval) * time.Millisecond
}

// GetMinCallDuration returns the minimum call duration as a time.Duration
func (c *Config) GetMinCallDuration() time.Duration {
	return time.Duration(c.FileMonitor.MinCallDuration) * time.Second
}
