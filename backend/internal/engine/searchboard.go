package engine

import (
	"caro-ai-pvp/internal/domain"
)

type undoEntry struct {
	x, y   int
	player domain.Player
	hash   uint64
}

type SearchBoard struct {
	cells     [domain.BoardSize * domain.BoardSize]domain.Player
	redBits   BitBoard
	blueBits  BitBoard
	hash      uint64
	undoStack []undoEntry
}

func NewSearchBoard(b domain.Board) SearchBoard {
	sb := SearchBoard{}
	rBits := b.BitBoardBits(domain.PlayerRed)
	bBits := b.BitBoardBits(domain.PlayerBlue)
	copy(sb.redBits[:], rBits[:])
	copy(sb.blueBits[:], bBits[:])
	sb.hash = b.Hash()

	for x := range domain.BoardSize {
		for y := range domain.BoardSize {
			sb.cells[x*domain.BoardSize+y] = b.GetPlayerAt(x, y)
		}
	}
	sb.undoStack = make([]undoEntry, 0, 64)
	return sb
}

func (sb *SearchBoard) Hash() uint64 { return sb.hash }

func (sb *SearchBoard) PlayerAt(x, y int) domain.Player {
	if x < 0 || x >= domain.BoardSize || y < 0 || y >= domain.BoardSize {
		return domain.PlayerNone
	}
	return sb.cells[x*domain.BoardSize+y]
}

func (sb *SearchBoard) BitBoardFor(player domain.Player) BitBoard {
	if player == domain.PlayerRed {
		return sb.redBits
	}
	return sb.blueBits
}

func (sb *SearchBoard) Occupied() BitBoard {
	return sb.redBits.Or(sb.blueBits)
}

func (sb *SearchBoard) IsEmpty(x, y int) bool {
	if x < 0 || x >= domain.BoardSize || y < 0 || y >= domain.BoardSize {
		return false
	}
	return sb.cells[x*domain.BoardSize+y] == domain.PlayerNone
}

func (sb *SearchBoard) MakeMove(x, y int, player domain.Player) {
	sb.undoStack = append(sb.undoStack, undoEntry{
		x: x, y: y,
		player: sb.cells[x*domain.BoardSize+y],
		hash:   sb.hash,
	})

	sb.cells[x*domain.BoardSize+y] = player
	if player == domain.PlayerRed {
		sb.redBits.Set(x, y)
	} else {
		sb.blueBits.Set(x, y)
	}
	sb.hash ^= domain.ZobristKey(x, y, player)
}

func (sb *SearchBoard) UnmakeMove() {
	entry := sb.undoStack[len(sb.undoStack)-1]
	sb.undoStack = sb.undoStack[:len(sb.undoStack)-1]

	currentPlayer := sb.cells[entry.x*domain.BoardSize+entry.y]
	if currentPlayer == domain.PlayerRed {
		sb.redBits.Clear(entry.x, entry.y)
	} else if currentPlayer == domain.PlayerBlue {
		sb.blueBits.Clear(entry.x, entry.y)
	}

	sb.cells[entry.x*domain.BoardSize+entry.y] = entry.player
	sb.hash = entry.hash
}

func (sb *SearchBoard) MakeNullMove() {
	sb.undoStack = append(sb.undoStack, undoEntry{x: -1, y: -1, player: domain.PlayerNone, hash: sb.hash})
	sb.hash ^= domain.ZobristNullMove()
}

func (sb *SearchBoard) UnmakeNullMove() {
	entry := sb.undoStack[len(sb.undoStack)-1]
	sb.undoStack = sb.undoStack[:len(sb.undoStack)-1]
	sb.hash = entry.hash
}
