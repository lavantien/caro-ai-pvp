package domain

var zobristTable [BoardSize * BoardSize * 2]uint64
var zobristNullMoveKey uint64

func init() {
	state := uint64(0x58A2C43F5A3B7E91)
	for i := range zobristTable {
		state += 0x9E3779B97F4A7C15
		z := state
		z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9
		z = (z ^ (z >> 27)) * 0x94D049BB133111EB
		zobristTable[i] = z ^ (z >> 31)
	}
	state += 0x9E3779B97F4A7C15
	z := state
	z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9
	z = (z ^ (z >> 27)) * 0x94D049BB133111EB
	zobristNullMoveKey = z ^ (z >> 31)
}

func ZobristKey(x, y int, player Player) uint64 {
	playerIndex := 0
	if player == PlayerBlue {
		playerIndex = 1
	}
	return zobristTable[x*BoardSize*2+y*2+playerIndex]
}

func ZobristNullMove() uint64 {
	return zobristNullMoveKey
}
