package discord

import (
	"bytes"
	"fmt"
	"log"

	"github.com/bwmarrin/discordgo"
)

// Client wraps the discordgo session for the MFCDiscordBot.
type Client struct {
	session *discordgo.Session
}

// NewClient creates a new Discord client.
func NewClient(token string) (*Client, error) {
	if token == "" {
		return nil, fmt.Errorf("discord token is required")
	}

	session, err := discordgo.New(token)
	if err != nil {
		return nil, fmt.Errorf("failed to create discord client: %w", err)
	}

	session.Identify.Intents = discordgo.IntentsGuilds

	return &Client{session: session}, nil
}

// Connect connects to Discord.
func (c *Client) Connect() error {
	err := c.session.Open()
	if err != nil {
		return fmt.Errorf("failed to connect to discord: %w", err)
	}
	log.Println("Connected to Discord")
	return nil
}

// Disconnect disconnects from Discord.
func (c *Client) Disconnect() error {
	return c.session.Close()
}

// RegisterCommands registers slash commands.
func (c *Client) RegisterCommands() error {
	commands := []*discordgo.ApplicationCommand{
		{
			Name:        "addmodel",
			Description: "Adds a model to watch on the server",
			Options: []*discordgo.ApplicationCommandOption{
				{Type: discordgo.ApplicationCommandOptionString, Name: "model", Description: "Model Name to watch", Required: true},
				{Type: discordgo.ApplicationCommandOptionString, Name: "modelid", Description: "Model ID number", Required: true},
				{Type: discordgo.ApplicationCommandOptionString, Name: "message", Description: "Message to announce", Required: true},
				{Type: discordgo.ApplicationCommandOptionChannel, Name: "channel", Description: "The channel to post into", Required: true},
			},
			DefaultMemberPermissions: int64Ptr(16), // ManageChannels
		},
		{
			Name:        "updatemodel",
			Description: "Updates a model to watch on the server",
			Options: []*discordgo.ApplicationCommandOption{
				{Type: discordgo.ApplicationCommandOptionString, Name: "model", Description: "Model Name to watch", Required: true},
				{Type: discordgo.ApplicationCommandOptionString, Name: "modelid", Description: "Model ID number", Required: true},
				{Type: discordgo.ApplicationCommandOptionString, Name: "message", Description: "Message to announce", Required: true},
				{Type: discordgo.ApplicationCommandOptionChannel, Name: "channel", Description: "The channel to update", Required: true},
			},
			DefaultMemberPermissions: int64Ptr(16),
		},
		{
			Name:        "removemodel",
			Description: "Removes a watched model",
			Options: []*discordgo.ApplicationCommandOption{
				{Type: discordgo.ApplicationCommandOptionString, Name: "modelid", Description: "Model ID number", Required: true},
				{Type: discordgo.ApplicationCommandOptionChannel, Name: "channel", Description: "The channel to post into", Required: true},
			},
			DefaultMemberPermissions: int64Ptr(16),
		},
		{
			Name:        "getmodelid",
			Description: "Gets a Model ID value",
			Options: []*discordgo.ApplicationCommandOption{
				{Type: discordgo.ApplicationCommandOptionString, Name: "modelname", Description: "Model's username", Required: true},
			},
			DefaultMemberPermissions: int64Ptr(16),
		},
		{
			Name:        "listmodels",
			Description: "List watched models",
			Options: []*discordgo.ApplicationCommandOption{
				{Type: discordgo.ApplicationCommandOptionChannel, Name: "channel", Description: "The channel posted to", Required: true},
			},
			DefaultMemberPermissions: int64Ptr(16),
		},
	}

	if c.session.State == nil || c.session.State.User == nil {
		return fmt.Errorf("discord session is not ready")
	}
	appID := c.session.State.User.ID
	for _, cmd := range commands {
		_, err := c.session.ApplicationCommandCreate(appID, "", cmd)
		if err != nil {
			log.Printf("Failed to register command %s: %v", cmd.Name, err)
		}
	}
	return nil
}

// AddHandler registers an event handler on the underlying session.
func (c *Client) AddHandler(handler interface{}) {
	c.session.AddHandler(handler)
}

func int64Ptr(i int64) *int64 { return &i }

// Session exposes the underlying discordgo session.
func (c *Client) Session() *discordgo.Session { return c.session }

// SendMessage sends a plain message to a channel.
func (c *Client) SendMessage(channelID, content string) error {
	_, err := c.session.ChannelMessageSend(channelID, content)
	if err != nil {
		return fmt.Errorf("send message to %s: %w", channelID, err)
	}
	return nil
}

// SendMessageWithFile sends a message with a file attachment to a channel.
func (c *Client) SendMessageWithFile(channelID, content, filename string, data []byte) error {
	_, err := c.session.ChannelMessageSendComplex(channelID, &discordgo.MessageSend{
		Content: content,
		Files: []*discordgo.File{
			{Name: filename, ContentType: "image/jpeg", Reader: bytes.NewReader(data)},
		},
	})
	if err != nil {
		return fmt.Errorf("send message with file to %s: %w", channelID, err)
	}
	return nil
}
