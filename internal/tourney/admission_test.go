package tourney

// The v0.23 casual-room gate tests: a match source riding the room manager
// exposes the machine's live core bookings, and startRun refuses with the
// held amount while any booking is live, because at startRun time every
// booking belongs to an open room outside the run (tournament rooms cannot
// predate their run) and MachineCores budgets the machine as a whole.

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

// ledgerFakeSource extends the scripted source with the room manager's
// ledger capability, the production RoomSource shape.
type ledgerFakeSource struct {
	fakeSource
	held int
}

func (s *ledgerFakeSource) LiveCoreBookings() int { return s.held }

func TestConductorStartRunRefusesWhileRoomsHoldCores(t *testing.T) {
	ts, _ := newTestStore(t)
	held := &ledgerFakeSource{fakeSource: fakeSource{script: easySweeps}, held: config.TierHard.Cores}

	_, _, err := NewConductor(held).startRun(context.Background(), ts, rosterTwo(),
		mustTC(1, 0), config.SeriesBO3, config.TournamentStartRating, 1, "test")
	if err == nil || !strings.Contains(err.Error(), fmt.Sprintf("%d live-search cores", config.TierHard.Cores)) {
		t.Fatalf("start run = %v, want the held-cores refusal naming %d", err, config.TierHard.Cores)
	}
	runs, rerr := ts.Runs(context.Background())
	if rerr != nil {
		t.Fatalf("read runs: %v", rerr)
	}
	if len(runs) != 0 {
		t.Errorf("runs after the refusal = %d, want 0: nothing persisted", len(runs))
	}

	// A source with no ledger to expose (the scripted tests' shape) starts
	// clean: the gate only reads the capability when the source carries it.
	if _, _, err := NewConductor(&fakeSource{script: easySweeps}).startRun(context.Background(), ts, rosterTwo(),
		mustTC(1, 0), config.SeriesBO3, config.TournamentStartRating, 1, "test"); err != nil {
		t.Fatalf("start run without ledger holdings: %v", err)
	}
}
