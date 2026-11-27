package types

import (
	"sort"
	"strings"
)

// GuildRoleEventData is the data of GUILD_ROLE_CREATE / GUILD_ROLE_UPDATE.
type GuildRoleEventData struct {
	GuildID string `json:"guild_id"`
	Role    Role   `json:"role"`
}

// GuildRoleDeleteEventData is the data of GUILD_ROLE_DELETE.
type GuildRoleDeleteEventData struct {
	GuildID string `json:"guild_id"`
	RoleID  string `json:"role_id"`
}

// GuildEmojisUpdateEventData is the data of GUILD_EMOJIS_UPDATE.
type GuildEmojisUpdateEventData struct {
	GuildID string  `json:"guild_id"`
	Emojis  []Emoji `json:"emojis"`
}

// ThreadListSyncEventData is the data of THREAD_LIST_SYNC.
type ThreadListSyncEventData struct {
	GuildID    string    `json:"guild_id"`
	ChannelIDs []string  `json:"channel_ids,omitempty"`
	Threads    []Channel `json:"threads"`
}

// UserProfile is returned by the user profile endpoint
// (GET /users/{id}/profile), which only user accounts can use.
type UserProfile struct {
	User               User               `json:"user"`
	UserProfile        ProfileMetadata    `json:"user_profile"`
	ConnectedAccounts  []ConnectedAccount `json:"connected_accounts"`
	PremiumType        FlexInt            `json:"premium_type,omitempty"`
	PremiumSince       string             `json:"premium_since,omitempty"`
	PremiumGuildSince  string             `json:"premium_guild_since,omitempty"`
	Badges             []ProfileBadge     `json:"badges,omitempty"`
	GuildBadges        []ProfileBadge     `json:"guild_badges,omitempty"`
	MutualGuilds       []MutualGuild      `json:"mutual_guilds,omitempty"`
	MutualFriendsCount int                `json:"mutual_friends_count,omitempty"`
	MutualFriends      []User             `json:"mutual_friends,omitempty"`
	GuildMember        *GuildMember       `json:"guild_member,omitempty"`
	GuildMemberProfile *ProfileMetadata   `json:"guild_member_profile,omitempty"`
	LegacyUsername     string             `json:"legacy_username,omitempty"`
}

// ProfileMetadata is the bio / pronouns / banner part of a profile.
type ProfileMetadata struct {
	GuildID     string `json:"guild_id,omitempty"`
	Bio         string `json:"bio"`
	Pronouns    string `json:"pronouns"`
	Banner      string `json:"banner,omitempty"`
	AccentColor int    `json:"accent_color,omitempty"`
	ThemeColors []int  `json:"theme_colors,omitempty"`
}

// ConnectedAccount is an external account (GitHub, Spotify, ...) shown on a profile.
type ConnectedAccount struct {
	Type     string `json:"type"`
	ID       string `json:"id"`
	Name     string `json:"name"`
	Verified bool   `json:"verified"`
}

// ProfileBadge is a badge shown on a profile.
type ProfileBadge struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
	Link        string `json:"link,omitempty"`
}

// MutualGuild is a guild shared with the profile's user.
type MutualGuild struct {
	ID   string `json:"id"`
	Nick string `json:"nick,omitempty"`
}

// Invite is a guild / group DM invite. Fetching an invite reveals basic
// information about guilds you are not a member of.
type Invite struct {
	Code                     string   `json:"code"`
	Type                     int      `json:"type"`
	Guild                    *Guild   `json:"guild,omitempty"`
	GuildID                  string   `json:"guild_id,omitempty"`
	Channel                  *Channel `json:"channel,omitempty"`
	Inviter                  *User    `json:"inviter,omitempty"`
	ApproximateMemberCount   int      `json:"approximate_member_count,omitempty"`
	ApproximatePresenceCount int      `json:"approximate_presence_count,omitempty"`
	ExpiresAt                string   `json:"expires_at,omitempty"`
}

// GuildPreview is the public preview of a discoverable guild.
type GuildPreview struct {
	ID                       string    `json:"id"`
	Name                     string    `json:"name"`
	Icon                     string    `json:"icon"`
	Splash                   string    `json:"splash"`
	DiscoverySplash          string    `json:"discovery_splash"`
	Emojis                   []Emoji   `json:"emojis"`
	Features                 []string  `json:"features"`
	ApproximateMemberCount   int       `json:"approximate_member_count"`
	ApproximatePresenceCount int       `json:"approximate_presence_count"`
	Description              string    `json:"description"`
	Stickers                 []Sticker `json:"stickers"`
}

// ChannelCategory groups the channels of one category, as in the Discord
// sidebar. Category is nil for channels that are not in a category.
type ChannelCategory struct {
	Category *Channel
	Channels []Channel
}

// SortChannels sorts channels the way Discord displays them: text-like
// channels before voice channels, then by position, then by ID.
func SortChannels(channels []Channel) {
	sort.SliceStable(channels, func(i, j int) bool {
		a, b := &channels[i], &channels[j]
		if va, vb := a.IsVoice(), b.IsVoice(); va != vb {
			return !va
		}
		if a.Position != b.Position {
			return a.Position < b.Position
		}
		return snowflakeLess(a.ID, b.ID)
	})
}

// SortRoles sorts roles from the highest to the lowest position.
func SortRoles(roles []Role) {
	sort.SliceStable(roles, func(i, j int) bool {
		if roles[i].Position != roles[j].Position {
			return roles[i].Position > roles[j].Position
		}
		return snowflakeLess(roles[i].ID, roles[j].ID)
	})
}

// BuildChannelTree groups guild channels by category, in display order.
// Uncategorized channels come first (Category == nil); threads are skipped.
func BuildChannelTree(channels []Channel) []ChannelCategory {
	var categories, others []Channel
	for _, c := range channels {
		switch {
		case c.IsThread():
		case c.IsCategory():
			categories = append(categories, c)
		default:
			others = append(others, c)
		}
	}
	SortChannels(categories)
	SortChannels(others)

	index := make(map[string]int, len(categories))
	tree := []ChannelCategory{{}}
	for i := range categories {
		cat := categories[i]
		index[cat.ID] = len(tree)
		tree = append(tree, ChannelCategory{Category: &cat})
	}
	for _, c := range others {
		pos, ok := index[c.ParentID]
		if !ok {
			pos = 0
		}
		tree[pos].Channels = append(tree[pos].Channels, c)
	}
	if len(tree[0].Channels) == 0 {
		tree = tree[1:]
	}
	return tree
}

// FindByName returns the first channel whose name equals name
// (case-insensitive, a leading "#" is ignored), or nil.
func FindByName(channels []Channel, name string) *Channel {
	name = strings.TrimPrefix(name, "#")
	for i := range channels {
		if strings.EqualFold(channels[i].Name, name) {
			return &channels[i]
		}
	}
	return nil
}

func snowflakeLess(a, b string) bool {
	if len(a) != len(b) {
		return len(a) < len(b)
	}
	return a < b
}
