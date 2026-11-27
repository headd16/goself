# GoSelf

[![Go Reference](https://pkg.go.dev/badge/github.com/headd16/goself.svg)](https://pkg.go.dev/github.com/headd16/goself)
[![Go Report Card](https://goreportcard.com/badge/github.com/headd16/goself)](https://goreportcard.com/report/github.com/headd16/goself)
![Code size](https://img.shields.io/github/languages/code-size/headd16/GoSelf)

GoSelf is a fast Discord library for **user accounts** ("self-bots") written in Go. It
connects to the gateway as the Discord web client does, decodes events into
typed structs and comes with REST helpers for messages, reactions, buttons,
select menus and slash commands.

> [!WARNING]
> Automating a user account is against Discord's Terms of Service and can get
> the account banned. Use this library at your own risk, ideally on an account
> you can afford to lose, and never share your token: anyone who has it fully
> controls your account.

- [Features](#features)
- [Installation](#installation)
- [Quick start](#quick-start)
- [Running the examples](#running-the-examples)
- [Connecting](#connecting)
- [Events](#events)
- [Fetching data: guilds, channels, roles, users, members](#fetching-data-guilds-channels-roles-users-members)
- [Sending messages and other REST calls](#sending-messages-and-other-rest-calls)
- [Buttons, select menus and slash commands](#buttons-select-menus-and-slash-commands)
- [Presence, members and voice](#presence-members-and-voice)
- [Configuration](#configuration)
- [Error handling](#error-handling)
- [Running the tests](#running-the-tests)
- [Upgrading from older versions](#upgrading-from-older-versions)

## Features

- **Reliable gateway**: heartbeats with zombie-connection detection, automatic
  resume/reconnect with backoff, and session invalidation handling.
- **Typed events**: messages, edits, deletes, reactions, typing, presences,
  guilds, members, channels, relationships and interaction results. Any other
  event is available as raw JSON.
- **Fast data access**: a built-in cache of guilds, channels, categories,
  threads, roles, emojis, users, members, presences, friends and recent
  messages, filled automatically from the gateway. Lookups are instant.
  `Fetch*` helpers fall back to the API only for what is missing, bulk
  fetches run in parallel, and permissions are computed for you.
- **REST helpers**: send, reply, edit, delete and fetch messages; reactions;
  typing; DMs; guilds; roles; users and profiles; invites. `Request` covers
  any other endpoint.
- **Interactions**: click buttons (including Components V2 layouts), pick
  select-menu values and run slash commands.
- **Web-client fingerprint**: super properties, the current client build number
  (scraped automatically), locale and timezone headers.
- **Safe by default**: a panicking handler is recovered, handlers run
  concurrently, and 429 rate limits are retried.

## Installation

Requires [Go](https://go.dev/dl/) 1.22 or newer. You do not need to download
or clone this repository. Add the package to your project and Go fetches it
for you:

```sh
go get github.com/headd16/goself@latest
```

```go
import (
	"github.com/headd16/goself"       // the client
	"github.com/headd16/goself/types" // events, config, Discord objects
)
```

### Start a new project in one minute

```sh
mkdir mybot && cd mybot
go mod init mybot
go get github.com/headd16/goself@latest
```

Create `main.go` with the [quick start](#quick-start) code below, then run or build it:

```sh
# Linux / macOS
export DISCORD_TOKEN="your-token"
go run .            # run directly
go build -o mybot . # or build a binary: ./mybot

# Windows (PowerShell)
$env:DISCORD_TOKEN = "your-token"
go run .
go build -o mybot.exe .
```

To update to the newest release later: `go get -u github.com/headd16/goself`.

## Quick start

```go
package main

import (
	"log"
	"os"

	"github.com/headd16/goself"
	"github.com/headd16/goself/types"
)

func main() {
	client := goself.NewClient(os.Getenv("DISCORD_TOKEN"), nil) // nil = default config

	client.AddHandler(types.GatewayEventReady, func(e *types.ReadyEventData) {
		log.Printf("logged in as %s", e.User.Tag())
	})

	client.AddHandler(types.GatewayEventMessageCreate, func(m *types.MessageEventData) {
		// Only answer commands typed by the account itself.
		if m.Author.ID == client.User().ID && m.Content == "!ping" {
			client.Reply(m, "pong!")
		}
	})

	// Connect blocks until Close is called or the session dies (e.g. bad token).
	log.Fatal(client.Connect())
}
```

### Getting your token

Log in at <https://discord.com/app> in a desktop browser, open the developer
tools (F12), go to the **Network** tab and click any channel. Then select a
request to `discord.com/api/...` and copy the value of its `Authorization`
request header. Keep it secret.

## Running the examples

The [`examples`](examples) folder contains ready-to-run programs. Each one
reads the token from the `DISCORD_TOKEN` environment variable.

| Example | What it does |
| --- | --- |
| [`examples/basic`](examples/basic/main.go) | Logs messages and answers `!ping`, `!latency` and `!status <text>` when you type them. Shuts down cleanly on Ctrl+C. |
| [`examples/buttons`](examples/buttons/main.go) | Watches a channel and clicks a button by label, custom ID or emoji, then reports whether the interaction succeeded. |
| [`examples/slash`](examples/slash/main.go) | Lists the slash commands of a guild, or runs one with an optional string option. |
| [`examples/fetch`](examples/fetch/main.go) | Prints your guilds, then for one guild its channel tree (with the channels hidden from you), roles and your permissions. Also fetches users, members and profiles by ID and resolves invites. |

You can run them **without downloading the source**: `go run` fetches the
published module. Linux / macOS:

```sh
export DISCORD_TOKEN="your-token"
go run github.com/headd16/goself/examples/basic@latest
go run github.com/headd16/goself/examples/fetch@latest
go run github.com/headd16/goself/examples/fetch@latest -guild 111111111111111111 -users 222222222222222222,333333333333333333 -invite discord.gg/abc
go run github.com/headd16/goself/examples/buttons@latest -channel 123456789012345678 -button "Claim"
go run github.com/headd16/goself/examples/slash@latest -guild 111111111111111111 -channel 222222222222222222
go run github.com/headd16/goself/examples/slash@latest -guild 111111111111111111 -channel 222222222222222222 -command say -option text=hello
```

Windows (PowerShell):

```powershell
$env:DISCORD_TOKEN = "your-token"
go run github.com/headd16/goself/examples/basic@latest
```

To install an example as a program on your PATH:
`go install github.com/headd16/goself/examples/fetch@latest` (it is installed
as `fetch` in `$(go env GOPATH)/bin`). Inside a clone of this repository,
`go run ./examples/basic` works too.

## Connecting

```go
client := goself.NewClient(token, nil)

// Option 1: block until the client stops.
err := client.Connect()

// Option 2: connect, do other work, then wait or close.
if err := client.Open(); err != nil { // returns after READY
	log.Fatal(err)
}
// ... use client.SendMessage etc. ...
client.Close()      // ends the session and stops reconnecting
err = client.Wait() // nil after Close, otherwise the fatal error
```

After `Open` succeeds, the client keeps the connection alive by itself:

| Situation | What the client does |
| --- | --- |
| Network drop, missed heartbeat ACK, Discord asks to reconnect (op 7) | Reconnects and **resumes**; missed events are replayed. Fires `RECONNECT`, then `RESUMED`. |
| Session invalidated (op 9) | Waits 1–5 s, then identifies again. Fires `INVALIDATED`, then `READY`. |
| Fatal close code (e.g. `4004` authentication failed) | Stops. `Connect` / `Wait` return a `*discord.CloseError` and `DISCONNECT` fires with it. |

Other useful state: `client.User()`, `client.SessionID()`, `client.Latency()`,
`client.IsOpen()`, `client.IsConnected()`.

## Events

Register handlers with `AddHandler(eventName, func)`. It returns an error if
the function signature does not match the event. Handlers can be added at any
time, before or after connecting.

| Event constant (`types.`) | Handler signature |
| --- | --- |
| `GatewayEventReady` | `func(*types.ReadyEventData)` |
| `GatewayEventResumed` | `func()` |
| `GatewayEventMessageCreate`, `GatewayEventMessageUpdate` | `func(*types.MessageEventData)` |
| `GatewayEventMessageDelete` | `func(*types.MessageDeleteEventData)` |
| `GatewayEventMessageDeleteBulk` | `func(*types.MessageDeleteBulkEventData)` |
| `GatewayEventMessageReactionAdd`, `GatewayEventMessageReactionRemove` | `func(*types.MessageReactionEventData)` |
| `GatewayEventTypingStart` | `func(*types.TypingStartEventData)` |
| `GatewayEventPresenceUpdate` | `func(*types.PresenceUpdateEventData)` |
| `GatewayEventUserUpdate` | `func(*types.User)` |
| `GatewayEventGuildCreate`, `GatewayEventGuildUpdate` | `func(*types.Guild)` |
| `GatewayEventGuildDelete` | `func(*types.GuildDeleteEventData)` |
| `GatewayEventGuildMemberAdd` / `Update` / `Remove` | `func(*types.GuildMemberEventData)` |
| `GatewayGuildMembersChunk` | `func(*types.GuildMembersChunkEventData)` |
| `GatewayEventChannelCreate` / `Update` / `Delete` | `func(*types.Channel)` |
| `GatewayEventThreadCreate` / `Update` / `Delete` | `func(*types.Channel)` |
| `GatewayEventThreadListSync` | `func(*types.ThreadListSyncEventData)` |
| `GatewayEventGuildRoleCreate`, `GatewayEventGuildRoleUpdate` | `func(*types.GuildRoleEventData)` |
| `GatewayEventGuildRoleDelete` | `func(*types.GuildRoleDeleteEventData)` |
| `GatewayEventGuildEmojisUpdate` | `func(*types.GuildEmojisUpdateEventData)` |
| `GatewayEventRelationshipAdd` / `Update` / `Remove` | `func(*types.Relationship)` |
| `GatewayEventInteractionCreate` / `Success` / `Failure` | `func(*types.InteractionEventData)` |
| `GatewayEventReconnect`, `GatewayEventInvalidated` | `func()` |
| `GatewayEventDisconnect` | `func(error)`: nil after `Close` |
| `GatewayEventAll` (`"*"`) | `func(*types.RawEvent)`: every dispatch |

**Any other event** (for example `"THREAD_CREATE"` or `"CALL_CREATE"`) can be
received with a raw handler and decoded into your own struct:

```go
client.AddHandler("THREAD_CREATE", func(e *types.RawEvent) {
	var thread types.Channel
	if err := e.Decode(&thread); err == nil {
		log.Println("new thread:", thread.Name)
	}
})
```

By default every event runs in its own goroutine, so a slow handler never
blocks the connection. Set `config.SyncEvents = true` to run handlers one
after another on the reading goroutine. A handler that panics is recovered
and logged.

### Working with messages

```go
client.AddHandler(types.GatewayEventMessageCreate, func(m *types.MessageEventData) {
	m.Author.Tag()                 // "username" or "username#1234"
	m.IsReply()                    // m.ReferencedMessage holds the original
	m.MentionsUser(client.User().ID)
	m.Buttons()                    // every button, even inside containers/sections
	m.SelectMenus()
	m.FindButton("Accept")         // by label (case-insensitive), custom ID or emoji
	m.MessageSnapshots             // content of forwarded messages
})
```

## Fetching data: guilds, channels, roles, users, members

The client keeps a **cache** (`client.State`) of everything the gateway sends.
READY alone gives you every guild, channel, category, thread, role, emoji,
DM, friend and your own member objects. Live events keep the cache up to
date (new channels, role edits, member updates, messages, presences...).

There are three ways to read data:

| Way | Speed | When to use |
| --- | --- | --- |
| `client.State.X(...)` | Instant (memory lookup), never calls the API | Data the gateway already sent. Returns `(value, ok)`. |
| `client.FetchX(...)` | Instant when cached, otherwise one API call (and cached) | The default choice. Returns `(value, error)`. |
| `client.GetX(...)` | Always calls the API (and refreshes the cache) | When you need the freshest data. |

### Guilds, channels and categories

```go
guilds := client.State.Guilds()                    // all guilds, sorted by name
guild, ok := client.State.Guild(guildID)           // guild.Channels & guild.Threads filled
guild, err := client.FetchGuild(guildID)           // cache, then GET /guilds/{id}
g, ok := client.State.FindGuild("My Server")

channel, err := client.FetchChannel(channelID)     // any channel: text, voice, category, thread, DM
channels, err := client.FetchGuildChannels(guildID) // display order, threads excluded
categories, err := client.FetchCategories(guildID)
inCategory, err := client.FetchCategoryChannels(categoryID)

tree, err := client.FetchChannelTree(guildID)      // grouped like the Discord sidebar
for _, group := range tree {
	if group.Category != nil {                     // nil = channels without a category
		fmt.Println(group.Category.Name)
	}
	for _, c := range group.Channels {
		fmt.Println("  #" + c.Name)
	}
}

client.State.TextChannels(guildID)
client.State.VoiceChannels(guildID)
client.State.Threads(guildID)
client.State.FindChannel(guildID, "#general")
client.State.PrivateChannels()                     // DMs and group DMs, most recent first
client.State.DMChannel(userID)
```

### Roles and emojis

```go
roles, err := client.FetchRoles(guildID)           // highest first
role, err := client.FetchRole(guildID, roleID)
role, guildID, ok := client.State.RoleByID(roleID) // search every guild
role, ok := client.State.FindRole(guildID, "Moderator")
memberRoles := client.State.MemberRoles(guildID, userID)

emojis, err := client.FetchEmojis(guildID)
```

### Users, members, presences and profiles

```go
user, err := client.FetchUser(userID)
user, ok := client.State.FindUser("someone")

member, err := client.FetchMember(guildID, userID) // nick, roles, joined_at...
member.DisplayName()
member.HasRole(roleID)

presence, ok := client.State.Presence(userID)      // status + activities
friends := client.State.Friends()

profile, err := client.GetUserProfile(userID, guildID) // bio, pronouns, badges, connections,
fmt.Println(profile.UserProfile.Bio, len(profile.MutualGuilds)) // Nitro, mutual guilds/friends
```

### Fetching many at once

Bulk helpers return cached entries immediately and fetch the rest **in
parallel** (`Config.FetchConcurrency` requests at a time, default 5).
Duplicate IDs are fetched once. The error joins the failures, and everything
that succeeded is still returned.

```go
users, err := client.FetchUsers([]string{id1, id2, id3})      // map[id]*types.User
channels, err := client.FetchChannels(channelIDs)
guilds, err := client.FetchGuilds(guildIDs)
profiles, err := client.FetchUserProfiles(userIDs, guildID)

// Members go over the gateway: one round trip per 100 IDs. Users who are
// not members are simply missing from the map.
members, err := client.FetchMembers(guildID, userIDs)
```

`FetchMembers` uses the gateway when connected and falls back to the profile
API otherwise. `client.SearchMembers(guildID, "prefix", 50)` searches by name,
but Discord only answers it for accounts with moderation permissions.

### Permissions

```go
perms, err := client.FetchPermissions(userID, channelID) // loads what is missing
if types.HasPermission(perms, types.PermissionSendMessages) { /* ... */ }
fmt.Println(types.PermissionNames(perms))                // ["ViewChannel", "SendMessages", ...]

perms, err = client.State.UserChannelPermissions(client.User().ID, channelID) // cache only
perms, err = client.FetchGuildPermissions(guildID, userID)
```

Permissions follow Discord's rules: owner, then `@everyone` and role
permissions, Administrator, then channel overwrites (everyone, roles,
member). Threads use their parent channel.

### Messages, invites and other data

```go
msg, err := client.FetchMessage(channelID, messageID) // cache (last 50 per channel), then API
recent := client.State.Messages(channelID)           // cached messages, oldest first
pins, err := client.GetPinnedMessages(channelID)

invite, err := client.GetInvite("discord.gg/abc")    // works for guilds you're not in
fmt.Println(invite.Guild.Name, invite.ApproximateMemberCount)
preview, err := client.GetGuildPreview(guildID)      // discoverable guilds

stats := client.State.Stats()                        // how much is cached
```

Values returned by the cache are copies, so you can change them freely.
Treat slices inside them (such as `guild.Roles`) as read-only.

## Sending messages and other REST calls

All helpers are methods on the client and return `(result, error)`.

```go
msg, err := client.SendMessage(channelID, "hello \"world\"\nmulti-line works")
client.Reply(msg, "replying without a ping")
client.ReplyMessage(channelID, messageID, "reply by IDs")
client.EditMessage(channelID, msg.ID, "edited")
client.DeleteMessage(channelID, msg.ID)

client.SendMessageComplex(channelID, &types.MessageSend{
	Content:         "silent message",
	Flags:           types.MessageFlagSuppressNotifications,
	AllowedMentions: &types.AllowedMentions{Parse: []string{}},
})

history, err := client.GetMessages(channelID, 50, "", "", "") // limit, before, after, around
one, err := client.GetMessage(channelID, messageID)

client.AddReaction(channelID, messageID, "👍")
client.AddReaction(channelID, messageID, "<:custom:123456789012345678>")
client.RemoveReaction(channelID, messageID, "👍")
client.TriggerTyping(channelID)

dm, err := client.CreateDM(userID)
client.SendMessage(dm.ID, "hi in DMs")

ch, err := client.GetChannel(channelID)
guild, err := client.GetGuild(guildID)
guilds, err := client.GetGuilds()
channels, err := client.GetGuildChannels(guildID)
me, err := client.GetCurrentUser()
user, err := client.GetUser(userID)
friends, err := client.GetRelationships()
client.LeaveGuild(guildID)
```

For an endpoint without a helper, use `Request`. It sends the same headers
and handles rate limits the same way:

```go
var connections []map[string]any
err := client.Request("GET", "/users/@me/connections", nil, &connections)

// Pin a message (payload nil, no response body).
err = client.Request("PUT", "/channels/"+channelID+"/pins/"+messageID, nil, nil)
```

REST calls do not need a gateway connection. You can create a client and call
them without `Open` (interactions are the exception, see below).

## Buttons, select menus and slash commands

Interactions use the gateway session ID, so call them after `Open`. Each one
returns a **nonce**. Discord then sends `INTERACTION_SUCCESS` or
`INTERACTION_FAILURE` with the same nonce.

```go
client.AddHandler(types.GatewayEventMessageCreate, func(m *types.MessageEventData) {
	if b := m.FindButton("Claim"); b != nil && !b.Disabled {
		nonce, err := client.ClickButton(m, b.CustomID)
		log.Println("clicked", nonce, err)
	}

	for _, menu := range m.SelectMenus() {
		// String selects take option values; user/role/channel selects take IDs.
		if menu.Type == types.ComponentStringSelect && len(menu.Options) > 0 {
			client.SelectMenu(m, menu.CustomID, []string{menu.Options[0].Value})
		}
	}
})

client.AddHandler(types.GatewayEventInteractionSuccess, func(e *types.InteractionEventData) {
	log.Println("interaction succeeded:", e.Nonce)
})
```

Slash commands:

```go
index, err := client.GetSlashCommands(guildID)       // guild commands
// client.GetChannelSlashCommands(dmChannelID)       // commands usable in a DM
// client.GetUserSlashCommands()                      // user-installed apps

cmd := index.Find("say", "")                          // name, optional application ID
nonce, err := client.SendSlashCommand(channelID, guildID, cmd,
	types.ApplicationCommandInteractionOption{Type: types.CommandOptionString, Name: "text", Value: "hello"},
)
```

For sub-commands, pass an option of type `types.CommandOptionSubCommand` whose
`Options` hold the sub-command arguments.

## Presence, members and voice

```go
client.UpdatePresence(types.StatusDND, types.Activity{Name: "Go", Type: types.ActivityPlaying})
client.SetCustomStatus("brb", "☕")

// Low-level member request: the answer arrives as GUILD_MEMBERS_CHUNK events.
// (FetchMembers does this for you and returns the members directly.)
client.AddHandler(types.GatewayGuildMembersChunk, func(e *types.GuildMembersChunkEventData) {
	for _, m := range e.Members {
		log.Println(m.User.Tag(), m.Nick, m.Roles)
	}
})
client.RequestGuildMembers(guildID, []string{userID1, userID2}, true)

client.UpdateVoiceState(guildID, voiceChannelID, false, true) // join (self-deafened)
client.UpdateVoiceState(guildID, "", false, false)            // leave

client.SendOp(14, map[string]any{ /* any other gateway opcode */ })
```

The presence set with `UpdatePresence` is sent again automatically after a
reconnect.

## Configuration

`NewClient(token, nil)` uses `types.DefaultConfig`, which identifies as Chrome
on Windows. To change it, start from a copy:

```go
config := types.NewConfig()     // copy of DefaultConfig, safe to modify
config.Presence = types.StatusInvisible
config.Logger = slog.New(slog.NewTextHandler(os.Stderr, nil))
client := goself.NewClient(token, config)
```

| Field | Default | Description |
| --- | --- | --- |
| `Presence` | `"offline"` | Status sent when identifying: `online`, `idle`, `dnd`, `invisible`, `offline`. |
| `ApiVersion` | `"9"` | API and gateway version. |
| `Browser`, `BrowserVersion`, `Os`, `OsVersion`, `Device`, `UserAgent` | Chrome 131 / Windows 10 | Client fingerprint sent in the super properties and HTTP headers. Keep them consistent with each other. |
| `Capabilities` | `4093` | Gateway capabilities bit field. |
| `BuildNumber` | `0` (auto) | Web client build number. `0` scrapes it from discord.com once per process and falls back to `discord.FallbackBuildNumber`. |
| `Locale` | system locale | `X-Discord-Locale` header, e.g. `en-US`. |
| `Timezone` | `$TZ` or `UTC` | `X-Discord-Timezone` header, e.g. `Europe/Berlin`. |
| `GatewayURL`, `APIBaseURL` | Discord's | Override endpoints (useful for tests or proxies). |
| `SyncEvents` | `false` | Run handlers sequentially on the reading goroutine. |
| `MaxRESTRetries` | `3` | Retries after HTTP 429 (and 5xx on GET). Negative disables retries. |
| `DisableState` | `false` | Turn off the cache. `Fetch*` then always calls the API. |
| `StateMaxMessages` | `50` | Recent messages cached per channel. Negative disables the message cache. |
| `FetchConcurrency` | `5` | Parallel requests used by the bulk `Fetch*` helpers. |
| `Logger` | `nil` | A `*slog.Logger` for connection, reconnect and error logs. |

The gateway honors the `HTTPS_PROXY` / `HTTP_PROXY` environment variables.

## Error handling

```go
_, err := client.SendMessage(channelID, "hi")

var restErr *discord.RESTError
if errors.As(err, &restErr) {
	log.Println(restErr.StatusCode, restErr.Code, restErr.Message) // e.g. 403 50001 Missing Access
}

err = client.Connect()
var closeErr *discord.CloseError
if errors.As(err, &closeErr) && closeErr.Code == 4004 {
	log.Fatal("invalid token")
}
```

Other sentinel errors: `discord.ErrEmptyToken`, `discord.ErrAlreadyOpen`,
`discord.ErrNotOpen`, `discord.ErrNotConnected`, `discord.ErrClosed`.

## Running the tests

```sh
go test ./...
```

The unit tests run offline against a mock gateway and a mock REST server.
They cover identify, resume, heartbeats, zombie detection, invalid sessions,
auth failure, payload encoding, event decoding, the state cache (user-account
READY, live updates, message cache), permission computation, cache-first and
parallel fetching, and member batches over the gateway. Live tests against
Discord run only when a token is provided:

```sh
DISCORD_TOKEN="your-token" go test -run Live -v .
# or put the token in a file named test_token (it is git-ignored)
```

## Upgrading from older versions

| Before | Now |
| --- | --- |
| `discord.SendMessage(client.Gateway, ch, text) bool` | `client.SendMessage(ch, text)` returns `(*types.Message, error)` (the old function still works but is deprecated). |
| `discord.ClickButton(client.Gateway, msg, id) bool` | `client.ClickButton(msg, id)` returns `(nonce, error)`. |
| `discord.GetSlashCommands(client.Gateway, guild)` | `client.GetSlashCommands(guild)` |
| `row.Buttons` on `types.MessageComponent` | `row.Components`, or `msg.Buttons()` / `msg.FindButton(...)` to search the whole message. |
| `gateway.SessionID`, `gateway.LastSeq` fields | `client.SessionID()`, `client.Sequence()` methods. |
| `client.Selfbot.User` (racy) | `client.User()` |
| `MessageData.ReferencedMessage` was a `MessageReference` | The reference is now `MessageData.MessageReference`. `MessageEventData.ReferencedMessage` is the replied-to message. |
| `Nonce string` | `Nonce types.FlexString` (accepts string or integer nonces). |
| `GuildMembersChunkEventData` anonymous structs | Named `types.GuildMember` / `types.PresenceUpdateEventData`. Use `m.User.ID` instead of `m.User.Id`. |
| `Presence.Activities []any` | `[]types.Activity` |
| `SuperProperties.ClientBuildNumber string` | `int`, as Discord expects. |
| Importing the package did a network request and could `panic` | Nothing happens at import time. The build number is fetched lazily, with a fallback. |

## Credits

GoSelf started as a fork of
[discordgo-self](https://github.com/QuartzWarrior/discordgo-self) by skifli,
Luminal and QuartzWarrior. It has since been largely rewritten and extended.

## License

[MIT](LICENSE)
