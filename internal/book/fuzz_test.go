package book

import (
	"bytes"
	"encoding/binary"
	"strconv"
	"strings"
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

func specSeedMeta() []byte {
	return []byte(`{"version":` + strconv.Itoa(config.BookFormatVersion) + `,"max_ply":16,"generator":"f","corpus_games":` + strconv.Itoa(config.BookCorpusGames) + `,"visit_floor":` + strconv.Itoa(config.BookVisitFloor) + `,"created":"20260101-000000"}` + "\n")
}

func FuzzBookLoad(f *testing.F) {
	var validKey [config.BookKeyBytes]byte
	binary.LittleEndian.PutUint64(validKey[:], 1<<8)
	validEntry := specEntry(validKey, rules.Move(17), uint32(config.BookVisitFloor), config.BookSourceMajority)
	var oobKey [config.BookKeyBytes]byte
	binary.LittleEndian.PutUint64(oobKey[:], 1<<9)
	oobEntry := specEntry(oobKey, rules.Move(config.BoardCells+5), uint32(config.BookVisitFloor), config.BookSourceMajority)
	occupiedEntry := specEntry(validKey, rules.Move(8), uint32(config.BookVisitFloor), config.BookSourceMajority)
	badSourceEntry := specEntry(validKey, rules.Move(17), uint32(config.BookVisitFloor), 7)
	dirtySeeds := make([][]byte, 0, config.BookReservedBytes)
	for i := range config.BookReservedBytes {
		dirty := specEntry(validKey, rules.Move(17), uint32(config.BookVisitFloor), config.BookSourceMajority)
		dirty[config.BookKeyBytes+config.BookMoveBytes+config.BookVisitsBytes+config.BookSourceBytes+i] = 1
		dirtySeeds = append(dirtySeeds, specBook(specMetaLine(config.BookFormatVersion), dirty))
	}
	var highKey [config.BookKeyBytes]byte
	binary.LittleEndian.PutUint64(highKey[:], 1<<10)
	reversed := specBook(specMetaLine(config.BookFormatVersion), specEntry(highKey, 0, uint32(config.BookVisitFloor), config.BookSourceMajority), specEntry(validKey, 0, uint32(config.BookVisitFloor), config.BookSourceMajority))
	partial := make([]byte, config.BookEntryBytes+7)
	copy(partial, validEntry)
	fatMeta := []byte(config.BookMagicLine + `{"version":` + strconv.Itoa(config.BookFormatVersion) + `,"generator":"` + strings.Repeat("A", config.BookMetaLineMaxBytes) + `"}` + "\n")
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
		specBook(specSeedMeta(), validEntry),
		specBook(specMetaLine(config.BookFormatVersion), validEntry, oobEntry),
		specBook(specMetaLine(config.BookFormatVersion), occupiedEntry),
		specBook(specMetaLine(config.BookFormatVersion), badSourceEntry),
		reversed,
	}
	seeds = append(seeds, dirtySeeds...)
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
		if err2 != nil || again == nil || again.Len() != bk.Len() {
			t.Fatalf("Load nondeterministic: second pass err %v len %d want len %d", err2, again.Len(), bk.Len())
		}
		metaEnd := bytes.IndexByte(data[len(config.BookMagicLine):], '\n')
		if metaEnd < 0 {
			t.Fatal("loaded book lacks a meta newline")
		}
		region := data[len(config.BookMagicLine)+metaEnd+1:]
		if len(region) != bk.Len()*config.BookEntryBytes {
			t.Fatalf("entry region %d bytes disagrees with %d entries", len(region), bk.Len())
		}
		if bk.Len() == 0 {
			return
		}
		var probeKey [config.BookKeyBytes]byte
		copy(probeKey[:], region[:config.BookKeyBytes])
		stored := rules.Move(binary.LittleEndian.Uint16(region[config.BookKeyBytes:]))
		oracle := rules.NewBoard()
		for i := range oracle.Red {
			oracle.Red[i] = binary.LittleEndian.Uint64(probeKey[i*8:])
		}
		for i := range oracle.Blue {
			oracle.Blue[i] = binary.LittleEndian.Uint64(probeKey[(config.BoardWordsPerColor+i)*8:])
		}
		got, ok := bk.Probe(oracle)
		if !ok || got != stored {
			t.Fatalf("probe oracle: entry 0 stores move %d, probe = (%d, %t)", uint16(stored), uint16(got), ok)
		}
		b := rules.NewBoard()
		for i := 3; i < len(data) && b.MoveCount < 64; i++ {
			c := rules.Cell(data[i])
			if b.IsLegal(c) {
				b.Make(c)
			}
		}
		mv, hit := bk.Probe(b)
		mv2, hit2 := bk.Probe(b)
		if mv != mv2 || hit != hit2 {
			t.Fatalf("probe nondeterministic: (%d,%t) then (%d,%t)", uint16(mv), hit, uint16(mv2), hit2)
		}
	})
}
