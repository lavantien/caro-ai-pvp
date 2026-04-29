package persistence

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewGameLogService(t *testing.T) {
	dir := t.TempDir()
	svc, err := NewGameLogService(filepath.Join(dir, "test.db"))
	require.NoError(t, err)
	defer svc.Close()

	assert.NotNil(t, svc)
}

func TestLogEventAndQuery(t *testing.T) {
	dir := t.TempDir()
	svc, err := NewGameLogService(filepath.Join(dir, "test.db"))
	require.NoError(t, err)
	defer svc.Close()

	err = svc.LogEvent("game-123", "move", `{"x":7,"y":7}`)
	require.NoError(t, err)

	err = svc.LogEvent("game-123", "move", `{"x":8,"y":8}`)
	require.NoError(t, err)

	err = svc.LogEvent("game-456", "move", `{"x":3,"y":3}`)
	require.NoError(t, err)

	events, err := svc.QueryByGameID("game-123")
	require.NoError(t, err)
	assert.Len(t, events, 2)
	assert.Equal(t, "move", events[0].EventType)
	assert.Equal(t, `{"x":7,"y":7}`, events[0].Data)

	events2, err := svc.QueryByGameID("game-456")
	require.NoError(t, err)
	assert.Len(t, events2, 1)
}

func TestQueryNonexistentGame(t *testing.T) {
	dir := t.TempDir()
	svc, err := NewGameLogService(filepath.Join(dir, "test.db"))
	require.NoError(t, err)
	defer svc.Close()

	events, err := svc.QueryByGameID("nonexistent")
	require.NoError(t, err)
	assert.Empty(t, events)
}

func TestCloseIdempotent(t *testing.T) {
	dir := t.TempDir()
	svc, err := NewGameLogService(filepath.Join(dir, "test.db"))
	require.NoError(t, err)
	svc.Close()
	svc.Close() // should not panic
}

func TestFTS5Search(t *testing.T) {
	dir := t.TempDir()
	svc, err := NewGameLogService(filepath.Join(dir, "test.db"))
	require.NoError(t, err)
	defer svc.Close()

	err = svc.LogEvent("game-1", "game_over", `{"winner":"red","reason":"five_in_a_row"}`)
	require.NoError(t, err)

	results, err := svc.Search("five_in_a_row")
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "game-1", results[0].GameID)
}

func TestNewGameLogServiceCreatesDir(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "subdir", "nested", "test.db")
	svc, err := NewGameLogService(dbPath)
	require.NoError(t, err)
	defer svc.Close()

	_, err = os.Stat(dbPath)
	assert.NoError(t, err)
}
