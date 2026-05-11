package api

import (
	"bytes"
	"caro-ai-pvp/internal/domain"
	"caro-ai-pvp/internal/engine"
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
	return NewHandler(NewInMemoryStore(), nil, nil)
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

func TestCreateGameThreeZero(t *testing.T) {
	h := testHandler()
	req := httptest.NewRequest(http.MethodPost, "/api/games", bytes.NewReader(
		[]byte(`{"timeControl":"3+0","gameMode":"pvp"}`),
	))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.CreateGame(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	resp := decodeResponse(t, w)
	state := resp["state"].(map[string]any)
	assert.Equal(t, "3+0", state["timeControl"])
	assert.Equal(t, 180.0, state["initialTime"])
	assert.Equal(t, 0.0, state["increment"])
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
	h := NewHandler(store, nil, nil)

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
	h := NewHandler(store, nil, nil)

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
	h := NewHandler(store, nil, nil)

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
	h := NewHandler(store, ms, nil)

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
	h := NewHandler(store, ms, nil)

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
	h := NewHandler(store, ms, nil)

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

func TestFormatStatlineNodes(t *testing.T) {
	assert.Equal(t, "0", formatStatlineNodes(0))
	assert.Equal(t, "42", formatStatlineNodes(42))
	assert.Equal(t, "999", formatStatlineNodes(999))
	assert.Equal(t, "1.5K", formatStatlineNodes(1500))
	assert.Equal(t, "1.2M", formatStatlineNodes(1_200_000))
	assert.Equal(t, "2.0M", formatStatlineNodes(2_000_000))
}

func TestFormatStatlineNPS(t *testing.T) {
	assert.Equal(t, "500", formatStatlineNPS(500))
	assert.Equal(t, "5K", formatStatlineNPS(5000))
	assert.Equal(t, "142K", formatStatlineNPS(142000))
	assert.Equal(t, "1M", formatStatlineNPS(1_000_000))
}

func TestBuildMoveDetail(t *testing.T) {
	h := testHandler()
	resp := GameResponse{
		CurrentPlayer:     "red",
		MoveNumber:        3,
		RedTimeRemaining:  415.5,
		BlueTimeRemaining: 300.2,
	}
	stats := engine.SearchStats{
		DepthAchieved:   12,
		NodesSearched:   1_200_000,
		NodesPerSecond:  142000,
		SearchScore:     340,
		TableHitRate:    0.87,
		AllocatedTimeMs: 12000,
		ThreadCount:     4,
	}

	detail := h.buildMoveDetail(resp, "blue", 8, 8, stats, 10800)

	assert.Equal(t, 2, detail.MoveNumber)
	assert.Equal(t, "blue", detail.Player)
	assert.Equal(t, "i9", detail.Pos)
	assert.Equal(t, int64(10800), detail.ThinkTimeMs)
	assert.Equal(t, int64(300200), detail.RemainingTimeMs)

	assert.Contains(t, detail.Statline, "M 2")
	assert.Contains(t, detail.Statline, "blue")
	assert.Contains(t, detail.Statline, "i9")
	assert.Contains(t, detail.Statline, "d=12")
	assert.Contains(t, detail.Statline, "n=1.2M")
	assert.Contains(t, detail.Statline, "nps=142K")
	assert.Contains(t, detail.Statline, "tt= 87%")
	assert.Contains(t, detail.Statline, "s=+340")
	assert.Contains(t, detail.Statline, "t=10.8s")

	assert.Equal(t, 12, detail.EngineStats.Depth)
	assert.Equal(t, int64(1_200_000), detail.EngineStats.Nodes)
	assert.InDelta(t, 142000, detail.EngineStats.NPS, 0.01)
	assert.InDelta(t, 0.87, detail.EngineStats.TTHitRate, 0.01)
	assert.Equal(t, 340, detail.EngineStats.Score)
	assert.Equal(t, 4, detail.EngineStats.Threads)
	assert.Equal(t, int64(12000), detail.EngineStats.AllocatedTimeMs)
	assert.Equal(t, "exact", detail.EngineStats.MoveType)
}

func TestMakeAIMoveReturnsLastMove(t *testing.T) {
	h := testHandler()
	req := httptest.NewRequest(http.MethodPost, "/api/games", bytes.NewReader(
		[]byte(`{"gameMode":"pvai","blueDifficulty":1}`),
	))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.CreateGame(w, req)
	gameID := decodeResponse(t, w)["gameId"].(string)

	req2 := httptest.NewRequest(http.MethodPost, "/api/games/"+gameID+"/move", bytes.NewReader(
		[]byte(`{"x":7,"y":7}`),
	))
	req2.SetPathValue("id", gameID)
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	h.MakeMove(w2, req2)
	require.Equal(t, http.StatusOK, w2.Code)

	req3 := httptest.NewRequest(http.MethodPost, "/api/games/"+gameID+"/ai-move", nil)
	req3.SetPathValue("id", gameID)
	w3 := httptest.NewRecorder()
	h.MakeAIMove(w3, req3)
	assert.Equal(t, http.StatusOK, w3.Code)

	resp := decodeResponse(t, w3)
	lastMove, ok := resp["lastMove"].(map[string]any)
	require.True(t, ok, "response should contain lastMove")

	assert.Equal(t, 1.0, lastMove["moveNumber"])
	assert.Equal(t, "blue", lastMove["player"])
	assert.NotEmpty(t, lastMove["statline"])

	es, ok := lastMove["engineStats"].(map[string]any)
	require.True(t, ok, "lastMove should contain engineStats")
	assert.NotNil(t, es["depth"])
	assert.NotNil(t, es["nodes"])
}
