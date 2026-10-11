package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

func TestHTTPLoginBodySizeBoundary(t *testing.T) {
	s := newStack(t)
	srv := httptest.NewServer(NewHTTPAPI(s.store, s.rm))
	defer srv.Close()
	c := srv.Client()

	loginBodyOfSize := func(total int) json.RawMessage {
		b, err := json.Marshal(map[string]string{"username": "alice", "password": "p", "pad": ""})
		if err != nil {
			t.Fatalf("marshal login skeleton: %v", err)
		}
		b, err = json.Marshal(map[string]string{
			"username": "alice", "password": "p", "pad": strings.Repeat("x", total-len(b)),
		})
		if err != nil {
			t.Fatalf("marshal padded login: %v", err)
		}
		if len(b) != total {
			t.Fatalf("login body = %d bytes, want exactly %d", len(b), total)
		}
		return json.RawMessage(b)
	}
	for _, tc := range []struct {
		size   int
		status int
	}{
		{httpBodyLimitBytes, http.StatusOK},
		{httpBodyLimitBytes + 1<<10, http.StatusBadRequest},
	} {
		got := doJSON(t, c, http.MethodPost, srv.URL+"/api/login", "", loginBodyOfSize(tc.size))
		if got.status != tc.status {
			t.Errorf("login with a %d-byte body: status %d body %s, want %d",
				tc.size, got.status, got.body, tc.status)
		}
	}
}

func TestHTTPLogoutCookieExpiresExactly(t *testing.T) {
	s := newStack(t)
	srv := httptest.NewServer(NewHTTPAPI(s.store, s.rm))
	defer srv.Close()

	got := doJSON(t, srv.Client(), http.MethodPost, srv.URL+"/api/logout", "", nil)
	wantStatus(t, got, http.StatusNoContent, nil)
	var cleared *http.Cookie
	for _, ck := range got.cookies {
		if ck.Name == sessionCookieName {
			cleared = ck
		}
	}
	if cleared == nil {
		t.Fatalf("logout cookies = %+v, want the expired %s cookie", got.cookies, sessionCookieName)
	}
	if cleared.MaxAge != -1 {
		t.Errorf("logout cookie MaxAge = %d, want -1", cleared.MaxAge)
	}
	if !cleared.HttpOnly {
		t.Error("logout cookie missing HttpOnly")
	}
	if cleared.Value != "" || cleared.Path != "/" {
		t.Errorf("logout cookie = value %q path %q, want empty on /", cleared.Value, cleared.Path)
	}
}

func TestHTTPSSEDefaultHandlerOpensStream(t *testing.T) {
	s := newStack(t)
	srv := httptest.NewServer(NewHTTPAPI(s.store, s.rm))
	defer srv.Close()
	alice := seedUser(t, s.store, "alice")
	r, err := s.rm.Create(alice.ID, 0, config.SeriesBO3, nil)
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	_, _ = openSSE(t, srv.Client(), srv.URL+"/api/rooms/"+r.ID()+"/events", 250*time.Millisecond)
}

func TestHTTPSSEKeepaliveKeepsStreamOpen(t *testing.T) {
	s := newStack(t)
	alice := seedUser(t, s.store, "alice")
	r, err := s.rm.Create(alice.ID, 0, config.SeriesBO3, nil)
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	api := &apiServer{store: s.store, rooms: s.rm, keepalive: 20 * time.Millisecond}
	srv := httptest.NewServer(api.routes())
	defer srv.Close()

	sr, _ := openSSE(t, srv.Client(), srv.URL+"/api/rooms/"+r.ID()+"/events", 5*time.Second)
	keepalives := 0
	for keepalives < 2 {
		line, err := sr.br.ReadString('\n')
		if err != nil {
			t.Fatalf("stream ended after %d keepalive frame(s): %v, want it open for the next tick",
				keepalives, err)
		}
		if strings.HasPrefix(line, ":") {
			keepalives++
		}
	}
}

func TestHTTPSSEDefaultKeepaliveCadence(t *testing.T) {
	s := newStack(t)
	srv := httptest.NewServer(NewHTTPAPI(s.store, s.rm))
	defer srv.Close()
	alice := seedUser(t, s.store, "alice")
	r, err := s.rm.Create(alice.ID, 0, config.SeriesBO3, nil)
	if err != nil {
		t.Fatalf("create room: %v", err)
	}

	cadence := time.Duration(config.SSEKeepaliveMs) * time.Millisecond
	sr, _ := openSSE(t, srv.Client(), srv.URL+"/api/rooms/"+r.ID()+"/events", 2*cadence)
	start := time.Now()
	for {
		line, err := sr.br.ReadString('\n')
		if err != nil {
			t.Fatalf("first keepalive read: %v", err)
		}
		if strings.HasPrefix(line, ":") {
			break
		}
	}
	elapsed := time.Since(start)
	lo, hi := cadence-500*time.Millisecond, cadence+500*time.Millisecond
	if elapsed < lo || elapsed > hi {
		t.Fatalf("first keepalive after %v, want inside (%v, %v)", elapsed, lo, hi)
	}
}
