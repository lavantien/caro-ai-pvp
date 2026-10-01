package rules

import "github.com/lavantien/caro-ai-pvp/internal/config"

type Color uint8

const (
	Red Color = iota
	Blue
	Empty
)

const colorCount = 2

func (c Color) Opponent() Color { return c ^ 1 }

var lineDirs = [4][2]int{{0, 1}, {1, 0}, {1, 1}, {1, -1}}

func inBoard(r, c int) bool {
	return r >= 0 && r < config.BoardSize && c >= 0 && c < config.BoardSize
}
