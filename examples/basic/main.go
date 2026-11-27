// Basic example: log in, print incoming messages and answer a few commands
// that you type yourself.
//
//	DISCORD_TOKEN=your-token go run github.com/headd16/goself/examples/basic@latest
//
// Then type "!ping", "!latency" or "!status <text>" in any channel.
package main

import (
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/headd16/goself"
	"github.com/headd16/goself/types"
)

func main() {
	token := os.Getenv("DISCORD_TOKEN")
	if token == "" {
		log.Fatal("set the DISCORD_TOKEN environment variable")
	}

	config := types.NewConfig()
	config.Presence = types.StatusOnline
	config.Logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	client := goself.NewClient(token, config)

	must(client.AddHandler(types.GatewayEventReady, func(e *types.ReadyEventData) {
		fmt.Printf("Logged in as %s (%s), %d guilds, %d DMs\n",
			e.User.Tag(), e.User.ID, len(e.Guilds), len(e.PrivateChannels))
	}))

	must(client.AddHandler(types.GatewayEventMessageCreate, func(m *types.MessageEventData) {
		fmt.Printf("[%s] %s: %s\n", m.ChannelID, m.Author.Tag(), m.Content)

		if m.Author.ID != client.User().ID {
			return
		}

		switch {
		case m.Content == "!ping":
			if _, err := client.EditMessage(m.ChannelID, m.ID, "pong!"); err != nil {
				log.Println("edit failed:", err)
			}
		case m.Content == "!latency":
			if _, err := client.Reply(m, fmt.Sprintf("Gateway latency: %s", client.Latency())); err != nil {
				log.Println("reply failed:", err)
			}
		case strings.HasPrefix(m.Content, "!status "):
			text := strings.TrimPrefix(m.Content, "!status ")
			if err := client.SetCustomStatus(text, "🤖"); err != nil {
				log.Println("status failed:", err)
			}
			_ = client.AddReaction(m.ChannelID, m.ID, "✅")
		}
	}))

	must(client.AddHandler(types.GatewayEventReconnect, func() {
		log.Println("connection lost, reconnecting...")
	}))

	// Open returns once READY has been received; events keep flowing in the
	// background.
	if err := client.Open(); err != nil {
		log.Fatal("could not connect: ", err)
	}
	fmt.Println("Running. Press Ctrl+C to exit.")

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// Wait for Ctrl+C, or for the gateway to stop on its own (e.g. bad token).
	stopped := make(chan error, 1)
	go func() { stopped <- client.Wait() }()

	select {
	case <-stop:
		fmt.Println("Shutting down...")
		_ = client.Close()
	case err := <-stopped:
		log.Fatal("gateway stopped: ", err)
	}
}

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
