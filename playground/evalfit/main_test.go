package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestSplitDataPathsKeepsGamesOffTheDBLoader(t *testing.T) {
	dbPaths, gamePaths := splitDataPaths("a.db,b.db", "g1.txt, ,g2.txt")
	if strings.Join(dbPaths, ",") != "a.db,b.db" {
		t.Fatalf("db paths = %v, want only the dbs", dbPaths)
	}
	if strings.Join(gamePaths, ",") != "g1.txt,g2.txt" {
		t.Fatalf("game paths = %v, want the games with blanks dropped", gamePaths)
	}
	path := filepath.Join(t.TempDir(), "games.txt")
	writeGamesFixture(t, path, gamesFixtureHeader)
	if _, err := loadDBGames(path, nil); err == nil || !strings.Contains(err.Error(), "not a database") {
		t.Fatalf("err = %v, want the sqlite loader to refuse a games file", err)
	}
}
