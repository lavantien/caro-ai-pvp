package api

import (
	"sync"
	"time"
)

type InMemoryStore struct {
	mu    sync.RWMutex
	games map[string]*GameSession
}

func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{games: make(map[string]*GameSession)}
}

func (s *InMemoryStore) Set(id string, session *GameSession) {
	s.mu.Lock()
	s.games[id] = session
	s.mu.Unlock()
}

func (s *InMemoryStore) Get(id string) (*GameSession, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	g, ok := s.games[id]
	return g, ok
}

func (s *InMemoryStore) Delete(id string) {
	s.mu.Lock()
	if g, ok := s.games[id]; ok {
		g.DisposeAI()
		delete(s.games, id)
	}
	s.mu.Unlock()
}

func (s *InMemoryStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.games)
}

func (s *InMemoryStore) ActiveGameCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	count := 0
	for _, g := range s.games {
		if !g.IsGameOver() {
			count++
		}
	}
	return count
}

func (s *InMemoryStore) CleanupCompleted() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	removed := 0
	now := time.Now()
	for id, g := range s.games {
		if g.IsGameOver() || now.Sub(g.LastActivityAt()) > 5*time.Minute {
			g.DisposeAI()
			delete(s.games, id)
			removed++
		}
	}
	return removed
}

func (s *InMemoryStore) CleanupAll() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	count := len(s.games)
	for id, g := range s.games {
		g.DisposeAI()
		delete(s.games, id)
	}
	return count
}
