package discord

import (
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/goccy/go-json"
	"github.com/headd16/goself/types"
)

// newRESTGateway returns a gateway whose REST calls go to handler.
func newRESTGateway(t *testing.T, handler http.HandlerFunc) *Gateway {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return NewGateway("rest-token", testConfig("", srv.URL+"/api/v9"))
}

func decodeBody(t *testing.T, r *http.Request, v any) {
	t.Helper()
	b, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, v); err != nil {
		t.Fatalf("invalid JSON body %s: %v", b, err)
	}
}

func TestSendMessageEscapesContentAndSetsHeaders(t *testing.T) {
	const content = "he said \"hi\"\nand left \\o/"
	g := newRESTGateway(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v9/channels/c1/messages" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "rest-token" {
			t.Errorf("authorization = %q", r.Header.Get("Authorization"))
		}
		if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
			t.Errorf("content type = %q", r.Header.Get("Content-Type"))
		}
		if r.Header.Get("User-Agent") == "" || r.Header.Get("X-Discord-Locale") != "en-US" || r.Header.Get("X-Discord-Timezone") != "UTC" {
			t.Errorf("missing client headers: %v", r.Header)
		}
		if r.Header.Get("Referer") != "https://discord.com/channels/@me/c1" {
			t.Errorf("referer = %q", r.Header.Get("Referer"))
		}
		raw, err := base64.StdEncoding.DecodeString(r.Header.Get("X-Super-Properties"))
		if err != nil {
			t.Fatalf("super properties are not base64: %v", err)
		}
		var props map[string]any
		_ = json.Unmarshal(raw, &props)
		if props["client_build_number"] != float64(1234) {
			t.Errorf("client_build_number must be the number 1234, got %#v", props["client_build_number"])
		}

		var body types.MessageSend
		decodeBody(t, r, &body)
		if body.Content != content {
			t.Errorf("content = %q, want %q", body.Content, content)
		}
		if body.Nonce == "" {
			t.Error("nonce missing")
		}
		_, _ = w.Write([]byte(`{"id":"m1","channel_id":"c1","content":"ok","nonce":"123"}`))
	})

	msg, err := g.SendMessage("c1", content)
	if err != nil {
		t.Fatal(err)
	}
	if msg.ID != "m1" {
		t.Errorf("message id = %q", msg.ID)
	}
}

func TestReplyMessage(t *testing.T) {
	g := newRESTGateway(t, func(w http.ResponseWriter, r *http.Request) {
		var body types.MessageSend
		decodeBody(t, r, &body)
		if body.MessageReference == nil || body.MessageReference.MessageID != "m0" {
			t.Errorf("missing message reference: %+v", body.MessageReference)
		}
		if body.AllowedMentions == nil || body.AllowedMentions.RepliedUser {
			t.Errorf("reply should not ping: %+v", body.AllowedMentions)
		}
		_, _ = w.Write([]byte(`{"id":"m1"}`))
	})
	if _, err := g.Reply(&types.Message{MessageData: types.MessageData{ID: "m0", ChannelID: "c1"}}, "hi"); err != nil {
		t.Fatal(err)
	}
}

func TestRateLimitIsRetried(t *testing.T) {
	var calls atomic.Int32
	g := newRESTGateway(t, func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"message":"You are being rate limited.","retry_after":0.05,"global":false}`))
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	if err := g.DeleteMessage("c1", "m1"); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 2 {
		t.Errorf("calls = %d, want 2", calls.Load())
	}
}

func TestRESTErrorIsDecoded(t *testing.T) {
	g := newRESTGateway(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message":"Missing Access","code":50001}`))
	})
	_, err := g.GetChannel("c1")
	var re *RESTError
	if !errors.As(err, &re) || re.StatusCode != 403 || re.Code != 50001 || re.Message != "Missing Access" {
		t.Fatalf("err = %v, want RESTError 403/50001", err)
	}
	if SendMessage(g, "c1", "x") {
		t.Error("deprecated SendMessage reported success on 403")
	}
}

func TestClickButtonPayload(t *testing.T) {
	g := newRESTGateway(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v9/interactions" {
			t.Errorf("path = %s", r.URL.Path)
		}
		var body map[string]any
		decodeBody(t, r, &body)
		data, _ := body["data"].(map[string]any)
		switch {
		case body["type"] != float64(3):
			t.Errorf("type = %v", body["type"])
		case body["application_id"] != "app-1":
			t.Errorf("application_id = %v", body["application_id"])
		case body["guild_id"] != nil:
			t.Errorf("guild_id should be omitted in DMs, got %v", body["guild_id"])
		case body["message_flags"] != float64(64):
			t.Errorf("message_flags = %v", body["message_flags"])
		case len(body["session_id"].(string)) != 32:
			t.Errorf("session_id = %v", body["session_id"])
		case data["custom_id"] != `quote"button` || data["component_type"] != float64(2):
			t.Errorf("data = %v", data)
		}
		w.WriteHeader(http.StatusNoContent)
	})

	msg := &types.Message{MessageData: types.MessageData{
		ID: "m1", ChannelID: "dm1", Flags: 64, ApplicationID: "app-1",
		Author: types.User{ID: "bot-user"},
	}}
	nonce, err := g.ClickButton(msg, `quote"button`)
	if err != nil || nonce == "" {
		t.Fatalf("ClickButton = %q, %v", nonce, err)
	}
	if g.SessionID() != "" {
		t.Error("ClickButton must not change the gateway session")
	}
}

func TestSelectMenuUsesMenuType(t *testing.T) {
	g := newRESTGateway(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		decodeBody(t, r, &body)
		data := body["data"].(map[string]any)
		if data["component_type"] != float64(types.ComponentUserSelect) || len(data["values"].([]any)) != 1 {
			t.Errorf("data = %v", data)
		}
		if body["application_id"] != "bot-user" {
			t.Errorf("application_id should fall back to the author, got %v", body["application_id"])
		}
		w.WriteHeader(http.StatusNoContent)
	})
	msg := &types.Message{MessageData: types.MessageData{
		ID: "m1", ChannelID: "c1", GuildID: "g1", Author: types.User{ID: "bot-user"},
		Components: []types.Component{{Type: types.ComponentActionRow, Components: []types.Component{
			{Type: types.ComponentUserSelect, CustomID: "pick"},
		}}},
	}}
	if _, err := g.SelectMenu(msg, "pick", []string{"42"}); err != nil {
		t.Fatal(err)
	}
}

func TestSendSlashCommandMultipart(t *testing.T) {
	g := newRESTGateway(t, func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatalf("not multipart: %v", err)
		}
		var body map[string]any
		if err := json.Unmarshal([]byte(r.FormValue("payload_json")), &body); err != nil {
			t.Fatal(err)
		}
		data := body["data"].(map[string]any)
		opts := data["options"].([]any)
		if body["type"] != float64(2) || data["name"] != "ping" || data["version"] != "v1" || len(opts) != 1 {
			t.Errorf("unexpected payload: %v", body)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	idx := types.ApplicationCommandIndex{ApplicationCommand: []types.ApplicationCommand{
		{ID: "cmd1", ApplicationID: "app1", Version: "v1", Name: "ping", Type: 1},
	}}
	cmd := idx.Find("ping", "app1")
	if cmd == nil {
		t.Fatal("Find did not return the command")
	}
	opt := types.ApplicationCommandInteractionOption{Type: types.CommandOptionString, Name: "text", Value: "hello"}
	if _, err := g.SendSlashCommand("c1", "g1", cmd, opt); err != nil {
		t.Fatal(err)
	}
}

func TestGetSlashCommandsDecodesFlags(t *testing.T) {
	g := newRESTGateway(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Error("user agent missing")
		}
		_, _ = w.Write([]byte(`{"applications":[{"id":"a","name":"A","flags":8953856},{"id":"b","flags":"12"}],
			"application_commands":[{"id":"1","name":"ping","application_id":"a","version":"9","type":1,
			"options":[{"type":3,"name":"text","description":"d","required":true}]}]}`))
	})
	idx, err := g.GetSlashCommands("g1")
	if err != nil {
		t.Fatal(err)
	}
	if idx.Applications[0].Flags != 8953856 || idx.Applications[1].Flags != 12 {
		t.Errorf("flags = %v / %v", idx.Applications[0].Flags, idx.Applications[1].Flags)
	}
	if cmd := idx.Find("ping", ""); cmd == nil || len(cmd.Options) != 1 {
		t.Errorf("command not decoded: %+v", idx.ApplicationCommand)
	}
}

func TestReactionPathEscaping(t *testing.T) {
	var paths []string
	g := newRESTGateway(t, func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.EscapedPath())
		w.WriteHeader(http.StatusNoContent)
	})
	if err := g.AddReaction("c1", "m1", "👍"); err != nil {
		t.Fatal(err)
	}
	if err := g.AddReaction("c1", "m1", "<a:party:123>"); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"/api/v9/channels/c1/messages/m1/reactions/%F0%9F%91%8D/@me",
		"/api/v9/channels/c1/messages/m1/reactions/party:123/@me",
	}
	for i := range want {
		if i >= len(paths) || paths[i] != want[i] {
			t.Errorf("path %d = %v, want %s", i, paths, want[i])
		}
	}
}

func TestTypingAndHistory(t *testing.T) {
	g := newRESTGateway(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v9/channels/c1/typing":
			w.WriteHeader(http.StatusNoContent)
		case "/api/v9/channels/c1/messages":
			if r.URL.Query().Get("limit") != "100" || r.URL.Query().Get("before") != "m9" {
				t.Errorf("query = %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`[{"id":"m8","content":"a"},{"id":"m7","content":"b"}]`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	})
	if err := g.TriggerTyping("c1"); err != nil {
		t.Fatal(err)
	}
	msgs, err := g.GetMessages("c1", 500, "m9", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 2 || msgs[1].Content != "b" {
		t.Errorf("messages = %+v", msgs)
	}
}

func TestSnowflakeHelpers(t *testing.T) {
	ts, err := SnowflakeTime("175928847299117063")
	if err != nil {
		t.Fatal(err)
	}
	if ts.Year() != 2016 {
		t.Errorf("snowflake time = %v", ts)
	}
	if n1, n2 := GenerateNonce(), GenerateNonce(); n1 == "" || n1 == n2 {
		t.Errorf("nonces should be unique: %s %s", n1, n2)
	}
}

func TestBuildNumberRegex(t *testing.T) {
	html := []byte(`window.GLOBAL_ENV = {"NODE_ENV":"production","BUILD_NUMBER":"681625","PROJECT_ENV":"production"}`)
	m := htmlBuildRegex.FindSubmatch(html)
	if m == nil || string(m[1]) != "681625" {
		t.Fatalf("build number not extracted: %q", m)
	}
}
