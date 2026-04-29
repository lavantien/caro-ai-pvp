package domain

type Position struct {
	X int
	Y int
}

func (p Position) IsValid() bool {
	return p.X >= 0 && p.X < BoardSize && p.Y >= 0 && p.Y < BoardSize
}

func (p Position) Offset(dx, dy int) Position {
	return Position{X: p.X + dx, Y: p.Y + dy}
}
