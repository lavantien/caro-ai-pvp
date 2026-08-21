package engine

import (
	"caro-ai-pvp/internal/domain"
)

// PredictReply reads the TT entry the previous search stored for the
// position b (opponent to move) and returns its best move as the predicted
// opponent reply. The stored move came from the search's filtered candidate
// list (open rule included), so legality is inherent; the depth and
// emptiness checks guard against zeroed, stale, or colliding entries.
func (ai *MinimaxAI) PredictReply(b domain.Board) (domain.Position, bool) {
	entry, ok := ai.tt.Lookup(b.Hash())
	if !ok || entry.Depth == 0 {
		return domain.Position{}, false
	}
	p := domain.Position{X: int(entry.MoveX), Y: int(entry.MoveY)}
	if !p.IsValid() || !b.IsEmptyAt(p.X, p.Y) {
		return domain.Position{}, false
	}
	return p, true
}
