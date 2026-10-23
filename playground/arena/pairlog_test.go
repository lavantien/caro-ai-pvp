package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writePairLog(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestParsePairLogHeader(t *testing.T) {
	a, b, geometry, tc, games, ok := parsePairLogHeader("pairlog v1 seats master,master-nobook tc 1 games 20")
	if !ok || a != "master" || b != "master-nobook" || geometry != "square" || tc != 1 || games != 20 {
		t.Fatalf("parsePairLogHeader = %q %q %q %d %d %v", a, b, geometry, tc, games, ok)
	}
	for _, bad := range []string{
		"",
		"pairlog v2 seats a,b tc 0 games 1",
		"pairlog v1 seats a tc 0 games 1",
		"pairlog v1 seats a,b,b tc 0 games 1",
		"pairlog v1 seats a,b tc x games 1",
		"pairlog v1 seats a,b tc 0 games",
		"pairlog v1 seats ,b tc 0 games 1",
	} {
		if _, _, _, _, _, ok := parsePairLogHeader(bad); ok {
			t.Errorf("parsePairLogHeader(%q) ok, want reject", bad)
		}
	}
}

func TestParsePairLogGameLine(t *testing.T) {
	n, winner, moves, ok := parsePairLogGameLine("game 7 winner blue moves 51")
	if !ok || n != 7 || winner != winnerBlue || moves != 51 {
		t.Fatalf("parsePairLogGameLine = %d %q %d %v", n, winner, moves, ok)
	}
	for _, bad := range []string{
		"",
		"game 0 winner red moves 9",
		"game x winner red moves 9",
		"game 3 winner green moves 9",
		"game 3 winner red moves x",
		"game 3 winner red",
		"game 3 red moves 9",
	} {
		if _, _, _, ok := parsePairLogGameLine(bad); ok {
			t.Errorf("parsePairLogGameLine(%q) ok, want reject", bad)
		}
	}
}

func TestLoadPairLogRejectsHeaderMismatch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "p.pairlog")
	writePairLog(t, path, "pairlog v1 seats a,b tc 1 games 4\ngame 1 winner red moves 9\n")
	for name, args := range map[string][4]any{
		"seats":   {"b", "a", 1, 4},
		"tc":      {"a", "b", 2, 4},
		"games":   {"a", "b", 1, 5},
		"tcaseat": {"A", "b", 1, 4},
	} {
		pl, err := loadPairLog(path, args[0].(string), args[1].(string), args[2].(int), args[3].(int), "square")
		if err == nil || !strings.Contains(err.Error(), "header") {
			t.Errorf("loadPairLog with %s mismatch = %v, want header error", name, err)
		}
		if pl.done != 0 {
			t.Errorf("loadPairLog with %s mismatch done = %d, want 0", name, pl.done)
		}
	}
}

func TestLoadPairLogCountsAndRecoversTornTail(t *testing.T) {
	path := filepath.Join(t.TempDir(), "p.pairlog")
	full := "pairlog v1 seats a,b tc 1 games 4\ngame 1 winner red moves 9\ngame 2 winner draw moves 250\ngame 3 winner blue moves 60\n"
	writePairLog(t, path, full)
	pl, err := loadPairLog(path, "a", "b", 1, 4, "square")
	if err != nil {
		t.Fatal(err)
	}
	if pl.done != 3 {
		t.Fatalf("done = %d, want 3", pl.done)
	}
	wantWinners := []string{winnerRed, winnerDraw, winnerBlue}
	for i, w := range wantWinners {
		if pl.winners[i] != w {
			t.Fatalf("winners[%d] = %q, want %q", i, pl.winners[i], w)
		}
	}
	torn := full + "game 4 winner re"
	writePairLog(t, path, torn)
	pl, err = loadPairLog(path, "a", "b", 1, 4, "square")
	if err != nil {
		t.Fatal(err)
	}
	if pl.done != 3 {
		t.Fatalf("torn tail done = %d, want 3", pl.done)
	}
}

func TestLoadPairLogRejectsOutOfOrderAndJunk(t *testing.T) {
	base := "pairlog v1 seats a,b tc 1 games 4\n"
	for name, text := range map[string]string{
		"skip":     base + "game 1 winner red moves 9\ngame 3 winner red moves 9\n",
		"junk":     base + "game 1 winner red moves 9\nmidnight\n",
		"restart":  base + "game 1 winner red moves 9\ngame 1 winner red moves 9\n",
		"complete": base + "game 1 winner red moves 9\ncomplete 1 games\n",
	} {
		path := filepath.Join(t.TempDir(), name+".pairlog")
		writePairLog(t, path, text)
		if _, err := loadPairLog(path, "a", "b", 1, 4, "square"); err == nil {
			t.Errorf("loadPairLog %s: want error", name)
		}
	}
}

func TestLoadPairLogAcceptsCompleteMarker(t *testing.T) {
	path := filepath.Join(t.TempDir(), "p.pairlog")
	writePairLog(t, path, "pairlog v1 seats a,b tc 1 games 1\ngame 1 winner red moves 9\ncomplete 1 games\n")
	pl, err := loadPairLog(path, "a", "b", 1, 1, "square")
	if err != nil {
		t.Fatal(err)
	}
	if pl.done != 1 || !pl.complete {
		t.Fatalf("done = %d complete = %v, want 1 true", pl.done, pl.complete)
	}
}

func TestLoadPairLogMissingFileStartsFresh(t *testing.T) {
	path := filepath.Join(t.TempDir(), "absent.pairlog")
	pl, err := loadPairLog(path, "a", "b", 1, 4, "square")
	if err != nil {
		t.Fatal(err)
	}
	if pl.done != 0 || pl.complete || pl.f != nil {
		t.Fatalf("fresh load = %+v, want zero state", pl)
	}
}

func TestPairLogAppendTruncatesTornTail(t *testing.T) {
	path := filepath.Join(t.TempDir(), "p.pairlog")
	torn := "pairlog v1 seats a,b tc 1 games 2\ngame 1 winner red moves 9\ngame 2 winner bl"
	writePairLog(t, path, torn)
	pl, err := openPairLogForAppend(path, "a", "b", 1, 2, "square")
	if err != nil {
		t.Fatal(err)
	}
	if pl.done != 1 {
		t.Fatalf("done = %d, want 1", pl.done)
	}
	if err := pl.appendGame(2, winnerBlue, 40); err != nil {
		t.Fatal(err)
	}
	if err := pl.appendComplete(); err != nil {
		t.Fatal(err)
	}
	if err := pl.close(); err != nil {
		t.Fatal(err)
	}
	want := "pairlog v1 seats a,b tc 1 games 2\ngame 1 winner red moves 9\ngame 2 winner blue moves 40\ncomplete 2 games\n"
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != want {
		t.Fatalf("recovered pairlog = %q, want %q", data, want)
	}
	if _, err := loadPairLog(path, "a", "b", 1, 2, "square"); err != nil {
		t.Fatalf("reload after recovery: %v", err)
	}
}

func TestPairLogAppendWritesGameAndComplete(t *testing.T) {
	path := filepath.Join(t.TempDir(), "p.pairlog")
	pl, err := openPairLogForAppend(path, "a", "b", 1, 2, "square")
	if err != nil {
		t.Fatal(err)
	}
	if err := pl.appendGame(1, winnerRed, 9); err != nil {
		t.Fatal(err)
	}
	if err := pl.appendGame(2, winnerDraw, 30); err != nil {
		t.Fatal(err)
	}
	if err := pl.appendComplete(); err != nil {
		t.Fatal(err)
	}
	if err := pl.close(); err != nil {
		t.Fatal(err)
	}
	want := "pairlog v1 seats a,b tc 1 games 2\ngame 1 winner red moves 9\ngame 2 winner draw moves 30\ncomplete 2 games\n"
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != want {
		t.Fatalf("pairlog = %q, want %q", data, want)
	}
	back, err := loadPairLog(path, "a", "b", 1, 2, "square")
	if err != nil {
		t.Fatal(err)
	}
	if back.done != 2 || !back.complete {
		t.Fatalf("reload done = %d complete = %v, want 2 true", back.done, back.complete)
	}
}

func TestPairLogCompleteMarkerOnlyAfterFullCount(t *testing.T) {
	path := filepath.Join(t.TempDir(), "p.pairlog")
	writePairLog(t, path, "pairlog v1 seats a,b tc 1 games 4\ncomplete 4 games\n")
	if _, err := loadPairLog(path, "a", "b", 1, 4, "square"); err == nil {
		t.Fatal("complete marker with 0 game lines: want error")
	}
}
