package engine

import (
	"caro-ai-pvp/internal/domain"
)

const (
	fiveScore       = domain.WinScore
	flex4WinBonus   = 15_000
	doubleB4Bonus   = 14_000
	b4f3Bonus       = 13_000
	doubleF3Bonus   = 12_000
	flex4Score      = 10_000
	block4Score     = 5_000
	flex3Score      = 1_000
	block3Score     = 100
	flex2Score      = 100
	block2Score     = 30
	flex1Score      = 10

	maxCorrectedEval = domain.MaxEval
)

// Evaluate reads the incremental pattern aggregates maintained by
// MakeMove/UnmakeMove: O(1) per call.
func Evaluate(sb *SearchBoard, player domain.Player) int {
	opponent := player.Opponent()
	score := scorePatterns(sb.patternAgg[player], sb.stoneCount[player]) -
		scorePatterns(sb.patternAgg[opponent], sb.stoneCount[opponent])
	score += sb.centerSum[player] - sb.centerSum[opponent]

	if score > maxCorrectedEval {
		score = maxCorrectedEval
	}
	if score < -maxCorrectedEval {
		score = -maxCorrectedEval
	}
	return score
}

// evaluateSlow recomputes everything from scratch. It is the reference the
// incremental differential test and the benchmark compare against.
func evaluateSlow(sb *SearchBoard, player domain.Player) int {
	playerScore := scorePatterns(ClassifyBoard(sb, player), countStonesFor(sb, player))
	opponentScore := scorePatterns(ClassifyBoard(sb, player.Opponent()), countStonesFor(sb, player.Opponent()))

	score := playerScore - opponentScore
	score += centerBonus(sb, player) - centerBonus(sb, player.Opponent())

	if score > maxCorrectedEval {
		score = maxCorrectedEval
	}
	if score < -maxCorrectedEval {
		score = -maxCorrectedEval
	}
	return score
}

func countStonesFor(sb *SearchBoard, player domain.Player) int {
	n := 0
	for x := range domain.BoardSize {
		for y := range domain.BoardSize {
			if sb.PlayerAt(x, y) == player {
				n++
			}
		}
	}
	return n
}

func scorePatterns(pp PlayerPattern4, stoneCount int) int {
	if pp.Exactly5Count > 0 {
		return fiveScore
	}

	if pp.Flex4Count > 0 {
		score := flex4WinBonus
		score += pp.Block4Count * block4Score
		score += pp.Flex3Count * flex3Score
		return score
	}

	if pp.Block4Count >= 2 {
		score := doubleB4Bonus
		score += pp.Block4Count * block4Score
		score += pp.Flex3Count * flex3Score
		return score
	}

	if pp.Flex3Count >= 2 {
		score := doubleF3Bonus
		score += pp.Block4Count * block4Score
		score += pp.Flex3Count * flex3Score
		return score
	}

	if pp.Block4Count >= 1 && pp.Flex3Count >= 1 {
		score := b4f3Bonus
		score += pp.Block4Count * block4Score
		score += pp.Flex3Count * flex3Score
		return score
	}

	score := 0
	score += pp.Flex4Count * flex4Score
	score += pp.Block4Count * block4Score
	score += pp.Flex3Count * flex3Score
	score += pp.Block3Count * block3Score
	score += pp.Flex2Count * flex2Score
	score += pp.Block2Count * block2Score
	score += stoneCount * flex1Score
	return score
}

func centerBonus(sb *SearchBoard, player domain.Player) int {
	center := domain.BoardSize / 2
	bonus := 0
	bits := sb.BitBoardFor(player)
	for x := range domain.BoardSize {
		for y := range domain.BoardSize {
			if bits.Get(x, y) {
				dist := abs(x-center) + abs(y-center)
				bonus += (domain.BoardSize - dist) * 2
			}
		}
	}
	return bonus
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
