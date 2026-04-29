package persistence

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMatchStoreCreateAndRetrieveGame(t *testing.T) {
	dir := t.TempDir()
	svc, err := NewMatchStore(filepath.Join(dir, "test.db"))
	require.NoError(t, err)
	defer svc.Close()

	diff := 5
	game := GameRecord{
		ID:             "abc123",
		GameMode:       "aivai",
		TimeControl:    "3+2",
		RedType:        "bot",
		BlueType:       "bot",
		RedDifficulty:  &diff,
		BlueDifficulty: &diff,
	}
	require.NoError(t, svc.CreateGame(game))

	got, err := svc.GetGame("abc123")
	require.NoError(t, err)
	assert.Equal(t, "abc123", got.ID)
	assert.Equal(t, "aivai", got.GameMode)
	assert.Equal(t, "3+2", got.TimeControl)
	assert.Equal(t, "bot", got.RedType)
	assert.Equal(t, "bot", got.BlueType)
	require.NotNil(t, got.RedDifficulty)
	assert.Equal(t, 5, *got.RedDifficulty)
	assert.Equal(t, "none", got.Winner)
	assert.Nil(t, got.CompletedAt)
}

func TestMatchStoreRecordMoves(t *testing.T) {
	dir := t.TempDir()
	svc, err := NewMatchStore(filepath.Join(dir, "test.db"))
	require.NoError(t, err)
	defer svc.Close()

	diff := 3
	svc.CreateGame(GameRecord{ID: "g1", GameMode: "pvai", TimeControl: "7+5", RedType: "human", BlueType: "bot", BlueDifficulty: &diff})

	thinkMs := int64(1200)
	remMs := int64(415000)
	nodes := int64(54321)
	nps := 45267.5
	hitRate := 0.35
	score := 42
	threads := 4
	allocMs := int64(2000)
	mt := "exact"

	svc.RecordMove(MoveRecord{
		GameID: "g1", MoveNumber: 1, Player: "red", PosX: 8, PosY: 8,
		IsBot: false,
	})
	svc.RecordMove(MoveRecord{
		GameID: "g1", MoveNumber: 2, Player: "blue", PosX: 7, PosY: 7,
		IsBot: true, Difficulty: &diff,
		ThinkTimeMs: &thinkMs, RemainingTimeMs: &remMs,
		SearchDepth: &score, NodesSearched: &nodes, NPS: &nps,
		TTHitRate: &hitRate, SearchScore: &score, ThreadsUsed: &threads,
		AllocatedTimeMs: &allocMs, MoveType: &mt,
	})

	moves, err := svc.GetMoves("g1")
	require.NoError(t, err)
	require.Len(t, moves, 2)

	assert.Equal(t, 1, moves[0].MoveNumber)
	assert.Equal(t, "red", moves[0].Player)
	assert.Equal(t, 8, moves[0].PosX)
	assert.False(t, moves[0].IsBot)
	assert.Nil(t, moves[0].SearchDepth)

	assert.Equal(t, 2, moves[1].MoveNumber)
	assert.Equal(t, "blue", moves[1].Player)
	assert.True(t, moves[1].IsBot)
	require.NotNil(t, moves[1].SearchDepth)
	assert.Equal(t, 42, *moves[1].SearchDepth)
	require.NotNil(t, moves[1].NodesSearched)
	assert.Equal(t, int64(54321), *moves[1].NodesSearched)
	require.NotNil(t, moves[1].NPS)
	assert.InDelta(t, 45267.5, *moves[1].NPS, 0.01)
	require.NotNil(t, moves[1].TTHitRate)
	assert.InDelta(t, 0.35, *moves[1].TTHitRate, 0.01)
}

func TestMatchStoreCompleteGame(t *testing.T) {
	dir := t.TempDir()
	svc, err := NewMatchStore(filepath.Join(dir, "test.db"))
	require.NoError(t, err)
	defer svc.Close()

	svc.CreateGame(GameRecord{ID: "g2", GameMode: "pvp", TimeControl: "1+0", RedType: "human", BlueType: "human"})

	require.NoError(t, svc.CompleteGame("g2", "red", 27))

	got, err := svc.GetGame("g2")
	require.NoError(t, err)
	assert.Equal(t, "red", got.Winner)
	assert.Equal(t, 27, got.MoveCount)
	assert.NotNil(t, got.CompletedAt)
}

func TestMatchStoreGetGameNotFound(t *testing.T) {
	dir := t.TempDir()
	svc, err := NewMatchStore(filepath.Join(dir, "test.db"))
	require.NoError(t, err)
	defer svc.Close()

	_, err = svc.GetGame("nonexistent")
	assert.Error(t, err)
}

func TestMatchStoreCloseIdempotent(t *testing.T) {
	dir := t.TempDir()
	svc, err := NewMatchStore(filepath.Join(dir, "test.db"))
	require.NoError(t, err)
	svc.Close()
	svc.Close()
}

func TestMatchStoreDirectoryCreation(t *testing.T) {
	dir := t.TempDir()
	svc, err := NewMatchStore(filepath.Join(dir, "sub", "dir", "test.db"))
	require.NoError(t, err)
	svc.Close()
}
