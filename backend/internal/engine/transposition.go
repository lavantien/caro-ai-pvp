package engine

import (
	"caro-ai-pvp/internal/domain"
	"sync"
	"sync/atomic"
	"unsafe"
)

const (
	TTExact      uint8 = 0
	TTLowerBound uint8 = 1
	TTUpperBound uint8 = 2
)

type TTEntry struct {
	Hash       uint64
	Score      int32
	StaticEval int32
	Depth      uint8
	MoveX      int8
	MoveY      int8
	Flag       uint8
	Age        uint8
}

type ttSlot struct {
	hash       uint64
	score      int32
	staticEval int32
	depth      uint8
	moveX      int8
	moveY      int8
	flag       uint8
	age        uint8
}

type ttShard struct {
	mu    sync.RWMutex
	slots []ttSlot
	mask  uint64
}

type TranspositionTable struct {
	shards [domain.TTShardCount]ttShard
	sizeMB int
	age    atomic.Uint32
	probes atomic.Int64
	hits   atomic.Int64
}

func NewTranspositionTable(sizeMB int) *TranspositionTable {
	tt := &TranspositionTable{sizeMB: sizeMB}
	entriesPerShard := (sizeMB * 1024 * 1024 / domain.TTShardCount) / int(unsafe.Sizeof(ttSlot{}))
	mask := uint64(1)
	for mask < uint64(entriesPerShard) {
		mask <<= 1
	}
	mask--

	for i := range tt.shards {
		tt.shards[i].slots = make([]ttSlot, mask+1)
		tt.shards[i].mask = mask
	}
	return tt
}

func (tt *TranspositionTable) shardIndex(hash uint64) int {
	return int((hash >> 32) & (domain.TTShardCount - 1))
}

func (tt *TranspositionTable) Store(entry TTEntry) {
	si := tt.shardIndex(entry.Hash)
	shard := &tt.shards[si]
	idx := entry.Hash & shard.mask

	currentAge := uint8(tt.age.Load())

	entryPrio := int(entry.Depth) - 8*int(currentAge-entry.Age)

	shard.mu.Lock()
	slot := &shard.slots[idx]
	existingHash := slot.hash
	existingDepth := slot.depth
	existingAge := slot.age

	existingPrio := int(existingDepth) - 8*int(currentAge-existingAge)

	if existingHash == entry.Hash {
		if existingDepth > entry.Depth {
			shard.mu.Unlock()
			return
		}
	} else if existingHash != 0 && existingPrio >= entryPrio {
		shard.mu.Unlock()
		return
	}
	slot.hash = entry.Hash
	slot.score = entry.Score
	slot.staticEval = entry.StaticEval
	slot.depth = entry.Depth
	slot.moveX = entry.MoveX
	slot.moveY = entry.MoveY
	slot.flag = entry.Flag
	slot.age = entry.Age
	shard.mu.Unlock()
}

func (tt *TranspositionTable) Lookup(hash uint64) (TTEntry, bool) {
	tt.probes.Add(1)
	si := tt.shardIndex(hash)
	shard := &tt.shards[si]
	idx := hash & shard.mask

	shard.mu.RLock()
	slot := &shard.slots[idx]
	entry := TTEntry{
		Hash:       slot.hash,
		Score:      slot.score,
		StaticEval: slot.staticEval,
		Depth:      slot.depth,
		MoveX:      slot.moveX,
		MoveY:      slot.moveY,
		Flag:       slot.flag,
		Age:        slot.age,
	}
	shard.mu.RUnlock()

	if entry.Hash != hash {
		return TTEntry{}, false
	}
	tt.hits.Add(1)
	return entry, true
}

func (tt *TranspositionTable) Clear() {
	for i := range tt.shards {
		tt.shards[i].mu.Lock()
		for j := range tt.shards[i].slots {
			tt.shards[i].slots[j] = ttSlot{}
		}
		tt.shards[i].mu.Unlock()
	}
}

func (tt *TranspositionTable) Dispose() {
	for i := range tt.shards {
		tt.shards[i].slots = nil
		tt.shards[i].mask = 0
	}
}

func (tt *TranspositionTable) IncrementAge() {
	tt.age.Add(1)
}

func (tt *TranspositionTable) Stats() (probes, hits int64) {
	return tt.probes.Load(), tt.hits.Load()
}

func (tt *TranspositionTable) ResetStats() {
	tt.probes.Store(0)
	tt.hits.Store(0)
}
