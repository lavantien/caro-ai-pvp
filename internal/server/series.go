package server

import (
	"errors"
	"slices"
	"sync"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

// SeriesState is the lifecycle of one best-of series: Created until both
// players ready, Ready once the handshake completes and game 1 goes live,
// InGame while later games run, Finished after the majority falls, the
// schedule runs out, or a forfeit sweeps the remaining games.
type SeriesState int

const (
	SeriesCreated SeriesState = iota
	SeriesReady
	SeriesInGame
	SeriesFinished
)

func (st SeriesState) String() string {
	switch st {
	case SeriesCreated:
		return "created"
	case SeriesReady:
		return "ready"
	case SeriesInGame:
		return "in-game"
	case SeriesFinished:
		return "finished"
	default:
		return "unknown"
	}
}

// SeriesSide names the holder of a series-level result.
type SeriesSide int

// SideNone is the zero value so a live or drawn series never reports a
// phantom winner.
const (
	SideNone SeriesSide = iota
	SideHost
	SideGuest
)

func (sd SeriesSide) String() string {
	switch sd {
	case SideHost:
		return "host"
	case SideGuest:
		return "guest"
	case SideNone:
		return "none"
	default:
		return "unknown"
	}
}

var (
	ErrBadTimeControl  = errors.New("server: time control index out of range")
	ErrBadSeriesLength = errors.New("server: series length not offered by config.SeriesLengths")
	ErrSamePlayer      = errors.New("server: host and guest must differ")
	ErrNotParticipant  = errors.New("server: user is not a participant of this series")
	ErrNotReady        = errors.New("server: both players must ready before match 1")
	ErrSeriesFinished  = errors.New("server: series already finished")
	ErrInvalidOutcome  = errors.New("server: outcome outside the enum")
)

// GameResult is one decided game in a series, colors resolved to users. The
// forfeit sweep publishes its synthetic games in this shape so the caller can
// feed rating events without re-deriving the red rotation.
type GameResult struct {
	GameNo     int
	RedUserID  int64
	BlueUserID int64
	Outcome    Outcome
}

// Series is the first-class state machine of one PvP best-of series per
// first-cause.md Scenario 1: the host takes red in game 1, the loser of a
// decisive game takes red in the next one while a draw lets red keep the
// seat, and the series ends when one side reaches the majority of
// boLen/2+1 wins or the schedule runs out (the plurality leader takes a
// majorityless series, equal wins leave it drawn). Bot series run the
// benchmark seating instead: both players alternately take red the same
// number of times, so red passes after every game, draws included. It
// carries no clocks and no persistence: the room layer owns the engine,
// the store owns timestamps.
type Series struct {
	mu           sync.Mutex
	host         int64
	guest        int64
	tcIdx        int
	boLen        int
	hostReady    bool
	guestReady   bool
	state        SeriesState
	gamesPlayed  int
	redIsHost    bool
	alternateRed bool
	hostWins     int
	guestWins    int
	winner       SeriesSide
	synthetic    []GameResult
}

// NewSeries validates the room settings against the config hubs and seats
// the host on red for game 1.
func NewSeries(hostUserID, guestUserID int64, tcIdx, boLen int) (*Series, error) {
	if hostUserID == guestUserID {
		return nil, ErrSamePlayer
	}
	if tcIdx < 0 || tcIdx >= len(config.TimeControls) {
		return nil, ErrBadTimeControl
	}
	if !slices.Contains(config.SeriesLengths[:], boLen) {
		return nil, ErrBadSeriesLength
	}
	return &Series{host: hostUserID, guest: guestUserID, tcIdx: tcIdx, boLen: boLen, redIsHost: true}, nil
}

// NewBotSeries is the benchmark and tournament seating: red alternates
// every game. Across a twice-paired round robin, where the mirrored
// pairing hosts the other way, each participant's red count balances.
func NewBotSeries(hostUserID, guestUserID int64, tcIdx, boLen int) (*Series, error) {
	s, err := NewSeries(hostUserID, guestUserID, tcIdx, boLen)
	if err != nil {
		return nil, err
	}
	s.alternateRed = true
	return s, nil
}

// Ready marks one participant ready. Double-ready is idempotent: a reconnect
// or retry must not fail the handshake. The second ready completes the
// handshake and game 1 goes live with the host on red.
func (s *Series) Ready(userID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state == SeriesFinished {
		return ErrSeriesFinished
	}
	switch userID {
	case s.host:
		s.hostReady = true
	case s.guest:
		s.guestReady = true
	default:
		return ErrNotParticipant
	}
	if s.state == SeriesCreated && s.hostReady && s.guestReady {
		s.state = SeriesReady
	}
	return nil
}

// RecordResult feeds the outcome of the live game and advances the machine.
// It fails before the handshake completes, after the series finished, and on
// an outcome outside the enum; the game index can never run past the series
// because every terminal schedule transition lands in SeriesFinished.
func (s *Series) RecordResult(outcome Outcome) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch outcome {
	case Draw, RedWins, BlueWins:
	default:
		return ErrInvalidOutcome
	}
	switch s.state {
	case SeriesCreated:
		return ErrNotReady
	case SeriesReady, SeriesInGame:
	case SeriesFinished:
		return ErrSeriesFinished
	}
	s.applyGame(outcome)
	s.settle()
	return nil
}

// Forfeit ends the series for a quitting participant: every remaining
// unplayed game, the live one included, is booked as a quitter loss, so the
// score line and the level reflect the full charge and the caller derives
// the per-game rating events straight from SyntheticGames. The sweep never
// stops at the majority, the spec bills every remaining game; the winner is
// then the plurality leader of the final line, which is the non-quitter
// unless draws held the quitter to an uncatchable lead.
func (s *Series) Forfeit(userID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state == SeriesFinished {
		return ErrSeriesFinished
	}
	if userID != s.host && userID != s.guest {
		return ErrNotParticipant
	}
	for s.gamesPlayed < s.boLen {
		red := s.redUserID()
		outcome := RedWins
		if red == userID {
			outcome = BlueWins
		}
		s.synthetic = append(s.synthetic, GameResult{
			GameNo:     s.gamesPlayed + 1,
			RedUserID:  red,
			BlueUserID: s.other(red),
			Outcome:    outcome,
		})
		s.applyGame(outcome)
	}
	s.winner = SideNone
	switch {
	case s.hostWins > s.guestWins:
		s.winner = SideHost
	case s.guestWins > s.hostWins:
		s.winner = SideGuest
	}
	s.state = SeriesFinished
	return nil
}

// applyGame books one decided game: the winner color banks a win. Under
// the PvP law red passes to the decisive loser while a draw holds the
// seat; under the bot law red passes after every game.
func (s *Series) applyGame(outcome Outcome) {
	switch outcome {
	case RedWins:
		if s.redIsHost {
			s.hostWins++
		} else {
			s.guestWins++
		}
		if !s.alternateRed {
			s.redIsHost = !s.redIsHost
		}
	case BlueWins:
		if s.redIsHost {
			s.guestWins++
		} else {
			s.hostWins++
		}
	}
	if s.alternateRed {
		s.redIsHost = !s.redIsHost
	}
	s.gamesPlayed++
}

// settle finishes the series on the majority or on schedule exhaustion,
// otherwise the next game goes live.
func (s *Series) settle() {
	need := s.boLen/2 + 1
	switch {
	case s.hostWins >= need:
		s.winner, s.state = SideHost, SeriesFinished
	case s.guestWins >= need:
		s.winner, s.state = SideGuest, SeriesFinished
	case s.gamesPlayed == s.boLen:
		s.state = SeriesFinished
		s.winner = SideNone
		switch {
		case s.hostWins > s.guestWins:
			s.winner = SideHost
		case s.guestWins > s.hostWins:
			s.winner = SideGuest
		}
	default:
		s.state = SeriesInGame
	}
}

func (s *Series) redUserID() int64 {
	if s.redIsHost {
		return s.host
	}
	return s.guest
}

func (s *Series) other(userID int64) int64 {
	if userID == s.host {
		return s.guest
	}
	return s.host
}

func (s *Series) State() SeriesState {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state
}

func (s *Series) HostUserID() int64 {
	return s.host
}

func (s *Series) GuestUserID() int64 {
	return s.guest
}

func (s *Series) TCIdx() int {
	return s.tcIdx
}

func (s *Series) BOLen() int {
	return s.boLen
}

// WinsNeeded is the majority that ends a series early.
func (s *Series) WinsNeeded() int {
	return s.boLen/2 + 1
}

func (s *Series) GamesPlayed() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.gamesPlayed
}

// Score is the current or final win line, host first. A forfeit can push the
// winner past WinsNeeded because the sweep bills every remaining game.
func (s *Series) Score() (hostWins, guestWins int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.hostWins, s.guestWins
}

// RedUserID is the user holding red in the game awaiting a result; after a
// draw it is the same user as the game before.
func (s *Series) RedUserID() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.redUserID()
}

// Winner is valid once State reports SeriesFinished: the majority or
// plurality leader, SideNone when the schedule ended level.
func (s *Series) Winner() SeriesSide {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.winner
}

func (s *Series) HostReady() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.hostReady
}

func (s *Series) GuestReady() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.guestReady
}

// SyntheticGames are the forfeit's booked losses in game order, nil for a
// naturally played series.
func (s *Series) SyntheticGames() []GameResult {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.synthetic
}
