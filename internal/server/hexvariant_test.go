package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/engine"
	"github.com/lavantien/caro-ai-pvp/internal/geom"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

func mustHexGeometry(t *testing.T) *geom.Geometry {
	t.Helper()
	g, err := geom.ByName("hex")
	if err != nil {
		t.Fatalf("hex geometry: %v", err)
	}
	return g
}

func hexCell(t *testing.T, name string) rules.Cell {
	t.Helper()
	cell, err := mustHexGeometry(t).ParseCell(name)
	if err != nil {
		t.Fatalf("parse hex cell %s: %v", name, err)
	}
	return rules.Cell(cell)
}

type scriptedHexBot struct {
	g     *geom.Geometry
	moves []string
	next  int
}

func (b *scriptedHexBot) pick() rules.Move {
	if b.next < len(b.moves) {
		name := b.moves[b.next]
		b.next++
		cell, err := b.g.ParseCell(name)
		if err == nil {
			return rules.Move(cell)
		}
	}
	return rules.Move(b.g.Center)
}

func (b *scriptedHexBot) Search(*rules.Board, engine.Deadline) (rules.Move, engine.SearchStats, string) {
	return b.pick(), engine.SearchStats{}, ""
}
func (b *scriptedHexBot) StartPonder(*rules.Board) {}
func (b *scriptedHexBot) StopPonder() (rules.Move, engine.SearchStats, string) {
	return rules.MoveNone, engine.SearchStats{}, ""
}
func (b *scriptedHexBot) Close() {}

func TestCreateRejectsUnknownVariant(t *testing.T) {
	s := newStack(t)
	alice := seedUser(t, s.store, "alice")

	if _, err := s.rm.Create(alice.ID, 0, config.SeriesBO3, nil, "hexagon"); err != ErrUnknownVariant {
		t.Errorf("unknown variant create = %v, want ErrUnknownVariant", err)
	}
	if _, err := s.rm.CreateBotVsBot(&config.TierEasy, "", &config.TierEasy, "", 0, config.SeriesBO3, "dodeca"); err != ErrUnknownVariant {
		t.Errorf("unknown variant bot room = %v, want ErrUnknownVariant", err)
	}
	if _, err := s.rm.Create(alice.ID, 0, config.SeriesBO3, nil, "hex"); err != nil {
		t.Errorf("hex create = %v, want nil", err)
	}
}

func TestHexRoomPlaysHexMovesAndStoresBothSides(t *testing.T) {
	s := newStack(t)
	alice := seedUser(t, s.store, "alice")
	casualDir := t.TempDir()
	s.store.casual = newCasualLogs(casualDir, config.CasualSeriesLogFormat)

	g := mustHexGeometry(t)
	bot := &scriptedHexBot{g: g, moves: []string{"C10", "C11", "C12", "C13"}}
	s.rm.makeSearcher = func(_ string, _ config.Tier) searcher { return bot }

	room, err := s.rm.Create(alice.ID, 0, config.SeriesBO3, &config.TierEasy, "hex")
	if err != nil {
		t.Fatalf("create hex room: %v", err)
	}
	if v := room.Variant(); v != "hex" {
		t.Fatalf("room variant = %q, want hex", v)
	}
	if err := room.Ready(alice.ID); err != nil {
		t.Fatalf("host ready: %v", err)
	}

	human := []string{"P12", "T12", "Q12", "R12", "S12"}
	for i, name := range human {
		waitHumanTurn(t, room, alice.ID)
		if err := room.PlayMove(alice.ID, hexCell(t, name)); err != nil {
			t.Fatalf("human move %s: %v", name, err)
		}
		if i == len(human)-1 {
			break
		}
	}
	deadline := time.Now().Add(20 * time.Second)
	for {
		games, gerr := s.store.MatchHistory(alice.ID)
		if gerr == nil && len(games) == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("hex game never completed: history err %v", gerr)
		}
		time.Sleep(10 * time.Millisecond)
	}
	room.Close()

	games, err := s.store.MatchHistory(alice.ID)
	if err != nil || len(games) != 1 {
		t.Fatalf("match history = %d games err %v, want the finished hex game", len(games), err)
	}
	if games[0].Variant != "hex" {
		t.Errorf("history row variant = %q, want hex", games[0].Variant)
	}
	sr, err := s.store.SeriesByID(room.SeriesID())
	if err != nil {
		t.Fatalf("series row: %v", err)
	}
	if sr.Variant != "hex" {
		t.Errorf("series variant = %q, want hex", sr.Variant)
	}
	moves, err := DecodeMovesForVariant(games[0].Moves, "hex")
	if err != nil {
		t.Fatalf("decode hex blob: %v", err)
	}
	if len(moves) != 9 {
		t.Fatalf("hex game moves = %d, want 9", len(moves))
	}
	if got := string(g.AppendName(nil, int(moves[0]))); got != "P12" {
		t.Errorf("first hex move = %q, want P12", got)
	}
	if _, err := DecodeMoves(games[0].Moves, geom.Default()); err == nil {
		t.Error("hex blob decoded under the square geometry, want rejection")
	}

	casual := readCasualDir(t, casualDir)
	if !strings.Contains(casual, "tc 1+0 bo3 hex outcome") {
		t.Errorf("casual tc line = %q, want the hex variant on it", firstLineContaining(casual, "tc "))
	}
	if !strings.Contains(casual, ", C10,") || !strings.Contains(casual, ", C13,") {
		t.Errorf("casual text missing the bot's hex move names in the m-lines")
	}
}

func waitHumanTurn(t *testing.T, room *Room, userID int64) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for {
		snap := room.gameSnapshot()
		if snap == nil {
			return
		}
		if snap.TurnUserID == userID {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("human turn never came back, turn user %d", snap.TurnUserID)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func readCasualDir(t *testing.T, dir string) string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) == 0 {
		t.Fatalf("casual dir read = %v entries %d", err, len(entries))
	}
	data, err := os.ReadFile(filepath.Join(dir, entries[0].Name()))
	if err != nil {
		t.Fatalf("casual read: %v", err)
	}
	return string(data)
}

func firstLineContaining(text, sub string) string {
	for _, line := range strings.Split(text, "\n") {
		if strings.Contains(line, sub) {
			return line
		}
	}
	return ""
}

func TestDecodeMovesRejectsOffRegionFlats(t *testing.T) {
	g := mustHexGeometry(t)
	if _, err := DecodeMoves([]byte{0x00, 0x00}, g); err == nil {
		t.Error("off-region flat 0 decoded under the hex geometry, want rejection")
	}
	if _, err := DecodeMoves([]byte{0xFF, 0x00}, g); err != nil {
		t.Errorf("hex-region flat 255 rejected: %v", err)
	}
	if _, err := DecodeMoves([]byte{0x00, 0x00}, geom.Default()); err != nil {
		t.Errorf("square flat 0 rejected under the square geometry: %v", err)
	}
}

func TestMovesBlobRoundTripsPerVariant(t *testing.T) {
	g := mustHexGeometry(t)
	squareTop, err := geom.Default().ParseCell("P16")
	if err != nil {
		t.Fatalf("square P16: %v", err)
	}
	hexTop, err := g.ParseCell("W12")
	if err != nil {
		t.Fatalf("hex W12: %v", err)
	}

	sq := EncodeMoves(nil, []rules.Move{rules.Move(squareTop)})
	if _, err := DecodeMoves(sq, geom.Default()); err != nil {
		t.Errorf("square blob under the square geometry: %v", err)
	}
	if _, err := DecodeMoves(sq, g); err != nil {
		t.Errorf("square in-region blob under the hex geometry: %v", err)
	}

	hx := EncodeMoves(nil, []rules.Move{rules.Move(hexTop)})
	if _, err := DecodeMoves(hx, geom.Default()); err == nil {
		t.Error("hex cell decoded under the square geometry, want rejection")
	}
	back, err := DecodeMovesForVariant(hx, "hex")
	if err != nil || len(back) != 1 || back[0] != rules.Move(hexTop) {
		t.Errorf("hex blob round trip = %v err %v, want W23", back, err)
	}
	if _, err := DecodeMovesForVariant(hx, "hexagon"); err == nil {
		t.Error("unknown variant decode succeeded, want rejection")
	}
}

func TestWonByTagCountsHexCellsBeyondTheSquareBound(t *testing.T) {
	g := mustHexGeometry(t)
	b := rules.NewBoardFor(g)
	for _, name := range []string{"N15", "O15", "P15", "Q15"} {
		setStone(t, b, hexCell(t, name), rules.Red)
	}
	for _, name := range []string{"R15", "M17", "C10"} {
		setStone(t, b, hexCell(t, name), rules.Blue)
	}
	for _, name := range []string{"N17", "O17", "P17", "Q17"} {
		setStone(t, b, hexCell(t, name), rules.Red)
	}
	b.Side = rules.Blue

	if tag := WonByTag(b, rules.Red); tag != WonByDoubleFour {
		t.Errorf("hex double-four tag = %q, want %q", tag, WonByDoubleFour)
	}
}

func setStone(t *testing.T, b *rules.Board, cell rules.Cell, c rules.Color) {
	t.Helper()
	w := int(cell) / 64
	m := uint64(1) << uint(int(cell)%64)
	if c == rules.Red {
		b.Red[w] |= m
	} else {
		b.Blue[w] |= m
	}
	b.Full[w] |= m
	b.MoveCount++
}
