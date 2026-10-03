package tourney

import (
	"strings"
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

// TestMustTCPanicsOnUnconfiguredTimeControl pins the spec resolver's guard:
// a driver naming a clock shape the config hub does not hold is a spec bug,
// and it panics at resolve time instead of silently seating a wrong index.
func TestMustTCPanicsOnUnconfiguredTimeControl(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("mustTC returned, want the panic on the unconfigured shape")
		}
		msg, ok := r.(string)
		if !ok || !strings.Contains(msg, "not configured") || !strings.Contains(msg, "0+7") {
			t.Errorf("panic = %v, want it naming the 0+7 shape as unconfigured", r)
		}
	}()
	mustTC(0, 7)
}

func TestHeadlessDriversMatchSpec(t *testing.T) {
	easy, medium, hard := config.TierEasy.Name, config.TierMedium.Name, config.TierHard.Name
	// Sorted pairs, the same normalization the seen-set applies.
	wantMatchups := [][2]string{
		{easy, easy}, {easy, hard}, {easy, medium},
		{hard, hard}, {hard, medium}, {medium, medium},
	}
	for _, tc := range []struct {
		name      string
		spec      RunSpec
		initial   int
		increment int
	}{
		{"smoke32", SmokeRoster32(), 3, 2},
		{"smoke10", SmokeRoster10(), 1, 0},
		{"full", FullRoster24(), 2, 1},
	} {
		wantTC, ok := config.TCIndex(tc.initial, tc.increment)
		if !ok {
			t.Fatalf("%s: %d+%d is not a configured time control", tc.name, tc.initial, tc.increment)
		}
		if tc.spec.TCIdx != wantTC {
			t.Errorf("%s tc = %d, want the %d+%d index %d", tc.name, tc.spec.TCIdx, tc.initial, tc.increment, wantTC)
		}
		if tc.spec.BOLen != config.SeriesBO3 {
			t.Errorf("%s bo = %d, want bo%d", tc.name, tc.spec.BOLen, config.SeriesBO3)
		}
		if tc.spec.StartRating != config.TournamentStartRating {
			t.Errorf("%s start rating = %d, want %d", tc.name, tc.spec.StartRating, config.TournamentStartRating)
		}
		if got := len(tc.spec.Roster); got != 6 {
			t.Fatalf("%s roster = %d participants, want 6", tc.name, got)
		}
		for i, p := range tc.spec.Roster {
			if p.Slot != i {
				t.Errorf("%s slot %d at position %d", tc.name, p.Slot, i)
			}
		}
		// Every spec matchup type meets in the twice-pair schedule.
		seen := make(map[[2]string]bool)
		for _, pair := range Pairings(tc.spec.Roster) {
			a, b := pair.RedFirst.Tier, pair.BlueFirst.Tier
			if a > b {
				a, b = b, a
			}
			seen[[2]string{a, b}] = true
		}
		for _, want := range wantMatchups {
			if !seen[want] {
				t.Errorf("%s schedule misses the %v matchup", tc.name, want)
			}
		}
	}
}
