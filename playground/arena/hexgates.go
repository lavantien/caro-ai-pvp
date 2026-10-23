package main

import (
	"fmt"
	"io"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

const (
	passThresholdEasy   = 0.90
	passThresholdMedium = 0.75
	passThresholdHard   = 0.60
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

func passThreshold(opp string) (float64, error) {
	switch opp {
	case "easy":
		return passThresholdEasy, nil
	case "medium":
		return passThresholdMedium, nil
	case "hard":
		return passThresholdHard, nil
	}
	return 0, fmt.Errorf("passgate opponent %q is outside easy, medium, hard", opp)
}

func runHexPassGate(w io.Writer, env gameEnv, tcIdx, games int, opp string, fixedMs int64) error {
	threshold, err := passThreshold(opp)
	if err != nil {
		return err
	}
	tc, err := gateTC(tcIdx)
	if err != nil {
		return err
	}
	if _, ok := seats[opp]; !ok {
		return fmt.Errorf("passgate opponent %q is not a known seat", opp)
	}
	if games < 1 {
		return fmt.Errorf("passgate needs games >= 1, got %d", games)
	}
	master := &tally{name: "master"}
	challenger := &tally{name: opp}
	results := map[string]*tally{master.name: master, challenger.name: challenger}
	fmt.Fprintf(w, "passgate master vs %s tc %d+%d games %d geometry %s\n", opp, tc.InitialMin, tc.IncrementSec, games, env.variant)
	for g := 0; g < games; g++ {
		red, blue := master, challenger
		if g%2 == 1 {
			red, blue = blue, red
		}
		fmt.Fprintf(w, "game %d: %s (red) vs %s (blue)\n", g+1, red.name, blue.name)
		side, drawn, moves := play(env, red, blue, tcIdx, g+1, fixedMs)
		winnerSeat := gateWinner(side, drawn)
		record(results, red.name, blue.name, winnerSeat, moves)
		fmt.Fprintln(w, gameLine(g+1, red.name, blue.name, winnerSeat, moves))
	}
	wd := float64(master.wins+master.draws) / float64(games)
	fmt.Fprintln(w, passVerdictLine(wd, threshold, opp, tcIdx))
	return nil
}

func passVerdictLine(wd, threshold float64, opp string, tcIdx int) string {
	if wd >= threshold {
		return "hex-passgates: DONE"
	}
	return fmt.Sprintf("hex-passgates: FAILED opp=%s tc=%d wd=%.3f threshold=%.3f", opp, tcIdx, wd, threshold)
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
