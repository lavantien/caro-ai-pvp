package domain

import "errors"

var (
	ErrCellOccupied   = errors.New("cell already occupied")
	ErrPositionBounds = errors.New("position out of bounds")
	ErrGameOver       = errors.New("game is over")
	ErrOpenRule       = errors.New("open rule violation")
	ErrGameNotFound   = errors.New("game not found")
	ErrTooManyGames   = errors.New("too many concurrent games")
	ErrInvalidLevel   = errors.New("difficulty must be 1-5")
	ErrNoMoves        = errors.New("no moves to undo")
)
