package monitoring

import (
	"context"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"

	"Meiko/internal/config"
	"Meiko/internal/discord"
	"Meiko/internal/logger"
)

// Monitor monitors system health and performance
type Monitor struct {
	config  config.MonitoringConfig
	discord *discord.Client
	logger  *logger.Logger
}

// SystemMonitor is an alias for backward compatibility
type SystemMonitor = Monitor

// SystemStats represents current system statistics
type SystemStats struct {
	CPU         float64   `json:"cpu"`
	Memory      float64   `json:"memory"`
	Disk        float64   `json:"disk"`
	Temperature float64   `json:"temperature"`
	Timestamp   time.Time `json:"timestamp"`
}

// New creates a new system monitor
func New(config config.MonitoringConfig, discord *discord.Client, logger *logger.Logger) *Monitor {
	return &Monitor{
		config:  config,
		discord: discord,
		logger:  logger,
	}
}

// Start begins system monitoring
func (m *Monitor) Start(ctx context.Context) {
	if !m.config.Enabled {
		return
	}

	go m.monitor(ctx)
}

// monitor runs the monitoring loop
func (m *Monitor) monitor(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(m.config.CheckInterval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			m.logger.Info("System monitor stopping...")
			return
		case <-ticker.C:
			m.checkSystem()
		}
	}
}

// checkSystem performs system health checks
func (m *Monitor) checkSystem() {
	stats, err := m.getSystemStats()
	if err != nil {
		m.logger.Error("Failed to get system stats", "error", err)
		return
	}

	// Check thresholds and alert if necessary
	m.checkThresholds(stats)
}

// getSystemStats retrieves current system statistics
func (m *Monitor) getSystemStats() (*SystemStats, error) {
	stats := &SystemStats{
		Timestamp: time.Now(),
	}

	// Get CPU usage
	cpuPercent, err := cpu.Percent(time.Second, false)
	if err != nil {
		return nil, err
	}
	if len(cpuPercent) > 0 {
		stats.CPU = cpuPercent[0]
	}

	// Get memory usage
	memInfo, err := mem.VirtualMemory()
	if err != nil {
		return nil, err
	}
	stats.Memory = memInfo.UsedPercent

	// Get disk usage
	diskInfo, err := disk.Usage("/")
	if err != nil {
		return nil, err
	}
	stats.Disk = diskInfo.UsedPercent

	// Temperature (placeholder - would need platform-specific implementation)
	stats.Temperature = 0.0

	return stats, nil
}

// checkThresholds checks if any thresholds are exceeded
func (m *Monitor) checkThresholds(stats *SystemStats) {
	if stats.CPU > m.config.Thresholds.CPUUsage {
		m.logger.Warn("High CPU usage detected", "usage", stats.CPU, "threshold", m.config.Thresholds.CPUUsage)
	}

	if stats.Memory > m.config.Thresholds.MemoryUsage {
		m.logger.Warn("High memory usage detected", "usage", stats.Memory, "threshold", m.config.Thresholds.MemoryUsage)
	}

	if stats.Disk > m.config.Thresholds.DiskUsage {
		m.logger.Warn("High disk usage detected", "usage", stats.Disk, "threshold", m.config.Thresholds.DiskUsage)
	}
}

// GetCurrentStats returns the current system statistics
func (m *Monitor) GetCurrentStats() *SystemStats {
	stats, err := m.getSystemStats()
	if err != nil {
		m.logger.Error("Failed to get current stats", "error", err)
		return &SystemStats{
			CPU:         0,
			Memory:      0,
			Disk:        0,
			Temperature: 0,
			Timestamp:   time.Now(),
		}
	}
	return stats
}

// GetSystemInfo returns system information
func (m *Monitor) GetSystemInfo() map[string]interface{} {
	return map[string]interface{}{
		"os":           "Unknown",
		"architecture": "Unknown",
		"hostname":     "Unknown",
		"uptime":       0,
	}
}
