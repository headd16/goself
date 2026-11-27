// Buttons example: watch a channel and click a button (found by its label,
// custom ID or emoji) whenever a message containing it is posted or edited.
//
//	DISCORD_TOKEN=your-token go run github.com/headd16/goself/examples/buttons@latest -channel 123456789 -button "Claim"
package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/headd16/goself"
	"github.com/headd16/goself/types"
)

func main() {
	channelID := flag.String("channel", "", "ID of the channel to watch (required)")
	button := flag.String("button", "", "label, custom ID or emoji of the button to click (required)")
	flag.Parse()

	token := os.Getenv("DISCORD_TOKEN")
	if token == "" || *channelID == "" || *button == "" {
		flag.Usage()
		log.Fatal("DISCORD_TOKEN, -channel and -button are required")
	}

	client := goself.NewClient(token, nil)

	// Remember which messages were already clicked so edits don't click twice.
	var mu sync.Mutex
	clicked := map[string]bool{}

	tryClick := func(m *types.MessageEventData) {
		if m.ChannelID != *channelID {
			return
		}
		b := m.FindButton(*button)
		if b == nil || b.Disabled {
			return
		}
		mu.Lock()
		if clicked[m.ID] {
			mu.Unlock()
			return
		}
		clicked[m.ID] = true
		mu.Unlock()

		time.Sleep(500 * time.Millisecond) // look a bit more human
		nonce, err := client.ClickButton(m, b.CustomID)
		if err != nil {
			log.Printf("click on %s failed: %v", m.ID, err)
			return
		}
		log.Printf("clicked %q on message %s (nonce %s)", b.Label, m.ID, nonce)
	}

	must(client.AddHandler(types.GatewayEventMessageCreate, tryClick))
	must(client.AddHandler(types.GatewayEventMessageUpdate, tryClick))

	// Discord tells user accounts whether the bot accepted the interaction.
	must(client.AddHandler(types.GatewayEventInteractionSuccess, func(e *types.InteractionEventData) {
		log.Printf("interaction %s succeeded", e.Nonce)
	}))
	must(client.AddHandler(types.GatewayEventInteractionFailure, func(e *types.InteractionEventData) {
		log.Printf("interaction %s failed (the bot did not respond in time)", e.Nonce)
	}))

	if err := client.Open(); err != nil {
		log.Fatal(err)
	}
	log.Printf("logged in as %s, watching channel %s", client.User().Tag(), *channelID)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	<-stop
	_ = client.Close()
}

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
