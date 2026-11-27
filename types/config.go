package types

import "log/slog"

// Config controls how the client identifies itself to Discord and how the
// gateway behaves. Start from DefaultConfig (or NewConfig) and override the
// fields you need; zero values fall back to sensible defaults.
type Config struct {
	// Presence is the status sent when identifying: "online", "idle", "dnd",
	// "invisible" or "offline".
	Presence string
	// ApiVersion is the Discord API / gateway version (default "9").
	ApiVersion string

	// Browser fingerprint used for the super properties and HTTP headers.
	Browser        string
	BrowserVersion string
	Capabilities   int64
	Device         string
	Os             string
	OsVersion      string
	UserAgent      string

	// BuildNumber is the Discord web client build number. When 0 it is
	// scraped from discord.com once per process (with a built-in fallback).
	BuildNumber int
	// Locale is sent in X-Discord-Locale and the super properties, for
	// example "en-US". Empty means "detect from the operating system".
	Locale string
	// Timezone is sent in X-Discord-Timezone, for example "Europe/Berlin".
	// Empty means "detect from the TZ environment variable / system", or "UTC".
	Timezone string

	// GatewayURL overrides the gateway endpoint (default wss://gateway.discord.gg).
	GatewayURL string
	// APIBaseURL overrides the REST endpoint (default https://discord.com/api/v{ApiVersion}).
	APIBaseURL string

	// SyncEvents makes the gateway call event handlers one after another on
	// the reading goroutine. By default every event is handled in its own
	// goroutine so slow handlers never block the connection.
	SyncEvents bool
	// MaxRESTRetries is how many times a request is retried after a 429 rate
	// limit (or a 5xx error on GET requests). Default 3.
	MaxRESTRetries int

	// DisableState turns off the in-memory cache of guilds, channels, roles,
	// users, members, presences and messages. When disabled, Fetch* helpers
	// always call the API.
	DisableState bool
	// StateMaxMessages is how many recent messages are cached per channel.
	// Default 50; a negative value disables the message cache.
	StateMaxMessages int
	// FetchConcurrency is how many requests the bulk Fetch* helpers
	// (FetchUsers, FetchChannels, ...) run in parallel. Default 5.
	FetchConcurrency int
	// Logger receives debug/info/error logs. Nil disables logging.
	Logger *slog.Logger
}

// DefaultConfig mimics the Discord web client running on Chrome / Windows.
//
// Pass a copy (see NewConfig) if you plan to modify it, so the shared default
// stays untouched.
var DefaultConfig = Config{
	Presence:         "offline",
	ApiVersion:       "9",
	Browser:          "Chrome",
	BrowserVersion:   "131.0.0.0",
	Capabilities:     4093,
	Device:           "",
	Os:               "Windows",
	OsVersion:        "10",
	UserAgent:        "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36",
	MaxRESTRetries:   3,
	StateMaxMessages: 50,
	FetchConcurrency: 5,
}

// NewConfig returns a fresh copy of DefaultConfig that is safe to modify.
func NewConfig() *Config {
	c := DefaultConfig
	return &c
}
