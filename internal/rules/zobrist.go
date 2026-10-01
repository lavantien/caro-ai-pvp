package rules

import "github.com/lavantien/caro-ai-pvp/internal/config"

// splitmix64 finalizer constants
const (
	splitmixMulA = 0xBF58476D1CE4E5B9
	splitmixMulB = 0x94D049BB133111EB
	splitmixS1   = 30
	splitmixS2   = 27
	splitmixS3   = 31
)

var (
	zobristPieces [colorCount][config.BoardCells]uint64
	zobristSide   uint64
)

func splitmix64(state uint64) uint64 {
	state += config.ZobristSeed
	z := state
	z = (z ^ (z >> splitmixS1)) * splitmixMulA
	z = (z ^ (z >> splitmixS2)) * splitmixMulB
	return z ^ (z >> splitmixS3)
}

func init() {
	state := config.ZobristSeed
	for color := 0; color < colorCount; color++ {
		for cell := 0; cell < config.BoardCells; cell++ {
			state = splitmix64(state)
			zobristPieces[color][cell] = state
		}
	}
	state = splitmix64(state)
	zobristSide = state
}
