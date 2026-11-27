package discord

import (
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fasthttp/websocket"
	"github.com/goccy/go-json"
	"github.com/headd16/goself/types"
)

// Errors returned by the gateway.
var (
	ErrAlreadyOpen  = errors.New("discord: gateway is already open")
	ErrNotOpen      = errors.New("discord: gateway is not open")
	ErrNotConnected = errors.New("discord: gateway is not connected")
	ErrClosed       = errors.New("discord: gateway was closed")
	ErrEmptyToken   = errors.New("discord: token is empty")
)

// Internal reasons for a reconnect.
var (
	errReconnectRequested = errors.New("discord: gateway requested a reconnect")
	errInvalidSession     = errors.New("discord: gateway session invalidated")
	errHeartbeatTimeout   = errors.New("discord: heartbeat was not acknowledged")
)

// CloseError is returned when Discord closes the gateway with a close code
// that does not allow reconnecting, such as 4004 (authentication failed).
type CloseError struct {
	Code   int
	Reason string
	Event  *types.CloseEventCode
}

func (e *CloseError) Error() string {
	if e.Event != nil {
		return fmt.Sprintf("discord: gateway closed with code %d (%s): %s", e.Code, e.Event.Description, e.Event.Explanation)
	}
	return fmt.Sprintf("discord: gateway closed with code %d: %s", e.Code, e.Reason)
}

const (
	helloTimeout      = 30 * time.Second
	readyTimeout      = 60 * time.Second
	writeTimeout      = 10 * time.Second
	maxReconnectDelay = time.Minute
	// Closing with 1000/1001 invalidates the session, so internal reconnects
	// use a 4xxx code to keep it resumable.
	closeCodeReconnect = 4900
)

// invalidSessionDelay is how long to wait before identifying again after an
// invalid session (Discord asks for a random 1-5 seconds). Tests override it.
var invalidSessionDelay = func() time.Duration {
	return time.Second + rand.N(4*time.Second)
}

// Gateway is a connection to the Discord gateway plus the REST helpers of the
// library. Create one with NewGateway (or goself.NewClient), register event
// handlers with AddHandler, then call Open or Connect.
//
// The gateway reconnects and resumes the session automatically until Close
// is called or Discord rejects the session (for example with an invalid token).
type Gateway struct {
	Config   *types.Config
	Selfbot  *Selfbot
	Handlers Handlers
	// State caches guilds, channels, roles, users, members, presences and
	// messages seen on the gateway or fetched over REST. Never nil.
	State *State

	chunkMu      sync.Mutex
	chunkWaiters map[string]chan *types.GuildMembersChunkEventData

	mu        sync.RWMutex
	conn      *wsConn
	sessionID string
	resumeURL string
	seq       int64
	presence  *types.Presence
	running   bool
	closing   chan struct{}
	done      chan struct{}
	err       error

	latency atomic.Int64
}

// wsConn is a single websocket connection. A Gateway goes through many of
// them over its lifetime (one per reconnect).
type wsConn struct {
	ws        *websocket.Conn
	writeMu   sync.Mutex
	done      chan struct{}
	closeOnce sync.Once
	acked     atomic.Bool
	zombie    atomic.Bool
	lastBeat  atomic.Int64
}

func newWSConn(ws *websocket.Conn) *wsConn {
	c := &wsConn{ws: ws, done: make(chan struct{})}
	c.acked.Store(true)
	return c
}

func (c *wsConn) writeJSON(v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	select {
	case <-c.done:
		return ErrNotConnected
	default:
	}
	_ = c.ws.SetWriteDeadline(time.Now().Add(writeTimeout))
	return c.ws.WriteMessage(websocket.TextMessage, b)
}

// close sends a close frame (when code > 0) and closes the socket. Safe to
// call several times and from several goroutines.
func (c *wsConn) close(code int, text string) {
	c.closeOnce.Do(func() {
		close(c.done)
		if code > 0 {
			_ = c.ws.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(code, text), time.Now().Add(time.Second))
		}
		_ = c.ws.Close()
	})
}

type opPayload struct {
	Op int `json:"op"`
	D  any `json:"d"`
}

// NewGateway creates a gateway for the given account token. A nil config
// uses types.DefaultConfig. The config is copied, so later changes to the
// value you passed have no effect; use gateway.Config instead.
func NewGateway(token string, config *types.Config) *Gateway {
	return CreateGateway(&Selfbot{Token: token}, config)
}

// CreateGateway creates a gateway for selfbot. See NewGateway.
func CreateGateway(selfbot *Selfbot, config *types.Config) *Gateway {
	if selfbot == nil {
		selfbot = &Selfbot{}
	}
	selfbot.Token = strings.TrimSpace(selfbot.Token)

	cfg := types.DefaultConfig
	if config != nil {
		cfg = *config
	}
	applyDefaults(&cfg)

	return &Gateway{Config: &cfg, Selfbot: selfbot, State: newState(&cfg)}
}

func applyDefaults(c *types.Config) {
	d := types.DefaultConfig
	if c.Presence == "" {
		c.Presence = d.Presence
	}
	if c.ApiVersion == "" {
		c.ApiVersion = d.ApiVersion
	}
	if c.Browser == "" {
		c.Browser = d.Browser
	}
	if c.BrowserVersion == "" {
		c.BrowserVersion = d.BrowserVersion
	}
	if c.Capabilities == 0 {
		c.Capabilities = d.Capabilities
	}
	if c.Os == "" {
		c.Os = d.Os
	}
	if c.OsVersion == "" {
		c.OsVersion = d.OsVersion
	}
	if c.UserAgent == "" {
		c.UserAgent = d.UserAgent
	}
	if c.MaxRESTRetries == 0 {
		c.MaxRESTRetries = d.MaxRESTRetries
	}
	if c.StateMaxMessages == 0 {
		c.StateMaxMessages = d.StateMaxMessages
	}
	if c.FetchConcurrency <= 0 {
		c.FetchConcurrency = d.FetchConcurrency
	}
}

// Open connects to the gateway and returns once the session is READY. Events
// are then processed in the background until Close is called; use Wait to
// block until the gateway stops.
func (g *Gateway) Open() error {
	g.mu.Lock()
	if g.running {
		g.mu.Unlock()
		return ErrAlreadyOpen
	}
	if g.Selfbot.Token == "" {
		g.mu.Unlock()
		return ErrEmptyToken
	}
	g.running = true
	g.closing = make(chan struct{})
	g.done = make(chan struct{})
	g.err = nil
	g.mu.Unlock()

	g.BuildNumber() // resolve (and cache) before identifying

	c, err := g.connect()
	if err != nil {
		g.stop(err, false)
		return err
	}

	ready := make(chan error, 1)
	go g.run(c, ready)

	timer := time.NewTimer(readyTimeout)
	defer timer.Stop()
	select {
	case err := <-ready:
		return err
	case <-timer.C:
		_ = g.Close()
		return errors.New("discord: timed out waiting for READY")
	}
}

// Connect opens the gateway and blocks until it stops. It returns nil after
// Close, or the error that stopped the gateway.
func (g *Gateway) Connect() error {
	if err := g.Open(); err != nil {
		return err
	}
	return g.Wait()
}

// Wait blocks until the gateway stops and returns the reason (nil after Close).
func (g *Gateway) Wait() error {
	g.mu.RLock()
	done := g.done
	g.mu.RUnlock()
	if done == nil {
		return ErrNotOpen
	}
	<-done
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.err
}

// Close disconnects from the gateway and stops reconnecting. The session is
// terminated, so the next Open starts a new one.
func (g *Gateway) Close() error {
	g.mu.Lock()
	if !g.running {
		g.mu.Unlock()
		return ErrNotOpen
	}
	select {
	case <-g.closing:
	default:
		close(g.closing)
	}
	c, done := g.conn, g.done
	g.sessionID, g.resumeURL, g.seq = "", "", 0
	g.mu.Unlock()

	if c != nil {
		c.close(websocket.CloseNormalClosure, "")
	}

	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	select {
	case <-done:
	case <-timer.C:
	}
	return nil
}

// stop marks the gateway as stopped. notify fires the DISCONNECT handlers.
func (g *Gateway) stop(err error, notify bool) {
	g.mu.Lock()
	g.running = false
	g.conn = nil
	g.err = err
	if err != nil {
		g.sessionID, g.resumeURL, g.seq = "", "", 0
	}
	close(g.done)
	g.mu.Unlock()

	if err != nil {
		g.log(slog.LevelError, "gateway stopped", "error", err)
	}
	if notify {
		g.Handlers.dispatchInternal(g, types.GatewayEventDisconnect, err)
	}
}

// run owns the connection: it reads events and reconnects until the gateway
// is closed or hits a fatal error. ready receives the outcome of the first
// READY.
func (g *Gateway) run(c *wsConn, ready chan<- error) {
	opened := false
	var fatal error
	defer func() { g.stop(fatal, opened) }()

	onReady := func() {
		if !opened {
			opened = true
			ready <- nil
		}
	}

	for {
		err := g.readLoop(c, onReady)
		c.close(closeCodeReconnect, "reconnecting")
		g.clearConn(c)

		if g.isClosing() {
			if !opened {
				ready <- ErrClosed
			}
			return
		}

		delay, fatalErr := g.classify(err)
		if fatalErr != nil || !opened {
			if fatalErr == nil {
				fatalErr = err
			}
			fatal = fatalErr
			if !opened {
				ready <- fatal
			}
			return
		}

		g.log(slog.LevelInfo, "gateway disconnected, reconnecting", "reason", err, "delay", delay)
		g.Handlers.dispatchInternal(g, types.GatewayEventReconnect, nil)

		if c = g.reconnect(delay); c == nil {
			return // closed while reconnecting
		}
	}
}

// reconnect dials until it succeeds or the gateway is closed (nil result).
func (g *Gateway) reconnect(delay time.Duration) *wsConn {
	for attempt := 0; ; attempt++ {
		if delay > 0 {
			timer := time.NewTimer(delay)
			select {
			case <-g.closingChan():
				timer.Stop()
				return nil
			case <-timer.C:
			}
		}
		c, err := g.connect()
		if err == nil {
			return c
		}
		if g.isClosing() {
			return nil
		}
		g.log(slog.LevelWarn, "gateway reconnect failed", "attempt", attempt+1, "error", err)
		delay = backoff(attempt)
	}
}

// classify decides how to react to a lost connection: how long to wait before
// reconnecting, or a fatal error when reconnecting is pointless.
func (g *Gateway) classify(err error) (time.Duration, error) {
	switch {
	case errors.Is(err, errReconnectRequested), errors.Is(err, errHeartbeatTimeout):
		return 0, nil
	case errors.Is(err, errInvalidSession):
		return invalidSessionDelay(), nil
	}

	var ce *websocket.CloseError
	if errors.As(err, &ce) {
		if ev, ok := types.CloseEventCodes[ce.Code]; ok {
			if !ev.Reconnect {
				return 0, &CloseError{Code: ce.Code, Reason: ce.Text, Event: ev}
			}
			switch ce.Code {
			case types.CloseEventCodeNotAuthenticated.Code,
				types.CloseEventCodeAlreadyAuthenticated.Code,
				types.CloseEventCodeInvalidSeq.Code,
				types.CloseEventCodeSessionTimed.Code:
				g.resetSession()
			case types.CloseEventCodeRateLimited.Code:
				return 5 * time.Second, nil
			}
		}
	}
	return time.Second, nil
}

func backoff(attempt int) time.Duration {
	d := min(time.Second<<min(attempt, 6), maxReconnectDelay)
	return d/2 + rand.N(d/2+1)
}

// connect dials the gateway, waits for HELLO, starts heartbeating and sends
// IDENTIFY (or RESUME when a session exists).
func (g *Gateway) connect() (*wsConn, error) {
	g.mu.RLock()
	sessionID, resumeURL, seq := g.sessionID, g.resumeURL, g.seq
	g.mu.RUnlock()

	resume := sessionID != ""
	target := g.gatewayURL("")
	if resume && resumeURL != "" {
		target = g.gatewayURL(resumeURL)
	}

	header := http.Header{}
	header.Set("User-Agent", g.Config.UserAgent)
	header.Set("Origin", discordOrigin)
	dialer := websocket.Dialer{Proxy: http.ProxyFromEnvironment, HandshakeTimeout: helloTimeout}

	ws, resp, err := dialer.Dial(target, header)
	if err != nil {
		if resp != nil {
			return nil, fmt.Errorf("discord: dial gateway: %w (HTTP %d)", err, resp.StatusCode)
		}
		return nil, fmt.Errorf("discord: dial gateway: %w", err)
	}
	c := newWSConn(ws)

	_ = ws.SetReadDeadline(time.Now().Add(helloTimeout))
	_, msg, err := ws.ReadMessage()
	if err != nil {
		c.close(0, "")
		return nil, fmt.Errorf("discord: waiting for HELLO: %w", err)
	}
	_ = ws.SetReadDeadline(time.Time{})

	var hello types.HelloEvent
	if err := json.Unmarshal(msg, &hello); err != nil || hello.Op != types.OpcodeHello {
		c.close(0, "")
		return nil, fmt.Errorf("discord: expected HELLO (op %d), got %s", types.OpcodeHello, truncate(msg, 120))
	}
	interval := time.Duration(hello.D.HeartbeatInterval) * time.Millisecond
	if interval <= 0 {
		interval = 41250 * time.Millisecond
	}

	g.mu.Lock()
	select {
	case <-g.closing:
		g.mu.Unlock()
		c.close(websocket.CloseNormalClosure, "")
		return nil, ErrClosed
	default:
	}
	g.conn = c
	g.mu.Unlock()

	go g.heartbeat(c, interval)

	if resume {
		g.log(slog.LevelDebug, "resuming gateway session", "session", sessionID, "seq", seq)
		err = c.writeJSON(types.ResumePayload{
			Op: types.OpcodeResume,
			D:  types.ResumePayloadData{Token: g.token(), SessionID: sessionID, Seq: seq},
		})
	} else {
		g.log(slog.LevelDebug, "identifying")
		err = c.writeJSON(g.identifyPayload())
	}
	if err != nil {
		c.close(0, "")
		g.clearConn(c)
		return nil, fmt.Errorf("discord: send identify: %w", err)
	}
	return c, nil
}

func (g *Gateway) identifyPayload() types.IdentifyPayload {
	g.mu.RLock()
	presence := types.Presence{Status: g.Config.Presence, Activities: []types.Activity{}}
	if g.presence != nil {
		presence = *g.presence
	}
	g.mu.RUnlock()

	return types.IdentifyPayload{
		Op: types.OpcodeIdentify,
		D: types.IdentifyPayloadData{
			Token:        g.token(),
			Capabilities: g.Config.Capabilities,
			Properties:   g.SuperProperties(),
			Presence:     presence,
			Compress:     false,
			ClientState: types.ClientState{
				GuildVersions:            types.GuildVersions{},
				HighestLastMessageID:     "0",
				ReadStateVersion:         0,
				UserGuildSettingsVersion: -1,
				UserSettingsVersion:      -1,
				PrivateChannelsVersion:   "0",
				APICodeVersion:           0,
			},
		},
	}
}

func (g *Gateway) gatewayURL(base string) string {
	if base == "" {
		base = g.Config.GatewayURL
	}
	if base == "" {
		base = defaultGatewayURL
	}
	u, err := url.Parse(base)
	if err != nil {
		return base
	}
	q := u.Query()
	if q.Get("encoding") == "" {
		q.Set("encoding", "json")
	}
	if q.Get("v") == "" {
		q.Set("v", g.Config.ApiVersion)
	}
	u.RawQuery = q.Encode()
	if u.Path == "" {
		u.Path = "/"
	}
	return u.String()
}

func (g *Gateway) heartbeat(c *wsConn, interval time.Duration) {
	// The first heartbeat is jittered, as required by Discord.
	timer := time.NewTimer(rand.N(interval) + time.Millisecond)
	defer timer.Stop()

	for {
		select {
		case <-c.done:
			return
		case <-timer.C:
		}

		if !c.acked.Load() {
			g.log(slog.LevelWarn, "heartbeat not acknowledged, reconnecting")
			c.zombie.Store(true)
			c.close(closeCodeReconnect, "heartbeat timeout")
			return
		}
		c.acked.Store(false)
		if err := g.sendHeartbeat(c); err != nil {
			g.log(slog.LevelWarn, "failed to send heartbeat", "error", err)
			c.close(0, "")
			return
		}
		timer.Reset(interval)
	}
}

func (g *Gateway) sendHeartbeat(c *wsConn) error {
	g.mu.RLock()
	seq := g.seq
	g.mu.RUnlock()

	var d any
	if seq > 0 {
		d = seq
	}
	c.lastBeat.Store(monotonicNow())
	return c.writeJSON(opPayload{Op: types.OpcodeHeartbeat, D: d})
}

func (g *Gateway) readLoop(c *wsConn, onReady func()) error {
	for {
		_, msg, err := c.ws.ReadMessage()
		if err != nil {
			if c.zombie.Load() {
				return errHeartbeatTimeout
			}
			return err
		}
		if err := g.handlePayload(c, msg, onReady); err != nil {
			return err
		}
	}
}

func (g *Gateway) handlePayload(c *wsConn, msg []byte, onReady func()) error {
	var ev types.RawEvent
	if err := json.Unmarshal(msg, &ev); err != nil {
		g.log(slog.LevelError, "invalid gateway payload", "error", err, "payload", truncate(msg, 200))
		return nil
	}

	switch ev.Op {
	case types.OpcodeDispatch:
		if ev.Seq > 0 {
			g.mu.Lock()
			g.seq = ev.Seq
			g.mu.Unlock()
		}
		g.handleDispatch(&ev, onReady)
	case types.OpcodeHeartbeat:
		return g.sendHeartbeat(c)
	case types.OpcodeHeartbeatACK:
		c.acked.Store(true)
		if sent := c.lastBeat.Load(); sent > 0 {
			g.latency.Store(monotonicNow() - sent)
		}
	case types.OpcodeReconnect:
		return errReconnectRequested
	case types.OpcodeInvalidSession:
		var resumable bool
		_ = json.Unmarshal(ev.Data, &resumable)
		if !resumable {
			g.resetSession()
		}
		g.Handlers.dispatchInternal(g, types.GatewayEventInvalidated, nil)
		return errInvalidSession
	}
	return nil
}

func (g *Gateway) handleDispatch(ev *types.RawEvent, onReady func()) {
	switch ev.Type {
	case types.EventNameReady:
		var state struct {
			SessionID        string     `json:"session_id"`
			ResumeGatewayURL string     `json:"resume_gateway_url"`
			User             types.User `json:"user"`
			AuthToken        string     `json:"auth_token"`
		}
		if err := json.Unmarshal(ev.Data, &state); err != nil {
			g.log(slog.LevelError, "failed to decode READY", "error", err)
		}
		g.mu.Lock()
		g.sessionID = state.SessionID
		g.resumeURL = state.ResumeGatewayURL
		g.Selfbot.User = state.User
		if state.AuthToken != "" {
			g.Selfbot.Token = state.AuthToken
		}
		g.mu.Unlock()
		g.log(slog.LevelInfo, "gateway ready", "user", state.User.Username, "session", state.SessionID)
	case types.EventNameResumed:
		g.log(slog.LevelInfo, "gateway session resumed")
	case types.EventNameUserUpdate:
		var u types.User
		if json.Unmarshal(ev.Data, &u) == nil {
			g.mu.Lock()
			if u.ID == g.Selfbot.User.ID {
				g.Selfbot.User = u
			}
			g.mu.Unlock()
		}
	}

	decoded := g.State.onEvent(g, ev)
	if ev.Type == types.EventNameGuildMembersChunk {
		g.deliverChunk(ev, decoded)
	}

	g.Handlers.dispatch(g, ev, decoded)

	if ev.Type == types.EventNameReady || ev.Type == types.EventNameResumed {
		onReady()
	}
}

func (g *Gateway) clearConn(c *wsConn) {
	g.mu.Lock()
	if g.conn == c {
		g.conn = nil
	}
	g.mu.Unlock()
}

func (g *Gateway) resetSession() {
	g.mu.Lock()
	g.sessionID, g.resumeURL, g.seq = "", "", 0
	g.mu.Unlock()
}

func (g *Gateway) closingChan() <-chan struct{} {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.closing
}

func (g *Gateway) isClosing() bool {
	select {
	case <-g.closingChan():
		return true
	default:
		return false
	}
}

// SendOp sends a raw gateway payload {"op": op, "d": data}. Use it for
// opcodes that have no dedicated helper.
func (g *Gateway) SendOp(op int, data any) error {
	g.mu.RLock()
	c := g.conn
	g.mu.RUnlock()
	if c == nil {
		return ErrNotConnected
	}
	return c.writeJSON(opPayload{Op: op, D: data})
}

// UpdatePresence changes the status ("online", "idle", "dnd", "invisible")
// and activities of the session. It is re-sent automatically after a reconnect.
func (g *Gateway) UpdatePresence(status string, activities ...types.Activity) error {
	if activities == nil {
		activities = []types.Activity{}
	}
	p := types.Presence{Status: status, Since: 0, Activities: activities, Afk: false}
	g.mu.Lock()
	g.presence = &p
	g.mu.Unlock()
	return g.SendOp(types.OpcodePresenceUpdate, p)
}

// SetCustomStatus shows a custom status (with an optional unicode emoji) for
// this session, keeping the current status.
func (g *Gateway) SetCustomStatus(text, emoji string) error {
	g.mu.RLock()
	status := g.Config.Presence
	if g.presence != nil {
		status = g.presence.Status
	}
	g.mu.RUnlock()

	activity := types.Activity{Name: "Custom Status", Type: types.ActivityCustom, State: text}
	if emoji != "" {
		activity.Emoji = &types.ActivityEmoji{Name: emoji}
	}
	return g.UpdatePresence(status, activity)
}

// RequestGuildMembers asks the gateway for the given members of a guild.
// The answer arrives as GUILD_MEMBERS_CHUNK events.
func (g *Gateway) RequestGuildMembers(guildID string, userIDs []string, presences bool) error {
	return g.SendOp(types.OpcodeRequestGuildMembers, types.RequestGuildMembersPayload{
		GuildID:   guildID,
		UserIDs:   userIDs,
		Presences: presences,
	})
}

// GetMembers is RequestGuildMembers with presences enabled.
func (g *Gateway) GetMembers(guildID string, memberIDs []string) error {
	return g.RequestGuildMembers(guildID, memberIDs, true)
}

// UpdateVoiceState joins (or moves to) a voice channel. An empty channelID
// leaves voice in that guild. guildID is empty for DM calls.
func (g *Gateway) UpdateVoiceState(guildID, channelID string, selfMute, selfDeaf bool) error {
	p := types.VoiceStateUpdatePayload{SelfMute: selfMute, SelfDeaf: selfDeaf}
	if guildID != "" {
		p.GuildID = &guildID
	}
	if channelID != "" {
		p.ChannelID = &channelID
	}
	return g.SendOp(types.OpcodeVoiceStateUpdate, p)
}

// AddHandler registers an event handler. See types.GatewayEvent* for the
// event names and the handler signature each one expects.
func (g *Gateway) AddHandler(event string, handler any) error {
	return g.Handlers.Add(event, handler)
}

// SessionID returns the current gateway session ID (empty when not connected).
func (g *Gateway) SessionID() string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.sessionID
}

// Sequence returns the last sequence number received from the gateway.
func (g *Gateway) Sequence() int64 {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.seq
}

// User returns the logged-in user (filled after READY).
func (g *Gateway) User() types.User {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.Selfbot.User
}

// Latency returns the round trip time of the last heartbeat.
func (g *Gateway) Latency() time.Duration {
	return time.Duration(g.latency.Load())
}

// IsOpen reports whether Open succeeded and Close has not been called yet.
// The connection itself may be temporarily down while reconnecting.
func (g *Gateway) IsOpen() bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.running
}

// IsConnected reports whether a websocket connection is currently up.
func (g *Gateway) IsConnected() bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.running && g.conn != nil
}

// processStart anchors monotonicNow; time.Since uses the monotonic clock,
// which (unlike wall-clock UnixNano) is precise enough to time heartbeats.
var processStart = time.Now()

func monotonicNow() int64 { return int64(time.Since(processStart)) + 1 }
