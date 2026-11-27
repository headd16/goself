package goself

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/headd16/goself/types"
)

// liveToken returns the token used by the live tests, from $DISCORD_TOKEN or
// a file named test_token. The tests are skipped when neither exists.
func liveToken(t *testing.T) string {
	t.Helper()
	if token := strings.TrimSpace(os.Getenv("DISCORD_TOKEN")); token != "" {
		return token
	}
	if b, err := os.ReadFile("test_token"); err == nil && len(strings.TrimSpace(string(b))) > 0 {
		return strings.TrimSpace(string(b))
	}
	t.Skip("set DISCORD_TOKEN or create a test_token file to run live tests")
	return ""
}

func TestLiveConnectAndClose(t *testing.T) {
	client := NewClient(liveToken(t), nil)

	ready := make(chan *types.ReadyEventData, 1)
	if err := client.AddHandler(types.GatewayEventReady, func(e *types.ReadyEventData) {
		ready <- e
	}); err != nil {
		t.Fatal(err)
	}

	if err := client.Open(); err != nil {
		t.Fatalf("open: %v", err)
	}
	select {
	case e := <-ready:
		t.Logf("logged in as %s (%d guilds)", e.User.Tag(), len(e.Guilds))
	case <-time.After(10 * time.Second):
		t.Fatal("READY handler was not called")
	}
	if client.User().ID == "" {
		t.Fatal("user not set after READY")
	}

	if err := client.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if err := client.Wait(); err != nil {
		t.Fatalf("wait: %v", err)
	}
}

func TestLiveREST(t *testing.T) {
	client := NewClient(liveToken(t), nil)
	me, err := client.GetCurrentUser()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("REST user: %s", me.Tag())
}
