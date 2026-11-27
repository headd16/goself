package types

import (
	"fmt"
	"slices"
	"strings"

	"github.com/goccy/go-json"
)

// User is a Discord user.
type User struct {
	ID            string `json:"id"`
	Username      string `json:"username"`
	GlobalName    string `json:"global_name,omitempty"`
	Discriminator string `json:"discriminator"`
	Avatar        string `json:"avatar"`
	Bot           bool   `json:"bot,omitempty"`
	System        bool   `json:"system,omitempty"`
	MFAEnabled    bool   `json:"mfa_enabled,omitempty"`
	Banner        string `json:"banner,omitempty"`
	AccentColor   int    `json:"accent_color,omitempty"`
	Bio           string `json:"bio,omitempty"`
	Locale        string `json:"locale,omitempty"`
	Verified      bool   `json:"verified,omitempty"`
	Email         string `json:"email,omitempty"`
	Phone         string `json:"phone,omitempty"`
	Flags         uint64 `json:"flags,omitempty"`
	PremiumType   uint64 `json:"premium_type,omitempty"`
	PublicFlags   uint64 `json:"public_flags,omitempty"`
}

// DisplayName returns the global display name, falling back to the username.
func (u User) DisplayName() string {
	if u.GlobalName != "" {
		return u.GlobalName
	}
	return u.Username
}

// Mention returns the string that mentions the user in a message.
func (u User) Mention() string { return "<@" + u.ID + ">" }

// Tag returns "username" for migrated accounts or "username#1234" otherwise.
func (u User) Tag() string {
	if u.Discriminator == "" || u.Discriminator == "0" {
		return u.Username
	}
	return u.Username + "#" + u.Discriminator
}

// AvatarURL returns the CDN URL of the user's avatar (or the default avatar).
func (u User) AvatarURL() string {
	if u.Avatar == "" {
		return "https://cdn.discordapp.com/embed/avatars/0.png"
	}
	ext := "png"
	if strings.HasPrefix(u.Avatar, "a_") {
		ext = "gif"
	}
	return fmt.Sprintf("https://cdn.discordapp.com/avatars/%s/%s.%s", u.ID, u.Avatar, ext)
}

// GuildMember is a user's membership in a guild. User is empty for members
// embedded in MESSAGE_CREATE events (the author is in Message.Author).
type GuildMember struct {
	User User `json:"user"`
	// UserID is used instead of User in some user-account payloads
	// (READY merged_members). The state cache fills User from it.
	UserID                     string   `json:"user_id,omitempty"`
	Nick                       string   `json:"nick,omitempty"`
	Avatar                     string   `json:"avatar,omitempty"`
	Roles                      []string `json:"roles"`
	JoinedAt                   string   `json:"joined_at"`
	PremiumSince               string   `json:"premium_since,omitempty"`
	Deaf                       bool     `json:"deaf"`
	Mute                       bool     `json:"mute"`
	Flags                      int      `json:"flags"`
	Pending                    bool     `json:"pending,omitempty"`
	Permissions                string   `json:"permissions,omitempty"`
	CommunicationDisabledUntil string   `json:"communication_disabled_until,omitempty"`
}

// ID returns the member's user ID.
func (m *GuildMember) ID() string {
	if m.User.ID != "" {
		return m.User.ID
	}
	return m.UserID
}

// DisplayName returns the guild nickname, falling back to the user's
// display name.
func (m *GuildMember) DisplayName() string {
	if m.Nick != "" {
		return m.Nick
	}
	return m.User.DisplayName()
}

// HasRole reports whether the member has the given role.
func (m *GuildMember) HasRole(roleID string) bool {
	return slices.Contains(m.Roles, roleID)
}

// Guild is a Discord server.
//
// For user accounts the READY payload nests most guild fields inside a
// "properties" object; UnmarshalJSON flattens that so the fields below are
// always filled the same way.
type Guild struct {
	ID                          string        `json:"id"`
	Name                        string        `json:"name"`
	Icon                        string        `json:"icon"`
	IconHash                    string        `json:"icon_hash,omitempty"`
	Splash                      string        `json:"splash"`
	DiscoverySplash             string        `json:"discovery_splash"`
	Owner                       bool          `json:"owner,omitempty"`
	OwnerID                     string        `json:"owner_id"`
	Permissions                 string        `json:"permissions,omitempty"`
	Region                      string        `json:"region,omitempty"`
	AfkChannelID                string        `json:"afk_channel_id"`
	AfkTimeout                  int           `json:"afk_timeout"`
	WidgetEnabled               bool          `json:"widget_enabled,omitempty"`
	WidgetChannelID             string        `json:"widget_channel_id,omitempty"`
	VerificationLevel           uint64        `json:"verification_level"`
	DefaultMessageNotifications uint64        `json:"default_message_notifications"`
	ExplicitContentFilter       uint64        `json:"explicit_content_filter"`
	Roles                       []Role        `json:"roles"`
	Emojis                      []Emoji       `json:"emojis"`
	Features                    []string      `json:"features"`
	MFALevel                    uint64        `json:"mfa_level"`
	ApplicationID               string        `json:"application_id"`
	SystemChannelID             string        `json:"system_channel_id"`
	SystemChannelFlags          uint64        `json:"system_channel_flags"`
	RulesChannelID              string        `json:"rules_channel_id"`
	MaxPresences                int           `json:"max_presences,omitempty"`
	MaxMembers                  int           `json:"max_members,omitempty"`
	VanityUrl                   string        `json:"vanity_url_code"`
	Description                 string        `json:"description"`
	Banner                      string        `json:"banner"`
	PremiumTier                 uint64        `json:"premium_tier"`
	PremiumSubscriptionCount    int           `json:"premium_subscription_count,omitempty"`
	PreferredLocale             string        `json:"preferred_locale"`
	PublicUpdatesChannelID      string        `json:"public_updates_channel_id"`
	MaxVideoChannelUsers        int           `json:"max_video_channel_users,omitempty"`
	ApproximateMemberCount      int           `json:"approximate_member_count,omitempty"`
	ApproximatePresenceCount    int           `json:"approximate_presence_count,omitempty"`
	WelcomeScreen               WelcomeScreen `json:"welcome_screen,omitempty"`
	NSFWLevel                   uint64        `json:"nsfw_level"`
	Stickers                    []Sticker     `json:"stickers,omitempty"`
	PremiumProgressBarEnabled   bool          `json:"premium_progress_bar_enabled"`

	// Only present in gateway payloads (READY / GUILD_CREATE).
	JoinedAt    string        `json:"joined_at,omitempty"`
	Large       bool          `json:"large,omitempty"`
	Lazy        bool          `json:"lazy,omitempty"`
	MemberCount int           `json:"member_count,omitempty"`
	Channels    []Channel     `json:"channels,omitempty"`
	Threads     []Channel     `json:"threads,omitempty"`
	Members     []GuildMember `json:"members,omitempty"`

	Unavailable bool `json:"unavailable,omitempty"`
}

// UnmarshalJSON decodes a guild and merges the user-account "properties"
// object into the top-level fields.
func (g *Guild) UnmarshalJSON(b []byte) error {
	type plain Guild
	if err := json.Unmarshal(b, (*plain)(g)); err != nil {
		return err
	}
	var aux struct {
		Properties json.RawMessage `json:"properties"`
	}
	if err := json.Unmarshal(b, &aux); err != nil {
		return err
	}
	if len(aux.Properties) > 0 && string(aux.Properties) != "null" {
		return json.Unmarshal(aux.Properties, (*plain)(g))
	}
	return nil
}

// Role is a guild role.
type Role struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Color        int      `json:"color"`
	Hoist        bool     `json:"hoist"`
	Icon         string   `json:"icon,omitempty"`
	UnicodeEmoji string   `json:"unicode_emoji,omitempty"`
	Position     int      `json:"position"`
	Permissions  string   `json:"permissions"`
	Managed      bool     `json:"managed"`
	Mentionable  bool     `json:"mentionable"`
	Flags        int      `json:"flags,omitempty"`
	Tags         RoleTags `json:"tags,omitempty"`
}

// RoleTags describes what a role belongs to.
type RoleTags struct {
	BotID             string `json:"bot_id,omitempty"`
	IntegrationID     string `json:"integration_id,omitempty"`
	PremiumSubscriber bool   `json:"premium_subscriber,omitempty"`
}

// Emoji is a custom or unicode emoji.
type Emoji struct {
	ID            string   `json:"id"`
	Name          string   `json:"name,omitempty"`
	Roles         []string `json:"roles,omitempty"`
	User          User     `json:"user,omitempty"`
	RequireColons bool     `json:"require_colons,omitempty"`
	Managed       bool     `json:"managed,omitempty"`
	Animated      bool     `json:"animated,omitempty"`
	Available     bool     `json:"available,omitempty"`
}

// APIName returns the emoji in the format expected by the reaction endpoints:
// the unicode character itself, or "name:id" for custom emojis.
func (e Emoji) APIName() string {
	if e.ID == "" {
		return e.Name
	}
	return e.Name + ":" + e.ID
}

// WelcomeScreen is the guild welcome screen.
type WelcomeScreen struct {
	Description           string                 `json:"description"`
	WelcomeScreenChannels []WelcomeScreenChannel `json:"welcome_channels"`
}

// WelcomeScreenChannel is a channel shown on the welcome screen.
type WelcomeScreenChannel struct {
	ChannelID   string `json:"channel_id"`
	Description string `json:"description"`
	EmojiID     string `json:"emoji_id"`
	EmojiName   string `json:"emoji_name"`
}

// Sticker is a guild or standard sticker.
type Sticker struct {
	ID          string `json:"id"`
	PackID      string `json:"pack_id,omitempty"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Tags        string `json:"tags"`
	Asset       string `json:"asset,omitempty"`
	Type        uint64 `json:"type"`
	FormatType  uint64 `json:"format_type"`
	Available   bool   `json:"available,omitempty"`
	GuildID     string `json:"guild_id,omitempty"`
	User        User   `json:"user,omitempty"`
	SortValue   int    `json:"sort_value,omitempty"`
}

// Embed is a rich embed attached to a message.
type Embed struct {
	Title       string                 `json:"title,omitempty"`
	Type        string                 `json:"type,omitempty"`
	Description string                 `json:"description,omitempty"`
	URL         string                 `json:"url,omitempty"`
	Timestamp   *string                `json:"timestamp,omitempty"`
	Image       *MessageEmbedImage     `json:"image,omitempty"`
	Video       *MessageEmbedImage     `json:"video,omitempty"`
	Color       int                    `json:"color,omitempty"`
	Footer      EmbedFooter            `json:"footer,omitempty"`
	Thumbnail   *MessageEmbedThumbnail `json:"thumbnail,omitempty"`
	Provider    EmbedProvider          `json:"provider,omitempty"`
	Author      EmbedAuthor            `json:"author,omitempty"`
	Fields      []EmbedField           `json:"fields,omitempty"`
}

// MessageEmbedImage is an embed image or video.
type MessageEmbedImage struct {
	URL      string `json:"url,omitempty"`
	ProxyURL string `json:"proxy_url,omitempty"`
	Width    int    `json:"width,omitempty"`
	Height   int    `json:"height,omitempty"`
}

// EmbedField is a name/value field inside an embed.
type EmbedField struct {
	Name   string `json:"name,omitempty"`
	Value  string `json:"value,omitempty"`
	Inline bool   `json:"inline,omitempty"`
}

// EmbedFooter is the footer of an embed.
type EmbedFooter struct {
	Text         string `json:"text,omitempty"`
	IconURL      string `json:"icon_url,omitempty"`
	ProxyIconURL string `json:"proxy_icon_url,omitempty"`
}

// EmbedAuthor is the author block of an embed.
type EmbedAuthor struct {
	Name         string `json:"name,omitempty"`
	URL          string `json:"url,omitempty"`
	IconURL      string `json:"icon_url,omitempty"`
	ProxyIconURL string `json:"proxy_icon_url,omitempty"`
}

// MessageEmbedThumbnail is the thumbnail of an embed.
type MessageEmbedThumbnail struct {
	URL      string `json:"url,omitempty"`
	ProxyURL string `json:"proxy_url,omitempty"`
	Width    int    `json:"width,omitempty"`
	Height   int    `json:"height,omitempty"`
}

// EmbedProvider is the provider of an embed.
type EmbedProvider struct {
	Name string `json:"name,omitempty"`
	URL  string `json:"url,omitempty"`
}

// Reaction is a reaction on a message.
type Reaction struct {
	Emojis Emoji `json:"emoji,omitempty"`
	Count  int   `json:"count,omitempty"`
	Me     bool  `json:"me,omitempty"`
}

// Application is an application (bot) returned by the command index.
type Application struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Icon        string  `json:"icon"`
	BotID       string  `json:"bot_id"`
	Flags       FlexInt `json:"flags"`
}

// Application command types.
const (
	ApplicationCommandChatInput = 1 // slash command
	ApplicationCommandUser      = 2 // user context menu
	ApplicationCommandMessage   = 3 // message context menu
)

// Application command option types.
const (
	CommandOptionSubCommand      = 1
	CommandOptionSubCommandGroup = 2
	CommandOptionString          = 3
	CommandOptionInteger         = 4
	CommandOptionBoolean         = 5
	CommandOptionUser            = 6
	CommandOptionChannel         = 7
	CommandOptionRole            = 8
	CommandOptionMentionable     = 9
	CommandOptionNumber          = 10
	CommandOptionAttachment      = 11
)

// ApplicationCommand is a slash / context-menu command.
type ApplicationCommand struct {
	ID                       string                     `json:"id"`
	Type                     int                        `json:"type"`
	ApplicationID            string                     `json:"application_id"`
	GuildID                  string                     `json:"guild_id,omitempty"`
	Version                  string                     `json:"version"`
	Name                     string                     `json:"name"`
	Description              string                     `json:"description"`
	DescriptionDefault       string                     `json:"description_default,omitempty"`
	Options                  []ApplicationCommandOption `json:"options,omitempty"`
	DefaultMemberPermissions string                     `json:"default_member_permissions,omitempty"`
	DMPermission             bool                       `json:"dm_permission"`
	NSFW                     bool                       `json:"nsfw,omitempty"`
	IntegrationTypes         []int                      `json:"integration_types,omitempty"`
	Contexts                 []int                      `json:"contexts,omitempty"`
	GlobalPopularityRank     int                        `json:"global_popularity_rank,omitempty"`
}

// ApplicationCommandOption describes one option of a command.
type ApplicationCommandOption struct {
	Type         int                              `json:"type"`
	Name         string                           `json:"name"`
	Description  string                           `json:"description"`
	Required     bool                             `json:"required,omitempty"`
	Choices      []ApplicationCommandOptionChoice `json:"choices,omitempty"`
	Options      []ApplicationCommandOption       `json:"options,omitempty"`
	ChannelTypes []int                            `json:"channel_types,omitempty"`
	MinValue     *float64                         `json:"min_value,omitempty"`
	MaxValue     *float64                         `json:"max_value,omitempty"`
	MinLength    *int                             `json:"min_length,omitempty"`
	MaxLength    *int                             `json:"max_length,omitempty"`
	Autocomplete bool                             `json:"autocomplete,omitempty"`
}

// ApplicationCommandOptionChoice is a predefined value for an option.
type ApplicationCommandOptionChoice struct {
	Name  string `json:"name"`
	Value any    `json:"value"`
}

// ApplicationCommandInteractionOption is a value you pass when invoking a
// command. Use Options (and no Value) for sub-commands and groups.
type ApplicationCommandInteractionOption struct {
	Type    int                                   `json:"type"`
	Name    string                                `json:"name"`
	Value   any                                   `json:"value,omitempty"`
	Options []ApplicationCommandInteractionOption `json:"options,omitempty"`
}

// ApplicationCommandIndex is returned by the application-command-index endpoints.
type ApplicationCommandIndex struct {
	Applications       []Application        `json:"applications"`
	ApplicationCommand []ApplicationCommand `json:"application_commands"`
	Version            *string              `json:"version,omitempty"`
}

// Find returns the command with the given name. When applicationID is not
// empty only commands of that application are considered. It returns nil if
// no command matches.
func (idx *ApplicationCommandIndex) Find(name, applicationID string) *ApplicationCommand {
	for i := range idx.ApplicationCommand {
		cmd := &idx.ApplicationCommand[i]
		if cmd.Name == name && (applicationID == "" || cmd.ApplicationID == applicationID) {
			return cmd
		}
	}
	return nil
}

// Relationship is a friend, block or friend request of the current user.
type Relationship struct {
	ID       string `json:"id"`
	Type     int    `json:"type"`
	Nickname string `json:"nickname,omitempty"`
	UserID   string `json:"user_id,omitempty"`
	User     *User  `json:"user,omitempty"`
	Since    string `json:"since,omitempty"`
}

// Relationship types.
const (
	RelationshipFriend          = 1
	RelationshipBlocked         = 2
	RelationshipIncomingRequest = 3
	RelationshipOutgoingRequest = 4
)

// AllowedMentions controls who is pinged by a message you send.
type AllowedMentions struct {
	Parse       []string `json:"parse"`
	Users       []string `json:"users,omitempty"`
	Roles       []string `json:"roles,omitempty"`
	RepliedUser bool     `json:"replied_user"`
}

// MessageSend is the body of a "create message" request.
type MessageSend struct {
	Content           string            `json:"content"`
	Nonce             string            `json:"nonce,omitempty"`
	TTS               bool              `json:"tts"`
	Flags             int               `json:"flags"`
	MessageReference  *MessageReference `json:"message_reference,omitempty"`
	AllowedMentions   *AllowedMentions  `json:"allowed_mentions,omitempty"`
	StickerIDs        []string          `json:"sticker_ids,omitempty"`
	MobileNetworkType string            `json:"mobile_network_type,omitempty"`
}

// MessageEdit is the body of an "edit message" request.
type MessageEdit struct {
	Content *string `json:"content,omitempty"`
	Flags   *int    `json:"flags,omitempty"`
}
