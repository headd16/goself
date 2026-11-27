package discord

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/goccy/go-json"
	"github.com/headd16/goself/types"
)

func TestFetchUserUsesCacheFirst(t *testing.T) {
	var calls atomic.Int32
	g := newRESTGateway(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		id := strings.TrimPrefix(r.URL.Path, "/api/v9/users/")
		fmt.Fprintf(w, `{"id":%q,"username":"user-%s"}`, id, id)
	})

	for i := 0; i < 3; i++ {
		u, err := g.FetchUser("42")
		if err != nil || u.Username != "user-42" {
			t.Fatalf("FetchUser = %+v, %v", u, err)
		}
	}
	if calls.Load() != 1 {
		t.Errorf("API called %d times, want 1 (then cached)", calls.Load())
	}
	// Get* always refreshes.
	if _, err := g.GetUser("42"); err != nil || calls.Load() != 2 {
		t.Errorf("GetUser should hit the API: calls=%d err=%v", calls.Load(), err)
	}
}

func TestFetchUsersInParallelWithLimit(t *testing.T) {
	var inFlight, peak, calls atomic.Int32
	g := newRESTGateway(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		n := inFlight.Add(1)
		for {
			p := peak.Load()
			if n <= p || peak.CompareAndSwap(p, n) {
				break
			}
		}
		time.Sleep(30 * time.Millisecond)
		inFlight.Add(-1)
		id := strings.TrimPrefix(r.URL.Path, "/api/v9/users/")
		if id == "404" {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"Unknown User","code":10013}`))
			return
		}
		fmt.Fprintf(w, `{"id":%q,"username":"u%s"}`, id, id)
	})
	g.Config.FetchConcurrency = 3
	g.State.storeUsers(&types.User{ID: "cached", Username: "cached"})

	ids := []string{"cached", "404"}
	for i := 1; i <= 12; i++ {
		ids = append(ids, fmt.Sprint(i))
	}
	ids = append(ids, "1", "2") // duplicates are fetched once

	start := time.Now()
	users, err := g.FetchUsers(ids)
	elapsed := time.Since(start)

	if len(users) != 13 || users["cached"].Username != "cached" || users["7"].Username != "u7" {
		t.Errorf("got %d users: %v", len(users), users)
	}
	if err == nil || !strings.Contains(err.Error(), "404") {
		t.Errorf("error should mention the failed ID, got %v", err)
	}
	if calls.Load() != 13 {
		t.Errorf("API calls = %d, want 13 (12 + the failing one, no duplicates, no cached)", calls.Load())
	}
	if peak.Load() > 3 {
		t.Errorf("concurrency limit exceeded: %d requests in flight", peak.Load())
	}
	if peak.Load() < 2 || elapsed > 400*time.Millisecond {
		t.Errorf("requests did not run in parallel: peak=%d elapsed=%s", peak.Load(), elapsed)
	}
}

func TestFetchGuildChannelsCategoriesAndRoles(t *testing.T) {
	var channelCalls, roleCalls atomic.Int32
	g := newRESTGateway(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v9/guilds/g/channels":
			channelCalls.Add(1)
			_, _ = w.Write([]byte(`[
				{"id":"3","type":0,"name":"chat","parent_id":"1","position":1},
				{"id":"1","type":4,"name":"Cat","position":0},
				{"id":"2","type":2,"name":"Voice","parent_id":"1","position":0},
				{"id":"4","type":0,"name":"top","position":0}]`))
		case "/api/v9/guilds/g/roles":
			roleCalls.Add(1)
			_, _ = w.Write([]byte(`[{"id":"g","name":"@everyone","position":0,"permissions":"0"},{"id":"r","name":"Mod","position":5,"permissions":"8192"}]`))
		case "/api/v9/channels/1":
			_, _ = w.Write([]byte(`{"id":"1","type":4,"name":"Cat","guild_id":"g"}`))
		default:
			t.Errorf("unexpected %s", r.URL.Path)
		}
	})

	channels, err := g.FetchGuildChannels("g")
	if err != nil || len(channels) != 4 || channels[3].Name != "Voice" {
		t.Fatalf("channels = %+v, %v", channels, err)
	}
	cats, _ := g.FetchCategories("g")
	if len(cats) != 1 || cats[0].ID != "1" {
		t.Errorf("categories = %+v", cats)
	}
	inCat, err := g.FetchCategoryChannels("1")
	if err != nil || len(inCat) != 2 || inCat[0].Name != "chat" {
		t.Errorf("category channels = %+v, %v", inCat, err)
	}
	tree, _ := g.FetchChannelTree("g")
	if len(tree) != 2 || tree[0].Channels[0].Name != "top" {
		t.Errorf("tree = %+v", tree)
	}
	if channelCalls.Load() != 1 {
		t.Errorf("channels fetched %d times, want 1", channelCalls.Load())
	}

	roles, err := g.FetchRoles("g")
	if err != nil || len(roles) != 2 || roles[0].Name != "Mod" {
		t.Fatalf("roles = %+v, %v", roles, err)
	}
	if r, err := g.FetchRole("g", "r"); err != nil || r.Name != "Mod" {
		t.Errorf("FetchRole = %+v, %v", r, err)
	}
	if roleCalls.Load() != 1 {
		t.Errorf("roles fetched %d times, want 1", roleCalls.Load())
	}
	if _, err := g.FetchRole("g", "missing"); err == nil {
		t.Error("missing role should fail")
	}
}

func TestFetchMembersOverGateway(t *testing.T) {
	mock := newMockGateway(t, 45000)
	g := NewGateway("token", testConfig(mock.url(), ""))

	openErr := make(chan error, 1)
	go func() { openErr <- g.Open() }()
	ws := mock.next()
	expectOp(t, ws, types.OpcodeIdentify)
	dispatch(t, ws, "READY", 1, readyData(mock.url()))
	if err := waitFor(t, openErr, "Open"); err != nil {
		t.Fatal(err)
	}

	// The fake server answers every op 8 with two chunks.
	var requests atomic.Int32
	var batchSizes sync.Map
	go func() {
		seq := 2
		for {
			p := expectOpNoFail(ws, types.OpcodeRequestGuildMembers)
			if p == nil {
				return
			}
			n := requests.Add(1)
			var req types.RequestGuildMembersPayload
			_ = json.Unmarshal(p.D, &req)
			batchSizes.Store(n, len(req.UserIDs))
			half := len(req.UserIDs) / 2
			for idx, part := range [][]string{req.UserIDs[:half], req.UserIDs[half:]} {
				var members []any
				for _, id := range part {
					if id == "not-a-member" {
						continue
					}
					members = append(members, map[string]any{"user": map[string]any{"id": id, "username": "m" + id}, "roles": []string{}, "joined_at": "x"})
				}
				dispatch(t, ws, "GUILD_MEMBERS_CHUNK", seq, map[string]any{
					"guild_id": "g", "members": members, "chunk_index": idx, "chunk_count": 2, "nonce": req.Nonce,
					"not_found": []string{},
				})
				seq++
			}
		}
	}()

	ids := []string{"not-a-member"}
	for i := 0; i < 149; i++ {
		ids = append(ids, fmt.Sprint(1000+i))
	}
	members, err := g.FetchMembers("g", ids)
	if err != nil {
		t.Fatalf("FetchMembers: %v", err)
	}
	if len(members) != 149 || members["1000"].User.Username != "m1000" {
		t.Errorf("got %d members", len(members))
	}
	if requests.Load() != 2 {
		t.Errorf("op 8 sent %d times, want 2 batches", requests.Load())
	}
	if s, _ := batchSizes.Load(int32(1)); s != 100 {
		t.Errorf("first batch had %v IDs, want 100", s)
	}

	// Second call is served from the cache: no new op 8.
	m, err := g.FetchMember("g", "1001")
	if err != nil || m.User.Username != "m1001" || requests.Load() != 2 {
		t.Errorf("cached FetchMember = %+v, %v (requests=%d)", m, err, requests.Load())
	}
	if _, err := g.FetchMember("g", "not-a-member"); err != ErrMemberNotFound {
		t.Errorf("non-member err = %v", err)
	}
	_ = g.Close()
}

// expectOpNoFail is expectOp for helper goroutines: it returns nil instead
// of failing when the connection closes.
func expectOpNoFail(ws interface {
	ReadMessage() (int, []byte, error)
}, want int) *payload {
	for {
		_, msg, err := ws.ReadMessage()
		if err != nil {
			return nil
		}
		var p payload
		if json.Unmarshal(msg, &p) == nil && p.Op == want {
			return &p
		}
	}
}

func TestFetchMemberViaProfileWhenOffline(t *testing.T) {
	g := newRESTGateway(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v9/users/5/profile" || r.URL.Query().Get("guild_id") != "g" {
			t.Errorf("unexpected %s?%s", r.URL.Path, r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`{"user":{"id":"5","username":"five"},"user_profile":{"bio":"hi","pronouns":"they/them"},
			"guild_member":{"roles":["r"],"nick":"Fiver","joined_at":"x"},
			"mutual_guilds":[{"id":"g","nick":"Fiver"}],"connected_accounts":[{"type":"github","id":"1","name":"five","verified":true}],
			"badges":[{"id":"hypesquad","description":"HypeSquad","icon":"abc"}],"premium_type":2}`))
	})
	m, err := g.FetchMember("g", "5")
	if err != nil || m.Nick != "Fiver" || m.User.Username != "five" {
		t.Fatalf("FetchMember = %+v, %v", m, err)
	}
	if cached, ok := g.State.Member("g", "5"); !ok || cached.Nick != "Fiver" {
		t.Error("member from profile not cached")
	}

	p, err := g.GetUserProfile("5", "g")
	if err != nil || p.UserProfile.Bio != "hi" || len(p.ConnectedAccounts) != 1 || len(p.MutualGuilds) != 1 || p.PremiumType != 2 {
		t.Errorf("profile = %+v, %v", p, err)
	}
}

func TestFetchPermissionsEndToEnd(t *testing.T) {
	g := newRESTGateway(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v9/channels/c":
			_, _ = w.Write([]byte(fmt.Sprintf(`{"id":"c","guild_id":"g","type":0,"permission_overwrites":[
				{"id":"g","type":0,"allow":"0","deny":"%d"}]}`, types.PermissionSendMessages)))
		case "/api/v9/guilds/g":
			_, _ = w.Write([]byte(fmt.Sprintf(`{"id":"g","name":"G","owner_id":"1","roles":[
				{"id":"g","name":"@everyone","permissions":"%d"}]}`, types.PermissionViewChannel|types.PermissionSendMessages)))
		case "/api/v9/users/5/profile":
			_, _ = w.Write([]byte(`{"user":{"id":"5","username":"five"},"guild_member":{"roles":[],"joined_at":"x"}}`))
		default:
			t.Errorf("unexpected %s", r.URL.Path)
		}
	})
	perms, err := g.FetchPermissions("5", "c")
	if err != nil {
		t.Fatal(err)
	}
	if !types.HasPermission(perms, types.PermissionViewChannel) || types.HasPermission(perms, types.PermissionSendMessages) {
		t.Errorf("perms = %v", types.PermissionNames(perms))
	}
	if p, err := g.FetchGuildPermissions("g", "5"); err != nil || !types.HasPermission(p, types.PermissionSendMessages) {
		t.Errorf("guild perms = %v, %v", types.PermissionNames(p), err)
	}
}

func TestGetInviteAcceptsLinks(t *testing.T) {
	var paths []string
	g := newRESTGateway(t, func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		_, _ = w.Write([]byte(`{"code":"abc","guild":{"id":"g","name":"Guild"},"approximate_member_count":10}`))
	})
	for _, in := range []string{"abc", "discord.gg/abc", "https://discord.com/invite/abc?event=1", "https://discord.gg/abc/"} {
		inv, err := g.GetInvite(in)
		if err != nil || inv.Guild.Name != "Guild" || inv.ApproximateMemberCount != 10 {
			t.Fatalf("GetInvite(%q) = %+v, %v", in, inv, err)
		}
	}
	for _, p := range paths {
		if p != "/api/v9/invites/abc" {
			t.Errorf("path = %s", p)
		}
	}
}

func TestFetchWithStateDisabledAlwaysCallsAPI(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_, _ = w.Write([]byte(`{"id":"1","type":0,"name":"c"}`))
	}))
	t.Cleanup(srv.Close)
	cfg := testConfig("", srv.URL+"/api/v9")
	cfg.DisableState = true
	g := NewGateway("token", cfg)
	_, _ = g.FetchChannel("1")
	_, _ = g.FetchChannel("1")
	if calls.Load() != 2 {
		t.Errorf("calls = %d, want 2 with the cache disabled", calls.Load())
	}
}
