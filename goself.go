// Package goself is a Discord library for user accounts ("self-bots").
//
// Create a client with NewClient, register handlers with AddHandler and call
// Connect (blocking) or Open (non-blocking). All gateway and REST helpers of
// discord.Gateway are available directly on the client:
//
//	client := goself.NewClient(token, nil)
//	client.AddHandler(types.GatewayEventMessageCreate, func(m *types.MessageEventData) {
//		if m.Content == "!ping" && m.Author.ID == client.User().ID {
//			client.Reply(m, "pong")
//		}
//	})
//	log.Fatal(client.Connect())
package goself

import (
	"github.com/headd16/goself/discord"
	"github.com/headd16/goself/types"
)

// Client is the entry point of the library. It embeds *discord.Gateway, so
// every method documented there (SendMessage, ClickButton, UpdatePresence,
// ...) can be called on the client directly. The Selfbot and Config fields
// are promoted from the gateway as well.
type Client struct {
	*discord.Gateway
}

// NewClient creates a client for the given account token. A nil config uses
// types.DefaultConfig. Surrounding whitespace in the token is ignored.
func NewClient(token string, config *types.Config) *Client {
	return &Client{Gateway: discord.NewGateway(token, config)}
}
