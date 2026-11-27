package types

// Event names accepted by AddHandler, with the handler signature they expect.
//
// Every event (including ones not listed here) also accepts a
// func(*RawEvent) handler, which receives the undecoded payload.
const (
	// GatewayEventAll receives every dispatch: func(*RawEvent).
	GatewayEventAll = "*"

	GatewayEventReady   = "READY"   // func(*ReadyEventData)
	GatewayEventResumed = "RESUMED" // func()

	GatewayEventMessageCreate         = "MESSAGE_CREATE"          // func(*MessageEventData)
	GatewayEventMessageUpdate         = "MESSAGE_UPDATE"          // func(*MessageEventData)
	GatewayEventMessageDelete         = "MESSAGE_DELETE"          // func(*MessageDeleteEventData)
	GatewayEventMessageDeleteBulk     = "MESSAGE_DELETE_BULK"     // func(*MessageDeleteBulkEventData)
	GatewayEventMessageReactionAdd    = "MESSAGE_REACTION_ADD"    // func(*MessageReactionEventData)
	GatewayEventMessageReactionRemove = "MESSAGE_REACTION_REMOVE" // func(*MessageReactionEventData)
	GatewayEventTypingStart           = "TYPING_START"            // func(*TypingStartEventData)
	GatewayEventPresenceUpdate        = "PRESENCE_UPDATE"         // func(*PresenceUpdateEventData)
	GatewayEventUserUpdate            = "USER_UPDATE"             // func(*User)

	GatewayEventGuildCreate       = "GUILD_CREATE"        // func(*Guild)
	GatewayEventGuildUpdate       = "GUILD_UPDATE"        // func(*Guild)
	GatewayEventGuildDelete       = "GUILD_DELETE"        // func(*GuildDeleteEventData)
	GatewayEventGuildMemberAdd    = "GUILD_MEMBER_ADD"    // func(*GuildMemberEventData)
	GatewayEventGuildMemberUpdate = "GUILD_MEMBER_UPDATE" // func(*GuildMemberEventData)
	GatewayEventGuildMemberRemove = "GUILD_MEMBER_REMOVE" // func(*GuildMemberEventData)
	GatewayGuildMembersChunk      = "GUILD_MEMBERS_CHUNK" // func(*GuildMembersChunkEventData)

	GatewayEventGuildRoleCreate    = "GUILD_ROLE_CREATE"        // func(*GuildRoleEventData)
	GatewayEventGuildRoleUpdate    = "GUILD_ROLE_UPDATE"        // func(*GuildRoleEventData)
	GatewayEventGuildRoleDelete    = "GUILD_ROLE_DELETE"        // func(*GuildRoleDeleteEventData)
	GatewayEventGuildEmojisUpdate  = "GUILD_EMOJIS_UPDATE"      // func(*GuildEmojisUpdateEventData)
	GatewayEventReadySupplemental  = "READY_SUPPLEMENTAL"       // func(*RawEvent) - fills the state cache
	GatewayEventThreadListSync     = "THREAD_LIST_SYNC"         // func(*ThreadListSyncEventData)
	GatewayEventChannelCreate      = "CHANNEL_CREATE"           // func(*Channel)
	GatewayEventChannelUpdate      = "CHANNEL_UPDATE"           // func(*Channel)
	GatewayEventChannelDelete      = "CHANNEL_DELETE"           // func(*Channel)
	GatewayEventThreadCreate       = "THREAD_CREATE"            // func(*Channel)
	GatewayEventThreadUpdate       = "THREAD_UPDATE"            // func(*Channel)
	GatewayEventThreadDelete       = "THREAD_DELETE"            // func(*Channel)
	GatewayEventChannelPinsUpdate  = "CHANNEL_PINS_UPDATE"      // func(*RawEvent)
	GatewayEventVoiceStateUpdate   = "VOICE_STATE_UPDATE"       // func(*RawEvent)
	GatewayEventGuildMemberListUpd = "GUILD_MEMBER_LIST_UPDATE" // func(*RawEvent)

	GatewayEventRelationshipAdd    = "RELATIONSHIP_ADD"    // func(*Relationship)
	GatewayEventRelationshipUpdate = "RELATIONSHIP_UPDATE" // func(*Relationship)
	GatewayEventRelationshipRemove = "RELATIONSHIP_REMOVE" // func(*Relationship)

	GatewayEventInteractionCreate  = "INTERACTION_CREATE"  // func(*InteractionEventData)
	GatewayEventInteractionSuccess = "INTERACTION_SUCCESS" // func(*InteractionEventData)
	GatewayEventInteractionFailure = "INTERACTION_FAILURE" // func(*InteractionEventData)

	// GatewayEventReconnect fires when the connection dropped (or Discord
	// asked for a reconnect) and the client is reconnecting: func().
	GatewayEventReconnect = "RECONNECT"
	// GatewayEventInvalidated fires when Discord invalidated the session
	// (opcode 9); the client re-identifies automatically: func().
	GatewayEventInvalidated = "INVALIDATED"
	// GatewayEventDisconnect fires when the gateway stopped for good, either
	// because Close was called or because of a fatal error: func(error).
	// The error is nil after Close.
	GatewayEventDisconnect = "DISCONNECT"
)
