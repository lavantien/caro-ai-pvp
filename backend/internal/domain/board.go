package domain

type Cell struct {
	X      int
	Y      int
	Player Player
}

func (c Cell) IsEmpty() bool {
	return c.Player == PlayerNone
}

type Board struct {
	cells    [BoardSize * BoardSize]Player
	redBits  [4]uint64
	blueBits [4]uint64
	hash     uint64
}

func NewBoard() Board {
	return Board{}
}

func (b Board) GetCell(x, y int) Cell {
	if x < 0 || x >= BoardSize || y < 0 || y >= BoardSize {
		return Cell{X: x, Y: y, Player: PlayerNone}
	}
	return Cell{X: x, Y: y, Player: b.cells[x*BoardSize+y]}
}

func (b Board) Hash() uint64 {
	return b.hash
}

func (b Board) IsEmpty() bool {
	for i := range b.redBits {
		if b.redBits[i] != 0 || b.blueBits[i] != 0 {
			return false
		}
	}
	return true
}

func (b Board) IsEmptyAt(x, y int) bool {
	if x < 0 || x >= BoardSize || y < 0 || y >= BoardSize {
		return false
	}
	return b.cells[x*BoardSize+y] == PlayerNone
}

func (b Board) GetPlayerAt(x, y int) Player {
	if x < 0 || x >= BoardSize || y < 0 || y >= BoardSize {
		return PlayerNone
	}
	return b.cells[x*BoardSize+y]
}

func (b Board) BitBoardBits(player Player) [4]uint64 {
	if player == PlayerRed {
		return b.redBits
	}
	return b.blueBits
}

func (b Board) PlaceStone(x, y int, player Player) Board {
	newB, err := b.PlaceStoneChecked(x, y, player)
	if err != nil {
		panic(err)
	}
	return newB
}

func (b Board) PlaceStoneChecked(x, y int, player Player) (Board, error) {
	if x < 0 || x >= BoardSize || y < 0 || y >= BoardSize {
		return b, ErrPositionBounds
	}
	if b.cells[x*BoardSize+y] != PlayerNone {
		return b, ErrCellOccupied
	}

	newB := b
	newB.cells[x*BoardSize+y] = player

	bitIndex := y*BoardSize + x
	ulongIndex := bitIndex >> 6
	bitOffset := uint(bitIndex & 63)
	bitMask := uint64(1) << bitOffset

	if player == PlayerRed {
		newB.redBits[ulongIndex] |= bitMask
	} else {
		newB.blueBits[ulongIndex] |= bitMask
	}

	newB.hash = b.hash ^ ZobristKey(x, y, player)
	return newB, nil
}
