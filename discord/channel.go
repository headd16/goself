package discord

import (
	"errors"
	"net/url"
	"strconv"
	"strings"

	"github.com/headd16/goself/types"
)

func channelReferer(guildID, channelID string) string {
	if guildID == "" {
		guildID = "@me"
	}
	return discordOrigin + "/channels/" + guildID + "/" + channelID
}

// SendMessage sends a text message to a channel.
func (g *Gateway) SendMessage(channelID, content string) (*types.Message, error) {
	return g.SendMessageComplex(channelID, &types.MessageSend{Content: content})
}

// SendMessageComplex sends a message with full control over the payload
// (replies, allowed mentions, stickers, flags, TTS...).
func (g *Gateway) SendMessageComplex(channelID string, data *types.MessageSend) (*types.Message, error) {
	if data == nil {
		return nil, errors.New("discord: message data is nil")
	}
	body := *data
	if body.Nonce == "" {
		body.Nonce = GenerateNonce()
	}
	if body.MobileNetworkType == "" {
		body.MobileNetworkType = "unknown"
	}

	var msg types.Message
	if err := g.request("POST", "/channels/"+channelID+"/messages", &body, &msg, channelReferer("", channelID)); err != nil {
		return nil, err
	}
	return &msg, nil
}

// ReplyMessage replies to a message without pinging its author.
func (g *Gateway) ReplyMessage(channelID, messageID, content string) (*types.Message, error) {
	return g.SendMessageComplex(channelID, &types.MessageSend{
		Content:          content,
		MessageReference: &types.MessageReference{ChannelID: channelID, MessageID: messageID},
		AllowedMentions:  &types.AllowedMentions{Parse: []string{"users", "roles", "everyone"}, RepliedUser: false},
	})
}

// Reply replies to msg without pinging its author.
func (g *Gateway) Reply(msg *types.Message, content string) (*types.Message, error) {
	if msg == nil {
		return nil, errors.New("discord: message is nil")
	}
	return g.ReplyMessage(msg.ChannelID, msg.ID, content)
}

// EditMessage changes the content of one of your messages.
func (g *Gateway) EditMessage(channelID, messageID, content string) (*types.Message, error) {
	var msg types.Message
	err := g.request("PATCH", "/channels/"+channelID+"/messages/"+messageID,
		&types.MessageEdit{Content: &content}, &msg, channelReferer("", channelID))
	if err != nil {
		return nil, err
	}
	return &msg, nil
}

// DeleteMessage deletes a message.
func (g *Gateway) DeleteMessage(channelID, messageID string) error {
	return g.request("DELETE", "/channels/"+channelID+"/messages/"+messageID, nil, nil, channelReferer("", channelID))
}

// GetMessages returns up to limit (1-100) messages of a channel, newest first.
// At most one of before, after and around should be set.
func (g *Gateway) GetMessages(channelID string, limit int, before, after, around string) ([]types.Message, error) {
	q := url.Values{}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(min(limit, 100)))
	}
	if before != "" {
		q.Set("before", before)
	}
	if after != "" {
		q.Set("after", after)
	}
	if around != "" {
		q.Set("around", around)
	}
	path := "/channels/" + channelID + "/messages"
	if len(q) > 0 {
		path += "?" + q.Encode()
	}

	var msgs []types.Message
	if err := g.request("GET", path, nil, &msgs, channelReferer("", channelID)); err != nil {
		return nil, err
	}
	for i := range msgs {
		if msgs[i].ChannelID == "" {
			msgs[i].ChannelID = channelID
		}
		g.State.storeMessages(&msgs[i])
	}
	return msgs, nil
}

// GetMessage returns a single message. User accounts cannot use the bot-only
// "get message" endpoint, so this looks the message up in the history.
func (g *Gateway) GetMessage(channelID, messageID string) (*types.Message, error) {
	msgs, err := g.GetMessages(channelID, 1, "", "", messageID)
	if err != nil {
		return nil, err
	}
	for i := range msgs {
		if msgs[i].ID == messageID {
			return &msgs[i], nil
		}
	}
	return nil, &RESTError{Method: "GET", Path: "/channels/" + channelID + "/messages", StatusCode: 404, Code: 10008, Message: "Unknown Message"}
}

// AddReaction reacts to a message. emoji is a unicode emoji ("👍") or a custom
// emoji as "name:id" or "<:name:id>".
func (g *Gateway) AddReaction(channelID, messageID, emoji string) error {
	return g.request("PUT", reactionPath(channelID, messageID, emoji)+"/@me", nil, nil, channelReferer("", channelID))
}

// RemoveReaction removes your reaction from a message.
func (g *Gateway) RemoveReaction(channelID, messageID, emoji string) error {
	return g.request("DELETE", reactionPath(channelID, messageID, emoji)+"/@me", nil, nil, channelReferer("", channelID))
}

func reactionPath(channelID, messageID, emoji string) string {
	emoji = strings.TrimSuffix(strings.TrimPrefix(emoji, "<"), ">")
	emoji = strings.TrimPrefix(emoji, "a:")
	return "/channels/" + channelID + "/messages/" + messageID + "/reactions/" + url.PathEscape(emoji)
}

// TriggerTyping shows "<you> is typing..." in a channel for ~10 seconds.
func (g *Gateway) TriggerTyping(channelID string) error {
	return g.request("POST", "/channels/"+channelID+"/typing", nil, nil, channelReferer("", channelID))
}

// GetChannel returns a channel by ID.
func (g *Gateway) GetChannel(channelID string) (*types.Channel, error) {
	var ch types.Channel
	if err := g.request("GET", "/channels/"+channelID, nil, &ch, ""); err != nil {
		return nil, err
	}
	g.State.storeChannels(ch)
	return &ch, nil
}

// CreateDM opens (or returns the existing) DM channel with a user.
func (g *Gateway) CreateDM(userID string) (*types.Channel, error) {
	var ch types.Channel
	body := map[string][]string{"recipients": {userID}}
	if err := g.request("POST", "/users/@me/channels", body, &ch, ""); err != nil {
		return nil, err
	}
	g.State.storeChannels(ch)
	return &ch, nil
}

// SendMessage sends a message and reports whether it succeeded.
//
// Deprecated: use gateway.SendMessage, which returns the message and the error.
func SendMessage(gateway *Gateway, channelID string, content string) bool {
	_, err := gateway.SendMessage(channelID, content)
	return err == nil
}
