package discord

import (
	"errors"
	"log/slog"
	"sort"
	"strings"
	"sync"

	"github.com/goccy/go-json"
	"github.com/headd16/goself/types"
)

// Errors returned by the state cache.
var (
	ErrStateNotFound   = errors.New("discord: not found in the state cache")
	ErrMemberNotCached = errors.New("discord: member is not in the state cache (use FetchMember first)")
)

// State is an in-memory cache of everything the gateway has seen: guilds,
// channels (including categories, threads and DMs), roles, emojis, users,
// guild members, presences, relationships and recent messages.
//
// It is filled automatically from READY and the live events, and by the REST
// helpers. Every lookup is a map access and returns a copy, so it is safe to
// use from any goroutine. Treat slices inside returned values as read-only.
type State struct {
	mu          sync.RWMutex
	enabled     bool
	maxMessages int

	me            types.User
	guilds        map[string]*types.Guild
	channels      map[string]*types.Channel
	guildChannels map[string]map[string]struct{} // guild ID -> channel IDs (incl. threads)
	roleGuild     map[string]string              // role ID -> guild ID
	users         map[string]*types.User
	members       map[string]map[string]*types.GuildMember // guild ID -> user ID -> member
	presences     map[string]*types.PresenceUpdateEventData
	relationships map[string]*types.Relationship
	messages      map[string][]*types.Message // channel ID -> oldest..newest
}

// StateStats counts the cached objects.
type StateStats struct {
	Guilds, Channels, Roles, Users, Members, Presences, Relationships, Messages int
}

func newState(cfg *types.Config) *State {
	s := &State{enabled: !cfg.DisableState, maxMessages: cfg.StateMaxMessages}
	s.resetL()
	return s
}

func (s *State) resetL() {
	s.me = types.User{}
	s.guilds = make(map[string]*types.Guild)
	s.channels = make(map[string]*types.Channel)
	s.guildChannels = make(map[string]map[string]struct{})
	s.roleGuild = make(map[string]string)
	s.users = make(map[string]*types.User)
	s.members = make(map[string]map[string]*types.GuildMember)
	s.presences = make(map[string]*types.PresenceUpdateEventData)
	s.relationships = make(map[string]*types.Relationship)
	s.messages = make(map[string][]*types.Message)
}

// Clear empties the cache.
func (s *State) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.resetL()
}

// Enabled reports whether the cache stores data (see Config.DisableState).
func (s *State) Enabled() bool { return s.enabled }

// ---------------------------------------------------------------------------
// Writes (callers hold s.mu for writing)
// ---------------------------------------------------------------------------

func (s *State) putUserL(u *types.User) {
	if u == nil || u.ID == "" || u.Username == "" { // ignore partial users
		return
	}
	cp := *u
	s.users[u.ID] = &cp
	if s.me.ID == u.ID {
		s.me = cp
	}
}

func (s *State) putChannelL(c types.Channel) {
	if c.ID == "" {
		return
	}
	if len(c.Recipients) == 0 && len(c.RecipientIDs) > 0 {
		recipients := make([]types.User, 0, len(c.RecipientIDs))
		for _, id := range c.RecipientIDs {
			if u, ok := s.users[id]; ok {
				recipients = append(recipients, *u)
			} else {
				recipients = append(recipients, types.User{ID: id})
			}
		}
		c.Recipients = recipients
	}
	for i := range c.Recipients {
		s.putUserL(&c.Recipients[i])
	}
	s.channels[c.ID] = &c
	if c.GuildID != "" {
		ids := s.guildChannels[c.GuildID]
		if ids == nil {
			ids = make(map[string]struct{})
			s.guildChannels[c.GuildID] = ids
		}
		ids[c.ID] = struct{}{}
	}
}

func (s *State) deleteChannelL(id string) {
	if c, ok := s.channels[id]; ok && c.GuildID != "" {
		delete(s.guildChannels[c.GuildID], id)
	}
	delete(s.channels, id)
	delete(s.messages, id)
}

func (s *State) putGuildL(g *types.Guild, replaceChannels bool) {
	if g == nil || g.ID == "" {
		return
	}
	old := s.guilds[g.ID]
	if g.Unavailable && old != nil && g.Name == "" {
		cp := *old
		cp.Unavailable = true
		s.guilds[g.ID] = &cp
		return
	}

	cp := *g
	channels, threads, members := cp.Channels, cp.Threads, cp.Members
	cp.Channels, cp.Threads, cp.Members = nil, nil, nil
	if old != nil {
		if cp.Roles == nil {
			cp.Roles = old.Roles
		}
		if cp.Emojis == nil {
			cp.Emojis = old.Emojis
		}
		if cp.Stickers == nil {
			cp.Stickers = old.Stickers
		}
		if cp.JoinedAt == "" {
			cp.JoinedAt = old.JoinedAt
		}
		if cp.MemberCount == 0 {
			cp.MemberCount = old.MemberCount
		}
		for _, r := range old.Roles {
			delete(s.roleGuild, r.ID)
		}
	}
	for _, r := range cp.Roles {
		s.roleGuild[r.ID] = cp.ID
	}
	s.guilds[cp.ID] = &cp

	if replaceChannels && (channels != nil || threads != nil) {
		for id := range s.guildChannels[cp.ID] {
			s.deleteChannelL(id)
		}
	}
	for _, c := range channels {
		c.GuildID = cp.ID
		s.putChannelL(c)
	}
	for _, c := range threads {
		c.GuildID = cp.ID
		s.putChannelL(c)
	}
	for i := range members {
		s.putMemberL(cp.ID, &members[i])
	}
}

func (s *State) deleteGuildL(id string) {
	if g, ok := s.guilds[id]; ok {
		for _, r := range g.Roles {
			delete(s.roleGuild, r.ID)
		}
	}
	for chID := range s.guildChannels[id] {
		delete(s.channels, chID)
		delete(s.messages, chID)
	}
	delete(s.guildChannels, id)
	delete(s.members, id)
	delete(s.guilds, id)
}

// updateGuildL replaces a guild with a modified copy (copy-on-write).
func (s *State) updateGuildL(id string, fn func(g *types.Guild)) {
	g, ok := s.guilds[id]
	if !ok {
		return
	}
	cp := *g
	fn(&cp)
	s.guilds[id] = &cp
}

func (s *State) putRoleL(guildID string, role types.Role) {
	s.updateGuildL(guildID, func(g *types.Guild) {
		roles := make([]types.Role, 0, len(g.Roles)+1)
		replaced := false
		for _, r := range g.Roles {
			if r.ID == role.ID {
				r, replaced = role, true
			}
			roles = append(roles, r)
		}
		if !replaced {
			roles = append(roles, role)
		}
		g.Roles = roles
	})
	if _, ok := s.guilds[guildID]; ok {
		s.roleGuild[role.ID] = guildID
	}
}

func (s *State) deleteRoleL(guildID, roleID string) {
	s.updateGuildL(guildID, func(g *types.Guild) {
		roles := make([]types.Role, 0, len(g.Roles))
		for _, r := range g.Roles {
			if r.ID != roleID {
				roles = append(roles, r)
			}
		}
		g.Roles = roles
	})
	delete(s.roleGuild, roleID)
}

func (s *State) putMemberL(guildID string, m *types.GuildMember) {
	if guildID == "" || m == nil {
		return
	}
	cp := *m
	if cp.User.ID == "" && cp.UserID != "" {
		if u, ok := s.users[cp.UserID]; ok {
			cp.User = *u
		} else {
			cp.User.ID = cp.UserID
		}
	}
	id := cp.User.ID
	if id == "" {
		return
	}
	cp.UserID = id
	s.putUserL(&cp.User)

	byUser := s.members[guildID]
	if byUser == nil {
		byUser = make(map[string]*types.GuildMember)
		s.members[guildID] = byUser
	}
	if old, ok := byUser[id]; ok {
		if cp.JoinedAt == "" {
			cp.JoinedAt = old.JoinedAt
		}
		if cp.User.Username == "" {
			cp.User = old.User
		}
	}
	byUser[id] = &cp
}

func (s *State) putPresenceL(p *types.PresenceUpdateEventData) {
	id := p.User.ID
	if id == "" {
		id = p.UserID
	}
	if id == "" {
		return
	}
	s.putUserL(&p.User)
	cp := *p
	cp.UserID = id
	if cp.User.ID == "" {
		if u, ok := s.users[id]; ok {
			cp.User = *u
		} else {
			cp.User.ID = id
		}
	}
	s.presences[id] = &cp
}

func (s *State) putRelationshipL(r *types.Relationship) {
	cp := *r
	if cp.User != nil {
		s.putUserL(cp.User)
		if cp.UserID == "" {
			cp.UserID = cp.User.ID
		}
	} else if cp.UserID != "" {
		if u, ok := s.users[cp.UserID]; ok {
			uc := *u
			cp.User = &uc
		}
	}
	id := cp.UserID
	if id == "" {
		id = cp.ID
	}
	if id != "" {
		s.relationships[id] = &cp
	}
}

func (s *State) addMessageL(m *types.Message) {
	s.putUserL(&m.Author)
	for i := range m.Mentions {
		s.putUserL(&m.Mentions[i])
	}
	if m.Member != nil && m.GuildID != "" && m.WebhookID == "" {
		member := *m.Member
		member.User = m.Author
		s.putMemberL(m.GuildID, &member)
	}
	if ch, ok := s.channels[m.ChannelID]; ok {
		cp := *ch
		cp.LastMessageID = m.ID
		s.channels[m.ChannelID] = &cp
	}

	if s.maxMessages <= 0 {
		return
	}
	cp := *m
	msgs := s.messages[m.ChannelID]
	// Keep the slice ordered by ID (oldest first); fetched history can be
	// older than messages already cached.
	pos := len(msgs)
	for pos > 0 && !snowflakeGreater(m.ID, msgs[pos-1].ID) {
		if msgs[pos-1].ID == m.ID {
			msgs[pos-1] = &cp
			return
		}
		pos--
	}
	next := make([]*types.Message, 0, len(msgs)+1)
	next = append(next, msgs[:pos]...)
	next = append(next, &cp)
	next = append(next, msgs[pos:]...)
	if len(next) > s.maxMessages {
		next = next[len(next)-s.maxMessages:]
	}
	s.messages[m.ChannelID] = next
}

func (s *State) updateMessageL(m *types.Message) {
	msgs := s.messages[m.ChannelID]
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].ID != m.ID {
			continue
		}
		if m.Author.ID != "" { // full message
			cp := *m
			if cp.ReferencedMessage.ID == "" {
				cp.ReferencedMessage = msgs[i].ReferencedMessage
			}
			msgs[i] = &cp
			return
		}
		cp := *msgs[i] // partial update: merge the fields that are present
		if m.Content != "" {
			cp.Content = m.Content
		}
		if m.EditedTimestamp != "" {
			cp.EditedTimestamp = m.EditedTimestamp
		}
		if m.Embeds != nil {
			cp.Embeds = m.Embeds
		}
		if m.Components != nil {
			cp.Components = m.Components
		}
		if m.Attachments != nil {
			cp.Attachments = m.Attachments
		}
		if m.Flags != 0 {
			cp.Flags = m.Flags
		}
		msgs[i] = &cp
		return
	}
	if m.Author.ID != "" {
		s.addMessageL(m)
	}
}

func (s *State) removeMessagesL(channelID string, ids ...string) {
	msgs := s.messages[channelID]
	if len(msgs) == 0 {
		return
	}
	drop := make(map[string]bool, len(ids))
	for _, id := range ids {
		drop[id] = true
	}
	kept := make([]*types.Message, 0, len(msgs))
	for _, m := range msgs {
		if !drop[m.ID] {
			kept = append(kept, m)
		}
	}
	s.messages[channelID] = kept
}

// ---------------------------------------------------------------------------
// Gateway events
// ---------------------------------------------------------------------------

type readyState struct {
	types.ReadyEventData
	MergedMembers [][]types.GuildMember `json:"merged_members"`
}

type readySupplemental struct {
	Guilds []struct {
		ID string `json:"id"`
	} `json:"guilds"`
	MergedMembers   [][]types.GuildMember `json:"merged_members"`
	MergedPresences struct {
		Guilds  [][]types.PresenceUpdateEventData `json:"guilds"`
		Friends []types.PresenceUpdateEventData   `json:"friends"`
	} `json:"merged_presences"`
}

func decodeEvent[T any](g *Gateway, ev *types.RawEvent) *T {
	v := new(T)
	if err := json.Unmarshal(ev.Data, v); err != nil {
		g.log(slog.LevelError, "state: failed to decode event", "event", ev.Type, "error", err)
		return nil
	}
	return v
}

// onEvent updates the cache from a dispatch. It returns the decoded payload
// (of the type the event handlers expect) so it is not decoded twice, or nil.
func (s *State) onEvent(g *Gateway, ev *types.RawEvent) any {
	if !s.enabled {
		return nil
	}

	switch ev.Type {
	case types.EventNameReady:
		r := decodeEvent[readyState](g, ev)
		if r == nil {
			return nil
		}
		s.mu.Lock()
		s.resetL()
		me := r.User
		s.me = me
		s.users[me.ID] = &me
		for i := range r.Users {
			s.putUserL(&r.Users[i])
		}
		for i := range r.Relationships {
			s.putRelationshipL(&r.Relationships[i])
		}
		for _, c := range r.PrivateChannels {
			s.putChannelL(c)
		}
		for i := range r.Guilds {
			s.putGuildL(&r.Guilds[i], true)
			if i < len(r.MergedMembers) {
				for j := range r.MergedMembers[i] {
					s.putMemberL(r.Guilds[i].ID, &r.MergedMembers[i][j])
				}
			}
		}
		s.mu.Unlock()
		return &r.ReadyEventData

	case types.EventNameReadySupplemental:
		sup := decodeEvent[readySupplemental](g, ev)
		if sup == nil {
			return nil
		}
		s.mu.Lock()
		for i, guild := range sup.Guilds {
			if i < len(sup.MergedMembers) {
				for j := range sup.MergedMembers[i] {
					s.putMemberL(guild.ID, &sup.MergedMembers[i][j])
				}
			}
			if i < len(sup.MergedPresences.Guilds) {
				for j := range sup.MergedPresences.Guilds[i] {
					s.putPresenceL(&sup.MergedPresences.Guilds[i][j])
				}
			}
		}
		for i := range sup.MergedPresences.Friends {
			s.putPresenceL(&sup.MergedPresences.Friends[i])
		}
		s.mu.Unlock()
		return nil

	case types.EventNameGuildCreate, types.EventNameGuildUpdate:
		guild := decodeEvent[types.Guild](g, ev)
		if guild != nil {
			s.mu.Lock()
			s.putGuildL(guild, ev.Type == types.EventNameGuildCreate)
			s.mu.Unlock()
		}
		return guild

	case types.EventNameGuildDelete:
		d := decodeEvent[types.GuildDeleteEventData](g, ev)
		if d != nil {
			s.mu.Lock()
			if d.Unavailable {
				s.putGuildL(&types.Guild{ID: d.ID, Unavailable: true}, false)
			} else {
				s.deleteGuildL(d.ID)
			}
			s.mu.Unlock()
		}
		return d

	case types.EventNameChannelCreate, types.EventNameChannelUpdate,
		types.EventNameThreadCreate, types.EventNameThreadUpdate:
		c := decodeEvent[types.Channel](g, ev)
		if c != nil {
			s.mu.Lock()
			s.putChannelL(*c)
			s.mu.Unlock()
		}
		return c

	case types.EventNameChannelDelete, types.EventNameThreadDelete:
		c := decodeEvent[types.Channel](g, ev)
		if c != nil {
			s.mu.Lock()
			s.deleteChannelL(c.ID)
			s.mu.Unlock()
		}
		return c

	case types.EventNameThreadListSync:
		e := decodeEvent[types.ThreadListSyncEventData](g, ev)
		if e != nil {
			s.mu.Lock()
			for _, t := range e.Threads {
				if t.GuildID == "" {
					t.GuildID = e.GuildID
				}
				s.putChannelL(t)
			}
			s.mu.Unlock()
		}
		return e

	case types.EventNameGuildRoleCreate, types.EventNameGuildRoleUpdate:
		e := decodeEvent[types.GuildRoleEventData](g, ev)
		if e != nil {
			s.mu.Lock()
			s.putRoleL(e.GuildID, e.Role)
			s.mu.Unlock()
		}
		return e

	case types.EventNameGuildRoleDelete:
		e := decodeEvent[types.GuildRoleDeleteEventData](g, ev)
		if e != nil {
			s.mu.Lock()
			s.deleteRoleL(e.GuildID, e.RoleID)
			s.mu.Unlock()
		}
		return e

	case types.EventNameGuildEmojisUpdate:
		e := decodeEvent[types.GuildEmojisUpdateEventData](g, ev)
		if e != nil {
			s.mu.Lock()
			s.updateGuildL(e.GuildID, func(guild *types.Guild) { guild.Emojis = e.Emojis })
			s.mu.Unlock()
		}
		return e

	case types.EventNameGuildMemberAdd, types.EventNameGuildMemberUpdate:
		e := decodeEvent[types.GuildMemberEventData](g, ev)
		if e != nil {
			s.mu.Lock()
			s.putMemberL(e.GuildID, &e.GuildMember)
			if ev.Type == types.EventNameGuildMemberAdd {
				s.updateGuildL(e.GuildID, func(guild *types.Guild) { guild.MemberCount++ })
			}
			s.mu.Unlock()
		}
		return e

	case types.EventNameGuildMemberRemove:
		e := decodeEvent[types.GuildMemberEventData](g, ev)
		if e != nil {
			s.mu.Lock()
			delete(s.members[e.GuildID], e.User.ID)
			s.updateGuildL(e.GuildID, func(guild *types.Guild) {
				if guild.MemberCount > 0 {
					guild.MemberCount--
				}
			})
			s.mu.Unlock()
		}
		return e

	case types.EventNameGuildMembersChunk:
		e := decodeEvent[types.GuildMembersChunkEventData](g, ev)
		if e != nil {
			s.mu.Lock()
			for i := range e.Members {
				s.putMemberL(e.GuildId, &e.Members[i])
			}
			for i := range e.Presences {
				s.putPresenceL(&e.Presences[i])
			}
			s.mu.Unlock()
		}
		return e

	case types.EventNamePresenceUpdate:
		p := decodeEvent[types.PresenceUpdateEventData](g, ev)
		if p != nil {
			s.mu.Lock()
			s.putPresenceL(p)
			s.mu.Unlock()
		}
		return p

	case types.EventNameUserUpdate:
		u := decodeEvent[types.User](g, ev)
		if u != nil {
			s.mu.Lock()
			s.putUserL(u)
			s.mu.Unlock()
		}
		return u

	case types.EventNameMessageCreate:
		m := decodeEvent[types.MessageEventData](g, ev)
		if m != nil {
			s.mu.Lock()
			s.addMessageL(m)
			s.mu.Unlock()
		}
		return m

	case types.EventNameMessageUpdate:
		m := decodeEvent[types.MessageEventData](g, ev)
		if m != nil {
			s.mu.Lock()
			s.updateMessageL(m)
			s.mu.Unlock()
		}
		return m

	case types.EventNameMessageDelete:
		d := decodeEvent[types.MessageDeleteEventData](g, ev)
		if d != nil {
			s.mu.Lock()
			s.removeMessagesL(d.ChannelID, d.ID)
			s.mu.Unlock()
		}
		return d

	case types.EventNameMessageDeleteBulk:
		d := decodeEvent[types.MessageDeleteBulkEventData](g, ev)
		if d != nil {
			s.mu.Lock()
			s.removeMessagesL(d.ChannelID, d.IDs...)
			s.mu.Unlock()
		}
		return d

	case types.EventNameRelationshipAdd, types.EventNameRelationshipUpdate:
		r := decodeEvent[types.Relationship](g, ev)
		if r != nil {
			s.mu.Lock()
			s.putRelationshipL(r)
			s.mu.Unlock()
		}
		return r

	case types.EventNameRelationshipRemove:
		r := decodeEvent[types.Relationship](g, ev)
		if r != nil {
			s.mu.Lock()
			delete(s.relationships, r.ID)
			if r.UserID != "" {
				delete(s.relationships, r.UserID)
			}
			s.mu.Unlock()
		}
		return r
	}
	return nil
}

// ---------------------------------------------------------------------------
// Store helpers used by the REST methods
// ---------------------------------------------------------------------------

func (s *State) storeGuild(g *types.Guild) {
	if !s.enabled || g == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.putGuildL(g, false)
}

// storePartialGuilds stores the partial guilds of /users/@me/guilds without
// overwriting complete cached guilds (they lack owner_id and roles).
func (s *State) storePartialGuilds(guilds []types.Guild) {
	if !s.enabled {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range guilds {
		if _, ok := s.guilds[guilds[i].ID]; !ok {
			s.putGuildL(&guilds[i], false)
		}
	}
}

func (s *State) storeChannels(channels ...types.Channel) {
	if !s.enabled {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, c := range channels {
		s.putChannelL(c)
	}
}

// replaceGuildChannels stores the complete channel list of a guild, dropping
// cached channels that no longer exist (threads are kept).
func (s *State) replaceGuildChannels(guildID string, channels []types.Channel) {
	if !s.enabled {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for id := range s.guildChannels[guildID] {
		if c, ok := s.channels[id]; ok && !c.IsThread() {
			s.deleteChannelL(id)
		}
	}
	for _, c := range channels {
		if c.GuildID == "" {
			c.GuildID = guildID
		}
		s.putChannelL(c)
	}
}

func (s *State) storeRoles(guildID string, roles []types.Role) {
	if !s.enabled {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.guilds[guildID]; !ok {
		s.guilds[guildID] = &types.Guild{ID: guildID}
	}
	s.updateGuildL(guildID, func(g *types.Guild) {
		for _, r := range g.Roles {
			delete(s.roleGuild, r.ID)
		}
		g.Roles = append([]types.Role(nil), roles...)
	})
	for _, r := range roles {
		s.roleGuild[r.ID] = guildID
	}
}

func (s *State) storeEmojis(guildID string, emojis []types.Emoji) {
	if !s.enabled {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.updateGuildL(guildID, func(g *types.Guild) { g.Emojis = append([]types.Emoji(nil), emojis...) })
}

func (s *State) storeUsers(users ...*types.User) {
	if !s.enabled {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, u := range users {
		s.putUserL(u)
	}
}

func (s *State) storeMembers(guildID string, members ...*types.GuildMember) {
	if !s.enabled {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, m := range members {
		s.putMemberL(guildID, m)
	}
}

func (s *State) storeMessages(msgs ...*types.Message) {
	if !s.enabled {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, m := range msgs {
		s.addMessageL(m)
	}
}

func (s *State) storeRelationships(rels []types.Relationship) {
	if !s.enabled {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range rels {
		s.putRelationshipL(&rels[i])
	}
}

// ---------------------------------------------------------------------------
// Reads
// ---------------------------------------------------------------------------

// Me returns the logged-in user.
func (s *State) Me() types.User {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.me
}

// Guild returns a cached guild, with Channels and Threads filled and sorted.
func (s *State) Guild(guildID string) (*types.Guild, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	g, ok := s.guilds[guildID]
	if !ok {
		return nil, false
	}
	cp := *g
	cp.Channels = s.guildChannelsL(guildID, func(c *types.Channel) bool { return !c.IsThread() })
	cp.Threads = s.guildChannelsL(guildID, (*types.Channel).IsThread)
	return &cp, true
}

// Guilds returns every cached guild sorted by name (Channels are not filled;
// use GuildChannels).
func (s *State) Guilds() []types.Guild {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]types.Guild, 0, len(s.guilds))
	for _, g := range s.guilds {
		out = append(out, *g)
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })
	return out
}

// FindGuild returns the first guild whose name equals name (case-insensitive).
func (s *State) FindGuild(name string) (*types.Guild, bool) {
	for _, g := range s.Guilds() {
		if strings.EqualFold(g.Name, name) {
			return s.Guild(g.ID)
		}
	}
	return nil, false
}

// Channel returns a cached channel (guild channel, category, thread or DM).
func (s *State) Channel(channelID string) (*types.Channel, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.channels[channelID]
	if !ok {
		return nil, false
	}
	cp := *c
	return &cp, true
}

func (s *State) guildChannelsL(guildID string, keep func(*types.Channel) bool) []types.Channel {
	ids := s.guildChannels[guildID]
	out := make([]types.Channel, 0, len(ids))
	for id := range ids {
		if c, ok := s.channels[id]; ok && (keep == nil || keep(c)) {
			out = append(out, *c)
		}
	}
	types.SortChannels(out)
	return out
}

// GuildChannels returns the channels and categories of a guild in display
// order (threads excluded).
func (s *State) GuildChannels(guildID string) []types.Channel {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.guildChannelsL(guildID, func(c *types.Channel) bool { return !c.IsThread() })
}

// TextChannels returns the text-like channels of a guild (no categories,
// voice channels or threads).
func (s *State) TextChannels(guildID string) []types.Channel {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.guildChannelsL(guildID, func(c *types.Channel) bool {
		return !c.IsThread() && !c.IsCategory() && !c.IsVoice()
	})
}

// VoiceChannels returns the voice and stage channels of a guild.
func (s *State) VoiceChannels(guildID string) []types.Channel {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.guildChannelsL(guildID, (*types.Channel).IsVoice)
}

// Threads returns the cached threads of a guild.
func (s *State) Threads(guildID string) []types.Channel {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.guildChannelsL(guildID, (*types.Channel).IsThread)
}

// Categories returns the categories of a guild in display order.
func (s *State) Categories(guildID string) []types.Channel {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.guildChannelsL(guildID, (*types.Channel).IsCategory)
}

// CategoryChannels returns the channels inside a category.
func (s *State) CategoryChannels(categoryID string) []types.Channel {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cat, ok := s.channels[categoryID]
	if !ok {
		return nil
	}
	return s.guildChannelsL(cat.GuildID, func(c *types.Channel) bool {
		return c.ParentID == categoryID && !c.IsThread()
	})
}

// ChannelTree returns the guild channels grouped by category, as in the
// Discord sidebar.
func (s *State) ChannelTree(guildID string) []types.ChannelCategory {
	return types.BuildChannelTree(s.GuildChannels(guildID))
}

// FindChannel returns the first channel of a guild with the given name
// (case-insensitive, "#" optional).
func (s *State) FindChannel(guildID, name string) (*types.Channel, bool) {
	c := types.FindByName(s.GuildChannels(guildID), name)
	return c, c != nil
}

// PrivateChannels returns DMs and group DMs, most recently active first.
func (s *State) PrivateChannels() []types.Channel {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []types.Channel
	for _, c := range s.channels {
		if c.IsPrivate() {
			out = append(out, *c)
		}
	}
	sort.Slice(out, func(i, j int) bool { return snowflakeGreater(out[i].LastMessageID, out[j].LastMessageID) })
	return out
}

// DMChannel returns the cached DM channel with a user.
func (s *State) DMChannel(userID string) (*types.Channel, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, c := range s.channels {
		if c.Type != types.ChannelDM {
			continue
		}
		for _, r := range c.Recipients {
			if r.ID == userID {
				cp := *c
				return &cp, true
			}
		}
	}
	return nil, false
}

// Role returns a role of a guild.
func (s *State) Role(guildID, roleID string) (*types.Role, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	g, ok := s.guilds[guildID]
	if !ok {
		return nil, false
	}
	for _, r := range g.Roles {
		if r.ID == roleID {
			return &r, true
		}
	}
	return nil, false
}

// RoleByID finds a role in any cached guild and returns it with its guild ID.
func (s *State) RoleByID(roleID string) (*types.Role, string, bool) {
	s.mu.RLock()
	guildID, ok := s.roleGuild[roleID]
	s.mu.RUnlock()
	if !ok {
		return nil, "", false
	}
	r, ok := s.Role(guildID, roleID)
	return r, guildID, ok
}

// Roles returns the roles of a guild, highest first.
func (s *State) Roles(guildID string) []types.Role {
	s.mu.RLock()
	defer s.mu.RUnlock()
	g, ok := s.guilds[guildID]
	if !ok {
		return nil
	}
	roles := append([]types.Role(nil), g.Roles...)
	types.SortRoles(roles)
	return roles
}

// FindRole returns the first role of a guild with the given name (case-insensitive).
func (s *State) FindRole(guildID, name string) (*types.Role, bool) {
	name = strings.TrimPrefix(name, "@")
	for _, r := range s.Roles(guildID) {
		if strings.EqualFold(r.Name, name) {
			return &r, true
		}
	}
	return nil, false
}

// Emoji returns a custom emoji of a guild.
func (s *State) Emoji(guildID, emojiID string) (*types.Emoji, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	g, ok := s.guilds[guildID]
	if !ok {
		return nil, false
	}
	for _, e := range g.Emojis {
		if e.ID == emojiID {
			return &e, true
		}
	}
	return nil, false
}

// Emojis returns the custom emojis of a guild.
func (s *State) Emojis(guildID string) []types.Emoji {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if g, ok := s.guilds[guildID]; ok {
		return append([]types.Emoji(nil), g.Emojis...)
	}
	return nil
}

// User returns a cached user.
func (s *State) User(userID string) (*types.User, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.users[userID]
	if !ok {
		return nil, false
	}
	cp := *u
	return &cp, true
}

// FindUser returns the first cached user whose username or display name
// equals name (case-insensitive).
func (s *State) FindUser(name string) (*types.User, bool) {
	name = strings.TrimPrefix(name, "@")
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, u := range s.users {
		if strings.EqualFold(u.Username, name) || strings.EqualFold(u.GlobalName, name) {
			cp := *u
			return &cp, true
		}
	}
	return nil, false
}

// Member returns a cached guild member.
func (s *State) Member(guildID, userID string) (*types.GuildMember, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	m, ok := s.members[guildID][userID]
	if !ok {
		return nil, false
	}
	cp := *m
	return &cp, true
}

// Members returns the cached members of a guild. User accounts only receive
// members they interact with; use FetchMembers to load specific ones.
func (s *State) Members(guildID string) []types.GuildMember {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]types.GuildMember, 0, len(s.members[guildID]))
	for _, m := range s.members[guildID] {
		out = append(out, *m)
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].DisplayName()) < strings.ToLower(out[j].DisplayName())
	})
	return out
}

// MemberRoles returns the roles of a cached member, highest first.
func (s *State) MemberRoles(guildID, userID string) []types.Role {
	m, ok := s.Member(guildID, userID)
	if !ok {
		return nil
	}
	var out []types.Role
	for _, r := range s.Roles(guildID) {
		if m.HasRole(r.ID) {
			out = append(out, r)
		}
	}
	return out
}

// Presence returns the last known presence (status and activities) of a user.
func (s *State) Presence(userID string) (*types.PresenceUpdateEventData, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.presences[userID]
	if !ok {
		return nil, false
	}
	cp := *p
	return &cp, true
}

// Message returns a cached message.
func (s *State) Message(channelID, messageID string) (*types.Message, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, m := range s.messages[channelID] {
		if m.ID == messageID {
			cp := *m
			return &cp, true
		}
	}
	return nil, false
}

// Messages returns the cached messages of a channel, oldest first.
func (s *State) Messages(channelID string) []types.Message {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]types.Message, 0, len(s.messages[channelID]))
	for _, m := range s.messages[channelID] {
		out = append(out, *m)
	}
	return out
}

// Relationship returns the relationship (friend, blocked, request) with a user.
func (s *State) Relationship(userID string) (*types.Relationship, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.relationships[userID]
	if !ok {
		return nil, false
	}
	cp := *r
	return &cp, true
}

// Relationships returns every relationship.
func (s *State) Relationships() []types.Relationship {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]types.Relationship, 0, len(s.relationships))
	for _, r := range s.relationships {
		out = append(out, *r)
	}
	return out
}

// Friends returns the users you are friends with.
func (s *State) Friends() []types.User {
	var out []types.User
	for _, r := range s.Relationships() {
		if r.Type != types.RelationshipFriend {
			continue
		}
		if r.User != nil {
			out = append(out, *r.User)
		} else if u, ok := s.User(r.UserID); ok {
			out = append(out, *u)
		}
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Username) < strings.ToLower(out[j].Username) })
	return out
}

// UserPermissions returns the guild-level permissions of a cached member.
func (s *State) UserPermissions(guildID, userID string) (int64, error) {
	g, ok := s.Guild(guildID)
	if !ok {
		return 0, ErrStateNotFound
	}
	m, ok := s.Member(guildID, userID)
	if !ok {
		return 0, ErrMemberNotCached
	}
	return types.ComputeBasePermissions(g, m), nil
}

// UserChannelPermissions returns the permissions of a user in a channel,
// including permission overwrites. DMs return types.PermissionsDM.
func (s *State) UserChannelPermissions(userID, channelID string) (int64, error) {
	ch, ok := s.Channel(channelID)
	if !ok {
		return 0, ErrStateNotFound
	}
	if ch.GuildID == "" {
		return types.PermissionsDM, nil
	}
	if ch.IsThread() && ch.ParentID != "" {
		if parent, ok := s.Channel(ch.ParentID); ok {
			ch = parent
		}
	}
	g, ok := s.Guild(ch.GuildID)
	if !ok {
		return 0, ErrStateNotFound
	}
	m, ok := s.Member(ch.GuildID, userID)
	if !ok {
		return 0, ErrMemberNotCached
	}
	return types.ComputeChannelPermissions(g, m, ch), nil
}

// Stats returns how many objects are cached.
func (s *State) Stats() StateStats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	st := StateStats{
		Guilds:        len(s.guilds),
		Channels:      len(s.channels),
		Roles:         len(s.roleGuild),
		Users:         len(s.users),
		Presences:     len(s.presences),
		Relationships: len(s.relationships),
	}
	for _, m := range s.members {
		st.Members += len(m)
	}
	for _, m := range s.messages {
		st.Messages += len(m)
	}
	return st
}

func snowflakeGreater(a, b string) bool {
	if len(a) != len(b) {
		return len(a) > len(b)
	}
	return a > b
}
