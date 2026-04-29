package api

import (
	"bytes"
	"caro-ai-pvp/internal/domain"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testHandler() *Handler {
	return NewHandler(NewInMemoryStore())
}

func decodeResponse(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var resp map[string]any
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	return resp
}

func TestCreateGameDefault(t *testing.T) {
	h := testHandler()
	req := httptest.NewRequest(http.MethodPost, "/api/games", bytes.NewReader(
		[]byte(`{}`),
	))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.CreateGame(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	resp := decodeResponse(t, w)
	assert.NotEmpty(t, resp["gameId"])
	state, ok := resp["state"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "red", state["currentPlayer"])
	assert.Equal(t, "7+5", state["timeControl"])
}

func TestCreateGameBlitz(t *testing.T) {
	h := testHandler()
	req := httptest.NewRequest(http.MethodPost, "/api/games", bytes.NewReader(
		[]byte(`{"timeControl":"3+2","gameMode":"aivai","difficulty":3}`),
	))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.CreateGame(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	resp := decodeResponse(t, w)
	state := resp["state"].(map[string]any)
	assert.Equal(t, "3+2", state["timeControl"])
	assert.Equal(t, "aivai", state["gameMode"])
}

func TestCreateGameInvalidDifficulty(t *testing.T) {
	h := testHandler()
	req := httptest.NewRequest(http.MethodPost, "/api/games", bytes.NewReader(
		[]byte(`{"difficulty":0}`),
	))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.CreateGame(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateGameTooMany(t *testing.T) {
	h := testHandler()
	for range domain.MaxConcurrentGames {
		req := httptest.NewRequest(http.MethodPost, "/api/games", bytes.NewReader(
			[]byte(`{}`),
		))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.CreateGame(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/games", bytes.NewReader(
		[]byte(`{}`),
	))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.CreateGame(w, req)
	assert.Equal(t, http.StatusTooManyRequests, w.Code)
}

func TestGetGameNotFound(t *testing.T) {
	h := testHandler()
	req := httptest.NewRequest(http.MethodGet, "/api/games/nonexistent", nil)
	req.SetPathValue("id", "nonexistent")
	w := httptest.NewRecorder()
	h.GetGame(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGetGameFound(t *testing.T) {
	h := testHandler()
	req := httptest.NewRequest(http.MethodPost, "/api/games", bytes.NewReader(
		[]byte(`{}`),
	))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.CreateGame(w, req)
	created := decodeResponse(t, w)
	gameID := created["gameId"].(string)

	req2 := httptest.NewRequest(http.MethodGet, "/api/games/"+gameID, nil)
	req2.SetPathValue("id", gameID)
	w2 := httptest.NewRecorder()
	h.GetGame(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)
}

func TestMakeMoveNotFound(t *testing.T) {
	h := testHandler()
	req := httptest.NewRequest(http.MethodPost, "/api/games/nonexistent/move", bytes.NewReader(
		[]byte(`{"x":7,"y":7}`),
	))
	req.SetPathValue("id", "nonexistent")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.MakeMove(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestMakeMoveThenGet(t *testing.T) {
	h := testHandler()
	// Create game
	req := httptest.NewRequest(http.MethodPost, "/api/games", bytes.NewReader(
		[]byte(`{}`),
	))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.CreateGame(w, req)
	gameID := decodeResponse(t, w)["gameId"].(string)

	// Make move
	req2 := httptest.NewRequest(http.MethodPost, "/api/games/"+gameID+"/move", bytes.NewReader(
		[]byte(`{"x":7,"y":7}`),
	))
	req2.SetPathValue("id", gameID)
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	h.MakeMove(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)
	resp := decodeResponse(t, w2)
	state := resp["state"].(map[string]any)
	assert.Equal(t, 1.0, state["moveNumber"])
}

func TestDeleteGame(t *testing.T) {
	h := testHandler()
	req := httptest.NewRequest(http.MethodPost, "/api/games", bytes.NewReader(
		[]byte(`{}`),
	))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.CreateGame(w, req)
	gameID := decodeResponse(t, w)["gameId"].(string)

	req2 := httptest.NewRequest(http.MethodDelete, "/api/games/"+gameID, nil)
	req2.SetPathValue("id", gameID)
	w2 := httptest.NewRecorder()
	h.DeleteGame(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)

	// Verify deleted
	req3 := httptest.NewRequest(http.MethodGet, "/api/games/"+gameID, nil)
	req3.SetPathValue("id", gameID)
	w3 := httptest.NewRecorder()
	h.GetGame(w3, req3)
	assert.Equal(t, http.StatusNotFound, w3.Code)
}

func TestUndoMove(t *testing.T) {
	h := testHandler()
	req := httptest.NewRequest(http.MethodPost, "/api/games", bytes.NewReader(
		[]byte(`{}`),
	))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.CreateGame(w, req)
	gameID := decodeResponse(t, w)["gameId"].(string)

	// Make move
	req2 := httptest.NewRequest(http.MethodPost, "/api/games/"+gameID+"/move", bytes.NewReader(
		[]byte(`{"x":7,"y":7}`),
	))
	req2.SetPathValue("id", gameID)
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	h.MakeMove(w2, req2)
	require.Equal(t, http.StatusOK, w2.Code)

	// Undo
	req3 := httptest.NewRequest(http.MethodPost, "/api/games/"+gameID+"/undo", nil)
	req3.SetPathValue("id", gameID)
	w3 := httptest.NewRecorder()
	h.UndoMove(w3, req3)
	assert.Equal(t, http.StatusOK, w3.Code)
	resp := decodeResponse(t, w3)
	state := resp["state"].(map[string]any)
	assert.Equal(t, 0.0, state["moveNumber"])
}
