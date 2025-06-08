package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"Meiko/internal/config"
	"Meiko/internal/database"
	"Meiko/internal/discord"
	"Meiko/internal/logger"
	"Meiko/internal/monitoring"
	"Meiko/internal/preflight"
	"Meiko/internal/processor"
	"Meiko/internal/sdrtrunk"
	"Meiko/internal/talkgroups"
	"Meiko/internal/transcription"
	"Meiko/internal/watcher"
	"Meiko/internal/web"
)

const (
	AppName    = "Meiko"
	AppVersion = "1.0.0"
)

type Application struct {
	config      *config.Config
	logger      *logger.Logger
	db          *database.Database
	talkgroups  *talkgroups.Service
	discord     *discord.Client
	sdrtrunk    *sdrtrunk.Manager
	watcher     *watcher.FileWatcher
	transcriber *transcription.Service
	processor   *processor.CallProcessor
	monitor     *monitoring.SystemMonitor
	webServer   *web.Server
	ctx         context.Context
	cancel      context.CancelFunc
}

func main() {
	fmt.Printf("🎤 %s v%s - Unified SDRTrunk & Transcription System\n", AppName, AppVersion)
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	app := &Application{}

	// Setup graceful shutdown
	app.ctx, app.cancel = context.WithCancel(context.Background())

	// Initialize the application
	if err := app.initialize(); err != nil {
		fmt.Printf("❌ Failed to initialize: %v\n", err)
		os.Exit(1)
	}

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start the application
	if err := app.start(); err != nil {
		app.logger.Error("Failed to start application", "error", err)
		os.Exit(1)
	}

	// Wait for shutdown signal
	<-sigChan
	app.logger.Info("Shutdown signal received, gracefully shutting down...")

	// Shutdown the application
	app.shutdown()
}

func (app *Application) initialize() error {
	var err error

	// Load configuration
	app.config, err = config.Load("config.yaml")
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Initialize logger
	app.logger = logger.New(app.config.Logging)
	app.logger.Info("Configuration loaded successfully")

	// Run pre-flight checks
	if app.config.Preflight.Enabled {
		app.logger.Info("Running pre-flight checks...")
		checker := preflight.New(app.config, app.logger)
		if err := checker.RunAll(); err != nil {
			return fmt.Errorf("pre-flight checks failed: %w", err)
		}
		app.logger.Success("All pre-flight checks passed ✓")
	}

	// Initialize database
	app.db, err = database.New(app.config.Database, app.logger)
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}

	// Initialize talkgroups service
	app.talkgroups = talkgroups.New(app.config, app.logger)

	// Initialize Discord client
	if app.config.Discord.Token != "" {
		app.discord, err = discord.New(app.config.Discord, app.logger, app.talkgroups)
		if err != nil {
			app.logger.Warn("Failed to initialize Discord client", "error", err)
		}
	}

	// Initialize SDRTrunk manager
	app.sdrtrunk = sdrtrunk.New(app.config.SDRTrunk, app.logger)

	// Initialize transcription service
	app.transcriber, err = transcription.New(app.config.Transcription, app.logger)
	if err != nil {
		return fmt.Errorf("failed to initialize transcription service: %w", err)
	}

	// Initialize file watcher
	app.watcher, err = watcher.New(app.config.SDRTrunk.AudioOutputDir, app.config.FileMonitor, app.logger)
	if err != nil {
		return fmt.Errorf("failed to initialize file watcher: %w", err)
	}

	// Initialize call processor
	app.processor = processor.New(app.db, app.transcriber, app.discord, app.config, app.logger, app.talkgroups)

	// Initialize system monitor
	if app.config.Monitoring.Enabled {
		app.monitor = monitoring.New(app.config.Monitoring, app.discord, app.logger)
	}

	// Initialize web server
	if app.config.Web.Enabled {
		app.webServer, err = web.New(app.config, app.db, app.monitor, app.talkgroups)
		if err != nil {
			return fmt.Errorf("failed to initialize web server: %w", err)
		}
		app.logger.Info("Web server initialized", "port", app.config.Web.Port)
	}

	return nil
}

func (app *Application) start() error {
	app.logger.Info("Starting Meiko application...")

	// Start Discord client
	if app.discord != nil {
		if err := app.discord.Start(); err != nil {
			app.logger.Warn("Failed to start Discord client", "error", err)
		} else {
			// Send startup notification
			app.discord.SendStartupNotification(AppName, AppVersion)
		}
	}

	// Start SDRTrunk process
	app.logger.Info("Starting SDRTrunk process...")
	if err := app.sdrtrunk.Start(app.ctx); err != nil {
		return fmt.Errorf("failed to start SDRTrunk: %w", err)
	}

	// Start file watcher
	app.logger.Info("Starting file watcher...")
	if err := app.watcher.Start(app.ctx); err != nil {
		return fmt.Errorf("failed to start file watcher: %w", err)
	}

	// Start call processor
	app.logger.Info("Starting call processor...")
	app.processor.Start(app.ctx, app.watcher.Events())

	// Start system monitor
	if app.monitor != nil {
		app.logger.Info("Starting system monitor...")
		app.monitor.Start(app.ctx)
	}

	// Start web server
	if app.webServer != nil {
		app.logger.Info("Starting web server...")
		// Connect web server to processor for real-time updates
		app.processor.SetWebServer(app.webServer)
		go func() {
			if err := app.webServer.Start(); err != nil {
				app.logger.Error("Web server failed to start", "error", err)
			}
		}()
		app.logger.Success("Web dashboard available at http://localhost:%d", app.webServer.GetPort())
	}

	app.logger.Success("🚀 Meiko is now running!")
	app.logger.Info("Press Ctrl+C to shutdown gracefully")

	// Show status
	app.showStatus()

	return nil
}

func (app *Application) shutdown() {
	app.logger.Info("Initiating graceful shutdown...")

	// Cancel context to signal all goroutines to stop
	app.cancel()

	// Give components time to shutdown gracefully
	time.Sleep(2 * time.Second)

	// Send shutdown notification
	if app.discord != nil {
		app.discord.SendShutdownNotification()
		time.Sleep(500 * time.Millisecond) // Give Discord time to send
	}

	// Stop web server
	if app.webServer != nil {
		app.webServer.Stop()
	}

	// Close database
	if app.db != nil {
		app.db.Close()
	}

	// Stop Discord client
	if app.discord != nil {
		app.discord.Stop()
	}

	app.logger.Info("Shutdown complete. Goodbye! 👋")
}

func (app *Application) showStatus() {
	fmt.Println()
	fmt.Println("📊 System Status:")
	fmt.Printf("   SDRTrunk: %s\n", app.getSDRTrunkStatus())
	fmt.Printf("   Discord:  %s\n", app.getDiscordStatus())
	fmt.Printf("   Watcher:  %s\n", app.getWatcherStatus())
	fmt.Printf("   Monitor:  %s\n", app.getMonitorStatus())
	fmt.Println()
}

func (app *Application) getSDRTrunkStatus() string {
	if app.sdrtrunk.IsRunning() {
		return "🟢 Running"
	}
	return "🔴 Stopped"
}

func (app *Application) getDiscordStatus() string {
	if app.discord != nil && app.discord.IsConnected() {
		return "🟢 Connected"
	} else if app.discord == nil {
		return "⚪ Disabled"
	}
	return "🔴 Disconnected"
}

func (app *Application) getWatcherStatus() string {
	if app.watcher.IsWatching() {
		return "🟢 Monitoring"
	}
	return "🔴 Stopped"
}

func (app *Application) getMonitorStatus() string {
	if app.monitor != nil {
		return "🟢 Active"
	}
	return "⚪ Disabled"
}
