package main

import (
	"fmt"
	"io"
	"math"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

const (
	passThresholdEasy   = 0.90
	passThresholdMedium = 0.75
	passThresholdHard   = 0.60
)

const (
	gmPassThresholdEasy   = 0.90
	gmPassThresholdMedium = 0.80
	gmPassThresholdHard   = 0.75
	gmPassThresholdMaster = 2.0 / 3.0
)

var passRungs = map[string]map[string]float64{
	"master": {
		"easy":   passThresholdEasy,
		"medium": passThresholdMedium,
		"hard":   passThresholdHard,
	},
	"grandmaster": {
		"master": gmPassThresholdMaster,
		"hard":   gmPassThresholdHard,
		"medium": gmPassThresholdMedium,
		"easy":   gmPassThresholdEasy,
	},
}

const (
	passDecided = iota
	failDecided
	passUndecided
)

var hexLadderRoster = []string{"easy", "medium", "hard", "master"}

func gateTC(tcIdx int) (config.TimeControl, error) {
	if tcIdx < 0 || tcIdx >= len(config.TimeControls) {
		return config.TimeControl{}, fmt.Errorf("gate tc index %d is outside 0..%d", tcIdx, len(config.TimeControls)-1)
	}
	return config.TimeControls[tcIdx], nil
}

func gateWinner(side rules.Color, drawn bool) string {
	if drawn {
		return winnerDraw
	}
	if side == rules.Red {
		return winnerRed
	}
	return winnerBlue
}

func passThreshold(subject, opp string) (float64, error) {
	rungs, ok := passRungs[subject]
	if !ok {
		return 0, fmt.Errorf("passgate subject %q is outside master, grandmaster", subject)
	}
	threshold, ok := rungs[opp]
	if !ok {
		return 0, fmt.Errorf("passgate opponent %q is not a rung of subject %q", opp, subject)
	}
	return threshold, nil
}

func decidePass(winsDraws, games int, threshold, z float64) (float64, float64, float64, int) {
	wd := float64(winsDraws) / float64(games)
	half := z * math.Sqrt(wd*(1-wd)/float64(games))
	lb := math.Max(0, wd-half)
	ub := math.Min(1, wd+half)
	if lb >= threshold {
		return wd, lb, ub, passDecided
	}
	if ub < threshold {
		return wd, lb, ub, failDecided
	}
	return wd, lb, ub, passUndecided
}

func passBlockAction(block, maxBlocks, state int) (bool, string) {
	if state == passDecided || state == failDecided {
		return true, ""
	}
	if block == maxBlocks {
		return true, "extend: blocked, block cap reached"
	}
	return false, "extend: needed, run another block"
}

func runHexPassGate(w io.Writer, env gameEnv, tcIdx, games int, subject, opp string, fixedMs int64, maxBlocks int, z float64) error {
	threshold, err := passThreshold(subject, opp)
	if err != nil {
		return err
	}
	tc, err := gateTC(tcIdx)
	if err != nil {
		return err
	}
	if _, ok := seats[subject]; !ok {
		return fmt.Errorf("passgate subject %q is not a known seat", subject)
	}
	if _, ok := seats[opp]; !ok {
		return fmt.Errorf("passgate opponent %q is not a known seat", opp)
	}
	if games < 1 {
		return fmt.Errorf("passgate needs games >= 1, got %d", games)
	}
	if maxBlocks < 1 {
		return fmt.Errorf("passgate needs gateMaxBlocks >= 1, got %d", maxBlocks)
	}
	subjectTally := &tally{name: subject}
	challenger := &tally{name: opp}
	results := map[string]*tally{subjectTally.name: subjectTally, challenger.name: challenger}
	fmt.Fprintf(w, "passgate %s vs %s tc %d+%d games %d blocks %d geometry %s\n", subject, opp, tc.InitialMin, tc.IncrementSec, games, maxBlocks, env.variant)
	total := 0
	for block := 1; block <= maxBlocks; block++ {
		for g := 0; g < games; g++ {
			total++
			red, blue := challenger, subjectTally
			if total%2 == 1 {
				red, blue = subjectTally, challenger
			}
			fmt.Fprintf(w, "game %d: %s (red) vs %s (blue)\n", total, red.name, blue.name)
			side, drawn, moves := play(env, red, blue, tcIdx, total, fixedMs)
			winnerSeat := gateWinner(side, drawn)
			record(results, red.name, blue.name, winnerSeat, moves)
			fmt.Fprintln(w, gameLine(total, red.name, blue.name, winnerSeat, moves))
		}
		wd, lb, ub, state := decidePass(subjectTally.wins+subjectTally.draws, total, threshold, z)
		fmt.Fprintf(w, "extend: block %d/%d n=%d wd=%.3f lb=%.3f ub=%.3f\n", block, maxBlocks, total, wd, lb, ub)
		stop, note := passBlockAction(block, maxBlocks, state)
		if note != "" {
			fmt.Fprintln(w, note)
		}
		if stop {
			capped := state == passUndecided
			fmt.Fprintln(w, passVerdictLine(env.variant, wd, lb, ub, threshold, opp, tcIdx, total, capped))
			return nil
		}
	}
	return nil
}

func passVerdictLine(variant string, wd, lb, ub, threshold float64, opp string, tcIdx, games int, capped bool) string {
	if wd >= threshold {
		return variant + "-passgates: DONE"
	}
	return fmt.Sprintf("%s-passgates: FAILED opp=%s tc=%d wd=%.3f threshold=%.3f n=%d lb=%.3f ub=%.3f cap=%t", variant, opp, tcIdx, wd, threshold, games, lb, ub, capped)
}

func runHexLadder(w io.Writer, env gameEnv, tcIdx, games int, fixedMs int64) error {
	if games < 1 {
		return fmt.Errorf("ladder needs games >= 1, got %d", games)
	}
	tc, err := gateTC(tcIdx)
	if err != nil {
		return err
	}
	fmt.Fprintf(w, "ladder roster %v tc %d+%d games %d geometry %s\n", hexLadderRoster, tc.InitialMin, tc.IncrementSec, games, env.variant)
	gameNo := 0
	for i := 0; i < len(hexLadderRoster); i++ {
		for j := i + 1; j < len(hexLadderRoster); j++ {
			lower := &tally{name: hexLadderRoster[i]}
			higher := &tally{name: hexLadderRoster[j]}
			results := map[string]*tally{lower.name: lower, higher.name: higher}
			for g := 0; g < games; g++ {
				gameNo++
				red, blue := lower, higher
				if g%2 == 1 {
					red, blue = blue, red
				}
				fmt.Fprintf(w, "game %d: %s (red) vs %s (blue)\n", gameNo, red.name, blue.name)
				side, drawn, moves := play(env, red, blue, tcIdx, gameNo, fixedMs)
				winnerSeat := gateWinner(side, drawn)
				record(results, red.name, blue.name, winnerSeat, moves)
				fmt.Fprintln(w, gameLine(gameNo, red.name, blue.name, winnerSeat, moves))
			}
			fmt.Fprintf(w, "pair done: %s vs %s\n", lower.name, higher.name)
			wd := float64(higher.wins+higher.draws) / float64(games)
			if inv, line := ladderEdgeVerdict(lower.name, higher.name, tcIdx, wd, games); inv {
				fmt.Fprintln(w, line)
				return nil
			}
		}
	}
	fmt.Fprintf(w, "%s\n", ladderDoneLine(games))
	return nil
}

func ladderEdgeVerdict(lower, higher string, tcIdx int, wd float64, games int) (bool, string) {
	if wd > 0.5 {
		return false, ""
	}
	return true, fmt.Sprintf("hex-ladder: FAILED edge=%s-vs-%s tc=%d wd=%.3f n=%d", lower, higher, tcIdx, wd, games)
}

func ladderDoneLine(games int) string {
	return fmt.Sprintf("hex-ladder: DONE zero inversions at %d games per edge, deliberately underpowered", games)
}
