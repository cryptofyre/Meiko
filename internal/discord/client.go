package discord

import (
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"

	"Meiko/internal/config"
	"Meiko/internal/database"
	"Meiko/internal/logger"
)

// Client handles Discord integration
type Client struct {
	config    config.DiscordConfig
	logger    *logger.Logger
	session   *discordgo.Session
	connected bool
}

// New creates a new Discord client
func New(config config.DiscordConfig, logger *logger.Logger) (*Client, error) {
	if config.Token == "" {
		return nil, fmt.Errorf("Discord token is required")
	}

	session, err := discordgo.New("Bot " + config.Token)
	if err != nil {
		return nil, fmt.Errorf("failed to create Discord session: %w", err)
	}

	return &Client{
		config:  config,
		logger:  logger,
		session: session,
	}, nil
}

// Start connects to Discord
func (c *Client) Start() error {
	if err := c.session.Open(); err != nil {
		return fmt.Errorf("failed to open Discord session: %w", err)
	}

	c.connected = true
	c.logger.Success("Connected to Discord")
	return nil
}

// Stop disconnects from Discord
func (c *Client) Stop() error {
	if c.session != nil {
		if err := c.session.Close(); err != nil {
			c.logger.Error("Error closing Discord session", "error", err)
		}
	}
	c.connected = false
	return nil
}

// IsConnected returns whether the client is connected
func (c *Client) IsConnected() bool {
	return c.connected
}

// SendStartupNotification sends a startup notification
func (c *Client) SendStartupNotification(appName, version string) {
	if !c.config.Notifications.Startup {
		return
	}

	embed := &discordgo.MessageEmbed{
		Title:       "🚀 " + appName + " Started",
		Description: fmt.Sprintf("Version %s is now running", version),
		Color:       0x00ff00, // Green
		Timestamp:   time.Now().Format(time.RFC3339),
	}

	c.sendEmbed(embed)
}

// SendShutdownNotification sends a shutdown notification
func (c *Client) SendShutdownNotification() {
	if !c.config.Notifications.Shutdown {
		return
	}

	embed := &discordgo.MessageEmbed{
		Title:       "🛑 Meiko Shutdown",
		Description: "Application is shutting down",
		Color:       0xff9900, // Orange
		Timestamp:   time.Now().Format(time.RFC3339),
	}

	c.sendEmbed(embed)
}

// SendCallNotification sends a notification for a new call
func (c *Client) SendCallNotification(call *database.CallRecord) error {
	if !c.config.Notifications.Transcriptions {
		return nil
	}

	// Create transcription preview
	transcriptionPreview := "No transcription"
	if call.Transcription != "" {
		if len(call.Transcription) > 100 {
			transcriptionPreview = call.Transcription[:100] + "..."
		} else {
			transcriptionPreview = call.Transcription
		}
	}

	embed := &discordgo.MessageEmbed{
		Title: "📞 New Call Recorded",
		Color: 0x3b82f6, // Blue
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "Talkgroup",
				Value:  call.TalkgroupAlias,
				Inline: true,
			},
			{
				Name:   "System",
				Value:  call.TalkgroupGroup,
				Inline: true,
			},
			{
				Name:   "Duration",
				Value:  fmt.Sprintf("%ds", call.Duration),
				Inline: true,
			},
			{
				Name:   "Frequency",
				Value:  call.Frequency,
				Inline: true,
			},
			{
				Name:   "Time",
				Value:  call.Timestamp.Format("01/02 15:04:05"),
				Inline: true,
			},
			{
				Name:   "File",
				Value:  call.Filename,
				Inline: false,
			},
		},
		Timestamp: call.Timestamp.Format(time.RFC3339),
	}

	// Add transcription field if available
	if call.Transcription != "" {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   "Transcription",
			Value:  transcriptionPreview,
			Inline: false,
		})
	}

	c.sendEmbed(embed)
	return nil
}

// sendEmbed sends an embed to the configured channel
func (c *Client) sendEmbed(embed *discordgo.MessageEmbed) {
	if !c.connected || c.config.ChannelID == "" {
		return
	}

	_, err := c.session.ChannelMessageSendEmbed(c.config.ChannelID, embed)
	if err != nil {
		c.logger.Error("Failed to send Discord message", "error", err)
	}
}
