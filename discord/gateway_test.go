package discord

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fasthttp/websocket"
	"github.com/goccy/go-json"
	"github.com/headd16/goself/types"
)

// mockGateway is a tiny fake Discord gateway. Every websocket connection gets
// a HELLO and is handed to the test through conns.
type mockGateway struct {
	t        *testing.T
	srv      *httptest.Server
	conns    chan *websocket.Conn
	interval int // heartbeat interval in ms
}

func newMockGateway(t *testing.T, interval int) *mockGateway {
	t.Helper()
	m := &mockGateway{t: t, conns: make(chan *websocket.Conn, 4), interval: interval}
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	m.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("encoding") != "json" || r.URL.Query().Get("v") != "9" {
			t.Errorf("gateway URL is missing encoding/version: %s", r.URL)
		}
		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade: %v", err)
			return
		}
		writeJSON(t, ws, map[string]any{"op": 10, "d": map[string]any{"heartbeat_interval": m.interval}})
		m.conns <- ws
	}))
	t.Cleanup(m.srv.Close)
	return m
}

func (m *mockGateway) url() string { return "ws" + strings.TrimPrefix(m.srv.URL, "http") }

func (m *mockGateway) next() *websocket.Conn {
	m.t.Helper()
	select {
	case ws := <-m.conns:
		m.t.Cleanup(func() { ws.Close() })
		return ws
	case <-time.After(5 * time.Second):
		m.t.Fatal("client did not connect")
		return nil
	}
}

type payload struct {
	Op int             `json:"op"`
	T  string          `json:"t"`
	S  int64           `json:"s"`
	D  json.RawMessage `json:"d"`
}

func writeJSON(t *testing.T, ws *websocket.Conn, v any) {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if err := ws.WriteMessage(websocket.TextMessage, b); err != nil {
		t.Errorf("server write: %v", err)
	}
}

// expectOp reads payloads until one with the wanted opcode arrives,
// acknowledging heartbeats on the way.
func expectOp(t *testing.T, ws *websocket.Conn, want int) payload {
	t.Helper()
	_ = ws.SetReadDeadline(time.Now().Add(5 * time.Second))
	for {
		_, msg, err := ws.ReadMessage()
		if err != nil {
			t.Fatalf("waiting for op %d: %v", want, err)
		}
		var p payload
		if err := json.Unmarshal(msg, &p); err != nil {
			t.Fatalf("invalid client payload %s: %v", msg, err)
		}
		if p.Op == want {
			return p
		}
		if p.Op == types.OpcodeHeartbeat {
			writeJSON(t, ws, map[string]any{"op": 11})
			continue
		}
		t.Fatalf("expected op %d, got %s", want, msg)
	}
}

// expectClose waits for the client to close the connection and returns the code.
func expectClose(t *testing.T, ws *websocket.Conn) int {
	t.Helper()
	_ = ws.SetReadDeadline(time.Now().Add(5 * time.Second))
	for {
		_, _, err := ws.ReadMessage()
		if err == nil {
			continue
		}
		var ce *websocket.CloseError
		if errors.As(err, &ce) {
			return ce.Code
		}
		t.Fatalf("expected a close frame, got %v", err)
	}
}

func dispatch(t *testing.T, ws *websocket.Conn, name string, seq int, data any) {
	t.Helper()
	writeJSON(t, ws, map[string]any{"op": 0, "t": name, "s": seq, "d": data})
}

func readyData(resumeURL string) map[string]any {
	return map[string]any{
		"v":                  9,
		"session_id":         "session-1",
		"resume_gateway_url": resumeURL,
		"user":               map[string]any{"id": "42", "username": "me", "flags": 64, "public_flags": 64},
		"guilds": []any{
			map[string]any{"id": "g1", "properties": map[string]any{"name": "Guild One", "owner_id": "1"}, "roles": []any{}},
		},
	}
}

func testConfig(gatewayURL, apiURL string) *types.Config {
	cfg := types.NewConfig()
	cfg.GatewayURL = gatewayURL
	cfg.APIBaseURL = apiURL
	cfg.BuildNumber = 1234
	cfg.Locale = "en-US"
	cfg.Timezone = "UTC"
	return cfg
}

func waitFor[T any](t *testing.T, ch <-chan T, what string) T {
	t.Helper()
	select {
	case v := <-ch:
		return v
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for %s", what)
		var zero T
		return zero
	}
}

func TestGatewayLifecycle(t *testing.T) {
	mock := newMockGateway(t, 45000)
	g := NewGateway("  token-123\n", testConfig(mock.url(), ""))

	readyCh := make(chan *types.ReadyEventData, 1)
	msgCh := make(chan *types.MessageEventData, 1)
	rawCh := make(chan string, 10)
	resumedCh := make(chan struct{}, 1)
	reconnectCh := make(chan struct{}, 1)
	disconnectCh := make(chan error, 1)

	mustAdd(t, g, types.GatewayEventReady, func(e *types.ReadyEventData) { readyCh <- e })
	mustAdd(t, g, types.GatewayEventMessageCreate, func(e *types.MessageEventData) { msgCh <- e })
	mustAdd(t, g, types.GatewayEventAll, func(e *types.RawEvent) { rawCh <- e.Type })
	mustAdd(t, g, types.GatewayEventResumed, func() { resumedCh <- struct{}{} })
	mustAdd(t, g, types.GatewayEventReconnect, func() { reconnectCh <- struct{}{} })
	mustAdd(t, g, types.GatewayEventDisconnect, func(err error) { disconnectCh <- err })

	openErr := make(chan error, 1)
	go func() { openErr <- g.Open() }()

	// IDENTIFY
	ws := mock.next()
	identify := expectOp(t, ws, types.OpcodeIdentify)
	var id types.IdentifyPayloadData
	if err := json.Unmarshal(identify.D, &id); err != nil {
		t.Fatal(err)
	}
	if id.Token != "token-123" {
		t.Errorf("token not trimmed: %q", id.Token)
	}
	if id.Properties.ClientBuildNumber != 1234 || id.Properties.OS != "Windows" {
		t.Errorf("unexpected properties: %+v", id.Properties)
	}
	if id.Presence.Activities == nil {
		t.Error("presence activities must be an array, not null")
	}

	// READY
	dispatch(t, ws, "READY", 1, readyData(mock.url()))
	if err := waitFor(t, openErr, "Open"); err != nil {
		t.Fatalf("Open: %v", err)
	}
	ready := waitFor(t, readyCh, "READY handler")
	if ready.User.ID != "42" || ready.User.Flags != 64 || ready.User.PublicFlags != 64 {
		t.Errorf("unexpected user: %+v", ready.User)
	}
	if len(ready.Guilds) != 1 || ready.Guilds[0].Name != "Guild One" {
		t.Errorf("guild properties not merged: %+v", ready.Guilds)
	}
	if g.User().ID != "42" || g.SessionID() != "session-1" {
		t.Errorf("state not stored: user=%q session=%q", g.User().ID, g.SessionID())
	}

	// MESSAGE_CREATE with an integer nonce, channel mentions and nested buttons
	// (all of which used to break decoding).
	dispatch(t, ws, "MESSAGE_CREATE", 2, map[string]any{
		"id": "m1", "channel_id": "c1", "content": `he said "hi"`,
		"author":            map[string]any{"id": "7", "username": "bot", "bot": true},
		"nonce":             1234567890123,
		"mention_channels":  []any{map[string]any{"id": "c2", "guild_id": "g1", "type": 0, "name": "general"}},
		"message_reference": map[string]any{"message_id": "m0", "channel_id": "c1"},
		"referenced_message": map[string]any{
			"id": "m0", "content": "original", "author": map[string]any{"id": "42", "username": "me"},
		},
		"components": []any{map[string]any{
			"type": 17, "components": []any{map[string]any{
				"type": 1, "components": []any{map[string]any{"type": 2, "style": 1, "label": "Click me", "custom_id": "btn"}},
			}},
		}},
	})
	msg := waitFor(t, msgCh, "MESSAGE_CREATE handler")
	if msg.Nonce != "1234567890123" || len(msg.MentionChannels) != 1 {
		t.Errorf("message decoded incorrectly: nonce=%q mentions=%v", msg.Nonce, msg.MentionChannels)
	}
	if msg.MessageReference.MessageID != "m0" || msg.ReferencedMessage.Content != "original" {
		t.Errorf("reply info lost: ref=%+v referenced=%q", msg.MessageReference, msg.ReferencedMessage.Content)
	}
	if b := msg.FindButton("click me"); b == nil || b.CustomID != "btn" {
		t.Errorf("nested button not found: %+v", msg.Buttons())
	}

	// Heartbeat requested by the server must carry the last sequence number.
	writeJSON(t, ws, map[string]any{"op": 1})
	hb := expectOp(t, ws, types.OpcodeHeartbeat)
	if string(hb.D) != "2" {
		t.Errorf("heartbeat should carry seq 2, got %s", hb.D)
	}
	writeJSON(t, ws, map[string]any{"op": 11})

	// RECONNECT: the client must drop this connection without invalidating
	// the session, then RESUME on a new one.
	writeJSON(t, ws, map[string]any{"op": 7, "d": nil})
	if code := expectClose(t, ws); code == websocket.CloseNormalClosure || code == websocket.CloseGoingAway {
		t.Errorf("reconnect closed with %d, which invalidates the session", code)
	}
	waitFor(t, reconnectCh, "RECONNECT handler")

	ws2 := mock.next()
	resume := expectOp(t, ws2, types.OpcodeResume)
	var rd types.ResumePayloadData
	if err := json.Unmarshal(resume.D, &rd); err != nil {
		t.Fatal(err)
	}
	if rd.SessionID != "session-1" || rd.Seq != 2 || rd.Token != "token-123" {
		t.Errorf("unexpected resume payload: %+v", rd)
	}
	dispatch(t, ws2, "RESUMED", 3, map[string]any{})
	waitFor(t, resumedCh, "RESUMED handler")
	if g.Sequence() != 3 {
		t.Errorf("sequence = %d, want 3", g.Sequence())
	}

	// Close terminates the session with a normal closure.
	closed := make(chan int, 1)
	go func() { closed <- expectClose(t, ws2) }()
	if err := g.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if code := waitFor(t, closed, "close frame"); code != websocket.CloseNormalClosure {
		t.Errorf("Close sent code %d, want 1000", code)
	}
	if err := g.Wait(); err != nil {
		t.Errorf("Wait after Close = %v, want nil", err)
	}
	if err := waitFor(t, disconnectCh, "DISCONNECT handler"); err != nil {
		t.Errorf("DISCONNECT error = %v, want nil", err)
	}
	if g.IsOpen() {
		t.Error("gateway still open after Close")
	}
	if err := g.Close(); !errors.Is(err, ErrNotOpen) {
		t.Errorf("second Close = %v, want ErrNotOpen", err)
	}

	seen := map[string]bool{}
	for len(rawCh) > 0 {
		seen[<-rawCh] = true
	}
	for _, name := range []string{"READY", "MESSAGE_CREATE", "RESUMED"} {
		if !seen[name] {
			t.Errorf("raw handler did not see %s", name)
		}
	}
}

func TestGatewayAuthenticationFailed(t *testing.T) {
	mock := newMockGateway(t, 45000)
	g := NewGateway("bad-token", testConfig(mock.url(), ""))

	openErr := make(chan error, 1)
	go func() { openErr <- g.Open() }()

	ws := mock.next()
	expectOp(t, ws, types.OpcodeIdentify)
	_ = ws.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(4004, "Authentication failed."))

	err := waitFor(t, openErr, "Open")
	var ce *CloseError
	if !errors.As(err, &ce) || ce.Code != 4004 {
		t.Fatalf("Open = %v, want CloseError 4004", err)
	}
	if g.IsOpen() {
		t.Error("gateway should not be open")
	}
	if err := g.Wait(); err == nil {
		t.Error("Wait should return the fatal error")
	}
}

func TestGatewayInvalidSessionReidentifies(t *testing.T) {
	old := invalidSessionDelay
	invalidSessionDelay = func() time.Duration { return 10 * time.Millisecond }
	t.Cleanup(func() { invalidSessionDelay = old })

	mock := newMockGateway(t, 45000)
	g := NewGateway("token", testConfig(mock.url(), ""))
	invalidated := make(chan struct{}, 1)
	mustAdd(t, g, types.GatewayEventInvalidated, func() { invalidated <- struct{}{} })

	openErr := make(chan error, 1)
	go func() { openErr <- g.Open() }()
	ws := mock.next()
	expectOp(t, ws, types.OpcodeIdentify)
	dispatch(t, ws, "READY", 1, readyData(mock.url()))
	if err := waitFor(t, openErr, "Open"); err != nil {
		t.Fatal(err)
	}

	writeJSON(t, ws, map[string]any{"op": 9, "d": false})
	waitFor(t, invalidated, "INVALIDATED handler")

	// A non-resumable invalid session must lead to a fresh IDENTIFY.
	ws2 := mock.next()
	expectOp(t, ws2, types.OpcodeIdentify)
	dispatch(t, ws2, "READY", 1, readyData(mock.url()))

	_ = g.Close()
}

func TestGatewayHeartbeatAndZombieDetection(t *testing.T) {
	mock := newMockGateway(t, 50)
	g := NewGateway("token", testConfig(mock.url(), ""))

	openErr := make(chan error, 1)
	go func() { openErr <- g.Open() }()
	ws := mock.next()
	expectOp(t, ws, types.OpcodeIdentify)
	dispatch(t, ws, "READY", 1, readyData(mock.url()))
	if err := waitFor(t, openErr, "Open"); err != nil {
		t.Fatal(err)
	}

	// Heartbeats must arrive on their own and carry the sequence number. Three
	// heartbeats on the same connection also prove ACKs are processed
	// (otherwise the client would reconnect after the first one).
	for i := range 3 {
		hb := expectOp(t, ws, types.OpcodeHeartbeat)
		if string(hb.D) != "1" {
			t.Fatalf("heartbeat %d carries %s, want 1", i, hb.D)
		}
		if i == 2 {
			// Delay the last ACK: the Windows monotonic clock only ticks every
			// ~1-16ms, so an instant localhost round trip would measure as 0.
			time.Sleep(30 * time.Millisecond)
		}
		writeJSON(t, ws, map[string]any{"op": 11})
	}
	time.Sleep(10 * time.Millisecond)
	if g.Latency() <= 0 {
		t.Error("latency was not measured")
	}

	// Stop acknowledging: the client must treat the connection as dead and resume.
	var reconnected atomic.Bool
	mustAdd(t, g, types.GatewayEventReconnect, func() { reconnected.Store(true) })
	go func() {
		_ = ws.SetReadDeadline(time.Now().Add(5 * time.Second))
		for {
			if _, _, err := ws.ReadMessage(); err != nil {
				return
			}
		}
	}()

	ws2 := mock.next()
	expectOp(t, ws2, types.OpcodeResume)
	if !reconnected.Load() {
		time.Sleep(50 * time.Millisecond)
		if !reconnected.Load() {
			t.Error("RECONNECT handler not called after heartbeat timeout")
		}
	}
	_ = g.Close()
}

func TestHandlersValidation(t *testing.T) {
	var h Handlers
	if err := h.Add(types.GatewayEventMessageCreate, func(*types.ReadyEventData) {}); err == nil {
		t.Error("wrong signature accepted")
	}
	if err := h.Add("CALL_CREATE", func(*types.Channel) {}); err == nil {
		t.Error("typed handler accepted for an event without typed payload")
	}
	if err := h.Add("CALL_CREATE", func(*types.RawEvent) {}); err != nil {
		t.Errorf("raw handler rejected: %v", err)
	}
	if err := h.Add(types.GatewayEventReconnect, func(*types.RawEvent) {}); err == nil {
		t.Error("raw handler accepted for an internal event")
	}
	if err := h.Add(types.GatewayEventMessageCreate, nil); err == nil {
		t.Error("nil handler accepted")
	}
	if err := h.Add(types.GatewayGuildMembersChunk, func(*types.GuildMembersChunkEventData) {}); err != nil {
		t.Errorf("members chunk handler rejected: %v", err)
	}
}

func TestHandlerPanicDoesNotKillGateway(t *testing.T) {
	g := NewGateway("token", testConfig("", ""))
	g.Config.SyncEvents = true
	called := false
	mustAdd(t, g, types.GatewayEventMessageCreate, func(*types.MessageEventData) { panic("boom") })
	mustAdd(t, g, types.GatewayEventMessageCreate, func(*types.MessageEventData) { called = true })
	g.Handlers.dispatch(g, &types.RawEvent{Type: "MESSAGE_CREATE", Data: []byte(`{"id":"1"}`)}, nil)
	if !called {
		t.Error("second handler not called after the first panicked")
	}
}

func TestOpenWithoutToken(t *testing.T) {
	if err := NewGateway("   ", nil).Open(); !errors.Is(err, ErrEmptyToken) {
		t.Fatalf("Open = %v, want ErrEmptyToken", err)
	}
}

func mustAdd(t *testing.T, g *Gateway, event string, fn any) {
	t.Helper()
	if err := g.AddHandler(event, fn); err != nil {
		t.Fatal(err)
	}
}
