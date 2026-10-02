package server

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

// Transport failure arms that need a broken analytics surface or a broken
// client connection, both injected from inside the package.

// TestHTTPSummaryAndHistoryInternalWhenGamesTableBroken drops the games table
// with a live session held: authentication still passes (users and sessions
// are intact), but the summary and history reads fail and both routes answer
// the internal envelope alone, with no partial profile or listing body.
func TestHTTPSummaryAndHistoryInternalWhenGamesTableBroken(t *testing.T) {
	s := newStack(t)
	srv := httptest.NewServer(NewHTTPAPI(s.store, s.rm))
	defer srv.Close()
	c := srv.Client()
	alice := seedUser(t, s.store, "alice")
	token := mintSession(t, s.store, alice)

	if _, err := s.store.db.Exec(`DROP TABLE games`); err != nil {
		t.Fatalf("drop games: %v", err)
	}

	got := doJSON(t, c, http.MethodGet, srv.URL+"/api/me", token, nil)
	wantAPIError(t, got, http.StatusInternalServerError, "internal")
	if strings.Contains(string(got.body), `"username"`) {
		t.Errorf("me body = %s, want the error envelope with no partial summary", got.body)
	}

	got = doJSON(t, c, http.MethodGet, srv.URL+"/api/history", token, nil)
	wantAPIError(t, got, http.StatusInternalServerError, "internal")
	if strings.Contains(string(got.body), `"preview"`) {
		t.Errorf("history body = %s, want the error envelope with no partial listing", got.body)
	}
}

// slowSSEWriter is a ResponseWriter whose writes stall for a fixed delay:
// the connection of a reader that fell behind, the state whose hub backlog
// the retirement drain exists for.
type slowSSEWriter struct {
	rec   *httptest.ResponseRecorder
	delay time.Duration
}

func (w *slowSSEWriter) Header() http.Header  { return w.rec.Header() }
func (w *slowSSEWriter) WriteHeader(code int) { w.rec.WriteHeader(code) }
func (w *slowSSEWriter) Write(p []byte) (int, error) {
	time.Sleep(w.delay)
	return w.rec.Write(p)
}
func (w *slowSSEWriter) WriteString(str string) (int, error) {
	time.Sleep(w.delay)
	return w.rec.WriteString(str)
}

// deadSSEWriter is a connection the client already abandoned: every write
// fails.
type deadSSEWriter struct {
	rec *httptest.ResponseRecorder
}

var errDeadClient = errors.New("sse test: client gone")

func (w *deadSSEWriter) Header() http.Header             { return w.rec.Header() }
func (w *deadSSEWriter) WriteHeader(code int)            { w.rec.WriteHeader(code) }
func (w *deadSSEWriter) Write([]byte) (int, error)       { return 0, errDeadClient }
func (w *deadSSEWriter) WriteString(string) (int, error) { return 0, errDeadClient }

// runSSEAgainstWriter serves one room's event stream with a substituted
// ResponseWriter and reports when the handler returned.
func runSSEAgainstWriter(api *apiServer, roomID string, w http.ResponseWriter) <-chan struct{} {
	req := httptest.NewRequest(http.MethodGet, "/api/rooms/"+roomID+"/events", nil)
	done := make(chan struct{})
	go func() {
		defer close(done)
		api.routes().ServeHTTP(w, req)
	}()
	return done
}

// TestSSEStalledReaderGetsBufferedEventsThenEnds pins the retirement drain:
// with the reader stalled, the room's last events pile up in the hub buffer;
// once the room retires without a terminal frame, the stream still delivers
// every buffered event and then ends instead of keepalive-ing forever.
func TestSSEStalledReaderGetsBufferedEventsThenEnds(t *testing.T) {
	s := newStack(t)
	alice := seedUser(t, s.store, "alice")
	r, err := s.rm.Create(alice.ID, 0, config.SeriesBO3, nil)
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	api := &apiServer{store: s.store, rooms: s.rm, keepalive: time.Millisecond}
	rec := httptest.NewRecorder()
	done := runSSEAgainstWriter(api, r.ID(), &slowSSEWriter{rec: rec, delay: 4 * time.Millisecond})

	// Wait until the stream is registered on the hub, then bury it.
	var subscribed bool
	deadline := time.Now().Add(5 * time.Second)
	for !subscribed && time.Now().Before(deadline) {
		s.hub.mu.Lock()
		_, subscribed = s.hub.rooms[r.ID()]
		s.hub.mu.Unlock()
		if !subscribed {
			time.Sleep(time.Millisecond)
		}
	}
	if !subscribed {
		t.Fatal("stream never subscribed")
	}

	const events = 40
	for i := range events {
		s.hub.Publish(r.ID(), Event{Kind: EventKindMove, Payload: "D" + strconv.Itoa(i+1)})
	}
	// The terminal series frame rides at the tail of the backlog: whichever
	// loop delivers it, the stream ends on it.
	s.hub.Publish(r.ID(), Event{Kind: EventKindSeries, Payload: SideNone.String()})
	r.Close()

	select {
	case <-done:
	case <-time.After(15 * time.Second):
		t.Fatal("stream never ended after the retirement, want the drain to finish it")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("stream status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if n := strings.Count(body, "event: "+EventKindMove+"\n"); n != events {
		t.Errorf("delivered move frames = %d, want all %d buffered events", n, events)
	}
	if !strings.Contains(body, "event: "+EventKindSeries+"\ndata: "+SideNone.String()) {
		t.Errorf("stream body never carried the terminal series frame:\n%s", body)
	}
}

// TestSSEKeepaliveWriteErrorEndsStream pins the dead-connection arm: a client
// that stopped reading makes the keepalive write fail, and the stream ends
// on that error instead of ticking forever against a gone reader.
func TestSSEKeepaliveWriteErrorEndsStream(t *testing.T) {
	s := newStack(t)
	alice := seedUser(t, s.store, "alice")
	r, err := s.rm.Create(alice.ID, 0, config.SeriesBO3, nil)
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	api := &apiServer{store: s.store, rooms: s.rm, keepalive: time.Millisecond}
	rec := httptest.NewRecorder()
	done := runSSEAgainstWriter(api, r.ID(), &deadSSEWriter{rec: rec})

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("stream never ended on the failing keepalive write, want an immediate return")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("stream status = %d, want the headers already sent", rec.Code)
	}
	if body := rec.Body.String(); body != "" {
		t.Errorf("stream body = %q, want nothing delivered to the dead client", body)
	}
}
