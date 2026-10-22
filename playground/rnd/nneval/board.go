package main

import (
	"fmt"

	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

func midgameBoard() (*rules.Board, error) {
	b := rules.NewBoard()
	for _, name := range midgameMoves {
		cell, err := rules.ParseCell(name)
		if err != nil {
			return nil, fmt.Errorf("cell %q: %w", name, err)
		}
		b.Make(cell)
	}
	return b, nil
}
