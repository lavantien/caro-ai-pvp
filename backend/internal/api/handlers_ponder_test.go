package api

import (
	"bytes"
	"caro-ai-pvp/internal/domain"
	"caro-ai-pvp/internal/persistence"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newPonderHitFixture(t *testing.T) (*Handler, *persistence.MatchStore, *GameSession, string) {
	t.Helper()
	dir := t.TempDir()
	ms, err := persistence.NewMatchStore(filepath.Join(dir, "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { ms.Close() })

	store := NewInMemoryStore()
	h := NewHandler(store, ms, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/games", bytes.NewReader(
		[]byte(`{"gameMode":"pvai","timeControl":"1+0","blueDifficulty":5}`),
	))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.CreateGame(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	gameID := decodeResponse(t, w)["gameId"].(string)

	s, ok := store.Get(gameID)
	require.True(t, ok)
	s.ponderTimeCapMs = 300
	return h, ms, s, gameID
}

func postGameAction(t *testing.T, h *Handler, gameID, action, body string) *httptest.ResponseRecorder {
	t.Helper()
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(http.MethodPost, "/api/games/"+gameID+"/"+action, nil)
	} else {
		req = httptest.NewRequest(http.MethodPost, "/api/games/"+gameID+"/"+action, bytes.NewReader([]byte(body)))
		req.Header.Set("Content-Type", "application/json")
	}
	req.SetPathValue("id", gameID)
	w := httptest.NewRecorder()
	switch action {
	case "move":
		h.MakeMove(w, req)
	case "ai-move":
		h.MakeAIMove(w, req)
	default:
		t.Fatalf("unknown action %q", action)
	}
	return w
}

func lastMoveOf(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	last, ok := decodeResponse(t, w)["lastMove"].(map[string]any)
	require.True(t, ok, "response must carry lastMove")
	return last
}

// stagePonderHit drives the game to a staged ponder hit for blue: blue's
// searched move starts the ponder, the ponder completes, and the human
// plays the exact predicted reply.
func stagePonderHit(t *testing.T, h *Handler, s *GameSession, gameID string) {
	t.Helper()
	w := postGameAction(t, h, gameID, "move", `{"x":7,"y":7}`)
	require.Equal(t, http.StatusOK, w.Code)

	playSearchedAIMove(t, s, domain.PlayerBlue)
	require.NotNil(t, s.activePonder)
	pred := s.activePonder.predictedReply
	require.Eventually(t, func() bool { return !s.blueAI.PonderActive() },
		2*time.Second, 10*time.Millisecond)

	w = postGameAction(t, h, gameID, "move", `{"x":`+strconv.Itoa(pred.X)+`,"y":`+strconv.Itoa(pred.Y)+`}`)
	require.Equal(t, http.StatusOK, w.Code)
	require.NotNil(t, s.pendingPonder, "playing the predicted reply must stage a hit")
}

func moveBody(x, y int) string {
	return `{"x":` + strconv.Itoa(x) + `,"y":` + strconv.Itoa(y) + `}`
}

func TestMakeAIMovePonderHit(t *testing.T) {
	h, ms, s, gameID := newPonderHitFixture(t)
	stagePonderHit(t, h, s, gameID)

	start := time.Now()
	w := postGameAction(t, h, gameID, "ai-move", "")
	require.Equal(t, http.StatusOK, w.Code)
	require.Less(t, time.Since(start), 2*time.Second, "a ponder hit must move near-instantly")

	last := lastMoveOf(t, w)
	engineStats := last["engineStats"].(map[string]any)
	assert.Equal(t, "ponder-hit", engineStats["moveType"])
	assert.Contains(t, last["statline"], "[PONDER]")

	moves, err := ms.GetMoves(gameID)
	require.NoError(t, err)
	found := false
	for _, m := range moves {
		if m.IsBot && m.PonderDepth != nil {
			found = true
			require.NotNil(t, m.PonderNodes)
			require.NotNil(t, m.MoveType)
			assert.Equal(t, "ponder-hit", *m.MoveType)
		}
	}
	assert.True(t, found, "the ponder hit must persist ponder stats")
}

func TestMakeAIMovePonderMissFallsBack(t *testing.T) {
	h, ms, s, gameID := newPonderHitFixture(t)

	w := postGameAction(t, h, gameID, "move", `{"x":7,"y":7}`)
	require.Equal(t, http.StatusOK, w.Code)
	playSearchedAIMove(t, s, domain.PlayerBlue)
	require.NotNil(t, s.activePonder)
	pred := s.activePonder.predictedReply
	alt := legalAlternativeReply(t, s.game.Board, pred)

	// Shrink blue's clock so the fallback normal search stays fast.
	s.mu.Lock()
	s.blueTimeMs = 1500
	s.mu.Unlock()

	w = postGameAction(t, h, gameID, "move", moveBody(alt.X, alt.Y))
	require.Equal(t, http.StatusOK, w.Code)
	assert.Nil(t, s.pendingPonder, "a different reply is a miss")

	w = postGameAction(t, h, gameID, "ai-move", "")
	require.Equal(t, http.StatusOK, w.Code)
	last := lastMoveOf(t, w)
	engineStats := last["engineStats"].(map[string]any)
	assert.NotEqual(t, "ponder-hit", engineStats["moveType"])
	assert.NotContains(t, last["statline"], "[PONDER]")

	moves, err := ms.GetMoves(gameID)
	require.NoError(t, err)
	for _, m := range moves {
		assert.Nil(t, m.PonderDepth, "no ponder columns without a hit")
	}
}

func TestMakeAIMovePonderHitUndoFallback(t *testing.T) {
	h, _, s, gameID := newPonderHitFixture(t)
	stagePonderHit(t, h, s, gameID)

	req := httptest.NewRequest(http.MethodPost, "/api/games/"+gameID+"/undo", nil)
	req.SetPathValue("id", gameID)
	w := httptest.NewRecorder()
	h.UndoMove(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Nil(t, s.pendingPonder, "undo must drop the staged hit")

	s.mu.Lock()
	s.blueTimeMs = 1500
	s.mu.Unlock()

	w = postGameAction(t, h, gameID, "move", `{"x":7,"y":10}`)
	require.Equal(t, http.StatusOK, w.Code)

	w = postGameAction(t, h, gameID, "ai-move", "")
	require.Equal(t, http.StatusOK, w.Code)
	last := lastMoveOf(t, w)
	engineStats := last["engineStats"].(map[string]any)
	assert.NotEqual(t, "ponder-hit", engineStats["moveType"], "post-undo move must take the normal path")
}
