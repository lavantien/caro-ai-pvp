package api

import (
	"log/slog"
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewInMemoryStoreEmpty(t *testing.T) {
	s := NewInMemoryStore()
	assert.Equal(t, 0, s.Count())
	assert.Equal(t, 0, s.ActiveGameCount())
}

func TestStoreSetGetDelete(t *testing.T) {
	s := NewInMemoryStore()
	session := newTestSession()
	s.Set("game1", session)

	got, ok := s.Get("game1")
	assert.True(t, ok)
	assert.Same(t, session, got)

	_, ok = s.Get("nonexistent")
	assert.False(t, ok)

	s.Delete("game1")
	_, ok = s.Get("game1")
	assert.False(t, ok)
	assert.Equal(t, 0, s.Count())
}

func TestStoreActiveGameCount(t *testing.T) {
	s := NewInMemoryStore()
	s1 := newTestSession()
	s2 := newTestSession()
	s.Set("g1", s1)
	s.Set("g2", s2)
	assert.Equal(t, 2, s.ActiveGameCount())
}

func TestStoreCleanupAll(t *testing.T) {
	s := NewInMemoryStore()
	s.Set("g1", newTestSession())
	s.Set("g2", newTestSession())
	removed := s.CleanupAll()
	assert.Equal(t, 2, removed)
	assert.Equal(t, 0, s.Count())
}

func TestStoreConcurrentAccess(t *testing.T) {
	s := NewInMemoryStore()
	var wg sync.WaitGroup
	for i := range 100 {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			id := string(rune('a' + n%26))
			session := NewGameSession(
				"rapid", 300000, 2,
				0, nil, nil,
				slog.New(slog.NewTextHandler(os.Stderr, nil)),
				func() int { return 1 },
			)
			s.Set(id, session)
			_, _ = s.Get(id)
			if n%3 == 0 {
				s.Delete(id)
			}
		}(i)
	}
	wg.Wait()
}
