package main

import (
	"fmt"

	"github.com/lavantien/caro-ai-pvp/internal/geom"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

type gameEnv struct {
	variant string
	g       *geom.Geometry
}

func resolveGeometry(name string) (gameEnv, error) {
	g, err := geom.ByName(name)
	if err != nil {
		return gameEnv{}, fmt.Errorf("arena: %v", err)
	}
	return gameEnv{variant: name, g: g}, nil
}

func (e gameEnv) board() *rules.Board { return rules.NewBoardFor(e.g) }
