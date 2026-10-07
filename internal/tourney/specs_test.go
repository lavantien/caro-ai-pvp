package tourney

import (
	"slices"
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

// TestSmokeRoster105Spec pins the 10+5 plumbing smoke: exactly two hard
// seats meeting twice (each red-first once) at the fourth time control,
// small enough to validate the conductor path inside a night window.
func TestSmokeRoster105Spec(t *testing.T) {
	spec := SmokeRoster105()
	wantTC, ok := config.TCIndex(10, 5)
	if !ok {
		t.Fatal("10+5 is not a configured time control")
	}
	if spec.TCIdx != wantTC {
		t.Errorf("smoke105 tc = %d, want the 10+5 index %d", spec.TCIdx, wantTC)
	}
	if spec.BOLen != config.SeriesBO3 {
		t.Errorf("smoke105 bo = %d, want bo%d", spec.BOLen, config.SeriesBO3)
	}
	if spec.StartRating != config.TournamentStartRating {
		t.Errorf("smoke105 start rating = %d, want %d", spec.StartRating, config.TournamentStartRating)
	}
	want := []Participant{{Slot: 0, Name: "hard-1", Tier: config.TierHard.Name}, {Slot: 1, Name: "hard-2", Tier: config.TierHard.Name}}
	if len(spec.Roster) != len(want) {
		t.Fatalf("smoke105 roster = %d participants, want %d", len(spec.Roster), len(want))
	}
	for i, p := range spec.Roster {
		if p != want[i] {
			t.Errorf("smoke105 roster[%d] = %+v, want %+v", i, p, want[i])
		}
	}
	// Both directions pinned: counting only hard-1-as-red pairs stays green
	// under a single round robin, the exact regression the twice-pair shape
	// exists to prevent.
	redFirst := map[string]int{}
	pairCount := 0
	for _, pair := range Pairings(spec.Roster) {
		pairCount++
		redFirst[pair.RedFirst.Name+"-"+pair.BlueFirst.Name]++
	}
	if pairCount != 2 {
		t.Errorf("smoke105 schedule holds %d pairings, want 2 (twice-pair)", pairCount)
	}
	if redFirst["hard-1-hard-2"] != 1 || redFirst["hard-2-hard-1"] != 1 {
		t.Errorf("smoke105 red-first split = %v, want each seat red-first exactly once", redFirst)
	}
}

func TestHeadlessDriversMatchSpec(t *testing.T) {
	// Sorted tier-name pairs including self-meets, the same normalization
	// the seen-set applies, derived from the tier table so a new tier joins
	// the demanded matchups without editing this test.
	names := make([]string, len(config.Tiers))
	for i := range config.Tiers {
		names[i] = config.Tiers[i].Name
	}
	slices.Sort(names)
	var wantMatchups [][2]string
	for i := 0; i < len(names); i++ {
		for j := i; j < len(names); j++ {
			wantMatchups = append(wantMatchups, [2]string{names[i], names[j]})
		}
	}
	wantSeats := config.InstancesPerTier * len(config.Tiers)
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
		if got := len(tc.spec.Roster); got != wantSeats {
			t.Fatalf("%s roster = %d participants, want %d", tc.name, got, wantSeats)
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
