package types

import (
	"strings"

	"github.com/goccy/go-json"
)

// Opcode is the "op" field shared by every gateway payload.
type Opcode struct {
	Op int `json:"op"`
}

// HelloEvent is the first payload sent by the gateway.
type HelloEvent struct {
	Opcode
	D struct {
		HeartbeatInterval int `json:"heartbeat_interval"`
	} `json:"d"`
}

// ResumePayload resumes a dropped gateway session.
type ResumePayload struct {
	Op int               `json:"op"`
	D  ResumePayloadData `json:"d"`
}

// ResumePayloadData is the data of a ResumePayload.
type ResumePayloadData struct {
	Token     string `json:"token"`
	SessionID string `json:"session_id"`
	Seq       int64  `json:"seq"`
}

// IdentifyPayload starts a new gateway session.
type IdentifyPayload struct {
	Op int64               `json:"op"`
	D  IdentifyPayloadData `json:"d"`
}

// IdentifyPayloadData is the data of an IdentifyPayload.
type IdentifyPayloadData struct {
	Token        string          `json:"token"`
	Capabilities int64           `json:"capabilities"`
	Properties   SuperProperties `json:"properties"`
	Presence     Presence        `json:"presence"`
	Compress     bool            `json:"compress"`
	ClientState  ClientState     `json:"client_state"`
}

// ClientState tells the gateway what the client already has cached.
type ClientState struct {
	GuildVersions            GuildVersions `json:"guild_versions"`
	HighestLastMessageID     string        `json:"highest_last_message_id"`
	ReadStateVersion         int64         `json:"read_state_version"`
	UserGuildSettingsVersion int64         `json:"user_guild_settings_version"`
	UserSettingsVersion      int64         `json:"user_settings_version"`
	PrivateChannelsVersion   string        `json:"private_channels_version"`
	APICodeVersion           int64         `json:"api_code_version"`
}

// GuildVersions is an (empty) map of cached guild versions.
type GuildVersions struct {
}

// Presence statuses.
const (
	StatusOnline    = "online"
	StatusIdle      = "idle"
	StatusDND       = "dnd"
	StatusInvisible = "invisible"
	StatusOffline   = "offline"
)

// Presence is sent when identifying and with UpdatePresence (opcode 3).
type Presence struct {
	Status     string     `json:"status"`
	Since      int64      `json:"since"`
	Activities []Activity `json:"activities"`
	Afk        bool       `json:"afk"`
}

// Activity types.
const (
	ActivityPlaying   = 0
	ActivityStreaming = 1
	ActivityListening = 2
	ActivityWatching  = 3
	ActivityCustom    = 4
	ActivityCompeting = 5
)

// Activity is a "playing / listening / custom status" activity.
type Activity struct {
	Name          string              `json:"name"`
	Type          int                 `json:"type"`
	URL           string              `json:"url,omitempty"`
	CreatedAt     FlexInt             `json:"created_at,omitempty"`
	Timestamps    *ActivityTimestamps `json:"timestamps,omitempty"`
	ApplicationID string              `json:"application_id,omitempty"`
	Details       string              `json:"details,omitempty"`
	State         string              `json:"state,omitempty"`
	Emoji         *ActivityEmoji      `json:"emoji,omitempty"`
	Party         *ActivityParty      `json:"party,omitempty"`
	Assets        *ActivityAssets     `json:"assets,omitempty"`
	Buttons       []string            `json:"buttons,omitempty"`
	Flags         int                 `json:"flags,omitempty"`
}

// ActivityTimestamps are the start/end unix milliseconds of an activity.
type ActivityTimestamps struct {
	Start FlexInt `json:"start,omitempty"`
	End   FlexInt `json:"end,omitempty"`
}

// ActivityEmoji is the emoji of a custom status.
type ActivityEmoji struct {
	Name     string `json:"name"`
	ID       string `json:"id,omitempty"`
	Animated bool   `json:"animated,omitempty"`
}

// ActivityParty describes the party of an activity.
type ActivityParty struct {
	ID   string `json:"id,omitempty"`
	Size []int  `json:"size,omitempty"`
}

// ActivityAssets are the images of a rich presence.
type ActivityAssets struct {
	LargeImage string `json:"large_image,omitempty"`
	LargeText  string `json:"large_text,omitempty"`
	SmallImage string `json:"small_image,omitempty"`
	SmallText  string `json:"small_text,omitempty"`
}

// ClientStatus is the per-platform status of a user.
type ClientStatus struct {
	Desktop  string `json:"desktop,omitempty"`
	Mobile   string `json:"mobile,omitempty"`
	Web      string `json:"web,omitempty"`
	Embedded string `json:"embedded,omitempty"`
}

// SuperProperties describe the client to Discord. They are sent when
// identifying and, base64 encoded, in the X-Super-Properties header.
type SuperProperties struct {
	OS                     string `json:"os"`
	Browser                string `json:"browser"`
	Device                 string `json:"device"`
	SystemLocale           string `json:"system_locale"`
	HasClientMods          bool   `json:"has_client_mods"`
	BrowserUserAgent       string `json:"browser_user_agent"`
	BrowserVersion         string `json:"browser_version"`
	OSVersion              string `json:"os_version"`
	Referrer               string `json:"referrer"`
	ReferringDomain        string `json:"referring_domain"`
	ReferrerCurrent        string `json:"referrer_current"`
	ReferringDomainCurrent string `json:"referring_domain_current"`
	ReleaseChannel         string `json:"release_channel"`
	ClientBuildNumber      int    `json:"client_build_number"`
	ClientEventSource      any    `json:"client_event_source"`
}

// DefaultEvent is the generic gateway payload structure.
// https://discord.com/developers/docs/topics/gateway-events#payload-structure
type DefaultEvent struct {
	Op int    `json:"op"`
	T  string `json:"t,omitempty"`
	S  int    `json:"s,omitempty"`
	D  any    `json:"d,omitempty"`
}

// RawEvent is an undecoded gateway dispatch. Register a func(*RawEvent)
// handler for GatewayEventAll ("*") or for any event name to receive it.
type RawEvent struct {
	Op   int             `json:"op"`
	Type string          `json:"t"`
	Seq  int64           `json:"s"`
	Data json.RawMessage `json:"d"`
}

// Decode unmarshals the event data into v.
func (e *RawEvent) Decode(v any) error { return json.Unmarshal(e.Data, v) }

// ReadyEvent is the READY dispatch.
type ReadyEvent struct {
	Op int64          `json:"op"`
	D  ReadyEventData `json:"d"`
}

// ReadyEventData is the data of the READY event.
type ReadyEventData struct {
	Version          int            `json:"v"`
	User             User           `json:"user"`
	Users            []User         `json:"users,omitempty"`
	Guilds           []Guild        `json:"guilds"`
	PrivateChannels  []Channel      `json:"private_channels,omitempty"`
	Relationships    []Relationship `json:"relationships,omitempty"`
	SessionID        string         `json:"session_id"`
	ResumeGatewayURL string         `json:"resume_gateway_url"`
	CountryCode      string         `json:"country_code,omitempty"`
	// AuthToken is set when Discord rotates the account token; the client
	// switches to it automatically.
	AuthToken string `json:"auth_token,omitempty"`
}

// MessageEvent is a MESSAGE_CREATE / MESSAGE_UPDATE dispatch.
type MessageEvent struct {
	Op int              `json:"op"`
	D  MessageEventData `json:"d"`
}

// MemberEvent is a GUILD_MEMBERS_CHUNK dispatch.
type MemberEvent struct {
	Op int                        `json:"op"`
	D  GuildMembersChunkEventData `json:"d"`
}

// MessageEventData is a full message, as received in MESSAGE_CREATE /
// MESSAGE_UPDATE events and returned by the REST API.
type MessageEventData struct {
	// Data is in a different struct because it needs to be recursive.
	MessageData
	// ReferencedMessage is the message this one replies to (zero value if none).
	ReferencedMessage MessageData `json:"referenced_message"`
}

// Message is an alias of MessageEventData used by the REST helpers.
type Message = MessageEventData

// GuildMembersChunkEventData is the data of GUILD_MEMBERS_CHUNK, the answer to
// RequestGuildMembers.
type GuildMembersChunkEventData struct {
	GuildId    string                    `json:"guild_id"`
	Members    []GuildMember             `json:"members"`
	Presences  []PresenceUpdateEventData `json:"presences"`
	NotFound   []FlexString              `json:"not_found"`
	ChunkIndex int                       `json:"chunk_index"`
	ChunkCount int                       `json:"chunk_count"`
	Nonce      string                    `json:"nonce,omitempty"`
}

// Message types (subset).
const (
	MessageTypeDefault          = 0
	MessageTypeReply            = 19
	MessageTypeChatInputCommand = 20
)

// Message flags (subset).
const (
	MessageFlagCrossposted           = 1 << 0
	MessageFlagIsCrosspost           = 1 << 1
	MessageFlagSuppressEmbeds        = 1 << 2
	MessageFlagHasThread             = 1 << 5
	MessageFlagEphemeral             = 1 << 6
	MessageFlagLoading               = 1 << 7
	MessageFlagSuppressNotifications = 1 << 12
	MessageFlagIsVoiceMessage        = 1 << 13
	MessageFlagHasSnapshot           = 1 << 14
	MessageFlagIsComponentsV2        = 1 << 15
)

// MessageData holds the fields of a message.
type MessageData struct {
	ID                  string               `json:"id"`
	ChannelID           string               `json:"channel_id"`
	GuildID             string               `json:"guild_id"`
	Author              User                 `json:"author"`
	Member              *GuildMember         `json:"member,omitempty"`
	Content             string               `json:"content"`
	Timestamp           string               `json:"timestamp"`
	EditedTimestamp     string               `json:"edited_timestamp"`
	Tts                 bool                 `json:"tts"`
	MentionEveryone     bool                 `json:"mention_everyone"`
	Mentions            []User               `json:"mentions"`
	MentionRoles        []string             `json:"mention_roles"`
	MentionChannels     []ChannelMention     `json:"mention_channels"`
	Attachments         []Attachment         `json:"attachments"`
	Components          []Component          `json:"components"`
	Embeds              []Embed              `json:"embeds"`
	Reactions           []Reaction           `json:"reactions"`
	Nonce               FlexString           `json:"nonce"`
	Pinned              bool                 `json:"pinned"`
	WebhookID           string               `json:"webhook_id"`
	Type                int                  `json:"type"`
	Activity            MessageActivity      `json:"activity"`
	Application         MessageApplication   `json:"application"`
	ApplicationID       string               `json:"application_id"`
	Flags               int                  `json:"flags"`
	MessageReference    MessageReference     `json:"message_reference"`
	MessageSnapshots    []MessageSnapshot    `json:"message_snapshots,omitempty"`
	Interaction         MessageInteraction   `json:"interaction"`
	InteractionMetadata *InteractionMetadata `json:"interaction_metadata,omitempty"`
	Thread              Channel              `json:"thread"`
	StickerItems        []StickerItem        `json:"sticker_items"`
	Position            int                  `json:"position,omitempty"`
}

// Buttons returns every button of the message, including buttons nested in
// sections and containers.
func (m *MessageData) Buttons() []Component {
	var out []Component
	m.walkComponents(func(c *Component) {
		if c.Type == ComponentButton {
			out = append(out, *c)
		}
	})
	return out
}

// SelectMenus returns every select menu of the message.
func (m *MessageData) SelectMenus() []Component {
	var out []Component
	m.walkComponents(func(c *Component) {
		if c.IsSelectMenu() {
			out = append(out, *c)
		}
	})
	return out
}

// FindButton returns the first button whose label, custom ID or emoji name
// equals text (case-insensitive), or nil.
func (m *MessageData) FindButton(text string) *Component {
	var found *Component
	m.walkComponents(func(c *Component) {
		if found != nil || c.Type != ComponentButton {
			return
		}
		if strings.EqualFold(c.Label, text) || c.CustomID == text ||
			(c.Emoji != nil && c.Emoji.Name != "" && c.Emoji.Name == text) {
			found = c
		}
	})
	return found
}

// IsReply reports whether the message replies to another message.
func (m *MessageData) IsReply() bool {
	return m.Type == MessageTypeReply && m.MessageReference.MessageID != ""
}

// MentionsUser reports whether the message mentions the given user ID.
func (m *MessageData) MentionsUser(userID string) bool {
	for _, u := range m.Mentions {
		if u.ID == userID {
			return true
		}
	}
	return false
}

func (m *MessageData) walkComponents(fn func(*Component)) {
	for i := range m.Components {
		m.Components[i].Walk(fn)
	}
}

// Attachment is a file attached to a message.
type Attachment struct {
	ID                 string  `json:"id"`
	Filename           string  `json:"filename"`
	Title              string  `json:"title,omitempty"`
	Description        string  `json:"description,omitempty"`
	Size               int     `json:"size"`
	URL                string  `json:"url"`
	ProxyURL           string  `json:"proxy_url"`
	Width              int     `json:"width"`
	Height             int     `json:"height"`
	ContentType        string  `json:"content_type"`
	Placeholder        string  `json:"placeholder"`
	PlaceholderVersion int     `json:"placeholder_version"`
	Ephemeral          bool    `json:"ephemeral,omitempty"`
	DurationSecs       float64 `json:"duration_secs,omitempty"`
	Flags              int     `json:"flags,omitempty"`
}

// Message reference types.
const (
	MessageReferenceDefault = 0 // reply
	MessageReferenceForward = 1
)

// MessageReference points to another message (replies, forwards, crossposts).
type MessageReference struct {
	Type            int    `json:"type,omitempty"`
	MessageID       string `json:"message_id,omitempty"`
	ChannelID       string `json:"channel_id,omitempty"`
	GuildID         string `json:"guild_id,omitempty"`
	FailIfNotExists *bool  `json:"fail_if_not_exists,omitempty"`
}

// MessageSnapshot is a copy of a forwarded message.
type MessageSnapshot struct {
	Message MessageData `json:"message"`
}

// MessageActivity is attached to rich presence invites.
type MessageActivity struct {
	Type    int    `json:"type"`
	PartyID string `json:"party_id"`
}

// MessageApplication is attached to rich presence invites.
type MessageApplication struct {
	ID          string `json:"id"`
	CoverImage  string `json:"cover_image"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
	Name        string `json:"name"`
}

// MessageInteraction is the (deprecated) interaction info of a bot reply.
type MessageInteraction struct {
	ID   string `json:"id"`
	Type int    `json:"type"`
	Name string `json:"name"`
	User User   `json:"user"`
}

// InteractionMetadata describes the interaction a bot message answers.
type InteractionMetadata struct {
	ID                   string `json:"id"`
	Type                 int    `json:"type"`
	Name                 string `json:"name,omitempty"`
	User                 User   `json:"user"`
	OriginalResponseID   string `json:"original_response_message_id,omitempty"`
	InteractedMessageID  string `json:"interacted_message_id,omitempty"`
	TriggeringMetadataID string `json:"triggering_interaction_metadata_id,omitempty"`
}

// StickerItem is the short form of a sticker sent with a message.
type StickerItem struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	FormatType   uint64 `json:"format_type"`
	Asset        string `json:"asset"`
	PreviewAsset string `json:"preview_asset"`
	Type         uint64 `json:"type"`
}

// Channel types.
const (
	ChannelGuildText          = 0
	ChannelDM                 = 1
	ChannelGuildVoice         = 2
	ChannelGroupDM            = 3
	ChannelGuildCategory      = 4
	ChannelGuildAnnouncement  = 5
	ChannelAnnouncementThread = 10
	ChannelPublicThread       = 11
	ChannelPrivateThread      = 12
	ChannelGuildStageVoice    = 13
	ChannelGuildDirectory     = 14
	ChannelGuildForum         = 15
	ChannelGuildMedia         = 16
)

// Channel is a guild channel, thread, DM or group DM.
type Channel struct {
	ID                   string                `json:"id"`
	GuildID              string                `json:"guild_id"`
	Name                 string                `json:"name"`
	Type                 int                   `json:"type"`
	Topic                string                `json:"topic"`
	Nsfw                 bool                  `json:"nsfw"`
	Position             int                   `json:"position"`
	Icon                 string                `json:"icon"`
	LastMessageID        string                `json:"last_message_id"`
	ParentID             string                `json:"parent_id"`
	OwnerID              string                `json:"owner_id,omitempty"`
	LastPinTimestamp     string                `json:"last_pin_timestamp"`
	RateLimitPerUser     int                   `json:"rate_limit_per_user,omitempty"`
	Bitrate              int                   `json:"bitrate,omitempty"`
	UserLimit            int                   `json:"user_limit,omitempty"`
	Flags                int                   `json:"flags,omitempty"`
	Recipients           []User                `json:"recipients,omitempty"`
	RecipientIDs         []string              `json:"recipient_ids,omitempty"`
	PermissionOverwrites []PermissionOverwrite `json:"permission_overwrites,omitempty"`
	MessageCount         int                   `json:"message_count,omitempty"`
	MemberCount          int                   `json:"member_count,omitempty"`
	ThreadMetadata       *ThreadMetadata       `json:"thread_metadata,omitempty"`
	AppliedTags          []string              `json:"applied_tags,omitempty"`
	// Permissions is set on channels returned in interaction payloads.
	Permissions string `json:"permissions,omitempty"`
}

// ThreadMetadata holds thread-specific fields.
type ThreadMetadata struct {
	Archived            bool   `json:"archived"`
	AutoArchiveDuration int    `json:"auto_archive_duration"`
	ArchiveTimestamp    string `json:"archive_timestamp"`
	Locked              bool   `json:"locked"`
	Invitable           bool   `json:"invitable,omitempty"`
	CreateTimestamp     string `json:"create_timestamp,omitempty"`
}

// IsCategory reports whether the channel is a category.
func (c *Channel) IsCategory() bool { return c.Type == ChannelGuildCategory }

// IsVoice reports whether the channel is a voice or stage channel.
func (c *Channel) IsVoice() bool {
	return c.Type == ChannelGuildVoice || c.Type == ChannelGuildStageVoice
}

// Mention returns the string that mentions the channel in a message.
func (c *Channel) Mention() string { return "<#" + c.ID + ">" }

// IsThread reports whether the channel is a thread.
func (c *Channel) IsThread() bool {
	return c.Type == ChannelAnnouncementThread || c.Type == ChannelPublicThread || c.Type == ChannelPrivateThread
}

// IsPrivate reports whether the channel is a DM or group DM.
func (c *Channel) IsPrivate() bool { return c.Type == ChannelDM || c.Type == ChannelGroupDM }

// PermissionOverwrite is a role / member permission override on a channel.
type PermissionOverwrite struct {
	ID    string `json:"id"`
	Type  int    `json:"type"`
	Allow string `json:"allow"`
	Deny  string `json:"deny"`
}

// ChannelMention is a channel mentioned in a crossposted message.
type ChannelMention struct {
	ID      string `json:"id"`
	GuildID string `json:"guild_id"`
	Type    int    `json:"type"`
	Name    string `json:"name"`
}

// MessageDeleteEventData is the data of MESSAGE_DELETE.
type MessageDeleteEventData struct {
	ID        string `json:"id"`
	ChannelID string `json:"channel_id"`
	GuildID   string `json:"guild_id,omitempty"`
}

// MessageDeleteBulkEventData is the data of MESSAGE_DELETE_BULK.
type MessageDeleteBulkEventData struct {
	IDs       []string `json:"ids"`
	ChannelID string   `json:"channel_id"`
	GuildID   string   `json:"guild_id,omitempty"`
}

// MessageReactionEventData is the data of MESSAGE_REACTION_ADD / MESSAGE_REACTION_REMOVE.
type MessageReactionEventData struct {
	UserID          string       `json:"user_id"`
	ChannelID       string       `json:"channel_id"`
	MessageID       string       `json:"message_id"`
	GuildID         string       `json:"guild_id,omitempty"`
	Member          *GuildMember `json:"member,omitempty"`
	Emoji           Emoji        `json:"emoji"`
	MessageAuthorID string       `json:"message_author_id,omitempty"`
	Burst           bool         `json:"burst,omitempty"`
	Type            int          `json:"type,omitempty"`
}

// TypingStartEventData is the data of TYPING_START.
type TypingStartEventData struct {
	ChannelID string       `json:"channel_id"`
	GuildID   string       `json:"guild_id,omitempty"`
	UserID    string       `json:"user_id"`
	Timestamp int64        `json:"timestamp"`
	Member    *GuildMember `json:"member,omitempty"`
}

// PresenceUpdateEventData is the data of PRESENCE_UPDATE (and the presences
// of a GUILD_MEMBERS_CHUNK).
type PresenceUpdateEventData struct {
	User User `json:"user"`
	// UserID replaces User in READY_SUPPLEMENTAL merged presences.
	UserID       string       `json:"user_id,omitempty"`
	GuildID      string       `json:"guild_id,omitempty"`
	Status       string       `json:"status"`
	Activities   []Activity   `json:"activities"`
	ClientStatus ClientStatus `json:"client_status"`
}

// GuildDeleteEventData is the data of GUILD_DELETE. Unavailable is false when
// the user left or was removed from the guild.
type GuildDeleteEventData struct {
	ID          string `json:"id"`
	Unavailable bool   `json:"unavailable,omitempty"`
}

// GuildMemberEventData is the data of GUILD_MEMBER_ADD / UPDATE / REMOVE.
type GuildMemberEventData struct {
	GuildMember
	GuildID string `json:"guild_id"`
}

// InteractionEventData is the data of INTERACTION_CREATE, INTERACTION_SUCCESS
// and INTERACTION_FAILURE, which the gateway sends to user accounts after they
// click a button, pick a select option or run a slash command. Nonce matches
// the nonce returned by ClickButton / SelectMenu / SendSlashCommand.
type InteractionEventData struct {
	ID    string     `json:"id"`
	Nonce FlexString `json:"nonce"`
}

// VoiceStateUpdatePayload is sent with opcode 4 to join, move or leave voice.
type VoiceStateUpdatePayload struct {
	GuildID   *string `json:"guild_id"`
	ChannelID *string `json:"channel_id"`
	SelfMute  bool    `json:"self_mute"`
	SelfDeaf  bool    `json:"self_deaf"`
	SelfVideo bool    `json:"self_video"`
}

// RequestGuildMembersPayload is sent with opcode 8.
type RequestGuildMembersPayload struct {
	GuildID   string   `json:"guild_id"`
	UserIDs   []string `json:"user_ids,omitempty"`
	Query     *string  `json:"query,omitempty"`
	Limit     *int     `json:"limit,omitempty"`
	Presences bool     `json:"presences"`
	Nonce     string   `json:"nonce,omitempty"`
}
