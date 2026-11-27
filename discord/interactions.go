package discord

import (
	"errors"

	"github.com/headd16/goself/types"
)

// Interaction types sent by the client.
const (
	interactionApplicationCommand = 2
	interactionMessageComponent   = 3
)

type interactionRequest struct {
	Type              int    `json:"type"`
	ApplicationID     string `json:"application_id"`
	GuildID           string `json:"guild_id,omitempty"`
	ChannelID         string `json:"channel_id"`
	MessageFlags      *int   `json:"message_flags,omitempty"`
	MessageID         string `json:"message_id,omitempty"`
	SessionID         string `json:"session_id"`
	Data              any    `json:"data"`
	Nonce             string `json:"nonce"`
	AnalyticsLocation string `json:"analytics_location,omitempty"`
}

// GetSlashCommands returns the commands available in a guild.
func (g *Gateway) GetSlashCommands(guildID string) (types.ApplicationCommandIndex, error) {
	var idx types.ApplicationCommandIndex
	err := g.request("GET", "/guilds/"+guildID+"/application-command-index", nil, &idx, discordOrigin+"/channels/"+guildID)
	return idx, err
}

// GetUserSlashCommands returns the user-installed commands of the account.
func (g *Gateway) GetUserSlashCommands() (types.ApplicationCommandIndex, error) {
	var idx types.ApplicationCommandIndex
	err := g.request("GET", "/users/@me/application-command-index", nil, &idx, "")
	return idx, err
}

// GetChannelSlashCommands returns the commands available in a channel; use it
// for DMs and group DMs.
func (g *Gateway) GetChannelSlashCommands(channelID string) (types.ApplicationCommandIndex, error) {
	var idx types.ApplicationCommandIndex
	err := g.request("GET", "/channels/"+channelID+"/application-command-index", nil, &idx, channelReferer("", channelID))
	return idx, err
}

// ClickButton clicks the button with the given custom ID on msg. It returns
// the interaction nonce, which matches the Nonce of the INTERACTION_SUCCESS /
// INTERACTION_FAILURE event that follows.
func (g *Gateway) ClickButton(msg *types.Message, customID string) (string, error) {
	if msg == nil {
		return "", errors.New("discord: message is nil")
	}
	if customID == "" {
		return "", errors.New("discord: button custom ID is empty (link buttons cannot be clicked)")
	}
	data := map[string]any{
		"component_type": types.ComponentButton,
		"custom_id":      customID,
	}
	return g.sendComponentInteraction(msg, data)
}

// SelectMenu picks values in the select menu with the given custom ID on msg.
// For string selects the values are the option values; for user, role and
// channel selects they are IDs. It returns the interaction nonce.
func (g *Gateway) SelectMenu(msg *types.Message, customID string, values []string) (string, error) {
	if msg == nil {
		return "", errors.New("discord: message is nil")
	}
	componentType := types.ComponentStringSelect
	for _, menu := range msg.SelectMenus() {
		if menu.CustomID == customID {
			componentType = menu.Type
			break
		}
	}
	if values == nil {
		values = []string{}
	}
	data := map[string]any{
		"component_type": componentType,
		"custom_id":      customID,
		"type":           componentType,
		"values":         values,
	}
	return g.sendComponentInteraction(msg, data)
}

func (g *Gateway) sendComponentInteraction(msg *types.Message, data any) (string, error) {
	nonce := GenerateNonce()
	flags := msg.Flags
	req := interactionRequest{
		Type:          interactionMessageComponent,
		ApplicationID: applicationIDFor(msg),
		GuildID:       msg.GuildID,
		ChannelID:     msg.ChannelID,
		MessageFlags:  &flags,
		MessageID:     msg.ID,
		SessionID:     g.interactionSessionID(),
		Data:          data,
		Nonce:         nonce,
	}
	if err := g.request("POST", "/interactions", req, nil, channelReferer(msg.GuildID, msg.ChannelID)); err != nil {
		return "", err
	}
	return nonce, nil
}

// SendSlashCommand runs a slash command in a channel (guildID is empty in
// DMs). Get cmd from GetSlashCommands(...).Find(name, applicationID). It
// returns the interaction nonce.
func (g *Gateway) SendSlashCommand(channelID, guildID string, cmd *types.ApplicationCommand, options ...types.ApplicationCommandInteractionOption) (string, error) {
	if cmd == nil {
		return "", errors.New("discord: command is nil")
	}
	if options == nil {
		options = []types.ApplicationCommandInteractionOption{}
	}
	commandType := cmd.Type
	if commandType == 0 {
		commandType = types.ApplicationCommandChatInput
	}

	nonce := GenerateNonce()
	req := interactionRequest{
		Type:          interactionApplicationCommand,
		ApplicationID: cmd.ApplicationID,
		GuildID:       guildID,
		ChannelID:     channelID,
		SessionID:     g.interactionSessionID(),
		Data: map[string]any{
			"version":             cmd.Version,
			"id":                  cmd.ID,
			"name":                cmd.Name,
			"type":                commandType,
			"options":             options,
			"application_command": cmd,
			"attachments":         []any{},
		},
		Nonce:             nonce,
		AnalyticsLocation: "slash_ui",
	}
	if err := g.requestMultipart("POST", "/interactions", req, nil, channelReferer(guildID, channelID)); err != nil {
		return "", err
	}
	return nonce, nil
}

func (g *Gateway) interactionSessionID() string {
	if id := g.SessionID(); id != "" {
		return id
	}
	return GenerateSessionID()
}

// applicationIDFor returns the application that owns the components of msg.
func applicationIDFor(msg *types.Message) string {
	if msg.ApplicationID != "" {
		return msg.ApplicationID
	}
	return msg.Author.ID
}

// GetSlashCommands returns the commands available in a guild.
//
// Deprecated: use gateway.GetSlashCommands.
func GetSlashCommands(gateway *Gateway, guildID string) (types.ApplicationCommandIndex, error) {
	return gateway.GetSlashCommands(guildID)
}

// GetUserSlashCommands returns the user-installed commands of the account.
//
// Deprecated: use gateway.GetUserSlashCommands.
func GetUserSlashCommands(gateway *Gateway) (types.ApplicationCommandIndex, error) {
	return gateway.GetUserSlashCommands()
}

// ClickButton clicks a button and reports whether Discord accepted it.
//
// Deprecated: use gateway.ClickButton, which returns the nonce and the error.
func ClickButton(gateway *Gateway, e *types.MessageEventData, interactionID string) bool {
	_, err := gateway.ClickButton(e, interactionID)
	return err == nil
}
