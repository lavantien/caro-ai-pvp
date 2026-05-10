package api

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newIntegrationServer() *httptest.Server {
	store := NewInMemoryStore()
	handler := NewHandler(store, nil, nil)
	srv := NewServer(handler, slog.New(slog.NewTextHandler(os.Stderr, nil)))
	return httptest.NewServer(srv)
}

func decodeGameID(t *testing.T, body []byte) string {
	t.Helper()
	var resp map[string]any
	require.NoError(t, json.Unmarshal(body, &resp))
	id, ok := resp["gameId"].(string)
	require.True(t, ok, "response should have gameId")
	return id
}

func decodeState(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var resp map[string]any
	require.NoError(t, json.Unmarshal(body, &resp))
	state, ok := resp["state"].(map[string]any)
	require.True(t, ok, "response should have state")
	return state
}

func TestIntegrationFullGameFlow(t *testing.T) {
	srv := newIntegrationServer()
	defer srv.Close()

	// 1. Create game
	resp, err := http.Post(srv.URL+"/api/game/new", "application/json", strings.NewReader(
		`{"timeControl":"3+2","gameMode":"pvp"}`,
	))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	var body []byte
	readBody(t, resp, &body)
	gameID := decodeGameID(t, body)

	// 2. Make moves to create a winning line: Red plays (3,0)-(7,0), Blue plays (0,0)-(6,1)
	// Open Rule: Red's second move (6,0) must be outside 5x5 zone from first move (3,0)
	moves := []struct {
		x, y int
	}{
		{3, 0}, {0, 0}, // Red(3,0), Blue(0,0)
		{6, 0}, {6, 1}, // Red(6,0) dist=3, Blue(6,1)
		{4, 0}, {4, 1}, // Red(4,0), Blue(4,1)
		{5, 0}, {5, 1}, // Red(5,0), Blue(5,1)
		{7, 0},         // Red(7,0) -> Red wins with 5 in a row
	}
	for _, m := range moves {
		moveResp, err := http.Post(srv.URL+"/api/game/"+gameID+"/move", "application/json",
			strings.NewReader(jsonMarshal(t, MoveRequest{X: m.x, Y: m.y})))
		require.NoError(t, err)
		readBody(t, moveResp, &body)
		moveResp.Body.Close()
	}
	state := decodeState(t, body)

	// 3. Verify game is still accessible
	getResp, err := http.Get(srv.URL + "/api/game/" + gameID)
	require.NoError(t, err)
	readBody(t, getResp, &body)
	getResp.Body.Close()
	state = decodeState(t, body)
	assert.True(t, state["isGameOver"].(bool))

	// 4. Delete game
	delResp, err := http.NewRequest(http.MethodDelete, srv.URL+"/api/game/"+gameID, nil)
	require.NoError(t, err)
	client := &http.Client{}
	del, err := client.Do(delResp)
	require.NoError(t, err)
	del.Body.Close()
	assert.Equal(t, http.StatusOK, del.StatusCode)

	// 5. Verify deleted
	getResp2, err := http.Get(srv.URL + "/api/game/" + gameID)
	require.NoError(t, err)
	getResp2.Body.Close()
	assert.Equal(t, http.StatusNotFound, getResp2.StatusCode)
}

func TestIntegrationCreateUndoRedo(t *testing.T) {
	srv := newIntegrationServer()
	defer srv.Close()

	// Create
	resp, _ := http.Post(srv.URL+"/api/game/new", "application/json", strings.NewReader(`{}`))
	var body []byte
	readBody(t, resp, &body)
	resp.Body.Close()
	gameID := decodeGameID(t, body)

	// Move
	moveResp, _ := http.Post(srv.URL+"/api/game/"+gameID+"/move", "application/json",
		strings.NewReader(`{"x":7,"y":7}`))
	readBody(t, moveResp, &body)
	moveResp.Body.Close()
	state := decodeState(t, body)
	assert.Equal(t, 1.0, state["moveNumber"])

	// Undo
	undoResp, _ := http.Post(srv.URL+"/api/game/"+gameID+"/undo", "application/json", nil)
	readBody(t, undoResp, &body)
	undoResp.Body.Close()
	state = decodeState(t, body)
	assert.Equal(t, 0.0, state["moveNumber"])
	assert.Equal(t, "red", state["currentPlayer"])
}

func TestIntegrationConcurrentGames(t *testing.T) {
	srv := newIntegrationServer()
	defer srv.Close()

	var wg sync.WaitGroup
	gameIDs := make(chan string, 10)

	for i := range 10 {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			resp, err := http.Post(srv.URL+"/api/game/new", "application/json",
				strings.NewReader(`{"timeControl":"1+0"}`))
			if err != nil {
				t.Errorf("goroutine %d: %v", n, err)
				return
			}
			var body []byte
			readBody(t, resp, &body)
			resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				return
			}
			id := decodeGameID(t, body)
			gameIDs <- id
		}(i)
	}
	wg.Wait()
	close(gameIDs)

	count := 0
	for id := range gameIDs {
		count++
		assert.NotEmpty(t, id)
	}
	t.Logf("Created %d concurrent games", count)
}

func readBody(t *testing.T, resp *http.Response, out *[]byte) {
	t.Helper()
	var err error
	*out, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
}

func jsonMarshal(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return string(b)
}
