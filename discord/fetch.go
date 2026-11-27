package discord

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/goccy/go-json"
	"github.com/headd16/goself/types"
)

// Fetch helpers
//
// Get* methods always call the Discord API (and refresh the state cache).
// Fetch* methods return cached data instantly when it is available and only
// call the API for what is missing. Bulk helpers (FetchUsers, FetchChannels,
// FetchGuilds, FetchMembers) run requests in parallel.

// ErrMemberNotFound is returned when the user is not a member of the guild.
var ErrMemberNotFound = errors.New("discord: user is not a member of this guild")

const (
	memberChunkTimeout   = 10 * time.Second
	maxMemberIDsPerBatch = 100
)

// ---------------------------------------------------------------------------
// Extra REST endpoints
// ---------------------------------------------------------------------------

// GetRoles returns the roles of a guild from the API.
func (g *Gateway) GetRoles(guildID string) ([]types.Role, error) {
	var roles []types.Role
	if err := g.request("GET", "/guilds/"+guildID+"/roles", nil, &roles, discordOrigin+"/channels/"+guildID); err != nil {
		return nil, err
	}
	g.State.storeRoles(guildID, roles)
	types.SortRoles(roles)
	return roles, nil
}

// GetEmojis returns the custom emojis of a guild from the API.
func (g *Gateway) GetEmojis(guildID string) ([]types.Emoji, error) {
	var emojis []types.Emoji
	if err := g.request("GET", "/guilds/"+guildID+"/emojis", nil, &emojis, discordOrigin+"/channels/"+guildID); err != nil {
		return nil, err
	}
	g.State.storeEmojis(guildID, emojis)
	return emojis, nil
}

// GetUserProfile returns the full profile of a user: bio, pronouns, badges,
// connected accounts, Nitro info, mutual guilds and friends. With a guildID,
// the guild member and guild profile are included too.
func (g *Gateway) GetUserProfile(userID, guildID string) (*types.UserProfile, error) {
	q := url.Values{}
	q.Set("with_mutual_guilds", "true")
	q.Set("with_mutual_friends", "true")
	q.Set("with_mutual_friends_count", "true")
	if guildID != "" {
		q.Set("guild_id", guildID)
	}
	var p types.UserProfile
	if err := g.request("GET", "/users/"+userID+"/profile?"+q.Encode(), nil, &p, ""); err != nil {
		return nil, err
	}
	g.State.storeUsers(&p.User)
	if guildID != "" && p.GuildMember != nil {
		m := *p.GuildMember
		if m.User.ID == "" {
			m.User = p.User
		}
		p.GuildMember = &m
		g.State.storeMembers(guildID, &m)
	}
	return &p, nil
}

// GetInvite resolves an invite code or link ("abc", "discord.gg/abc",
// "https://discord.com/invite/abc"). The result includes the guild name,
// icon, features and approximate member counts, even for guilds you are not in.
func (g *Gateway) GetInvite(codeOrURL string) (*types.Invite, error) {
	code := inviteCode(codeOrURL)
	var inv types.Invite
	path := "/invites/" + url.PathEscape(code) + "?with_counts=true&with_expiration=true"
	if err := g.request("GET", path, nil, &inv, ""); err != nil {
		return nil, err
	}
	return &inv, nil
}

func inviteCode(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(s, "/")
	if i := strings.LastIndexByte(s, '/'); i >= 0 {
		s = s[i+1:]
	}
	if i := strings.IndexAny(s, "?#"); i >= 0 {
		s = s[:i]
	}
	return s
}

// GetGuildPreview returns the public preview of a discoverable guild.
func (g *Gateway) GetGuildPreview(guildID string) (*types.GuildPreview, error) {
	var p types.GuildPreview
	if err := g.request("GET", "/guilds/"+guildID+"/preview", nil, &p, ""); err != nil {
		return nil, err
	}
	return &p, nil
}

// GetPinnedMessages returns the pinned messages of a channel.
func (g *Gateway) GetPinnedMessages(channelID string) ([]types.Message, error) {
	var msgs []types.Message
	if err := g.request("GET", "/channels/"+channelID+"/pins", nil, &msgs, channelReferer("", channelID)); err != nil {
		return nil, err
	}
	return msgs, nil
}

// ---------------------------------------------------------------------------
// Cache-first fetches
// ---------------------------------------------------------------------------

// FetchGuild returns a guild from the cache, or from the API.
func (g *Gateway) FetchGuild(guildID string) (*types.Guild, error) {
	if guild, ok := g.State.Guild(guildID); ok && guild.Name != "" {
		return guild, nil
	}
	return g.GetGuild(guildID)
}

// FetchChannel returns a channel (category, thread or DM) from the cache, or
// from the API.
func (g *Gateway) FetchChannel(channelID string) (*types.Channel, error) {
	if c, ok := g.State.Channel(channelID); ok {
		return c, nil
	}
	return g.GetChannel(channelID)
}

// FetchGuildChannels returns every channel and category of a guild in
// display order (threads excluded), from the cache or the API.
func (g *Gateway) FetchGuildChannels(guildID string) ([]types.Channel, error) {
	if channels := g.State.GuildChannels(guildID); len(channels) > 0 {
		return channels, nil
	}
	channels, err := g.GetGuildChannels(guildID)
	if err != nil {
		return nil, err
	}
	out := make([]types.Channel, 0, len(channels))
	for _, c := range channels {
		if !c.IsThread() {
			out = append(out, c)
		}
	}
	types.SortChannels(out)
	return out, nil
}

// FetchCategories returns the categories of a guild in display order.
func (g *Gateway) FetchCategories(guildID string) ([]types.Channel, error) {
	channels, err := g.FetchGuildChannels(guildID)
	if err != nil {
		return nil, err
	}
	var out []types.Channel
	for _, c := range channels {
		if c.IsCategory() {
			out = append(out, c)
		}
	}
	return out, nil
}

// FetchCategoryChannels returns the channels inside a category.
func (g *Gateway) FetchCategoryChannels(categoryID string) ([]types.Channel, error) {
	cat, err := g.FetchChannel(categoryID)
	if err != nil {
		return nil, err
	}
	if !cat.IsCategory() {
		return nil, fmt.Errorf("discord: channel %s is not a category", categoryID)
	}
	channels, err := g.FetchGuildChannels(cat.GuildID)
	if err != nil {
		return nil, err
	}
	var out []types.Channel
	for _, c := range channels {
		if c.ParentID == categoryID {
			out = append(out, c)
		}
	}
	return out, nil
}

// FetchChannelTree returns the channels of a guild grouped by category, as
// in the Discord sidebar.
func (g *Gateway) FetchChannelTree(guildID string) ([]types.ChannelCategory, error) {
	channels, err := g.FetchGuildChannels(guildID)
	if err != nil {
		return nil, err
	}
	return types.BuildChannelTree(channels), nil
}

// FetchRoles returns the roles of a guild (highest first), from the cache or
// the API.
func (g *Gateway) FetchRoles(guildID string) ([]types.Role, error) {
	if roles := g.State.Roles(guildID); len(roles) > 0 {
		return roles, nil
	}
	return g.GetRoles(guildID)
}

// FetchRole returns one role of a guild.
func (g *Gateway) FetchRole(guildID, roleID string) (*types.Role, error) {
	if r, ok := g.State.Role(guildID, roleID); ok {
		return r, nil
	}
	roles, err := g.GetRoles(guildID)
	if err != nil {
		return nil, err
	}
	for i := range roles {
		if roles[i].ID == roleID {
			return &roles[i], nil
		}
	}
	return nil, &RESTError{Method: "GET", Path: "/guilds/" + guildID + "/roles", StatusCode: 404, Code: 10011, Message: "Unknown Role"}
}

// FetchEmojis returns the custom emojis of a guild.
func (g *Gateway) FetchEmojis(guildID string) ([]types.Emoji, error) {
	if guild, ok := g.State.Guild(guildID); ok && guild.Emojis != nil {
		return guild.Emojis, nil
	}
	return g.GetEmojis(guildID)
}

// FetchUser returns a user from the cache, or from the API.
func (g *Gateway) FetchUser(userID string) (*types.User, error) {
	if u, ok := g.State.User(userID); ok {
		return u, nil
	}
	return g.GetUser(userID)
}

// FetchMessage returns a message from the cache, or from the API.
func (g *Gateway) FetchMessage(channelID, messageID string) (*types.Message, error) {
	if m, ok := g.State.Message(channelID, messageID); ok {
		return m, nil
	}
	return g.GetMessage(channelID, messageID)
}

// FetchMember returns a guild member from the cache, or asks the gateway for
// it (or the profile API when the gateway is not connected).
func (g *Gateway) FetchMember(guildID, userID string) (*types.GuildMember, error) {
	members, err := g.FetchMembers(guildID, []string{userID})
	if m, ok := members[userID]; ok {
		return m, nil
	}
	if err != nil {
		return nil, err
	}
	return nil, ErrMemberNotFound
}

// FetchMembers returns guild members by user ID. Cached members are returned
// immediately; the others are requested over the gateway in batches of 100
// (one round trip per batch), or through the profile API when the gateway is
// not connected. Users that are not members are simply absent from the result.
func (g *Gateway) FetchMembers(guildID string, userIDs []string) (map[string]*types.GuildMember, error) {
	result := make(map[string]*types.GuildMember, len(userIDs))
	var missing []string
	for _, id := range uniqueIDs(userIDs) {
		if m, ok := g.State.Member(guildID, id); ok {
			result[id] = m
		} else {
			missing = append(missing, id)
		}
	}
	if len(missing) == 0 {
		return result, nil
	}

	if !g.IsConnected() {
		found, err := fetchMany(g, missing, func(id string) (*types.GuildMember, error) {
			p, err := g.GetUserProfile(id, guildID)
			if err != nil {
				return nil, err
			}
			if p.GuildMember == nil {
				return nil, ErrMemberNotFound
			}
			return p.GuildMember, nil
		})
		for id, m := range found {
			result[id] = m
		}
		return result, ignoreNotFound(err)
	}

	var errs []error
	for start := 0; start < len(missing); start += maxMemberIDsPerBatch {
		batch := missing[start:min(start+maxMemberIDsPerBatch, len(missing))]
		chunk, err := g.requestMembers(types.RequestGuildMembersPayload{
			GuildID: guildID, UserIDs: batch, Presences: true,
		})
		for i := range chunk {
			m := chunk[i]
			result[m.ID()] = &m
		}
		if err != nil {
			errs = append(errs, err)
		}
	}
	return result, errors.Join(errs...)
}

// SearchMembers asks the gateway for members whose username or nickname
// starts with query (at most limit, 1-100). Discord only answers this for
// accounts with moderation permissions (kick, ban or manage roles) in the
// guild; otherwise it times out with no results.
func (g *Gateway) SearchMembers(guildID, query string, limit int) ([]types.GuildMember, error) {
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	return g.requestMembers(types.RequestGuildMembersPayload{
		GuildID: guildID, Query: &query, Limit: &limit, Presences: true,
	})
}

// FetchPermissions returns the effective permissions of a user in a channel,
// loading the channel, guild, roles and member as needed. Check bits with
// types.HasPermission(perms, types.PermissionSendMessages).
func (g *Gateway) FetchPermissions(userID, channelID string) (int64, error) {
	ch, err := g.FetchChannel(channelID)
	if err != nil {
		return 0, err
	}
	if ch.GuildID == "" {
		return types.PermissionsDM, nil
	}
	if ch.IsThread() && ch.ParentID != "" {
		if ch, err = g.FetchChannel(ch.ParentID); err != nil {
			return 0, err
		}
	}
	guild, member, err := g.guildAndMember(ch.GuildID, userID)
	if err != nil {
		return 0, err
	}
	return types.ComputeChannelPermissions(guild, member, ch), nil
}

// FetchGuildPermissions returns the guild-level permissions of a user.
func (g *Gateway) FetchGuildPermissions(guildID, userID string) (int64, error) {
	guild, member, err := g.guildAndMember(guildID, userID)
	if err != nil {
		return 0, err
	}
	return types.ComputeBasePermissions(guild, member), nil
}

func (g *Gateway) guildAndMember(guildID, userID string) (*types.Guild, *types.GuildMember, error) {
	guild, err := g.FetchGuild(guildID)
	if err != nil {
		return nil, nil, err
	}
	if len(guild.Roles) == 0 {
		if guild.Roles, err = g.FetchRoles(guildID); err != nil {
			return nil, nil, err
		}
	}
	member, err := g.FetchMember(guildID, userID)
	if err != nil {
		return nil, nil, err
	}
	return guild, member, nil
}

// ---------------------------------------------------------------------------
// Bulk fetches (parallel)
// ---------------------------------------------------------------------------

// FetchUsers returns many users at once: cached users instantly, the rest
// fetched in parallel (Config.FetchConcurrency at a time). The error joins
// the failures; successful users are still returned.
func (g *Gateway) FetchUsers(userIDs []string) (map[string]*types.User, error) {
	return fetchMany(g, userIDs, g.FetchUser)
}

// FetchChannels returns many channels at once (see FetchUsers).
func (g *Gateway) FetchChannels(channelIDs []string) (map[string]*types.Channel, error) {
	return fetchMany(g, channelIDs, g.FetchChannel)
}

// FetchGuilds returns many guilds at once (see FetchUsers).
func (g *Gateway) FetchGuilds(guildIDs []string) (map[string]*types.Guild, error) {
	return fetchMany(g, guildIDs, g.FetchGuild)
}

// FetchUserProfiles returns the profiles of many users at once (see FetchUsers).
func (g *Gateway) FetchUserProfiles(userIDs []string, guildID string) (map[string]*types.UserProfile, error) {
	return fetchMany(g, userIDs, func(id string) (*types.UserProfile, error) {
		return g.GetUserProfile(id, guildID)
	})
}

func fetchMany[T any](g *Gateway, ids []string, fetch func(string) (*T, error)) (map[string]*T, error) {
	ids = uniqueIDs(ids)
	results := make(map[string]*T, len(ids))
	var (
		mu   sync.Mutex
		errs []error
		wg   sync.WaitGroup
	)
	sem := make(chan struct{}, max(g.Config.FetchConcurrency, 1))
	for _, id := range ids {
		wg.Add(1)
		sem <- struct{}{}
		go func(id string) {
			defer wg.Done()
			defer func() { <-sem }()
			v, err := fetch(id)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", id, err))
				return
			}
			results[id] = v
		}(id)
	}
	wg.Wait()
	return results, errors.Join(errs...)
}

func uniqueIDs(ids []string) []string {
	seen := make(map[string]bool, len(ids))
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id != "" && !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}

// ignoreNotFound drops "not a member" errors from a joined error.
func ignoreNotFound(err error) error {
	if err == nil {
		return nil
	}
	var kept []error
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		for _, e := range joined.Unwrap() {
			if !errors.Is(e, ErrMemberNotFound) {
				kept = append(kept, e)
			}
		}
		return errors.Join(kept...)
	}
	if errors.Is(err, ErrMemberNotFound) {
		return nil
	}
	return err
}

// ---------------------------------------------------------------------------
// Gateway member requests (opcode 8)
// ---------------------------------------------------------------------------

// requestMembers sends a Request Guild Members payload and collects every
// GUILD_MEMBERS_CHUNK answering it.
func (g *Gateway) requestMembers(p types.RequestGuildMembersPayload) ([]types.GuildMember, error) {
	nonce := GenerateNonce()
	p.Nonce = nonce
	ch := make(chan *types.GuildMembersChunkEventData, 64)

	g.chunkMu.Lock()
	if g.chunkWaiters == nil {
		g.chunkWaiters = make(map[string]chan *types.GuildMembersChunkEventData)
	}
	g.chunkWaiters[nonce] = ch
	g.chunkMu.Unlock()
	defer func() {
		g.chunkMu.Lock()
		delete(g.chunkWaiters, nonce)
		g.chunkMu.Unlock()
	}()

	if err := g.SendOp(types.OpcodeRequestGuildMembers, p); err != nil {
		return nil, err
	}

	timer := time.NewTimer(memberChunkTimeout)
	defer timer.Stop()
	var members []types.GuildMember
	seen := map[int]bool{}
	for {
		select {
		case chunk := <-ch:
			if !seen[chunk.ChunkIndex] {
				seen[chunk.ChunkIndex] = true
				members = append(members, chunk.Members...)
			}
			if len(seen) >= max(chunk.ChunkCount, 1) {
				return members, nil
			}
		case <-timer.C:
			return members, fmt.Errorf("discord: timed out waiting for guild members of %s", p.GuildID)
		case <-g.closingChan():
			return members, ErrClosed
		}
	}
}

// deliverChunk hands a GUILD_MEMBERS_CHUNK to the request waiting for it.
func (g *Gateway) deliverChunk(ev *types.RawEvent, decoded any) {
	chunk, _ := decoded.(*types.GuildMembersChunkEventData)
	if chunk == nil {
		chunk = new(types.GuildMembersChunkEventData)
		if err := json.Unmarshal(ev.Data, chunk); err != nil {
			return
		}
	}
	if chunk.Nonce == "" {
		return
	}
	g.chunkMu.Lock()
	ch := g.chunkWaiters[chunk.Nonce]
	g.chunkMu.Unlock()
	if ch != nil {
		select {
		case ch <- chunk:
		default:
		}
	}
}
