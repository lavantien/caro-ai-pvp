package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

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

func TestWriteDomainErrorMasksInternalAndPassesDomainText(t *testing.T) {
	s := newStack(t)
	srv := httptest.NewServer(NewHTTPAPI(s.store, s.rm))
	defer srv.Close()
	c := srv.Client()
	alice := seedUser(t, s.store, "alice")
	bob := seedUser(t, s.store, "bob")
	carol := seedUser(t, s.store, "carol")
	ta, tb, tcTok := mintSession(t, s.store, alice), mintSession(t, s.store, bob), mintSession(t, s.store, carol)

	got := doJSON(t, c, http.MethodPost, srv.URL+"/api/rooms", ta,
		map[string]any{"tcIdx": 0, "boLen": config.SeriesBO3})
	var room roomSummary
	wantStatus(t, got, http.StatusCreated, &room)
	base := srv.URL + "/api/rooms/" + room.ID
	wantStatus(t, doJSON(t, c, http.MethodPost, base+"/join", tb, nil), http.StatusNoContent, nil)

	got = doJSON(t, c, http.MethodPost, base+"/join", tcTok, nil)
	wantAPIError(t, got, http.StatusConflict, "room_full")
	var e apiError
	if err := json.Unmarshal(got.body, &e); err != nil {
		t.Fatalf("decode room_full body %q: %v", got.body, err)
	}
	if e.Message != ErrRoomFull.Error() {
		t.Errorf("room_full message = %q, want the sentinel text %q", e.Message, ErrRoomFull.Error())
	}

	if _, err := s.store.db.Exec(`DROP TABLE games`); err != nil {
		t.Fatalf("drop games: %v", err)
	}
	got = doJSON(t, c, http.MethodGet, srv.URL+"/api/me", ta, nil)
	wantAPIError(t, got, http.StatusInternalServerError, "internal")
	if err := json.Unmarshal(got.body, &e); err != nil {
		t.Fatalf("decode internal body %q: %v", got.body, err)
	}
	if e.Message != "internal error" {
		t.Errorf("internal message = %q, want the masked %q", e.Message, "internal error")
	}
}

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

type deadSSEWriter struct {
	rec *httptest.ResponseRecorder
}

var errDeadClient = errors.New("sse test: client gone")

func (w *deadSSEWriter) Header() http.Header             { return w.rec.Header() }
func (w *deadSSEWriter) WriteHeader(code int)            { w.rec.WriteHeader(code) }
func (w *deadSSEWriter) Write([]byte) (int, error)       { return 0, errDeadClient }
func (w *deadSSEWriter) WriteString(string) (int, error) { return 0, errDeadClient }

func runSSEAgainstWriter(api *apiServer, roomID string, w http.ResponseWriter) <-chan struct{} {
	req := httptest.NewRequest(http.MethodGet, "/api/rooms/"+roomID+"/events", nil)
	done := make(chan struct{})
	go func() {
		defer close(done)
		api.routes().ServeHTTP(w, req)
	}()
	return done
}

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
