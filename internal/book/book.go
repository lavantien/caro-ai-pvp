package book

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"sort"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

type Book struct {
	MaxPly      int
	Generator   string
	CorpusGames int
	VisitFloor  int
	Created     string
	entries     []byte
}

type metaLine struct {
	Version     int
	MaxPly      int
	Generator   string
	CorpusGames int
	VisitFloor  int
	Created     string
}

func Load(r io.Reader) (*Book, error) {
	br := bufio.NewReaderSize(struct{ io.Reader }{r}, config.BookMetaLineMaxBytes)
	magic, err := br.ReadSlice('\n')
	if err == bufio.ErrBufferFull {
		return nil, fmt.Errorf("book: oversized magic line exceeds %d bytes", config.BookMetaLineMaxBytes)
	}
	if err != nil {
		return nil, fmt.Errorf("book: truncated magic line: %w", err)
	}
	if string(magic) != config.BookMagicLine {
		return nil, fmt.Errorf("book: bad magic line %q", magic)
	}
	rawMeta, err := br.ReadSlice('\n')
	if err == bufio.ErrBufferFull {
		return nil, fmt.Errorf("book: oversized meta line exceeds %d bytes", config.BookMetaLineMaxBytes)
	}
	if err != nil {
		return nil, fmt.Errorf("book: truncated meta line: %w", err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(rawMeta, &fields); err != nil {
		return nil, fmt.Errorf("book: meta line parse: %w", err)
	}
	for _, f := range config.BookMetaFields {
		if _, ok := fields[f]; !ok {
			return nil, fmt.Errorf("book: meta line misses field %q", f)
		}
	}
	if len(fields) != len(config.BookMetaFields) {
		return nil, fmt.Errorf("book: meta line carries a field outside the contract")
	}
	var meta metaLine
	targets := [...]any{&meta.Version, &meta.MaxPly, &meta.Generator, &meta.CorpusGames, &meta.VisitFloor, &meta.Created}
	for i, f := range config.BookMetaFields {
		if err := json.Unmarshal(fields[f], targets[i]); err != nil {
			return nil, fmt.Errorf("book: meta field %q decode: %w", f, err)
		}
	}
	if meta.Version != config.BookFormatVersion {
		return nil, fmt.Errorf("book: meta version %d does not match format version %d", meta.Version, config.BookFormatVersion)
	}
	for _, n := range [3]struct {
		name string
		val  int
	}{
		{config.BookMetaFields[1], meta.MaxPly},
		{config.BookMetaFields[3], meta.CorpusGames},
		{config.BookMetaFields[4], meta.VisitFloor},
	} {
		if n.val < 0 {
			return nil, fmt.Errorf("book: meta field %q = %d is negative", n.name, n.val)
		}
	}
	entries, err := io.ReadAll(br)
	if err != nil {
		return nil, fmt.Errorf("book: entry region read: %w", err)
	}
	if len(entries)%config.BookEntryBytes != 0 {
		return nil, fmt.Errorf("book: entry region %d bytes is not a multiple of %d", len(entries), config.BookEntryBytes)
	}
	bitsPerWord := config.BoardCells / config.BoardWordsPerColor
	bytesPerWord := bitsPerWord / 8
	for i := range len(entries) / config.BookEntryBytes {
		off := i * config.BookEntryBytes
		if i > 0 {
			prev := entries[off-config.BookEntryBytes : off-config.BookEntryBytes+config.BookKeyBytes]
			cur := entries[off : off+config.BookKeyBytes]
			if bytes.Compare(prev, cur) >= 0 {
				return nil, fmt.Errorf("book: entry %d key breaks strict ascending order", i)
			}
		}
		move := binary.LittleEndian.Uint16(entries[off+config.BookKeyBytes:])
		if int(move) >= config.BoardCells {
			return nil, fmt.Errorf("book: entry %d move %d outside the board", i, move)
		}
		word := int(move) / bitsPerWord
		mask := uint64(1) << (uint(move) % uint(bitsPerWord))
		if binary.LittleEndian.Uint64(entries[off+word*bytesPerWord:])&mask != 0 || binary.LittleEndian.Uint64(entries[off+(config.BoardWordsPerColor+word)*bytesPerWord:])&mask != 0 {
			return nil, fmt.Errorf("book: entry %d move %d already occupied in its key", i, move)
		}
		sourceOff := off + config.BookKeyBytes + config.BookMoveBytes + config.BookVisitsBytes
		if source := entries[sourceOff]; source != config.BookSourceMajority && source != config.BookSourceSolver {
			return nil, fmt.Errorf("book: entry %d source %d not a known source", i, source)
		}
		for _, b := range entries[sourceOff+config.BookSourceBytes : off+config.BookEntryBytes] {
			if b != 0 {
				return nil, fmt.Errorf("book: entry %d carries nonzero reserved bytes", i)
			}
		}
	}
	return &Book{
		MaxPly:      meta.MaxPly,
		Generator:   meta.Generator,
		CorpusGames: meta.CorpusGames,
		VisitFloor:  meta.VisitFloor,
		Created:     meta.Created,
		entries:     entries,
	}, nil
}

func (bk *Book) Len() int {
	if bk == nil {
		return 0
	}
	return len(bk.entries) / config.BookEntryBytes
}

func (bk *Book) Probe(board *rules.Board) (rules.Move, bool) {
	if bk == nil || len(bk.entries) == 0 || board == nil {
		return 0, false
	}
	var key [config.BookKeyBytes]byte
	for i, w := range board.Red {
		binary.LittleEndian.PutUint64(key[i*8:], w)
	}
	blueOff := config.BoardWordsPerColor * 8
	for i, w := range board.Blue {
		binary.LittleEndian.PutUint64(key[blueOff+i*8:], w)
	}
	n := len(bk.entries) / config.BookEntryBytes
	idx := sort.Search(n, func(i int) bool {
		off := i * config.BookEntryBytes
		return bytes.Compare(bk.entries[off:off+config.BookKeyBytes], key[:]) >= 0
	})
	if idx == n {
		return 0, false
	}
	off := idx * config.BookEntryBytes
	if !bytes.Equal(bk.entries[off:off+config.BookKeyBytes], key[:]) {
		return 0, false
	}
	return rules.Move(binary.LittleEndian.Uint16(bk.entries[off+config.BookKeyBytes:])), true
}
