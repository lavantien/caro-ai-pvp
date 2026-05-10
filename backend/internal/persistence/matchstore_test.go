package persistence

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
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

func TestNewMatchStoreInvalidPath(t *testing.T) {
	_, err := NewMatchStore("/dev/null/impossible/path/test.db")
	assert.Error(t, err)
}

func TestMatchStoreGetMovesEmpty(t *testing.T) {
	dir := t.TempDir()
	svc, err := NewMatchStore(filepath.Join(dir, "test.db"))
	require.NoError(t, err)
	defer svc.Close()

	svc.CreateGame(GameRecord{ID: "g1", GameMode: "pvp", TimeControl: "1+0", RedType: "human", BlueType: "human"})

	moves, err := svc.GetMoves("g1")
	require.NoError(t, err)
	assert.Empty(t, moves)
}

func TestMatchStoreMigrationAddsColumns(t *testing.T) {
	dir := t.TempDir()
	svc, err := NewMatchStore(filepath.Join(dir, "test.db"))
	require.NoError(t, err)
	defer svc.Close()

	svc.CreateGame(GameRecord{ID: "g1", GameMode: "aivai", TimeControl: "3+2", RedType: "bot", BlueType: "bot"})

	diff := 3
	svc.RecordMove(MoveRecord{
		GameID: "g1", MoveNumber: 1, Player: "red", PosX: 8, PosY: 8, IsBot: true, Difficulty: &diff,
	})

	moves, err := svc.GetMoves("g1")
	require.NoError(t, err)
	require.Len(t, moves, 1)
	assert.Nil(t, moves[0].MasterPct)
	assert.Nil(t, moves[0].SlaveDepth)
	assert.Nil(t, moves[0].SlaveNodes)
	assert.Nil(t, moves[0].PonderDepth)
	assert.Nil(t, moves[0].PonderNodes)
}

func TestMatchStoreRecordFutureStats(t *testing.T) {
	dir := t.TempDir()
	svc, err := NewMatchStore(filepath.Join(dir, "test.db"))
	require.NoError(t, err)
	defer svc.Close()

	svc.CreateGame(GameRecord{ID: "g1", GameMode: "aivai", TimeControl: "3+2", RedType: "bot", BlueType: "bot"})

	diff := 5
	masterPct := 87.5
	slaveDepth := 10
	slaveNodes := int64(500000)
	ponderDepth := 8
	ponderNodes := int64(300000)

	svc.RecordMove(MoveRecord{
		GameID: "g1", MoveNumber: 1, Player: "red", PosX: 8, PosY: 8,
		IsBot: true, Difficulty: &diff,
		MasterPct: &masterPct, SlaveDepth: &slaveDepth, SlaveNodes: &slaveNodes,
		PonderDepth: &ponderDepth, PonderNodes: &ponderNodes,
	})

	moves, err := svc.GetMoves("g1")
	require.NoError(t, err)
	require.Len(t, moves, 1)
	require.NotNil(t, moves[0].MasterPct)
	assert.InDelta(t, 87.5, *moves[0].MasterPct, 0.01)
	require.NotNil(t, moves[0].SlaveDepth)
	assert.Equal(t, 10, *moves[0].SlaveDepth)
	require.NotNil(t, moves[0].SlaveNodes)
	assert.Equal(t, int64(500000), *moves[0].SlaveNodes)
	require.NotNil(t, moves[0].PonderDepth)
	assert.Equal(t, 8, *moves[0].PonderDepth)
	require.NotNil(t, moves[0].PonderNodes)
	assert.Equal(t, int64(300000), *moves[0].PonderNodes)
}

func TestMatchStoreMigrationFromOldSchema(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	db, err := sql.Open("sqlite3", dbPath)
	require.NoError(t, err)
	oldSchema := `
	CREATE TABLE games (
	    id TEXT PRIMARY KEY, game_mode TEXT NOT NULL, time_control TEXT NOT NULL,
	    red_type TEXT NOT NULL, blue_type TEXT NOT NULL, red_difficulty INTEGER,
	    blue_difficulty INTEGER, winner TEXT NOT NULL DEFAULT 'none',
	    move_count INTEGER NOT NULL DEFAULT 0, created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	    completed_at DATETIME
	);
	CREATE TABLE moves (
	    id INTEGER PRIMARY KEY AUTOINCREMENT, game_id TEXT NOT NULL REFERENCES games(id),
	    move_number INTEGER NOT NULL, player TEXT NOT NULL, pos_x INTEGER NOT NULL,
	    pos_y INTEGER NOT NULL, is_bot INTEGER NOT NULL DEFAULT 0, difficulty INTEGER,
	    think_time_ms INTEGER, remaining_time_ms INTEGER, search_depth INTEGER,
	    nodes_searched INTEGER, nps REAL, tt_hit_rate REAL, search_score INTEGER,
	    threads_used INTEGER, allocated_time_ms INTEGER, move_type TEXT,
	    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`
	_, err = db.Exec(oldSchema)
	require.NoError(t, err)
	db.Close()

	svc, err := NewMatchStore(dbPath)
	require.NoError(t, err)
	defer svc.Close()

	svc.CreateGame(GameRecord{ID: "g1", GameMode: "pvp", TimeControl: "1+0", RedType: "human", BlueType: "human"})

	moves, err := svc.GetMoves("g1")
	require.NoError(t, err)
	assert.Empty(t, moves)
}
