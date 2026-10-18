package engine

import "github.com/lavantien/caro-ai-pvp/internal/rules"

func (s *SMP) TTMove(b *rules.Board) rules.Move {
	return s.tt.move(b.Hash)
}
