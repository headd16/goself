// Fetch example: read guilds, channels, categories, roles, members, users,
// profiles, permissions and invites. Most answers come straight from the
// state cache filled by the gateway, so they are instant.
//
//	# overview of every guild
//	DISCORD_TOKEN=your-token go run github.com/headd16/goself/examples/fetch@latest
//
//	# details of one guild (channel tree, roles, your permissions)
//	DISCORD_TOKEN=your-token go run github.com/headd16/goself/examples/fetch@latest -guild 111111111111111111
//
//	# members and profiles by user ID, an invite lookup
//	DISCORD_TOKEN=your-token go run github.com/headd16/goself/examples/fetch@latest -guild 111 -users 222,333 -invite discord.gg/abc
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
	guildID := flag.String("guild", "", "guild ID to inspect")
	users := flag.String("users", "", "comma separated user IDs to fetch")
	invite := flag.String("invite", "", "invite code or link to resolve")
	flag.Parse()

	token := os.Getenv("DISCORD_TOKEN")
	if token == "" {
		log.Fatal("set the DISCORD_TOKEN environment variable")
	}

	client := goself.NewClient(token, nil)
	start := time.Now()
	if err := client.Open(); err != nil { // READY fills the cache
		log.Fatal(err)
	}
	defer client.Close()

	me := client.State.Me()
	st := client.State.Stats()
	fmt.Printf("Logged in as %s in %s\n", me.Tag(), time.Since(start).Round(time.Millisecond))
	fmt.Printf("Cached: %d guilds, %d channels, %d roles, %d users, %d members, %d friends\n\n",
		st.Guilds, st.Channels, st.Roles, st.Users, st.Members, len(client.State.Friends()))

	if *guildID == "" {
		printGuilds(client)
	} else {
		printGuild(client, *guildID)
	}

	if *users != "" {
		printUsers(client, *guildID, strings.Split(*users, ","))
	}

	if *invite != "" {
		inv, err := client.GetInvite(*invite)
		if err != nil {
			log.Println("invite:", err)
		} else if inv.Guild != nil {
			fmt.Printf("\nInvite %s -> %s (%s): %d members, %d online\n",
				inv.Code, inv.Guild.Name, inv.Guild.ID, inv.ApproximateMemberCount, inv.ApproximatePresenceCount)
		}
	}
}

func printGuilds(client *goself.Client) {
	for _, g := range client.State.Guilds() {
		fmt.Printf("%-20s %-32s %4d channels %3d roles %6d members\n",
			g.ID, g.Name, len(client.State.GuildChannels(g.ID)), len(g.Roles), g.MemberCount)
	}
	fmt.Println("\nRun again with -guild <id> for details.")
}

func printGuild(client *goself.Client, guildID string) {
	t := time.Now()
	guild, err := client.FetchGuild(guildID) // cache first, API if needed
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("== %s (%s), owner %s, %d members ==\n", guild.Name, guild.ID, guild.OwnerID, guild.MemberCount)

	tree, err := client.FetchChannelTree(guildID)
	if err != nil {
		log.Fatal(err)
	}
	me := client.State.Me().ID
	fmt.Println("\nChannels:")
	for _, group := range tree {
		if group.Category != nil {
			fmt.Printf("  %s\n", strings.ToUpper(group.Category.Name))
		}
		for _, c := range group.Channels {
			icon := "#"
			if c.IsVoice() {
				icon = "🔊"
			}
			access := ""
			if perms, err := client.State.UserChannelPermissions(me, c.ID); err == nil && !types.HasPermission(perms, types.PermissionViewChannel) {
				access = "  (hidden from you)"
			}
			fmt.Printf("    %s %s%s\n", icon, c.Name, access)
		}
	}

	roles, err := client.FetchRoles(guildID)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("\nRoles (%d):\n", len(roles))
	for _, r := range roles {
		fmt.Printf("  %-24s %s\n", r.Name, r.ID)
	}

	if perms, err := client.FetchGuildPermissions(guildID, me); err == nil {
		fmt.Printf("\nYour permissions: %s\n", strings.Join(types.PermissionNames(perms), ", "))
	}
	fmt.Printf("\n(fetched in %s)\n", time.Since(t).Round(time.Microsecond))
}

func printUsers(client *goself.Client, guildID string, ids []string) {
	t := time.Now()
	users, err := client.FetchUsers(ids) // parallel, cache first
	if err != nil {
		log.Println("some users failed:", err)
	}
	var members map[string]*types.GuildMember
	if guildID != "" {
		members, err = client.FetchMembers(guildID, ids) // one gateway round trip per 100 IDs
		if err != nil {
			log.Println("members:", err)
		}
	}
	fmt.Printf("\nUsers (fetched in %s):\n", time.Since(t).Round(time.Millisecond))
	for _, id := range ids {
		u, ok := users[strings.TrimSpace(id)]
		if !ok {
			continue
		}
		line := fmt.Sprintf("  %-20s %-24s", u.ID, u.Tag())
		if m, ok := members[u.ID]; ok {
			var names []string
			for _, r := range client.State.MemberRoles(guildID, u.ID) {
				names = append(names, r.Name)
			}
			line += fmt.Sprintf(" nick=%q roles=%v", m.Nick, names)
		}
		if p, ok := client.State.Presence(u.ID); ok {
			line += " status=" + p.Status
		}
		fmt.Println(line)

		if profile, err := client.GetUserProfile(u.ID, guildID); err == nil {
			if profile.UserProfile.Bio != "" {
				fmt.Printf("      bio: %s\n", strings.ReplaceAll(profile.UserProfile.Bio, "\n", " "))
			}
			fmt.Printf("      mutual guilds: %d, connections: %d\n", len(profile.MutualGuilds), len(profile.ConnectedAccounts))
		}
	}
}
