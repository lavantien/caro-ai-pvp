package rules

import (
	"errors"
	"fmt"

	"github.com/lavantien/caro-ai-pvp/internal/geom"
)

type Cell uint16

func ParseCell(name string) (Cell, error) {
	cell, err := squareGeometry.ParseCell(name)
	if err != nil {
		return 0, wrapCodecError(err)
	}
	return Cell(cell), nil
}

func CellName(cell Cell) (string, error) {
	name, err := squareGeometry.CellName(int(cell))
	if err != nil {
		return "", wrapCodecError(err)
	}
	return name, nil
}

func AppendCellName(dst []byte, cell Cell) []byte {
	return squareGeometry.AppendName(dst, int(cell))
}

func wrapCodecError(err error) error {
	var ne *geom.NameError
	if errors.As(err, &ne) {
		return fmt.Errorf("rules: %s", ne.Text)
	}
	return err
}
