package server

import (
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
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

func (s *bookSearcher) StartPonder(b *rules.Board) {
	if b.MoveCount < s.maxPly {
		if _, ok := s.book.Probe(b); ok {
			return
		}
	}
	s.inner.StartPonder(b)
}

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

var executablePath = os.Executable

var missingBookWarn = new(sync.Once)

func loadBotBook() *book.Book {
	f, tried, searchRoot := openBookArtifact()
	if f == nil {
		missingBookWarn.Do(func() {
			detail := strings.Join(tried, "; ")
			if searchRoot != "" {
				detail = detail + "; executable tree " + searchRoot
			}
			log.Printf("server: book artifact missing, tried %s: every book tier now plays without a book", detail)
		})
		return nil
	}
	defer func() { _ = f.Close() }()
	bk, err := book.Load(f)
	if err != nil {
		panic(fmt.Sprintf("server: book artifact at %s broke the CAROBOOK1 contract: %v", f.Name(), err))
	}
	return bk
}

func openBookArtifact() (*os.File, []string, string) {
	candidates, searchRoot := bookArtifactCandidates()
	tried := make([]string, 0, len(candidates))
	for _, p := range candidates {
		if abs, err := filepath.Abs(p); err == nil {
			p = abs
		}
		tried = append(tried, p)
		f, err := os.Open(p)
		if err == nil {
			return f, tried, searchRoot
		}
		if !errors.Is(err, fs.ErrNotExist) {
			panic(fmt.Sprintf("server: opening the book artifact at %s: %v", p, err))
		}
	}
	return nil, tried, searchRoot
}

func bookArtifactCandidates() ([]string, string) {
	configured := config.BookArtifactPath
	if filepath.IsAbs(configured) {
		return []string{configured}, ""
	}
	exe, err := executablePath()
	if err != nil {
		return []string{configured}, ""
	}
	root := filepath.Dir(exe)
	if isGoBuildTempTree(root) {
		return []string{configured}, ""
	}
	candidates := []string{configured}
	if anchored, ok := ancestorBookPath(root); ok {
		candidates = append(candidates, anchored)
	}
	return candidates, root
}

func isGoBuildTempTree(dir string) bool {
	for {
		if strings.HasPrefix(filepath.Base(dir), config.GoBuildTempDirPrefix) {
			return true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return false
		}
		dir = parent
	}
}

func ancestorBookPath(dir string) (string, bool) {
	for {
		cand := filepath.Join(dir, config.BookArtifactPath)
		if _, err := os.Stat(cand); err == nil {
			return cand, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}
