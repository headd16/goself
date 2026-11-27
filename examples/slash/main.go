// Slash command example: list the commands available in a guild, or run one.
//
//	# list every command of the guild
//	DISCORD_TOKEN=your-token go run github.com/headd16/goself/examples/slash@latest -guild 111 -channel 222
//
//	# run /ping (of any bot), or /say text:hello
//	DISCORD_TOKEN=your-token go run github.com/headd16/goself/examples/slash@latest -guild 111 -channel 222 -command ping
//	DISCORD_TOKEN=your-token go run github.com/headd16/goself/examples/slash@latest -guild 111 -channel 222 -command say -option text=hello
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/headd16/goself"
	"github.com/headd16/goself/types"
)

func main() {
	guildID := flag.String("guild", "", "guild ID (required)")
	channelID := flag.String("channel", "", "channel ID where the command runs (required)")
	command := flag.String("command", "", "command name to run; empty lists the commands")
	appID := flag.String("app", "", "only use commands of this application ID")
	option := flag.String("option", "", "optional string option as name=value")
	flag.Parse()

	token := os.Getenv("DISCORD_TOKEN")
	if token == "" || *guildID == "" || *channelID == "" {
		flag.Usage()
		log.Fatal("DISCORD_TOKEN, -guild and -channel are required")
	}

	client := goself.NewClient(token, nil)

	index, err := client.GetSlashCommands(*guildID)
	if err != nil {
		log.Fatal("could not fetch commands: ", err)
	}

	if *command == "" {
		apps := map[string]string{}
		for _, app := range index.Applications {
			apps[app.ID] = app.Name
		}
		for _, cmd := range index.ApplicationCommand {
			fmt.Printf("/%-24s %-20s %s\n", cmd.Name, apps[cmd.ApplicationID], cmd.Description)
		}
		return
	}

	cmd := index.Find(*command, *appID)
	if cmd == nil {
		log.Fatalf("command /%s not found in guild %s", *command, *guildID)
	}

	var options []types.ApplicationCommandInteractionOption
	if name, value, ok := strings.Cut(*option, "="); ok {
		options = append(options, types.ApplicationCommandInteractionOption{
			Type: types.CommandOptionString, Name: name, Value: value,
		})
	}

	// The outcome arrives as an INTERACTION_SUCCESS / INTERACTION_FAILURE event
	// whose nonce matches the one returned by SendSlashCommand. The event can
	// arrive before SendSlashCommand returns, so buffer everything.
	type outcome struct{ nonce, status string }
	events := make(chan outcome, 16)
	report := func(status string) func(*types.InteractionEventData) {
		return func(e *types.InteractionEventData) { events <- outcome{string(e.Nonce), status} }
	}
	must(client.AddHandler(types.GatewayEventInteractionSuccess, report("succeeded")))
	must(client.AddHandler(types.GatewayEventInteractionFailure, report("failed")))

	// Running a command needs a live gateway session, so connect first.
	if err := client.Open(); err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	nonce, err := client.SendSlashCommand(*channelID, *guildID, cmd, options...)
	if err != nil {
		log.Println("could not run command: ", err)
		return
	}
	fmt.Printf("sent /%s, waiting for the bot...\n", cmd.Name)

	timeout := time.After(15 * time.Second)
	for {
		select {
		case ev := <-events:
			if ev.nonce == nonce {
				fmt.Printf("/%s %s\n", cmd.Name, ev.status)
				return
			}
		case <-timeout:
			fmt.Println("no answer from Discord within 15s")
			return
		}
	}
}

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
