package api

import (
	"bytes"
	"caro-ai-pvp/internal/domain"
	"caro-ai-pvp/internal/persistence"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testHandler() *Handler {
	return NewHandler(NewInMemoryStore(), nil)
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

func TestUndoMoveNotFound(t *testing.T) {
	h := testHandler()
	req := httptest.NewRequest(http.MethodPost, "/api/games/nonexistent/undo", nil)
	req.SetPathValue("id", "nonexistent")
	w := httptest.NewRecorder()
	h.UndoMove(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestUndoMoveNoHistory(t *testing.T) {
	h := testHandler()
	req := httptest.NewRequest(http.MethodPost, "/api/games", bytes.NewReader(
		[]byte(`{}`),
	))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.CreateGame(w, req)
	gameID := decodeResponse(t, w)["gameId"].(string)

	// Undo with no moves → returns 500 since ErrNoMoves is not a known error type
	req2 := httptest.NewRequest(http.MethodPost, "/api/games/"+gameID+"/undo", nil)
	req2.SetPathValue("id", gameID)
	w2 := httptest.NewRecorder()
	h.UndoMove(w2, req2)
	assert.Equal(t, http.StatusInternalServerError, w2.Code)
}

func TestMakeAIMove(t *testing.T) {
	h := testHandler()
	req := httptest.NewRequest(http.MethodPost, "/api/games", bytes.NewReader(
		[]byte(`{"gameMode":"pvai","blueDifficulty":1}`),
	))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.CreateGame(w, req)
	gameID := decodeResponse(t, w)["gameId"].(string)

	// Make a move first (red = human)
	req2 := httptest.NewRequest(http.MethodPost, "/api/games/"+gameID+"/move", bytes.NewReader(
		[]byte(`{"x":7,"y":7}`),
	))
	req2.SetPathValue("id", gameID)
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	h.MakeMove(w2, req2)
	require.Equal(t, http.StatusOK, w2.Code)

	// AI move (blue = AI L1)
	req3 := httptest.NewRequest(http.MethodPost, "/api/games/"+gameID+"/ai-move", nil)
	req3.SetPathValue("id", gameID)
	w3 := httptest.NewRecorder()
	h.MakeAIMove(w3, req3)
	assert.Equal(t, http.StatusOK, w3.Code)
	resp := decodeResponse(t, w3)
	state := resp["state"].(map[string]any)
	assert.Equal(t, 2.0, state["moveNumber"])
}

func TestMakeAIMoveNotFound(t *testing.T) {
	h := testHandler()
	req := httptest.NewRequest(http.MethodPost, "/api/games/nonexistent/ai-move", nil)
	req.SetPathValue("id", "nonexistent")
	w := httptest.NewRecorder()
	h.MakeAIMove(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestDeleteGameNotFound(t *testing.T) {
	h := testHandler()
	req := httptest.NewRequest(http.MethodDelete, "/api/games/nonexistent", nil)
	req.SetPathValue("id", "nonexistent")
	w := httptest.NewRecorder()
	h.DeleteGame(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestMakeMoveInvalidJSON(t *testing.T) {
	h := testHandler()
	req := httptest.NewRequest(http.MethodPost, "/api/games", bytes.NewReader(
		[]byte(`{}`),
	))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.CreateGame(w, req)
	gameID := decodeResponse(t, w)["gameId"].(string)

	req2 := httptest.NewRequest(http.MethodPost, "/api/games/"+gameID+"/move", bytes.NewReader(
		[]byte(`not json`),
	))
	req2.SetPathValue("id", gameID)
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	h.MakeMove(w2, req2)
	assert.Equal(t, http.StatusBadRequest, w2.Code)
}

func TestCleanupCompleted(t *testing.T) {
	store := NewInMemoryStore()
	h := NewHandler(store, nil)

	// Create a game
	req := httptest.NewRequest(http.MethodPost, "/api/games", bytes.NewReader(
		[]byte(`{}`),
	))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.CreateGame(w, req)
	assert.Equal(t, 1, store.Count())

	// Cleanup should remove nothing (game is active)
	removed := store.CleanupCompleted()
	assert.Equal(t, 0, removed)
}

func TestCleanupAll(t *testing.T) {
	store := NewInMemoryStore()
	h := NewHandler(store, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/games", bytes.NewReader(
		[]byte(`{}`),
	))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.CreateGame(w, req)
	assert.Equal(t, 1, store.Count())

	removed := store.CleanupAll()
	assert.Equal(t, 1, removed)
	assert.Equal(t, 0, store.Count())
}

func TestOpponentOf(t *testing.T) {
	assert.Equal(t, "blue", opponentOf("red"))
	assert.Equal(t, "red", opponentOf("blue"))
	assert.Equal(t, "red", opponentOf("other"))
}

func TestCreateGameAIvAI(t *testing.T) {
	h := testHandler()
	req := httptest.NewRequest(http.MethodPost, "/api/games", bytes.NewReader(
		[]byte(`{"gameMode":"aivai","redDifficulty":3,"blueDifficulty":3}`),
	))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.CreateGame(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	resp := decodeResponse(t, w)
	state := resp["state"].(map[string]any)
	assert.Equal(t, "aivai", state["gameMode"])
}

func TestMakeMoveOccupied(t *testing.T) {
	h := testHandler()
	req := httptest.NewRequest(http.MethodPost, "/api/games", bytes.NewReader(
		[]byte(`{}`),
	))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.CreateGame(w, req)
	gameID := decodeResponse(t, w)["gameId"].(string)

	// First move
	req2 := httptest.NewRequest(http.MethodPost, "/api/games/"+gameID+"/move", bytes.NewReader(
		[]byte(`{"x":7,"y":7}`),
	))
	req2.SetPathValue("id", gameID)
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	h.MakeMove(w2, req2)
	require.Equal(t, http.StatusOK, w2.Code)

	// Same cell again
	req3 := httptest.NewRequest(http.MethodPost, "/api/games/"+gameID+"/move", bytes.NewReader(
		[]byte(`{"x":7,"y":7}`),
	))
	req3.SetPathValue("id", gameID)
	req3.Header.Set("Content-Type", "application/json")
	w3 := httptest.NewRecorder()
	h.MakeMove(w3, req3)
	assert.Equal(t, http.StatusBadRequest, w3.Code)
}

func TestActiveGameCount(t *testing.T) {
	store := NewInMemoryStore()
	h := NewHandler(store, nil)

	assert.Equal(t, 0, store.ActiveGameCount())

	req := httptest.NewRequest(http.MethodPost, "/api/games", bytes.NewReader(
		[]byte(`{}`),
	))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.CreateGame(w, req)
	assert.Equal(t, 1, store.ActiveGameCount())
}

func TestLogHumanMoveWithMatches(t *testing.T) {
	dir := t.TempDir()
	ms, err := persistence.NewMatchStore(filepath.Join(dir, "test.db"))
	require.NoError(t, err)
	defer ms.Close()

	store := NewInMemoryStore()
	h := NewHandler(store, ms)

	// Create PvP game
	req := httptest.NewRequest(http.MethodPost, "/api/games", bytes.NewReader(
		[]byte(`{"gameMode":"pvp"}`),
	))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.CreateGame(w, req)
	gameID := decodeResponse(t, w)["gameId"].(string)

	// Make a move → triggers logHumanMove
	req2 := httptest.NewRequest(http.MethodPost, "/api/games/"+gameID+"/move", bytes.NewReader(
		[]byte(`{"x":7,"y":7}`),
	))
	req2.SetPathValue("id", gameID)
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	h.MakeMove(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)
}

func TestLogAIMoveWithMatches(t *testing.T) {
	dir := t.TempDir()
	ms, err := persistence.NewMatchStore(filepath.Join(dir, "test.db"))
	require.NoError(t, err)
	defer ms.Close()

	store := NewInMemoryStore()
	h := NewHandler(store, ms)

	// Create PvAI game
	req := httptest.NewRequest(http.MethodPost, "/api/games", bytes.NewReader(
		[]byte(`{"gameMode":"pvai","blueDifficulty":1}`),
	))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.CreateGame(w, req)
	gameID := decodeResponse(t, w)["gameId"].(string)

	// Human move
	req2 := httptest.NewRequest(http.MethodPost, "/api/games/"+gameID+"/move", bytes.NewReader(
		[]byte(`{"x":7,"y":7}`),
	))
	req2.SetPathValue("id", gameID)
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	h.MakeMove(w2, req2)
	require.Equal(t, http.StatusOK, w2.Code)

	// AI move → triggers logAIMove
	req3 := httptest.NewRequest(http.MethodPost, "/api/games/"+gameID+"/ai-move", nil)
	req3.SetPathValue("id", gameID)
	w3 := httptest.NewRecorder()
	h.MakeAIMove(w3, req3)
	assert.Equal(t, http.StatusOK, w3.Code)
}

func TestDeleteGameWithMatches(t *testing.T) {
	dir := t.TempDir()
	ms, err := persistence.NewMatchStore(filepath.Join(dir, "test.db"))
	require.NoError(t, err)
	defer ms.Close()

	store := NewInMemoryStore()
	h := NewHandler(store, ms)

	req := httptest.NewRequest(http.MethodPost, "/api/games", bytes.NewReader(
		[]byte(`{}`),
	))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.CreateGame(w, req)
	gameID := decodeResponse(t, w)["gameId"].(string)

	// Delete → triggers match completion
	req2 := httptest.NewRequest(http.MethodDelete, "/api/games/"+gameID, nil)
	req2.SetPathValue("id", gameID)
	w2 := httptest.NewRecorder()
	h.DeleteGame(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)

	// Verify match is recorded as abandoned
	record, err := ms.GetGame(gameID)
	require.NoError(t, err)
	assert.Equal(t, "abandoned", record.Winner)
}
