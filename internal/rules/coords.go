package rules

import (
	"fmt"

	"github.com/lavantien/caro-ai-pvp/internal/geom"
)

type Cell uint16

func ParseCell(name string) (Cell, error) {
	cell, err := squareGeometry.ParseCell(name)
	if err != nil {
		return 0, fmt.Errorf("rules: %s", err.(*geom.NameError).Text)
	}
	return Cell(cell), nil
}

func CellName(cell Cell) (string, error) {
	name, err := squareGeometry.CellName(int(cell))
	if err != nil {
		return "", fmt.Errorf("rules: %s", err.(*geom.NameError).Text)
	}
	return name, nil
}

func AppendCellName(dst []byte, cell Cell) []byte {
	return squareGeometry.AppendName(dst, int(cell))
}
