package engine

import (
	"caro-ai-pvp/internal/domain"
	"testing"

	"github.com/stretchr/testify/assert"
)

// doubleThreatBoard leaves (7,7) empty with red owning the horizontal pair
// (5,7),(6,7) and the vertical pair (7,5),(7,6): playing the junction
// creates two open threes at once.
func doubleThreatBoard() domain.Board {
	return domain.NewBoard().
		PlaceStone(5, 7, domain.PlayerRed).
		PlaceStone(6, 7, domain.PlayerRed).
		PlaceStone(7, 5, domain.PlayerRed).
		PlaceStone(7, 6, domain.PlayerRed)
}

func TestIsTacticalMoveDoubleThreat(t *testing.T) {
	sb := NewSearchBoard(doubleThreatBoard())

	assert.True(t, isTacticalMove(&sb, 7, 7, domain.PlayerRed, domain.PlayerBlue),
		"a move creating open threes in two directions is forcing")
}

func TestIsTacticalMoveBlockingDoubleThreat(t *testing.T) {
	b := doubleThreatBoard().PlaceStone(7, 7, domain.PlayerRed)
	sb := NewSearchBoard(b)

	// Red just created the double three; the junction is gone, but the two
	// extension clusters keep generating double threats: any red move that
	// upgrades both lines stays forcing for blue to answer.
	assert.True(t, isTacticalMove(&sb, 4, 7, domain.PlayerBlue, domain.PlayerRed),
		"answering an existing double threat is forcing for the defender")
}

func TestIsTacticalMoveSingleOpenThreeNotTactical(t *testing.T) {
	b := domain.NewBoard().
		PlaceStone(5, 7, domain.PlayerRed).
		PlaceStone(6, 7, domain.PlayerRed).
		PlaceStone(10, 10, domain.PlayerBlue)
	sb := NewSearchBoard(b)

	assert.False(t, isTacticalMove(&sb, 7, 7, domain.PlayerRed, domain.PlayerBlue),
		"a lone open three is not forcing")
}
