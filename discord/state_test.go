package discord

import (
	"fmt"
	"testing"

	"github.com/goccy/go-json"
	"github.com/headd16/goself/types"
)

// feed sends a dispatch through the state cache like the gateway does.
func feed(t *testing.T, g *Gateway, name string, data any) any {
	t.Helper()
	b, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	return g.State.onEvent(g, &types.RawEvent{Op: 0, Type: name, Data: b})
}

const (
	gID       = "100"
	meID      = "1"
	friendID  = "2"
	modRole   = "300"
	adminRole = "301"
	catID     = "400"
	textID    = "401"
	voiceID   = "402"
	secretID  = "403"
	looseID   = "404"
)

// userReady mimics a user-account READY: guild fields nested in
// "properties", channels without guild_id, deduplicated users and
// merged_members with user_id instead of user objects.
func userReady() map[string]any {
	return map[string]any{
		"v": 9, "session_id": "s", "resume_gateway_url": "wss://x",
		"user":  map[string]any{"id": meID, "username": "me"},
		"users": []any{map[string]any{"id": friendID, "username": "friend", "global_name": "Friend"}},
		"relationships": []any{
			map[string]any{"id": friendID, "type": 1, "user_id": friendID},
		},
		"private_channels": []any{
			map[string]any{"id": "500", "type": 1, "recipient_ids": []string{friendID}, "last_message_id": "900"},
		},
		"guilds": []any{map[string]any{
			"id":         gID,
			"properties": map[string]any{"id": gID, "name": "Test Guild", "owner_id": "999"},
			"roles": []any{
				map[string]any{"id": gID, "name": "@everyone", "position": 0, "permissions": fmt.Sprint(types.PermissionViewChannel | types.PermissionSendMessages)},
				map[string]any{"id": modRole, "name": "Mod", "position": 1, "permissions": fmt.Sprint(types.PermissionManageMessages)},
				map[string]any{"id": adminRole, "name": "Admin", "position": 2, "permissions": fmt.Sprint(types.PermissionAdministrator)},
			},
			"emojis": []any{map[string]any{"id": "600", "name": "blob"}},
			"channels": []any{
				map[string]any{"id": voiceID, "type": 2, "name": "Voice", "position": 0, "parent_id": catID},
				map[string]any{"id": textID, "type": 0, "name": "general", "position": 1, "parent_id": catID},
				map[string]any{"id": catID, "type": 4, "name": "Main", "position": 0},
				map[string]any{"id": looseID, "type": 0, "name": "rules", "position": 0},
				map[string]any{"id": secretID, "type": 0, "name": "secret", "position": 2, "parent_id": catID,
					"permission_overwrites": []any{
						map[string]any{"id": gID, "type": 0, "allow": "0", "deny": fmt.Sprint(types.PermissionViewChannel)},
						map[string]any{"id": modRole, "type": 0, "allow": fmt.Sprint(types.PermissionViewChannel), "deny": "0"},
					}},
			},
			"threads": []any{map[string]any{"id": "405", "type": 11, "name": "a thread", "parent_id": secretID}},
		}},
		"merged_members": []any{[]any{
			map[string]any{"user_id": meID, "roles": []string{modRole}, "joined_at": "2020-01-01T00:00:00Z"},
		}},
	}
}

func readyGateway(t *testing.T) *Gateway {
	t.Helper()
	g := NewGateway("token", testConfig("", ""))
	v := feed(t, g, "READY", userReady())
	if r, ok := v.(*types.ReadyEventData); !ok || r.User.ID != meID {
		t.Fatalf("READY should return *ReadyEventData for the handlers, got %T", v)
	}
	return g
}

func TestStateReadyUserAccount(t *testing.T) {
	g := readyGateway(t)
	s := g.State

	guild, ok := s.Guild(gID)
	if !ok || guild.Name != "Test Guild" || guild.OwnerID != "999" {
		t.Fatalf("guild = %+v", guild)
	}
	if len(guild.Channels) != 5 || len(guild.Threads) != 1 {
		t.Errorf("guild channels=%d threads=%d", len(guild.Channels), len(guild.Threads))
	}
	if c, ok := s.Channel(textID); !ok || c.GuildID != gID {
		t.Errorf("channel guild_id not filled: %+v", c)
	}

	// Display order: text-like before voice, then by position.
	var names []string
	for _, c := range s.GuildChannels(gID) {
		names = append(names, c.Name)
	}
	if fmt.Sprint(names) != "[Main rules general secret Voice]" {
		t.Errorf("channel order = %v", names)
	}
	if cats := s.Categories(gID); len(cats) != 1 || cats[0].ID != catID {
		t.Errorf("categories = %+v", cats)
	}
	if inCat := s.CategoryChannels(catID); len(inCat) != 3 || inCat[2].ID != voiceID {
		t.Errorf("category channels = %+v", inCat)
	}
	tree := s.ChannelTree(gID)
	if len(tree) != 2 || tree[0].Category != nil || tree[0].Channels[0].ID != looseID ||
		tree[1].Category.ID != catID || len(tree[1].Channels) != 3 {
		t.Errorf("channel tree = %+v", tree)
	}
	if c, ok := s.FindChannel(gID, "#General"); !ok || c.ID != textID {
		t.Errorf("FindChannel = %+v", c)
	}
	if len(s.TextChannels(gID)) != 3 || len(s.VoiceChannels(gID)) != 1 || len(s.Threads(gID)) != 1 {
		t.Error("text/voice/thread filters are wrong")
	}

	roles := s.Roles(gID)
	if len(roles) != 3 || roles[0].ID != adminRole || roles[2].ID != gID {
		t.Errorf("roles should be sorted highest first: %+v", roles)
	}
	if r, guildID, ok := s.RoleByID(modRole); !ok || r.Name != "Mod" || guildID != gID {
		t.Errorf("RoleByID = %+v %s", r, guildID)
	}
	if r, ok := s.FindRole(gID, "@mod"); !ok || r.ID != modRole {
		t.Errorf("FindRole = %+v", r)
	}
	if e, ok := s.Emoji(gID, "600"); !ok || e.Name != "blob" {
		t.Errorf("emoji = %+v", e)
	}

	me, ok := s.Member(gID, meID)
	if !ok || me.User.Username != "me" || !me.HasRole(modRole) {
		t.Errorf("own member not resolved: %+v", me)
	}
	if mr := s.MemberRoles(gID, meID); len(mr) != 1 || mr[0].ID != modRole {
		t.Errorf("member roles = %+v", mr)
	}

	if u, ok := s.User(friendID); !ok || u.DisplayName() != "Friend" {
		t.Errorf("deduplicated user missing: %+v", u)
	}
	dms := s.PrivateChannels()
	if len(dms) != 1 || len(dms[0].Recipients) != 1 || dms[0].Recipients[0].Username != "friend" {
		t.Errorf("DM recipients not resolved: %+v", dms)
	}
	if dm, ok := s.DMChannel(friendID); !ok || dm.ID != "500" {
		t.Errorf("DMChannel = %+v", dm)
	}
	if f := s.Friends(); len(f) != 1 || f[0].ID != friendID {
		t.Errorf("friends = %+v", f)
	}
	if s.Me().ID != meID {
		t.Errorf("Me = %+v", s.Me())
	}
	st := s.Stats()
	if st.Guilds != 1 || st.Channels != 7 || st.Roles != 3 || st.Members != 1 {
		t.Errorf("stats = %+v", st)
	}
}

func TestStatePermissions(t *testing.T) {
	g := readyGateway(t)
	s := g.State

	// Mod: @everyone can view+send, Mod adds manage messages, and the role
	// overwrite unlocks #secret.
	perms, err := s.UserChannelPermissions(meID, secretID)
	if err != nil {
		t.Fatal(err)
	}
	want := types.PermissionViewChannel | types.PermissionSendMessages | types.PermissionManageMessages
	if perms != want {
		t.Errorf("mod perms in #secret = %v, want %v", types.PermissionNames(perms), types.PermissionNames(want))
	}
	// Threads use the parent's overwrites.
	if p, _ := s.UserChannelPermissions(meID, "405"); p != want {
		t.Errorf("thread perms = %v", types.PermissionNames(p))
	}

	// A plain member cannot see #secret at all.
	feed(t, g, "GUILD_MEMBER_ADD", map[string]any{"guild_id": gID, "user": map[string]any{"id": "7", "username": "pleb"}, "roles": []string{}})
	if p, _ := s.UserChannelPermissions("7", secretID); p != 0 {
		t.Errorf("pleb sees #secret: %v", types.PermissionNames(p))
	}
	if p, _ := s.UserChannelPermissions("7", textID); !types.HasPermission(p, types.PermissionSendMessages) {
		t.Errorf("pleb cannot write in #general: %v", types.PermissionNames(p))
	}

	// A member overwrite beats role overwrites.
	feed(t, g, "CHANNEL_UPDATE", map[string]any{"id": textID, "guild_id": gID, "type": 0, "name": "general",
		"permission_overwrites": []any{map[string]any{"id": "7", "type": 1, "allow": "0", "deny": fmt.Sprint(types.PermissionSendMessages)}}})
	if p, _ := s.UserChannelPermissions("7", textID); types.HasPermission(p, types.PermissionSendMessages) {
		t.Error("member overwrite deny ignored")
	}

	// Admins and the owner get everything.
	feed(t, g, "GUILD_MEMBER_ADD", map[string]any{"guild_id": gID, "user": map[string]any{"id": "8", "username": "admin"}, "roles": []string{adminRole}})
	feed(t, g, "GUILD_MEMBER_ADD", map[string]any{"guild_id": gID, "user": map[string]any{"id": "999", "username": "owner"}, "roles": []string{}})
	for _, id := range []string{"8", "999"} {
		if p, _ := s.UserPermissions(gID, id); p != types.PermissionAll {
			t.Errorf("user %s guild perms = %d, want all", id, p)
		}
	}

	if _, err := s.UserChannelPermissions("12345", textID); err != ErrMemberNotCached {
		t.Errorf("unknown member err = %v", err)
	}
	if p, _ := s.UserChannelPermissions(meID, "500"); p != types.PermissionsDM {
		t.Errorf("DM perms = %v", types.PermissionNames(p))
	}
}

func TestStateLiveUpdates(t *testing.T) {
	g := readyGateway(t)
	s := g.State

	// Roles
	feed(t, g, "GUILD_ROLE_CREATE", map[string]any{"guild_id": gID, "role": map[string]any{"id": "302", "name": "New", "position": 3, "permissions": "0"}})
	feed(t, g, "GUILD_ROLE_UPDATE", map[string]any{"guild_id": gID, "role": map[string]any{"id": modRole, "name": "Moderator", "position": 1, "permissions": "0"}})
	feed(t, g, "GUILD_ROLE_DELETE", map[string]any{"guild_id": gID, "role_id": adminRole})
	roles := s.Roles(gID)
	if len(roles) != 3 || roles[0].Name != "New" || roles[1].Name != "Moderator" {
		t.Errorf("roles after updates = %+v", roles)
	}
	if _, _, ok := s.RoleByID(adminRole); ok {
		t.Error("deleted role still indexed")
	}

	// Channels
	feed(t, g, "CHANNEL_CREATE", map[string]any{"id": "410", "guild_id": gID, "type": 0, "name": "new-channel", "parent_id": catID, "position": 9})
	if len(s.CategoryChannels(catID)) != 4 {
		t.Error("CHANNEL_CREATE not cached")
	}
	feed(t, g, "CHANNEL_DELETE", map[string]any{"id": "410", "guild_id": gID, "type": 0})
	if _, ok := s.Channel("410"); ok {
		t.Error("CHANNEL_DELETE not applied")
	}

	// Messages: author and member are cached, partial updates merge, deletes remove.
	v := feed(t, g, "MESSAGE_CREATE", map[string]any{
		"id": "1000", "channel_id": textID, "guild_id": gID, "content": "hello",
		"author": map[string]any{"id": "9", "username": "talker"},
		"member": map[string]any{"roles": []string{modRole}, "nick": "Talky", "joined_at": "2021-01-01T00:00:00Z"},
	})
	if _, ok := v.(*types.MessageEventData); !ok {
		t.Fatalf("MESSAGE_CREATE should return *MessageEventData, got %T", v)
	}
	if m, ok := s.Member(gID, "9"); !ok || m.DisplayName() != "Talky" || m.User.Username != "talker" {
		t.Errorf("message member not cached: %+v", m)
	}
	if c, _ := s.Channel(textID); c.LastMessageID != "1000" {
		t.Errorf("last_message_id = %q", c.LastMessageID)
	}
	feed(t, g, "MESSAGE_UPDATE", map[string]any{"id": "1000", "channel_id": textID, "content": "edited", "edited_timestamp": "2024"})
	if m, ok := s.Message(textID, "1000"); !ok || m.Content != "edited" || m.Author.ID != "9" {
		t.Errorf("partial update not merged: %+v", m)
	}
	feed(t, g, "MESSAGE_DELETE", map[string]any{"id": "1000", "channel_id": textID})
	if _, ok := s.Message(textID, "1000"); ok {
		t.Error("MESSAGE_DELETE not applied")
	}

	// Presence (READY_SUPPLEMENTAL style with user_id, then a live update).
	feed(t, g, "READY_SUPPLEMENTAL", map[string]any{
		"guilds":           []any{map[string]any{"id": gID}},
		"merged_members":   []any{[]any{map[string]any{"user_id": friendID, "roles": []string{}, "joined_at": "x"}}},
		"merged_presences": map[string]any{"guilds": []any{[]any{}}, "friends": []any{map[string]any{"user_id": friendID, "status": "idle"}}},
	})
	if p, ok := s.Presence(friendID); !ok || p.Status != "idle" || p.User.Username != "friend" {
		t.Errorf("supplemental presence = %+v", p)
	}
	if _, ok := s.Member(gID, friendID); !ok {
		t.Error("supplemental member not cached")
	}
	feed(t, g, "PRESENCE_UPDATE", map[string]any{"user": map[string]any{"id": friendID}, "status": "dnd", "activities": []any{map[string]any{"name": "Go", "type": 0}}})
	if p, _ := s.Presence(friendID); p.Status != "dnd" || len(p.Activities) != 1 {
		t.Errorf("presence = %+v", p)
	}

	// Guild removal drops everything that belongs to it.
	feed(t, g, "GUILD_DELETE", map[string]any{"id": gID})
	if _, ok := s.Guild(gID); ok || len(s.GuildChannels(gID)) != 0 || len(s.Members(gID)) != 0 {
		t.Error("GUILD_DELETE left data behind")
	}
	if _, ok := s.Channel(textID); ok {
		t.Error("guild channel survived GUILD_DELETE")
	}
}

func TestStateMessageCacheOrderAndLimit(t *testing.T) {
	cfg := testConfig("", "")
	cfg.StateMaxMessages = 3
	g := NewGateway("token", cfg)
	for _, id := range []string{"105", "101", "103", "104", "102"} {
		g.State.storeMessages(&types.Message{MessageData: types.MessageData{ID: id, ChannelID: "c", Author: types.User{ID: "1", Username: "a"}}})
	}
	var ids []string
	for _, m := range g.State.Messages("c") {
		ids = append(ids, m.ID)
	}
	if fmt.Sprint(ids) != "[103 104 105]" {
		t.Errorf("cache should keep the newest 3 in order, got %v", ids)
	}
}

func TestStateReturnsCopies(t *testing.T) {
	g := readyGateway(t)
	guild, _ := g.State.Guild(gID)
	guild.Name = "changed"
	if again, _ := g.State.Guild(gID); again.Name != "Test Guild" {
		t.Error("mutating a returned guild changed the cache")
	}
	u, _ := g.State.User(friendID)
	u.Username = "changed"
	if again, _ := g.State.User(friendID); again.Username != "friend" {
		t.Error("mutating a returned user changed the cache")
	}
}

func TestStateDisabled(t *testing.T) {
	cfg := testConfig("", "")
	cfg.DisableState = true
	g := NewGateway("token", cfg)
	if v := feed(t, g, "READY", userReady()); v != nil {
		t.Errorf("disabled state returned %T", v)
	}
	if _, ok := g.State.Guild(gID); ok || g.State.Enabled() {
		t.Error("disabled state cached data")
	}
}

func TestChannelTreeWithoutUncategorized(t *testing.T) {
	tree := types.BuildChannelTree([]types.Channel{
		{ID: "2", Type: types.ChannelGuildText, ParentID: "1", Position: 1},
		{ID: "1", Type: types.ChannelGuildCategory},
	})
	if len(tree) != 1 || tree[0].Category == nil || len(tree[0].Channels) != 1 {
		t.Errorf("tree = %+v", tree)
	}
}
