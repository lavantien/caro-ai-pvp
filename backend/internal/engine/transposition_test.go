package engine

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTTStoreAndLookup(t *testing.T) {
	tt := NewTranspositionTable(1)
	entry := TTEntry{
		Hash:  0x1234567890ABCDEF,
		Score: 1500,
		Depth: 8,
		MoveX: 5,
		MoveY: 5,
		Flag:  TTExact,
		Age:   0,
	}
	tt.Store(entry)

	got, ok := tt.Lookup(entry.Hash)
	assert.True(t, ok)
	assert.Equal(t, entry.Score, got.Score)
	assert.Equal(t, entry.Depth, got.Depth)
	assert.Equal(t, entry.MoveX, got.MoveX)
	assert.Equal(t, entry.MoveY, got.MoveY)
}

func TestTTMiss(t *testing.T) {
	tt := NewTranspositionTable(1)
	_, ok := tt.Lookup(0xDEADBEEF)
	assert.False(t, ok)
}

func TestTTClear(t *testing.T) {
	tt := NewTranspositionTable(1)
	tt.Store(TTEntry{Hash: 0x1, Score: 100, Depth: 5, Flag: TTExact})
	tt.Clear()
	_, ok := tt.Lookup(0x1)
	assert.False(t, ok)
}

func TestTTConcurrentAccess(t *testing.T) {
	tt := NewTranspositionTable(4)
	var wg sync.WaitGroup
	for i := range 100 {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			tt.Store(TTEntry{Hash: uint64(n), Score: int32(n), Depth: 5, Flag: TTExact})
			tt.Lookup(uint64(n))
		}(i)
	}
	wg.Wait()
}
