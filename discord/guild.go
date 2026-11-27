package discord

import "github.com/headd16/goself/types"

// GetGuild returns a guild (with approximate member counts).
func (g *Gateway) GetGuild(guildID string) (*types.Guild, error) {
	var guild types.Guild
	if err := g.request("GET", "/guilds/"+guildID+"?with_counts=true", nil, &guild, discordOrigin+"/channels/"+guildID); err != nil {
		return nil, err
	}
	g.State.storeGuild(&guild)
	return &guild, nil
}

// GetGuilds returns the (partial) guilds the account is a member of.
func (g *Gateway) GetGuilds() ([]types.Guild, error) {
	var guilds []types.Guild
	if err := g.request("GET", "/users/@me/guilds?with_counts=true", nil, &guilds, ""); err != nil {
		return nil, err
	}
	g.State.storePartialGuilds(guilds)
	return guilds, nil
}

// GetGuildChannels returns the channels of a guild.
func (g *Gateway) GetGuildChannels(guildID string) ([]types.Channel, error) {
	var channels []types.Channel
	if err := g.request("GET", "/guilds/"+guildID+"/channels", nil, &channels, discordOrigin+"/channels/"+guildID); err != nil {
		return nil, err
	}
	g.State.replaceGuildChannels(guildID, channels)
	return channels, nil
}

// LeaveGuild leaves a guild.
func (g *Gateway) LeaveGuild(guildID string) error {
	return g.request("DELETE", "/users/@me/guilds/"+guildID, map[string]bool{"lurking": false}, nil, "")
}

// GetCurrentUser fetches the logged-in user from the REST API.
func (g *Gateway) GetCurrentUser() (*types.User, error) {
	var u types.User
	if err := g.request("GET", "/users/@me", nil, &u, ""); err != nil {
		return nil, err
	}
	g.State.storeUsers(&u)
	return &u, nil
}

// GetUser fetches a user by ID.
func (g *Gateway) GetUser(userID string) (*types.User, error) {
	var u types.User
	if err := g.request("GET", "/users/"+userID, nil, &u, ""); err != nil {
		return nil, err
	}
	g.State.storeUsers(&u)
	return &u, nil
}

// GetRelationships returns friends, blocked users and pending friend requests.
func (g *Gateway) GetRelationships() ([]types.Relationship, error) {
	var rels []types.Relationship
	if err := g.request("GET", "/users/@me/relationships", nil, &rels, ""); err != nil {
		return nil, err
	}
	g.State.storeRelationships(rels)
	return rels, nil
}
