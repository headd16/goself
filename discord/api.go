package discord

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"mime/multipart"
	"strings"
	"time"

	"github.com/goccy/go-json"
	"github.com/headd16/goself/types"
	"github.com/valyala/fasthttp"
)

const (
	discordOrigin     = "https://discord.com"
	defaultGatewayURL = "wss://gateway.discord.gg"
)

var httpClient = &fasthttp.Client{
	// Discord sends very large headers (CSP), so keep the buffer generous.
	ReadBufferSize:                64 * 1024,
	ReadTimeout:                   30 * time.Second,
	WriteTimeout:                  30 * time.Second,
	MaxIdleConnDuration:           90 * time.Second,
	NoDefaultUserAgentHeader:      true,
	DisableHeaderNamesNormalizing: true,
	DisablePathNormalizing:        true,
}

// RESTError is returned when Discord answers a REST request with a non-2xx
// status code.
type RESTError struct {
	Method     string
	Path       string
	StatusCode int
	// Code and Message come from Discord's JSON error body, when present.
	Code    int
	Message string
	Body    []byte
}

func (e *RESTError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("discord: %s %s: HTTP %d: %s (code %d)", e.Method, e.Path, e.StatusCode, e.Message, e.Code)
	}
	return fmt.Sprintf("discord: %s %s: HTTP %d: %s", e.Method, e.Path, e.StatusCode, truncate(e.Body, 200))
}

func newRESTError(method, path string, status int, body []byte) *RESTError {
	e := &RESTError{Method: method, Path: path, StatusCode: status, Body: body}
	var payload struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	if json.Unmarshal(body, &payload) == nil {
		e.Code, e.Message = payload.Code, payload.Message
	}
	return e
}

// DefaultUserAgent is the user agent of types.DefaultConfig.
func DefaultUserAgent() string { return types.DefaultConfig.UserAgent }

// Request sends an authenticated request to the Discord REST API and decodes
// the JSON response into out (if out is not nil). path is relative to the API
// base, e.g. "/users/@me". payload is JSON encoded unless it is a []byte,
// which is sent as-is. Rate limits (HTTP 429) are retried automatically.
//
// Use it to call endpoints that do not have a dedicated helper.
func (g *Gateway) Request(method, path string, payload, out any) error {
	return g.request(method, path, payload, out, "")
}

func (g *Gateway) request(method, path string, payload, out any, referer string) error {
	var body []byte
	switch p := payload.(type) {
	case nil:
	case []byte:
		body = p
	case json.RawMessage:
		body = p
	default:
		b, err := json.Marshal(p)
		if err != nil {
			return fmt.Errorf("discord: encode %s %s: %w", method, path, err)
		}
		body = b
	}
	contentType := ""
	if body != nil {
		contentType = "application/json"
	}
	return g.requestRaw(method, path, contentType, body, out, referer)
}

// requestMultipart sends payload as the "payload_json" field of a
// multipart/form-data body, the way the web client sends interactions.
func (g *Gateway) requestMultipart(method, path string, payload, out any, referer string) error {
	b, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("discord: encode %s %s: %w", method, path, err)
	}
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	if err := w.WriteField("payload_json", string(b)); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return g.requestRaw(method, path, w.FormDataContentType(), buf.Bytes(), out, referer)
}

func (g *Gateway) requestRaw(method, path, contentType string, body []byte, out any, referer string) error {
	if referer == "" {
		referer = discordOrigin + "/channels/@me"
	}
	url := g.apiBase() + path

	retries := max(g.Config.MaxRESTRetries, 0)

	for attempt := 0; ; attempt++ {
		status, respBody, err := g.do(method, url, contentType, body, referer)
		if err != nil {
			if method == fasthttp.MethodGet && attempt < retries {
				time.Sleep(backoff(attempt))
				continue
			}
			return fmt.Errorf("discord: %s %s: %w", method, path, err)
		}

		switch {
		case status == fasthttp.StatusTooManyRequests && attempt < retries:
			wait := retryAfter(respBody)
			g.log(slog.LevelWarn, "rate limited, retrying", "method", method, "path", path, "retry_after", wait)
			time.Sleep(wait)
			continue
		case status >= 500 && method == fasthttp.MethodGet && attempt < retries:
			time.Sleep(backoff(attempt))
			continue
		case status < 200 || status >= 300:
			return newRESTError(method, path, status, respBody)
		}

		if out != nil && len(bytes.TrimSpace(respBody)) > 0 {
			if err := json.Unmarshal(respBody, out); err != nil {
				return fmt.Errorf("discord: decode %s %s: %w", method, path, err)
			}
		}
		return nil
	}
}

func (g *Gateway) do(method, url, contentType string, body []byte, referer string) (int, []byte, error) {
	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(resp)

	req.Header.SetMethod(method)
	req.SetRequestURI(url)
	g.setHeaders(&req.Header, referer)
	if contentType != "" {
		req.Header.SetContentType(contentType)
		req.SetBody(body)
	}

	if err := httpClient.Do(req, resp); err != nil {
		return 0, nil, err
	}
	return resp.StatusCode(), append([]byte(nil), resp.Body()...), nil
}

func retryAfter(body []byte) time.Duration {
	var rl struct {
		RetryAfter float64 `json:"retry_after"`
	}
	_ = json.Unmarshal(body, &rl)
	wait := time.Duration(rl.RetryAfter * float64(time.Second))
	if wait <= 0 {
		wait = time.Second
	}
	if wait > time.Minute {
		wait = time.Minute
	}
	return wait
}

func (g *Gateway) apiBase() string {
	if g.Config.APIBaseURL != "" {
		return strings.TrimRight(g.Config.APIBaseURL, "/")
	}
	return discordOrigin + "/api/v" + g.Config.ApiVersion
}

func (g *Gateway) setHeaders(h *fasthttp.RequestHeader, referer string) {
	cfg := g.Config
	loc := g.locale()

	h.Set("Accept", "*/*")
	h.Set("Accept-Language", acceptLanguage(loc))
	h.Set("Authorization", g.token())
	h.Set("Origin", discordOrigin)
	h.Set("Referer", referer)
	if ua := secChUa(cfg); ua != "" {
		h.Set("Sec-Ch-Ua", ua)
		h.Set("Sec-Ch-Ua-Mobile", "?0")
		h.Set("Sec-Ch-Ua-Platform", secChPlatform(cfg.Os))
	}
	h.Set("Sec-Fetch-Dest", "empty")
	h.Set("Sec-Fetch-Mode", "cors")
	h.Set("Sec-Fetch-Site", "same-origin")
	h.SetUserAgent(cfg.UserAgent)
	h.Set("X-Debug-Options", "bugReporterEnabled")
	h.Set("X-Discord-Locale", loc)
	h.Set("X-Discord-Timezone", g.timezone())
	h.Set("X-Super-Properties", g.SuperPropertiesHeader())
}

// SuperProperties returns the client description sent when identifying.
func (g *Gateway) SuperProperties() types.SuperProperties {
	cfg := g.Config
	return types.SuperProperties{
		OS:                     cfg.Os,
		Browser:                cfg.Browser,
		Device:                 cfg.Device,
		SystemLocale:           g.locale(),
		HasClientMods:          false,
		BrowserUserAgent:       cfg.UserAgent,
		BrowserVersion:         cfg.BrowserVersion,
		OSVersion:              cfg.OsVersion,
		Referrer:               "",
		ReferringDomain:        "",
		ReferrerCurrent:        "",
		ReferringDomainCurrent: "",
		ReleaseChannel:         "stable",
		ClientBuildNumber:      g.BuildNumber(),
		ClientEventSource:      nil,
	}
}

// SuperPropertiesHeader returns the base64 encoded super properties sent in
// the X-Super-Properties header.
func (g *Gateway) SuperPropertiesHeader() string {
	b, err := json.Marshal(g.SuperProperties())
	if err != nil {
		return ""
	}
	return base64.StdEncoding.EncodeToString(b)
}

// BuildNumber returns Config.BuildNumber, or the build number scraped from
// discord.com (see LatestBuildNumber).
func (g *Gateway) BuildNumber() int {
	if g.Config.BuildNumber > 0 {
		return g.Config.BuildNumber
	}
	return LatestBuildNumber()
}

func (g *Gateway) locale() string {
	if g.Config.Locale != "" {
		return g.Config.Locale
	}
	return SystemLocale()
}

func (g *Gateway) timezone() string {
	if g.Config.Timezone != "" {
		return g.Config.Timezone
	}
	return SystemTimezone()
}

func (g *Gateway) token() string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.Selfbot.Token
}

func (g *Gateway) log(level slog.Level, msg string, args ...any) {
	if l := g.Config.Logger; l != nil {
		l.Log(context.Background(), level, msg, args...)
	}
}

func acceptLanguage(loc string) string {
	if i := strings.IndexByte(loc, '-'); i > 0 {
		return loc + "," + loc[:i] + ";q=0.9"
	}
	return loc
}

func secChUa(cfg *types.Config) string {
	major := cfg.BrowserVersion
	if i := strings.IndexByte(major, '.'); i > 0 {
		major = major[:i]
	}
	if major == "" {
		return ""
	}
	switch strings.ToLower(cfg.Browser) {
	case "chrome":
		return fmt.Sprintf(`"Google Chrome";v="%s", "Chromium";v="%s", "Not_A Brand";v="24"`, major, major)
	case "edge", "microsoft edge":
		return fmt.Sprintf(`"Microsoft Edge";v="%s", "Chromium";v="%s", "Not_A Brand";v="24"`, major, major)
	case "opera":
		return fmt.Sprintf(`"Opera";v="%s", "Chromium";v="%s", "Not_A Brand";v="24"`, major, major)
	}
	return ""
}

func secChPlatform(os string) string {
	switch strings.ToLower(os) {
	case "windows":
		return `"Windows"`
	case "mac os x", "macos":
		return `"macOS"`
	case "linux":
		return `"Linux"`
	case "chrome os", "cros":
		return `"Chrome OS"`
	case "android":
		return `"Android"`
	case "":
		return `"Unknown"`
	}
	return `"` + os + `"`
}

func truncate(b []byte, n int) string {
	if len(b) > n {
		return string(b[:n]) + "..."
	}
	return string(b)
}
