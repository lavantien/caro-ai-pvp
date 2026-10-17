package book

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

func FuzzBookLoad(f *testing.F) {
	var validKey [config.BookKeyBytes]byte
	binary.LittleEndian.PutUint64(validKey[:], 1<<8)
	validEntry := specEntry(validKey, rules.Move(17), uint32(config.BookVisitFloor), config.BookSourceMajority)
	var oobKey [config.BookKeyBytes]byte
	binary.LittleEndian.PutUint64(oobKey[:], 1<<9)
	oobEntry := specEntry(oobKey, rules.Move(config.BoardCells+5), uint32(config.BookVisitFloor), config.BookSourceMajority)
	var highKey [config.BookKeyBytes]byte
	binary.LittleEndian.PutUint64(highKey[:], 1<<10)
	reversed := specBook(specMetaLine(config.BookFormatVersion), specEntry(highKey, 0, uint32(config.BookVisitFloor), config.BookSourceMajority), specEntry(validKey, 0, uint32(config.BookVisitFloor), config.BookSourceMajority))
	partial := make([]byte, config.BookEntryBytes+7)
	copy(partial, validEntry)
	fatMeta := []byte(config.BookMagicLine + `{"version":1,"generator":"` + strings.Repeat("A", config.BookMetaLineMaxBytes) + `"}` + "\n")
	seeds := [][]byte{
		nil,
		[]byte(config.BookFileMagic),
		[]byte("CAROBOOK2\n"),
		[]byte(config.BookMagicLine + "{nope\n"),
		fatMeta,
		specBook(specMetaLine(config.BookFormatVersion + 1)),
		specBook(nil),
		specBook(specMetaLine(config.BookFormatVersion), partial),
		specBook(specMetaLine(config.BookFormatVersion), validEntry),
		specBook(specMetaLine(config.BookFormatVersion), validEntry, oobEntry),
		reversed,
		[]byte(config.BookMagicLine + `{"version":1,"max_ply":16,"generator":"f","corpus_games":1,"visit_floor":1,"created":"20260101-000000"}` + "\n" + string(validEntry)),
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > config.FuzzInputMaxBytes {
			return
		}
		bk, err := Load(bytes.NewReader(data))
		if err != nil {
			if bk != nil {
				t.Fatalf("Load returned a book alongside error %v", err)
			}
			return
		}
		if bk == nil {
			t.Fatal("Load returned nil book with nil error")
		}
		again, err2 := Load(bytes.NewReader(data))
		if err2 != nil || again == nil {
			t.Fatalf("Load nondeterministic: second pass err %v", err2)
		}
		if again.Len() != bk.Len() {
			t.Fatalf("Load nondeterministic: len %d then %d", bk.Len(), again.Len())
		}
		b := rules.NewBoard()
		for i := 3; i < len(data) && b.MoveCount < 64; i++ {
			c := rules.Cell(data[i])
			if b.IsLegal(c) {
				b.Make(c)
			}
		}
		mv, ok := bk.Probe(b)
		if ok && int(mv) >= config.BoardCells {
			t.Fatalf("probe returned move %d outside the board", uint16(mv))
		}
		mv2, ok2 := bk.Probe(b)
		if mv != mv2 || ok != ok2 {
			t.Fatalf("probe nondeterministic: (%d,%t) then (%d,%t)", uint16(mv), ok, uint16(mv2), ok2)
		}
		if _, ok := bk.Probe(rules.NewBoard()); ok && bk.Len() == 0 {
			t.Fatal("empty book probed a hit on the fresh board")
		}
	})
}
