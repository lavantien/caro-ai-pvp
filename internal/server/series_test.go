package server

import (
	"errors"
	"sync"
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

const (
	seriesHostID     int64 = 101
	seriesGuestID    int64 = 202
	seriesOutsiderID int64 = 999
)

func newReadySeries(t *testing.T, boLen int) *Series {
	t.Helper()
	s, err := NewSeries(seriesHostID, seriesGuestID, 0, boLen)
	if err != nil {
		t.Fatalf("NewSeries: %v", err)
	}
	if err := s.Ready(seriesHostID); err != nil {
		t.Fatalf("host Ready: %v", err)
	}
	if err := s.Ready(seriesGuestID); err != nil {
		t.Fatalf("guest Ready: %v", err)
	}
	return s
}

func seriesRedIs(t *testing.T, s *Series, want int64) {
	t.Helper()
	if got := s.RedUserID(); got != want {
		t.Errorf("red = %d, want %d", got, want)
	}
}

func seriesScoreIs(t *testing.T, s *Series, wantHost, wantGuest int) {
	t.Helper()
	h, g := s.Score()
	if h != wantHost || g != wantGuest {
		t.Errorf("score = %d-%d, want %d-%d", h, g, wantHost, wantGuest)
	}
}

func TestSeriesNewValidatesInputs(t *testing.T) {
	for _, tcIdx := range []int{-1, len(config.TimeControls), len(config.TimeControls) + 5} {
		if _, err := NewSeries(seriesHostID, seriesGuestID, tcIdx, config.SeriesLengths[0]); !errors.Is(err, ErrBadTimeControl) {
			t.Errorf("tcIdx %d: err = %v, want ErrBadTimeControl", tcIdx, err)
		}
	}
	badBO := []int{0, 1, 2, 4, 9, config.SeriesLengths[len(config.SeriesLengths)-1] + 1}
	for _, bo := range badBO {
		if _, err := NewSeries(seriesHostID, seriesGuestID, 0, bo); !errors.Is(err, ErrBadSeriesLength) {
			t.Errorf("boLen %d: err = %v, want ErrBadSeriesLength", bo, err)
		}
	}
	if _, err := NewSeries(seriesHostID, seriesHostID, 0, config.SeriesLengths[0]); !errors.Is(err, ErrSamePlayer) {
		t.Errorf("self-play: err = %v, want ErrSamePlayer", err)
	}
	for tcIdx := range config.TimeControls {
		for _, n := range config.SeriesLengths {
			s, err := NewSeries(seriesHostID, seriesGuestID, tcIdx, n)
			if err != nil {
				t.Fatalf("tcIdx %d bo %d: %v", tcIdx, n, err)
			}
			if s.HostUserID() != seriesHostID || s.GuestUserID() != seriesGuestID || s.TCIdx() != tcIdx || s.BOLen() != n {
				t.Errorf("tcIdx %d bo %d: fields drifted", tcIdx, n)
			}
			if s.State() != SeriesCreated || s.GamesPlayed() != 0 || s.Winner() != SideNone {
				t.Errorf("tcIdx %d bo %d: fresh series not clean", tcIdx, n)
			}
			if s.WinsNeeded() != n/2+1 {
				t.Errorf("bo %d: WinsNeeded = %d, want %d", n, s.WinsNeeded(), n/2+1)
			}
			seriesRedIs(t, s, seriesHostID)
			seriesScoreIs(t, s, 0, 0)
			if len(s.SyntheticGames()) != 0 {
				t.Errorf("bo %d: fresh series carries synthetic games", n)
			}
		}
	}
}

func TestSeriesReadyHandshake(t *testing.T) {
	s, err := NewSeries(seriesHostID, seriesGuestID, 0, config.SeriesLengths[0])
	if err != nil {
		t.Fatalf("NewSeries: %v", err)
	}
	if s.HostReady() || s.GuestReady() {
		t.Fatal("fresh series must not be readied")
	}
	if err := s.Ready(seriesOutsiderID); !errors.Is(err, ErrNotParticipant) {
		t.Errorf("outsider Ready: err = %v, want ErrNotParticipant", err)
	}
	if s.HostReady() {
		t.Error("outsider Ready flipped the host flag")
	}
	if err := s.Ready(seriesHostID); err != nil {
		t.Fatalf("host Ready: %v", err)
	}
	if !s.HostReady() || s.GuestReady() || s.State() != SeriesCreated {
		t.Error("one ready must leave the series in Created")
	}
	if err := s.RecordResult(Draw); !errors.Is(err, ErrNotReady) {
		t.Errorf("result before handshake: err = %v, want ErrNotReady", err)
	}
	if s.GamesPlayed() != 0 {
		t.Error("rejected result advanced the game count")
	}
	if err := s.Ready(seriesHostID); err != nil {
		t.Errorf("double ready must be idempotent: %v", err)
	}
	if s.State() != SeriesCreated {
		t.Error("double ready moved the state")
	}
	if err := s.Ready(seriesGuestID); err != nil {
		t.Fatalf("guest Ready: %v", err)
	}
	if s.State() != SeriesReady {
		t.Fatalf("state = %v, want SeriesReady", s.State())
	}
	if !s.HostReady() || !s.GuestReady() {
		t.Error("handshake flags incomplete after both readies")
	}
	if err := s.Ready(seriesGuestID); err != nil {
		t.Errorf("double ready after handshake must be idempotent: %v", err)
	}
	if s.State() != SeriesReady {
		t.Error("double ready after handshake moved the state")
	}
	seriesRedIs(t, s, seriesHostID)
	if s.GamesPlayed() != 0 {
		t.Error("handshake must not play games")
	}
}

func TestSeriesBO3HappyPath(t *testing.T) {
	s := newReadySeries(t, config.SeriesLengths[0])

	seriesRedIs(t, s, seriesHostID)
	if err := s.RecordResult(RedWins); err != nil {
		t.Fatalf("game 1: %v", err)
	}
	seriesScoreIs(t, s, 1, 0)
	if s.State() != SeriesInGame {
		t.Fatalf("after game 1: state = %v, want SeriesInGame", s.State())
	}
	seriesRedIs(t, s, seriesGuestID)

	if err := s.RecordResult(RedWins); err != nil {
		t.Fatalf("game 2: %v", err)
	}
	seriesScoreIs(t, s, 1, 1)
	seriesRedIs(t, s, seriesHostID)

	// Game 3 red is the game 2 loser, the host, so blue is the guest.
	if err := s.RecordResult(BlueWins); err != nil {
		t.Fatalf("game 3: %v", err)
	}
	seriesScoreIs(t, s, 1, 2)
	if s.State() != SeriesFinished {
		t.Fatalf("state = %v, want SeriesFinished", s.State())
	}
	if s.Winner() != SideGuest {
		t.Errorf("winner = %v, want SideGuest", s.Winner())
	}
	if s.GamesPlayed() != config.SeriesLengths[0] {
		t.Errorf("games played = %d, want %d", s.GamesPlayed(), config.SeriesLengths[0])
	}
	if len(s.SyntheticGames()) != 0 {
		t.Error("natural series must not carry synthetic games")
	}
	if err := s.RecordResult(RedWins); !errors.Is(err, ErrSeriesFinished) {
		t.Errorf("result after finish: err = %v, want ErrSeriesFinished", err)
	}
	if err := s.Ready(seriesHostID); !errors.Is(err, ErrSeriesFinished) {
		t.Errorf("ready after finish: err = %v, want ErrSeriesFinished", err)
	}
	if err := s.Forfeit(seriesHostID); !errors.Is(err, ErrSeriesFinished) {
		t.Errorf("forfeit after finish: err = %v, want ErrSeriesFinished", err)
	}
	if s.GamesPlayed() != config.SeriesLengths[0] {
		t.Error("rejected post-finish actions changed the series")
	}
}

func TestSeriesMajorityEndsEarlyBO3(t *testing.T) {
	s := newReadySeries(t, config.SeriesLengths[0])
	if err := s.RecordResult(RedWins); err != nil { // host red wins game 1
		t.Fatalf("game 1: %v", err)
	}
	if err := s.RecordResult(BlueWins); err != nil { // host blue wins game 2
		t.Fatalf("game 2: %v", err)
	}
	seriesScoreIs(t, s, 2, 0)
	if s.State() != SeriesFinished || s.Winner() != SideHost {
		t.Fatalf("2-0 bo3 must finish for the host, got state %v winner %v", s.State(), s.Winner())
	}
	if s.GamesPlayed() != 2 {
		t.Errorf("games played = %d, want 2: no game 3 after the majority", s.GamesPlayed())
	}
	if err := s.RecordResult(Draw); !errors.Is(err, ErrSeriesFinished) {
		t.Errorf("game 3 result: err = %v, want ErrSeriesFinished", err)
	}
}

func TestSeriesDrawRetainsRed(t *testing.T) {
	s := newReadySeries(t, config.SeriesLengths[0])
	if err := s.RecordResult(Draw); err != nil {
		t.Fatalf("game 1 draw: %v", err)
	}
	// A draw has no loser, so red keeps the seat.
	seriesRedIs(t, s, seriesHostID)
	seriesScoreIs(t, s, 0, 0)
	if s.State() != SeriesInGame {
		t.Fatalf("state = %v, want SeriesInGame", s.State())
	}
	if err := s.RecordResult(RedWins); err != nil { // host red wins game 2
		t.Fatalf("game 2: %v", err)
	}
	seriesRedIs(t, s, seriesGuestID)
	if err := s.RecordResult(Draw); err != nil {
		t.Fatalf("game 3 draw: %v", err)
	}
	// Schedule exhausted at 1-0, no majority: plurality takes the series.
	seriesScoreIs(t, s, 1, 0)
	if s.State() != SeriesFinished || s.Winner() != SideHost {
		t.Fatalf("1-0 bo3 must finish for the host, got state %v winner %v", s.State(), s.Winner())
	}
	if s.GamesPlayed() != config.SeriesLengths[0] {
		t.Errorf("games played = %d, want %d", s.GamesPlayed(), config.SeriesLengths[0])
	}
}

func TestSeriesExhaustedSchedule(t *testing.T) {
	s := newReadySeries(t, config.SeriesLengths[0])
	for i := 0; i < config.SeriesLengths[0]; i++ {
		if err := s.RecordResult(Draw); err != nil {
			t.Fatalf("draw %d: %v", i, err)
		}
	}
	seriesScoreIs(t, s, 0, 0)
	if s.State() != SeriesFinished || s.Winner() != SideNone {
		t.Fatalf("all-draw bo3 must finish drawn, got state %v winner %v", s.State(), s.Winner())
	}

	s2 := newReadySeries(t, config.SeriesLengths[0])
	steps := []Outcome{RedWins, RedWins, Draw} // host red, guest red, host red
	for i, o := range steps {
		if err := s2.RecordResult(o); err != nil {
			t.Fatalf("game %d: %v", i+1, err)
		}
	}
	seriesScoreIs(t, s2, 1, 1)
	if s2.State() != SeriesFinished || s2.Winner() != SideNone {
		t.Fatalf("1-1 bo3 must finish drawn, got state %v winner %v", s2.State(), s2.Winner())
	}

	s3 := newReadySeries(t, config.SeriesLengths[0])
	// draw, guest blue win over host red, draw: 0-1 at exhaustion.
	for i, o := range []Outcome{Draw, BlueWins, Draw} {
		if err := s3.RecordResult(o); err != nil {
			t.Fatalf("game %d: %v", i+1, err)
		}
	}
	seriesScoreIs(t, s3, 0, 1)
	if s3.State() != SeriesFinished || s3.Winner() != SideGuest {
		t.Fatalf("0-1 bo3 must finish for the guest, got state %v winner %v", s3.State(), s3.Winner())
	}
}

func TestSeriesLengthsFromConfig(t *testing.T) {
	for _, n := range config.SeriesLengths {
		need := n/2 + 1

		hostSweep := newReadySeries(t, n)
		for hostSweep.State() != SeriesFinished {
			o := RedWins
			if hostSweep.RedUserID() == seriesGuestID {
				o = BlueWins
			}
			if err := hostSweep.RecordResult(o); err != nil {
				t.Fatalf("bo %d host sweep: %v", n, err)
			}
		}
		seriesScoreIs(t, hostSweep, need, 0)
		if hostSweep.GamesPlayed() != need {
			t.Errorf("bo %d host sweep played %d, want the majority %d", n, hostSweep.GamesPlayed(), need)
		}
		if hostSweep.Winner() != SideHost {
			t.Errorf("bo %d host sweep winner = %v", n, hostSweep.Winner())
		}

		guestSweep := newReadySeries(t, n)
		for guestSweep.State() != SeriesFinished {
			o := BlueWins
			if guestSweep.RedUserID() == seriesGuestID {
				o = RedWins
			}
			if err := guestSweep.RecordResult(o); err != nil {
				t.Fatalf("bo %d guest sweep: %v", n, err)
			}
		}
		seriesScoreIs(t, guestSweep, 0, need)
		if guestSweep.GamesPlayed() != need || guestSweep.Winner() != SideGuest {
			t.Errorf("bo %d guest sweep played %d winner %v", n, guestSweep.GamesPlayed(), guestSweep.Winner())
		}

		allDraw := newReadySeries(t, n)
		for i := 0; i < n; i++ {
			if err := allDraw.RecordResult(Draw); err != nil {
				t.Fatalf("bo %d draw %d: %v", n, i, err)
			}
		}
		seriesScoreIs(t, allDraw, 0, 0)
		if allDraw.State() != SeriesFinished || allDraw.Winner() != SideNone || allDraw.GamesPlayed() != n {
			t.Errorf("bo %d all-draw: state %v winner %v played %d", n, allDraw.State(), allDraw.Winner(), allDraw.GamesPlayed())
		}
	}
}

func TestSeriesForfeitFromReady(t *testing.T) {
	hostQuit := newReadySeries(t, config.SeriesLengths[0])
	if err := hostQuit.Forfeit(seriesHostID); err != nil {
		t.Fatalf("host Forfeit: %v", err)
	}
	wantHost := []GameResult{
		{1, seriesHostID, seriesGuestID, BlueWins},
		{2, seriesHostID, seriesGuestID, BlueWins},
		{3, seriesHostID, seriesGuestID, BlueWins},
	}
	if got := hostQuit.SyntheticGames(); len(got) != len(wantHost) {
		t.Fatalf("host quit synthetic count = %d, want %d", len(got), len(wantHost))
	} else {
		for i, g := range got {
			if g != wantHost[i] {
				t.Errorf("host quit synthetic[%d] = %+v, want %+v", i, g, wantHost[i])
			}
		}
	}
	seriesScoreIs(t, hostQuit, 0, 3)
	if hostQuit.State() != SeriesFinished || hostQuit.Winner() != SideGuest {
		t.Errorf("host quit: state %v winner %v, want finished guest", hostQuit.State(), hostQuit.Winner())
	}
	if hostQuit.GamesPlayed() != config.SeriesLengths[0] {
		t.Errorf("host quit played %d, want %d", hostQuit.GamesPlayed(), config.SeriesLengths[0])
	}

	guestQuit := newReadySeries(t, config.SeriesLengths[0])
	if err := guestQuit.Forfeit(seriesGuestID); err != nil {
		t.Fatalf("guest Forfeit: %v", err)
	}
	wantGuest := []GameResult{
		{1, seriesHostID, seriesGuestID, RedWins},
		{2, seriesGuestID, seriesHostID, BlueWins},
		{3, seriesGuestID, seriesHostID, BlueWins},
	}
	if got := guestQuit.SyntheticGames(); len(got) != len(wantGuest) {
		t.Fatalf("guest quit synthetic count = %d, want %d", len(got), len(wantGuest))
	} else {
		for i, g := range got {
			if g != wantGuest[i] {
				t.Errorf("guest quit synthetic[%d] = %+v, want %+v", i, g, wantGuest[i])
			}
		}
	}
	seriesScoreIs(t, guestQuit, 3, 0)
	if guestQuit.State() != SeriesFinished || guestQuit.Winner() != SideHost {
		t.Errorf("guest quit: state %v winner %v, want finished host", guestQuit.State(), guestQuit.Winner())
	}
	for _, s := range []*Series{hostQuit, guestQuit} {
		if err := s.RecordResult(Draw); !errors.Is(err, ErrSeriesFinished) {
			t.Errorf("result after forfeit: err = %v, want ErrSeriesFinished", err)
		}
		if err := s.Forfeit(seriesHostID); !errors.Is(err, ErrSeriesFinished) {
			t.Errorf("forfeit after forfeit: err = %v, want ErrSeriesFinished", err)
		}
	}
}

func TestSeriesForfeitMidSeries(t *testing.T) {
	s := newReadySeries(t, config.SeriesLengths[0])
	if err := s.RecordResult(RedWins); err != nil { // host red wins game 1, guest takes red
		t.Fatalf("game 1: %v", err)
	}
	if err := s.Forfeit(seriesGuestID); err != nil {
		t.Fatalf("guest Forfeit at 1-0: %v", err)
	}
	want := []GameResult{
		{2, seriesGuestID, seriesHostID, BlueWins},
		{3, seriesGuestID, seriesHostID, BlueWins},
	}
	if got := s.SyntheticGames(); len(got) != len(want) {
		t.Fatalf("synthetic count = %d, want %d", len(got), len(want))
	} else {
		for i, g := range got {
			if g != want[i] {
				t.Errorf("synthetic[%d] = %+v, want %+v", i, g, want[i])
			}
		}
	}
	seriesScoreIs(t, s, 3, 0)
	if s.State() != SeriesFinished || s.Winner() != SideHost {
		t.Errorf("state %v winner %v, want finished host", s.State(), s.Winner())
	}
	if s.GamesPlayed() != config.SeriesLengths[0] {
		t.Errorf("played %d, want %d: forfeit charges every remaining game", s.GamesPlayed(), config.SeriesLengths[0])
	}
}

func TestSeriesForfeitFromCreated(t *testing.T) {
	s, err := NewSeries(seriesHostID, seriesGuestID, 0, config.SeriesLengths[0])
	if err != nil {
		t.Fatalf("NewSeries: %v", err)
	}
	if err := s.Ready(seriesHostID); err != nil {
		t.Fatalf("host Ready: %v", err)
	}
	if err := s.Forfeit(seriesHostID); err != nil {
		t.Fatalf("Forfeit from Created: %v", err)
	}
	seriesScoreIs(t, s, 0, config.SeriesLengths[0])
	if s.State() != SeriesFinished || s.Winner() != SideGuest {
		t.Errorf("state %v winner %v, want finished guest", s.State(), s.Winner())
	}
	if got := s.SyntheticGames(); len(got) != config.SeriesLengths[0] {
		t.Errorf("synthetic count = %d, want %d", len(got), config.SeriesLengths[0])
	}
}

func TestSeriesForfeitErrors(t *testing.T) {
	s := newReadySeries(t, config.SeriesLengths[0])
	if err := s.Forfeit(seriesOutsiderID); !errors.Is(err, ErrNotParticipant) {
		t.Errorf("outsider Forfeit: err = %v, want ErrNotParticipant", err)
	}
	if s.State() != SeriesReady {
		t.Error("rejected forfeit moved the state")
	}
	if err := s.RecordResult(RedWins); err != nil {
		t.Fatalf("game 1: %v", err)
	}
	if err := s.RecordResult(BlueWins); err != nil {
		t.Fatalf("game 2: %v", err)
	}
	if err := s.Forfeit(seriesGuestID); !errors.Is(err, ErrSeriesFinished) {
		t.Errorf("forfeit after natural finish: err = %v, want ErrSeriesFinished", err)
	}
}

func TestSeriesInvalidOutcome(t *testing.T) {
	s := newReadySeries(t, config.SeriesLengths[0])
	if err := s.RecordResult(Outcome(9)); !errors.Is(err, ErrInvalidOutcome) {
		t.Errorf("err = %v, want ErrInvalidOutcome", err)
	}
	if s.State() != SeriesReady || s.GamesPlayed() != 0 {
		t.Error("invalid outcome mutated the series")
	}
}

func TestSeriesConcurrentDrive(t *testing.T) {
	boLen := config.SeriesLengths[0]
	s, err := NewSeries(seriesHostID, seriesGuestID, 0, boLen)
	if err != nil {
		t.Fatalf("NewSeries: %v", err)
	}
	var wg sync.WaitGroup
	for w := 0; w < 8; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			outcome := Outcome(w % 3)
			for i := 0; i < 50; i++ {
				var err error
				switch i % 3 {
				case 0:
					err = s.Ready(seriesHostID)
				case 1:
					err = s.Ready(seriesGuestID)
				default:
					err = s.RecordResult(outcome)
				}
				switch {
				case err == nil,
					errors.Is(err, ErrSeriesFinished),
					errors.Is(err, ErrNotReady),
					errors.Is(err, ErrInvalidOutcome):
				default:
					t.Errorf("worker %d op %d: unexpected err %v", w, i, err)
				}
			}
		}(w)
	}
	wg.Wait()
	if s.State() != SeriesFinished {
		t.Fatalf("state = %v, want SeriesFinished", s.State())
	}
	if s.GamesPlayed() != boLen {
		t.Errorf("games played = %d, want %d", s.GamesPlayed(), boLen)
	}
	hostWins, guestWins := s.Score()
	if hostWins+guestWins > boLen || hostWins > s.WinsNeeded() || guestWins > s.WinsNeeded() {
		t.Errorf("natural finish bounds broken: %d-%d of %d", hostWins, guestWins, boLen)
	}
	if len(s.SyntheticGames()) != 0 {
		t.Error("no forfeit happened, synthetic games must stay empty")
	}
	if red := s.RedUserID(); red != seriesHostID && red != seriesGuestID {
		t.Errorf("red = %d, want a participant", red)
	}
}

func TestSeriesStateString(t *testing.T) {
	for _, c := range []struct {
		st   SeriesState
		want string
	}{{SeriesCreated, "created"}, {SeriesReady, "ready"}, {SeriesInGame, "in-game"}, {SeriesFinished, "finished"}, {SeriesState(9), "unknown"}} {
		if got := c.st.String(); got != c.want {
			t.Errorf("SeriesState(%d).String() = %q, want %q", int(c.st), got, c.want)
		}
	}
	for _, c := range []struct {
		sd   SeriesSide
		want string
	}{{SideHost, "host"}, {SideGuest, "guest"}, {SideNone, "none"}, {SeriesSide(9), "unknown"}} {
		if got := c.sd.String(); got != c.want {
			t.Errorf("SeriesSide(%d).String() = %q, want %q", int(c.sd), got, c.want)
		}
	}
}
