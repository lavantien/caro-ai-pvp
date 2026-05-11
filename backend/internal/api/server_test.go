package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"log/slog"
	"os"
)

func newTestServer() *httptest.Server {
	store := NewInMemoryStore()
	handler := NewHandler(store, nil, nil)
	srv := NewServer(handler, slog.New(slog.NewTextHandler(os.Stderr, nil)))
	return httptest.NewServer(srv)
}

func TestServerCreateAndGetGame(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	// Create
	resp, err := http.Post(srv.URL+"/api/game/new", "application/json", strings.NewReader(`{}`))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var created map[string]any
	json.NewDecoder(resp.Body).Decode(&created)
	gameID := created["gameId"].(string)
	assert.NotEmpty(t, gameID)

	// Get
	resp2, err := http.Get(srv.URL + "/api/game/" + gameID)
	require.NoError(t, err)
	defer resp2.Body.Close()
	assert.Equal(t, http.StatusOK, resp2.StatusCode)
}

func TestServerMakeMove(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	// Create
	resp, err := http.Post(srv.URL+"/api/game/new", "application/json", strings.NewReader(`{}`))
	require.NoError(t, err)
	defer resp.Body.Close()
	var created map[string]any
	json.NewDecoder(resp.Body).Decode(&created)
	gameID := created["gameId"].(string)

	// Move
	resp2, err := http.Post(srv.URL+"/api/game/"+gameID+"/move", "application/json",
		strings.NewReader(`{"x":7,"y":7}`))
	require.NoError(t, err)
	defer resp2.Body.Close()
	assert.Equal(t, http.StatusOK, resp2.StatusCode)

	var moveResp map[string]any
	json.NewDecoder(resp2.Body).Decode(&moveResp)
	state := moveResp["state"].(map[string]any)
	assert.Equal(t, 1.0, state["moveNumber"])
}

func TestServerCORS(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodOptions, srv.URL+"/api/game/new", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	assert.Equal(t, "http://localhost:5173", resp.Header.Get("Access-Control-Allow-Origin"))
}

func TestServerNotFound(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/game/nonexistent")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestRecoveryMiddleware(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	})

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	recovered := RecoveryMiddleware(logger, handler)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	recovered.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
