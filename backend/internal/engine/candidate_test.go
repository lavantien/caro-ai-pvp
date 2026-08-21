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

func pickerYields(t *testing.T, sb *SearchBoard, player domain.Player) []domain.Position {
	t.Helper()
	candidates := GetCandidates(sb, domain.MaxSearchRadius)
	candidates = FilterOpenRule(candidates, sb, player)
	picker := NewMovePicker(candidates, sb, player, 4, nil, NewSearchHeuristics(), domain.Position{X: -1, Y: -1})
	var out []domain.Position
	for {
		m, ok := picker.Next()
		if !ok {
			return out
		}
		out = append(out, m)
	}
}

func TestMovePickerForcedOpenFour(t *testing.T) {
	// Blue to move; red has an open four on row 5 (7..10): the two
	// completions and their flanks are the only defensible replies.
	b := domain.NewBoard()
	for x := 7; x <= 10; x++ {
		b = b.PlaceStone(x, 5, domain.PlayerRed)
	}
	b = b.PlaceStone(3, 3, domain.PlayerBlue)
	sb := NewSearchBoard(b)

	moves := pickerYields(t, &sb, domain.PlayerBlue)
	allowed := map[domain.Position]bool{
		{X: 6, Y: 5}: true,  {X: 11, Y: 5}: true, // completions
		{X: 5, Y: 5}: true,  {X: 12, Y: 5}: true, // flanks
	}
	for _, m := range moves {
		assert.True(t, allowed[m], "quiet move (%d,%d) must not be searched in a forced position", m.X, m.Y)
	}
	assert.Len(t, moves, len(allowed), "exactly the forcing replies")
}

func TestMovePickerForcedBlockedFourKeepsFlankDefense(t *testing.T) {
	// Red four against the left edge: (0..3,5) with (4,5) completing the
	// five 0..4. The left flank is the board edge, so occupying (5,5)
	// leaves the completed five both-ends-blocked: a real defense in caro.
	// The picker must keep both the completion and that flank.
	b := domain.NewBoard()
	for x := 0; x <= 3; x++ {
		b = b.PlaceStone(x, 5, domain.PlayerRed)
	}
	b = b.PlaceStone(9, 9, domain.PlayerBlue)
	sb := NewSearchBoard(b)

	moves := pickerYields(t, &sb, domain.PlayerBlue)
	got := map[domain.Position]bool{}
	for _, m := range moves {
		got[m] = true
	}
	assert.True(t, got[domain.Position{X: 4, Y: 5}], "the completing block must be searched")
	assert.True(t, got[domain.Position{X: 5, Y: 5}], "the flank defense must be searched")
	assert.False(t, got[domain.Position{X: 10, Y: 10}], "quiet moves must be skipped in a forced position")
}
