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

func TestTTDeepEntryNotTrampledByShallow(t *testing.T) {
	tt := NewTranspositionTable(1)
	hash := uint64(0xDEADBEEF)
	tt.Store(TTEntry{Hash: hash, Score: 9000, Depth: 10, MoveX: 5, MoveY: 5, Flag: TTExact, Age: 0})
	tt.Store(TTEntry{Hash: hash, Score: 100, Depth: 2, MoveX: 3, MoveY: 3, Flag: TTExact, Age: 0})

	got, ok := tt.Lookup(hash)
	assert.True(t, ok)
	assert.Equal(t, int32(9000), got.Score, "deep entry should survive shallow store")
	assert.Equal(t, uint8(10), got.Depth)
	assert.Equal(t, int8(5), got.MoveX)
}

func TestTTSameHashExactReplacesUpperAtSameDepth(t *testing.T) {
	tt := NewTranspositionTable(1)
	hash := uint64(0xCAFEBABE)
	tt.Store(TTEntry{Hash: hash, Score: 500, Depth: 5, Flag: TTUpperBound, Age: 0})
	tt.Store(TTEntry{Hash: hash, Score: 800, Depth: 5, Flag: TTExact, Age: 0})

	got, ok := tt.Lookup(hash)
	assert.True(t, ok)
	assert.Equal(t, int32(800), got.Score, "exact at same depth should replace upper bound")
	assert.Equal(t, uint8(TTExact), got.Flag)
}

func TestTTDeepEntrySurvivesMultipleShallowStores(t *testing.T) {
	tt := NewTranspositionTable(1)
	hash := uint64(0x12345678)
	tt.Store(TTEntry{Hash: hash, Score: 7000, Depth: 8, Flag: TTExact, Age: 0})

	for d := uint8(1); d <= 6; d++ {
		tt.Store(TTEntry{Hash: hash, Score: int32(d * 100), Depth: d, Flag: TTExact, Age: 0})
	}

	got, ok := tt.Lookup(hash)
	assert.True(t, ok)
	assert.Equal(t, int32(7000), got.Score, "depth-8 entry should survive multiple shallow stores")
	assert.Equal(t, uint8(8), got.Depth)
}

func TestTTDifferentHashPriorityApplied(t *testing.T) {
	tt := NewTranspositionTable(1)

	// Store a high-priority entry
	tt.Store(TTEntry{Hash: 0xAAA, Score: 5000, Depth: 10, Flag: TTExact, Age: 0})

	// Try overwriting with different hash at lower depth — should be rejected
	tt.Store(TTEntry{Hash: 0xAAA, Score: 100, Depth: 3, Flag: TTExact, Age: 0})

	got, ok := tt.Lookup(0xAAA)
	assert.True(t, ok)
	assert.Equal(t, int32(5000), got.Score)
	assert.Equal(t, uint8(10), got.Depth)
}
