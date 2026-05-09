package engine

import (
	"sync/atomic"
	"unsafe"
)

const (
	ttShardCount = 16

	TTExact      uint8 = 0
	TTLowerBound uint8 = 1
	TTUpperBound uint8 = 2
)

type TTEntry struct {
	Hash  uint64
	Score int32
	Depth uint8
	MoveX int8
	MoveY int8
	Flag  uint8
	Age   uint8
}

type ttSlot struct {
	hash    uint64
	score   int32
	depth   uint8
	moveX   int8
	moveY   int8
	flag    uint8
	age     uint8
	version atomic.Uint32
}

type ttShard struct {
	slots []ttSlot
	mask  uint64
}

type TranspositionTable struct {
	shards [ttShardCount]ttShard
	sizeMB int
	age    atomic.Uint32
	probes atomic.Int64
	hits   atomic.Int64
}

func NewTranspositionTable(sizeMB int) *TranspositionTable {
	tt := &TranspositionTable{sizeMB: sizeMB}
	entriesPerShard := (sizeMB * 1024 * 1024 / ttShardCount) / int(unsafe.Sizeof(ttSlot{}))
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
	return int((hash >> 32) & (ttShardCount - 1))
}

func (tt *TranspositionTable) Store(entry TTEntry) {
	si := tt.shardIndex(entry.Hash)
	shard := &tt.shards[si]
	idx := entry.Hash & shard.mask
	slot := &shard.slots[idx]

	slot.version.Add(1)
	slot.hash = entry.Hash
	slot.score = entry.Score
	slot.depth = entry.Depth
	slot.moveX = entry.MoveX
	slot.moveY = entry.MoveY
	slot.flag = entry.Flag
	slot.age = entry.Age
	slot.version.Add(1)
}

func (tt *TranspositionTable) Lookup(hash uint64) (TTEntry, bool) {
	tt.probes.Add(1)
	si := tt.shardIndex(hash)
	shard := &tt.shards[si]
	idx := hash & shard.mask
	slot := &shard.slots[idx]

	v1 := slot.version.Load()
	if v1%2 != 0 {
		return TTEntry{}, false
	}

	entry := TTEntry{
		Hash:  slot.hash,
		Score: slot.score,
		Depth: slot.depth,
		MoveX: slot.moveX,
		MoveY: slot.moveY,
		Flag:  slot.flag,
		Age:   slot.age,
	}

	if slot.version.Load() != v1 {
		return TTEntry{}, false
	}
	if entry.Hash != hash {
		return TTEntry{}, false
	}
	tt.hits.Add(1)
	return entry, true
}

func (tt *TranspositionTable) Clear() {
	for i := range tt.shards {
		for j := range tt.shards[i].slots {
			tt.shards[i].slots[j] = ttSlot{}
		}
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
