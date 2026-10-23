package server

import (
	"errors"
	"slices"
	"sync"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

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

type SeriesSide int

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
	ErrUnknownVariant  = errors.New("server: unknown board variant")
)

type GameResult struct {
	GameNo     int
	RedUserID  int64
	BlueUserID int64
	Outcome    Outcome
}

type Series struct {
	mu           sync.Mutex
	host         int64
	guest        int64
	tcIdx        int
	boLen        int
	variant      string
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

func normalizeVariant(variant string) (string, error) {
	if variant == "" {
		return config.DefaultVariant, nil
	}
	for i := range config.Variants {
		if config.Variants[i].Name == variant {
			return variant, nil
		}
	}
	return "", ErrUnknownVariant
}

func NewSeries(hostUserID, guestUserID int64, tcIdx, boLen int) (*Series, error) {
	return NewSeriesFor(hostUserID, guestUserID, tcIdx, boLen, config.DefaultVariant)
}

func NewSeriesFor(hostUserID, guestUserID int64, tcIdx, boLen int, variant string) (*Series, error) {
	if hostUserID == guestUserID {
		return nil, ErrSamePlayer
	}
	if tcIdx < 0 || tcIdx >= len(config.TimeControls) {
		return nil, ErrBadTimeControl
	}
	if !slices.Contains(config.SeriesLengths[:], boLen) {
		return nil, ErrBadSeriesLength
	}
	v, err := normalizeVariant(variant)
	if err != nil {
		return nil, err
	}
	return &Series{host: hostUserID, guest: guestUserID, tcIdx: tcIdx, boLen: boLen, variant: v, redIsHost: true}, nil
}

func NewBotSeries(hostUserID, guestUserID int64, tcIdx, boLen int) (*Series, error) {
	return NewBotSeriesFor(hostUserID, guestUserID, tcIdx, boLen, config.DefaultVariant)
}

func NewBotSeriesFor(hostUserID, guestUserID int64, tcIdx, boLen int, variant string) (*Series, error) {
	s, err := NewSeriesFor(hostUserID, guestUserID, tcIdx, boLen, variant)
	if err != nil {
		return nil, err
	}
	s.alternateRed = true
	return s, nil
}

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

func (s *Series) Variant() string {
	return s.variant
}

func (s *Series) WinsNeeded() int {
	return s.boLen/2 + 1
}

func (s *Series) GamesPlayed() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.gamesPlayed
}

func (s *Series) Score() (hostWins, guestWins int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.hostWins, s.guestWins
}

func (s *Series) RedUserID() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.redUserID()
}

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

func (s *Series) SyntheticGames() []GameResult {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.synthetic
}
