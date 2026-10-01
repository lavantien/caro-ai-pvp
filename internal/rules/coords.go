package rules

import (
	"fmt"
	"strconv"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

type Cell uint16

func ParseCell(name string) (Cell, error) {
	if n := len(name); n < 2 || n > 3 {
		return 0, fmt.Errorf("rules: cell name %q: want 2 or 3 characters", name)
	}
	col := int(name[0]) - 'A'
	if col < 0 || col >= config.BoardSize {
		return 0, fmt.Errorf("rules: cell name %q: column out of A..%c", name, rune('A'+config.BoardSize-1))
	}
	row := 0
	for _, ch := range name[1:] {
		if ch < '0' || ch > '9' {
			return 0, fmt.Errorf("rules: cell name %q: stray character %q", name, ch)
		}
		row = row*10 + int(ch-'0')
	}
	if len(name) == 3 && name[1] == '0' {
		return 0, fmt.Errorf("rules: cell name %q: leading zero", name)
	}
	if row < 1 || row > config.BoardSize {
		return 0, fmt.Errorf("rules: cell name %q: row out of 1..%d", name, config.BoardSize)
	}
	return Cell((row-1)*config.BoardStride + col), nil
}

func CellName(cell Cell) (string, error) {
	if int(cell) >= config.BoardCells {
		return "", fmt.Errorf("rules: cell %d out of 0..%d", cell, config.BoardCells-1)
	}
	return string(rune('A'+rune(cell%config.BoardStride))) + strconv.Itoa(int(cell/config.BoardStride)+1), nil
}
