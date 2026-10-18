package server

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/book"
	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/engine"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

type bookSearcher struct {
	inner  searcher
	book   *book.Book
	maxPly int
	cores  int
}

func (s *bookSearcher) Search(b *rules.Board, dl engine.Deadline) (rules.Move, engine.SearchStats, string) {
	if b.MoveCount < s.maxPly {
		start := time.Now()
		if mv, ok := s.book.Probe(b); ok {
			return mv, bookStats(s.cores, dl, start, mv), config.BotLogTagBook
		}
	}
	return s.inner.Search(b, dl)
}

func (s *bookSearcher) Close() { s.inner.Close() }

func (s *bookSearcher) StartPonder(b *rules.Board) { s.inner.StartPonder(b) }

func (s *bookSearcher) StopPonder() (rules.Move, engine.SearchStats, string) {
	return s.inner.StopPonder()
}

func bookStats(cores int, dl engine.Deadline, start time.Time, mv rules.Move) engine.SearchStats {
	var st engine.SearchStats
	st.Threads = cores
	st.ElapsedNs = int64(time.Since(start))
	st.PVLen = 1
	st.PV[0] = mv
	if bg, ok := dl.(engine.Budgeter); ok {
		st.AllocNs = int64(bg.Budget())
	}
	return st
}

func loadBotBook() *book.Book {
	f, err := os.Open(config.BookArtifactPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		panic(fmt.Sprintf("server: opening the book artifact at %s: %v", config.BookArtifactPath, err))
	}
	defer func() { _ = f.Close() }()
	bk, err := book.Load(f)
	if err != nil {
		panic(fmt.Sprintf("server: book artifact at %s broke the CAROBOOK1 contract: %v", config.BookArtifactPath, err))
	}
	return bk
}
