package discord

import (
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"

	"Meiko/internal/config"
	"Meiko/internal/database"
	"Meiko/internal/logger"
	"Meiko/internal/talkgroups"
)

// Client handles Discord integration
type Client struct {
	config     config.DiscordConfig
	logger     *logger.Logger
	session    *discordgo.Session
	talkgroups *talkgroups.Service
	connected  bool
}

// New creates a new Discord client
func New(config config.DiscordConfig, logger *logger.Logger, talkgroupService *talkgroups.Service) (*Client, error) {
	if config.Token == "" {
		return nil, fmt.Errorf("Discord token is required")
	}

	session, err := discordgo.New("Bot " + config.Token)
	if err != nil {
		return nil, fmt.Errorf("failed to create Discord session: %w", err)
	}

	return &Client{
		config:     config,
		logger:     logger,
		session:    session,
		talkgroups: talkgroupService,
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

	// Get enhanced talkgroup information
	var deptInfo *talkgroups.DepartmentType
	var talkgroupInfo *talkgroups.TalkgroupInfo
	var colorHex int

	if c.talkgroups != nil {
		talkgroupInfo = c.talkgroups.GetTalkgroupInfo(call.TalkgroupID)
		deptInfo = c.talkgroups.GetDepartmentInfo(call.TalkgroupID)

		// Convert hex color to Discord integer
		if color, err := parseHexColor(deptInfo.Color); err == nil {
			colorHex = color
		}
	}

	// Fallback to default values if talkgroup service unavailable
	if deptInfo == nil {
		deptInfo = &talkgroups.DepartmentType{
			Color: "#0099ff",
			Emoji: "🔔",
			Type:  talkgroups.ServiceOther,
		}
		colorHex = 0x0099ff
	}

	if talkgroupInfo == nil {
		talkgroupInfo = &talkgroups.TalkgroupInfo{
			ID:    call.TalkgroupID,
			Name:  call.TalkgroupAlias,
			Group: call.TalkgroupGroup,
		}
	}

	// Create transcription preview
	transcriptionPreview := "No transcription available"
	if call.Transcription != "" {
		if len(call.Transcription) > 300 {
			transcriptionPreview = call.Transcription[:300] + "..."
		} else {
			transcriptionPreview = call.Transcription
		}
	}

	// Format duration
	duration := float64(call.Duration)
	durationStr := fmt.Sprintf("%.1fs", duration)

	// Create Swimtrunks-style title and subtitle
	title := fmt.Sprintf("📞 Incoming call from %s %s", deptInfo.Emoji, talkgroupInfo.Group)
	subtitle := fmt.Sprintf("📻 %s", talkgroupInfo.Name)

	// Build the description
	description := subtitle
	if call.Transcription != "" {
		description += "\n\n" + transcriptionPreview
	}

	embed := &discordgo.MessageEmbed{
		Title:       title,
		Description: description,
		Color:       colorHex,
		Fields: []*discordgo.MessageEmbedField{
			{
				Name: "Details",
				Value: fmt.Sprintf("📟 Unit: `%s` • ⏱️ Duration: `%s` • 🕒 <t:%d:F>",
					call.TalkgroupID,
					durationStr,
					call.Timestamp.Unix()),
				Inline: false,
			},
		},
		Timestamp: call.Timestamp.Format(time.RFC3339),
		Footer: &discordgo.MessageEmbedFooter{
			Text: fmt.Sprintf("TalkGroup: %s • Meiko Scanner", call.TalkgroupID),
		},
	}

	// Add frequency if available
	if call.Frequency != "" {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   "Frequency",
			Value:  call.Frequency,
			Inline: true,
		})
	}

	c.sendEmbed(embed)

	// Log notification details
	c.logger.Info("Discord notification sent",
		"talkgroup", call.TalkgroupID,
		"department", talkgroupInfo.Group,
		"service_type", string(deptInfo.Type),
		"duration", fmt.Sprintf("%.1fs", duration),
		"has_transcription", call.Transcription != "")

	return nil
}

// parseHexColor converts a hex color string to Discord color integer
func parseHexColor(hexColor string) (int, error) {
	// Remove # if present
	if len(hexColor) > 0 && hexColor[0] == '#' {
		hexColor = hexColor[1:]
	}

	// Parse hex string
	if len(hexColor) != 6 {
		return 0x0099ff, fmt.Errorf("invalid hex color format")
	}

	var color int
	for _, char := range hexColor {
		var digit int
		if char >= '0' && char <= '9' {
			digit = int(char - '0')
		} else if char >= 'a' && char <= 'f' {
			digit = int(char - 'a' + 10)
		} else if char >= 'A' && char <= 'F' {
			digit = int(char - 'A' + 10)
		} else {
			return 0x0099ff, fmt.Errorf("invalid hex character")
		}
		color = color*16 + digit
	}

	return color, nil
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
