package types

import (
	"strconv"
	"strings"
)

// Permission bits. See https://discord.com/developers/docs/topics/permissions
const (
	PermissionCreateInstantInvite              int64 = 1 << 0
	PermissionKickMembers                      int64 = 1 << 1
	PermissionBanMembers                       int64 = 1 << 2
	PermissionAdministrator                    int64 = 1 << 3
	PermissionManageChannels                   int64 = 1 << 4
	PermissionManageGuild                      int64 = 1 << 5
	PermissionAddReactions                     int64 = 1 << 6
	PermissionViewAuditLog                     int64 = 1 << 7
	PermissionPrioritySpeaker                  int64 = 1 << 8
	PermissionStream                           int64 = 1 << 9
	PermissionViewChannel                      int64 = 1 << 10
	PermissionSendMessages                     int64 = 1 << 11
	PermissionSendTTSMessages                  int64 = 1 << 12
	PermissionManageMessages                   int64 = 1 << 13
	PermissionEmbedLinks                       int64 = 1 << 14
	PermissionAttachFiles                      int64 = 1 << 15
	PermissionReadMessageHistory               int64 = 1 << 16
	PermissionMentionEveryone                  int64 = 1 << 17
	PermissionUseExternalEmojis                int64 = 1 << 18
	PermissionViewGuildInsights                int64 = 1 << 19
	PermissionConnect                          int64 = 1 << 20
	PermissionSpeak                            int64 = 1 << 21
	PermissionMuteMembers                      int64 = 1 << 22
	PermissionDeafenMembers                    int64 = 1 << 23
	PermissionMoveMembers                      int64 = 1 << 24
	PermissionUseVAD                           int64 = 1 << 25
	PermissionChangeNickname                   int64 = 1 << 26
	PermissionManageNicknames                  int64 = 1 << 27
	PermissionManageRoles                      int64 = 1 << 28
	PermissionManageWebhooks                   int64 = 1 << 29
	PermissionManageGuildExpressions           int64 = 1 << 30
	PermissionUseApplicationCommands           int64 = 1 << 31
	PermissionRequestToSpeak                   int64 = 1 << 32
	PermissionManageEvents                     int64 = 1 << 33
	PermissionManageThreads                    int64 = 1 << 34
	PermissionCreatePublicThreads              int64 = 1 << 35
	PermissionCreatePrivateThreads             int64 = 1 << 36
	PermissionUseExternalStickers              int64 = 1 << 37
	PermissionSendMessagesInThreads            int64 = 1 << 38
	PermissionUseEmbeddedActivities            int64 = 1 << 39
	PermissionModerateMembers                  int64 = 1 << 40
	PermissionViewCreatorMonetizationAnalytics int64 = 1 << 41
	PermissionUseSoundboard                    int64 = 1 << 42
	PermissionCreateGuildExpressions           int64 = 1 << 43
	PermissionCreateEvents                     int64 = 1 << 44
	PermissionUseExternalSounds                int64 = 1 << 45
	PermissionSendVoiceMessages                int64 = 1 << 46
	PermissionSendPolls                        int64 = 1 << 49
	PermissionUseExternalApps                  int64 = 1 << 50

	// PermissionAll has every permission bit set.
	PermissionAll int64 = 1<<51 - 1

	// PermissionsDM are the effective permissions in DMs and group DMs.
	PermissionsDM = PermissionViewChannel | PermissionSendMessages | PermissionSendTTSMessages |
		PermissionEmbedLinks | PermissionAttachFiles | PermissionReadMessageHistory |
		PermissionAddReactions | PermissionUseExternalEmojis | PermissionUseExternalStickers |
		PermissionConnect | PermissionSpeak | PermissionStream | PermissionUseVAD |
		PermissionUseApplicationCommands | PermissionSendVoiceMessages | PermissionSendPolls |
		PermissionUseExternalApps
)

// Permission overwrite types.
const (
	OverwriteRole   = 0
	OverwriteMember = 1
)

var permissionNames = []struct {
	bit  int64
	name string
}{
	{PermissionAdministrator, "Administrator"}, {PermissionManageGuild, "ManageGuild"},
	{PermissionManageRoles, "ManageRoles"}, {PermissionManageChannels, "ManageChannels"},
	{PermissionKickMembers, "KickMembers"}, {PermissionBanMembers, "BanMembers"},
	{PermissionModerateMembers, "ModerateMembers"}, {PermissionManageMessages, "ManageMessages"},
	{PermissionManageThreads, "ManageThreads"}, {PermissionManageNicknames, "ManageNicknames"},
	{PermissionManageWebhooks, "ManageWebhooks"}, {PermissionManageGuildExpressions, "ManageGuildExpressions"},
	{PermissionManageEvents, "ManageEvents"}, {PermissionViewAuditLog, "ViewAuditLog"},
	{PermissionViewGuildInsights, "ViewGuildInsights"}, {PermissionMentionEveryone, "MentionEveryone"},
	{PermissionViewChannel, "ViewChannel"}, {PermissionSendMessages, "SendMessages"},
	{PermissionSendMessagesInThreads, "SendMessagesInThreads"}, {PermissionCreatePublicThreads, "CreatePublicThreads"},
	{PermissionCreatePrivateThreads, "CreatePrivateThreads"}, {PermissionSendTTSMessages, "SendTTSMessages"},
	{PermissionEmbedLinks, "EmbedLinks"}, {PermissionAttachFiles, "AttachFiles"},
	{PermissionReadMessageHistory, "ReadMessageHistory"}, {PermissionAddReactions, "AddReactions"},
	{PermissionUseExternalEmojis, "UseExternalEmojis"}, {PermissionUseExternalStickers, "UseExternalStickers"},
	{PermissionUseApplicationCommands, "UseApplicationCommands"}, {PermissionUseExternalApps, "UseExternalApps"},
	{PermissionSendVoiceMessages, "SendVoiceMessages"}, {PermissionSendPolls, "SendPolls"},
	{PermissionCreateInstantInvite, "CreateInstantInvite"}, {PermissionChangeNickname, "ChangeNickname"},
	{PermissionConnect, "Connect"}, {PermissionSpeak, "Speak"}, {PermissionStream, "Stream"},
	{PermissionUseVAD, "UseVAD"}, {PermissionPrioritySpeaker, "PrioritySpeaker"},
	{PermissionMuteMembers, "MuteMembers"}, {PermissionDeafenMembers, "DeafenMembers"},
	{PermissionMoveMembers, "MoveMembers"}, {PermissionRequestToSpeak, "RequestToSpeak"},
	{PermissionUseEmbeddedActivities, "UseEmbeddedActivities"}, {PermissionUseSoundboard, "UseSoundboard"},
	{PermissionUseExternalSounds, "UseExternalSounds"}, {PermissionCreateGuildExpressions, "CreateGuildExpressions"},
	{PermissionCreateEvents, "CreateEvents"}, {PermissionViewCreatorMonetizationAnalytics, "ViewCreatorMonetizationAnalytics"},
}

// HasPermission reports whether perms contains every bit of perm.
// Administrator implies every permission.
func HasPermission(perms, perm int64) bool {
	if perms&PermissionAdministrator != 0 {
		return true
	}
	return perms&perm == perm
}

// PermissionNames returns the names of the permission bits set in perms,
// e.g. ["ViewChannel", "SendMessages"].
func PermissionNames(perms int64) []string {
	var names []string
	for _, p := range permissionNames {
		if perms&p.bit != 0 {
			names = append(names, p.name)
		}
	}
	return names
}

// ParsePermissions parses a permission string ("2147483648") as sent by Discord.
func ParsePermissions(s string) int64 {
	n, _ := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	return n
}

// ComputeBasePermissions returns the guild-level permissions of a member.
func ComputeBasePermissions(guild *Guild, member *GuildMember) int64 {
	if guild.OwnerID != "" && guild.OwnerID == member.ID() {
		return PermissionAll
	}
	roles := make(map[string]*Role, len(guild.Roles))
	for i := range guild.Roles {
		roles[guild.Roles[i].ID] = &guild.Roles[i]
	}
	var perms int64
	if everyone, ok := roles[guild.ID]; ok {
		perms = ParsePermissions(everyone.Permissions)
	}
	for _, id := range member.Roles {
		if r, ok := roles[id]; ok {
			perms |= ParsePermissions(r.Permissions)
		}
	}
	if perms&PermissionAdministrator != 0 {
		return PermissionAll
	}
	return perms
}

// ComputeChannelPermissions returns the permissions of a member in a guild
// channel, applying the channel's permission overwrites to the base
// permissions. For threads, pass the parent channel.
func ComputeChannelPermissions(guild *Guild, member *GuildMember, channel *Channel) int64 {
	perms := ComputeBasePermissions(guild, member)
	if perms&PermissionAdministrator != 0 {
		return PermissionAll
	}

	var roleAllow, roleDeny int64
	var memberOverwrite *PermissionOverwrite
	for i := range channel.PermissionOverwrites {
		ow := &channel.PermissionOverwrites[i]
		switch {
		case ow.Type == OverwriteRole && ow.ID == guild.ID: // @everyone
			perms &^= ParsePermissions(ow.Deny)
			perms |= ParsePermissions(ow.Allow)
		case ow.Type == OverwriteRole && member.HasRole(ow.ID):
			roleDeny |= ParsePermissions(ow.Deny)
			roleAllow |= ParsePermissions(ow.Allow)
		case ow.Type == OverwriteMember && ow.ID == member.ID():
			memberOverwrite = ow
		}
	}
	perms &^= roleDeny
	perms |= roleAllow
	if memberOverwrite != nil {
		perms &^= ParsePermissions(memberOverwrite.Deny)
		perms |= ParsePermissions(memberOverwrite.Allow)
	}

	// Without View Channel nothing else in the channel is usable.
	if perms&PermissionViewChannel == 0 {
		return 0
	}
	return perms
}
