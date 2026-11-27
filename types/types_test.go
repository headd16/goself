package types

import (
	"testing"

	"github.com/goccy/go-json"
)

func TestFlexTypes(t *testing.T) {
	var v struct {
		A FlexString `json:"a"`
		B FlexString `json:"b"`
		C FlexString `json:"c"`
		D FlexInt    `json:"d"`
		E FlexInt    `json:"e"`
		F FlexInt    `json:"f"`
	}
	if err := json.Unmarshal([]byte(`{"a":"x\"y","b":123456789012345678,"c":null,"d":"42","e":7,"f":null}`), &v); err != nil {
		t.Fatal(err)
	}
	if v.A != `x"y` || v.B != "123456789012345678" || v.C != "" || v.D != 42 || v.E != 7 || v.F != 0 {
		t.Errorf("decoded %+v", v)
	}
}

func TestGuildPropertiesAreMerged(t *testing.T) {
	var g Guild
	data := `{"id":"1","properties":{"id":"1","name":"Test","owner_id":"9","premium_tier":2},
		"roles":[{"id":"r","name":"@everyone","permissions":"0","tags":{"premium_subscriber":null}}],
		"channels":[{"id":"c","type":0,"name":"general"}],"member_count":5}`
	if err := json.Unmarshal([]byte(data), &g); err != nil {
		t.Fatal(err)
	}
	if g.Name != "Test" || g.OwnerID != "9" || g.PremiumTier != 2 || len(g.Roles) != 1 || len(g.Channels) != 1 || g.MemberCount != 5 {
		t.Errorf("guild = %+v", g)
	}

	// Bot-style guilds without "properties" keep working.
	var plain Guild
	if err := json.Unmarshal([]byte(`{"id":"2","name":"Plain"}`), &plain); err != nil || plain.Name != "Plain" {
		t.Errorf("plain guild = %+v, %v", plain, err)
	}
}

func TestMessageComponentsAndSnapshots(t *testing.T) {
	data := `{"id":"m","nonce":42,"flags":32768,
		"components":[{"type":17,"components":[
			{"type":9,"components":[{"type":10,"content":"hello"}],"accessory":{"type":2,"style":1,"label":"Inner","custom_id":"inner"}},
			{"type":1,"components":[{"type":2,"style":3,"label":"Yes","custom_id":"yes","emoji":{"name":"✅"}},
			                        {"type":3,"custom_id":"menu","options":[{"label":"A","value":"a"}]}]}
		]}],
		"message_snapshots":[{"message":{"content":"forwarded"}}]}`
	var m MessageEventData
	if err := json.Unmarshal([]byte(data), &m); err != nil {
		t.Fatal(err)
	}
	if got := len(m.Buttons()); got != 2 {
		t.Errorf("buttons = %d, want 2", got)
	}
	if b := m.FindButton("✅"); b == nil || b.CustomID != "yes" {
		t.Errorf("FindButton by emoji = %+v", b)
	}
	if b := m.FindButton("inner"); b == nil || b.Label != "Inner" {
		t.Errorf("FindButton by custom id = %+v", b)
	}
	if menus := m.SelectMenus(); len(menus) != 1 || menus[0].Options[0].Value != "a" {
		t.Errorf("select menus = %+v", menus)
	}
	if len(m.MessageSnapshots) != 1 || m.MessageSnapshots[0].Message.Content != "forwarded" {
		t.Errorf("snapshots = %+v", m.MessageSnapshots)
	}
	if m.Nonce != "42" {
		t.Errorf("nonce = %q", m.Nonce)
	}
}

func TestUserHelpers(t *testing.T) {
	u := User{ID: "1", Username: "name", Discriminator: "0", Avatar: "a_hash"}
	if u.Tag() != "name" || u.DisplayName() != "name" || u.Mention() != "<@1>" {
		t.Errorf("helpers: %s %s %s", u.Tag(), u.DisplayName(), u.Mention())
	}
	if u.AvatarURL() != "https://cdn.discordapp.com/avatars/1/a_hash.gif" {
		t.Errorf("avatar = %s", u.AvatarURL())
	}
	old := User{Username: "old", Discriminator: "1234", GlobalName: "Old"}
	if old.Tag() != "old#1234" || old.DisplayName() != "Old" {
		t.Errorf("legacy tag = %s", old.Tag())
	}
}

func TestNewConfigIsACopy(t *testing.T) {
	c := NewConfig()
	c.Presence = "dnd"
	if DefaultConfig.Presence == "dnd" {
		t.Error("NewConfig returned the shared default")
	}
}
