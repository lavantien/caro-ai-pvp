package server

import (
	"errors"
	"slices"
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

// FuzzRatingLaw pins the per-match rating law on every input of its domain.
// K = 10^((R_loser-R_winner)/D) with D >= RatingDecayMin > 0, so K < 1
// strictly on the favorite branch (winner pre-rated above the loser: delta
// in [0, RatingDelta]) and K >= 1 on the upset branch (delta >= RatingDelta,
// growing with the gap). A blanket |delta| <= RatingDelta is a false
// property: an upset gap of 1500 already earns 95. Zero-sum, pool
// conservation, pre+delta afters, inert draws, the color-swap mirror, and
// the documented panic on outcomes outside the enum all hold for every
// input.
//
// Encoding: data[0:2] rRed, data[2:4] rBlue, both little-endian int16 taken
// modulo 10001 into [-10000, 10000]. The worst product the law reaches at
// the 20000 max gap is RatingDelta*10^(20000/RatingDivisorFull) ~= 1.4e8,
// far under MaxInt64, so math.Round and the int conversion stay exact and
// the zero-sum assertions are arithmetic, not wraparound. data[4] selects
// the outcome: 0 draw, 1 red, 2 blue, 3 an Outcome outside the enum.
func FuzzRatingLaw(f *testing.F) {
	seeds := [][]byte{
		{0, 0, 0, 0, 0},
		{0, 0, 0, 0, 1},
		{0, 0, 21, 0, 1},
		{0, 0, 22, 0, 1},
		{44, 12, 0, 0, 1},
		{16, 39, 1, 3, 2},
		{15, 216, 0, 0, 2},
		{1, 4, 0, 0, 3},
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) < 5 {
			return
		}
		rRed := int(int16(uint16(data[0])|uint16(data[1])<<8)) % 10001
		rBlue := int(int16(uint16(data[2])|uint16(data[3])<<8)) % 10001
		if data[4]%4 == 3 {
			outcome := Outcome(3 + (int(data[4])>>2)%200)
			func() {
				defer func() {
					if recover() == nil {
						t.Fatalf("outcome %d outside the enum must panic", int(outcome))
					}
				}()
				RatingDeltas(rRed, rBlue, outcome)
			}()
			return
		}
		outcome := [...]Outcome{Draw, RedWins, BlueWins}[data[4]%4]
		dRed, dBlue, afterRed, afterBlue := RatingDeltas(rRed, rBlue, outcome)
		if outcome == Draw && (dRed != 0 || dBlue != 0) {
			t.Fatalf("draw %d/%d moved the ratings by %d/%d", rRed, rBlue, dRed, dBlue)
		}
		if dRed != -dBlue {
			t.Fatalf("%d/%d/%v: deltas %d/%d break zero-sum", rRed, rBlue, outcome, dRed, dBlue)
		}
		if afterRed != rRed+dRed || afterBlue != rBlue+dBlue {
			t.Fatalf("%d/%d/%v: afters %d/%d are not pre plus delta", rRed, rBlue, outcome, afterRed, afterBlue)
		}
		if afterRed+afterBlue != rRed+rBlue {
			t.Fatalf("%d/%d/%v: rating pool not conserved", rRed, rBlue, outcome)
		}
		if outcome == Draw {
			return
		}
		rWinner, rLoser, dWinner := rRed, rBlue, dRed
		mirror := BlueWins
		if outcome == BlueWins {
			rWinner, rLoser, dWinner = rBlue, rRed, dBlue
			mirror = RedWins
		}
		if rWinner <= rLoser {
			if dWinner < config.RatingDelta {
				t.Fatalf("upset %d over %d gained %d, under %d", rWinner, rLoser, dWinner, config.RatingDelta)
			}
		} else if dWinner < 0 || dWinner > config.RatingDelta {
			t.Fatalf("favorite %d over %d gained %d, want [0, %d]", rWinner, rLoser, dWinner, config.RatingDelta)
		}
		mDRed, mDBlue, mAfterRed, mAfterBlue := RatingDeltas(rBlue, rRed, mirror)
		if mDRed != dBlue || mDBlue != dRed || mAfterRed != afterBlue || mAfterBlue != afterRed {
			t.Fatalf("%d/%d/%v: color swap moved the law", rRed, rBlue, outcome)
		}
	})
}

// fuzzSeriesModel is the shadow replay of the documented series law, written
// from first-cause.md Scenario 1 rather than from series.go: host red in
// game 1, the decisive loser takes red while a draw retains it, the majority
// boLen/2+1 or the exhausted schedule finishes the series, and a forfeit
// bills every remaining game as a quitter loss. FuzzSeriesDrive asserts the
// Series machine against this model after every operation.
type fuzzSeriesModel struct {
	host, guest           int64
	boLen                 int
	readyHost, readyGuest bool
	state                 SeriesState
	played                int
	hostWins, guestWins   int
	redIsHost             bool
	winner                SeriesSide
	synth                 []GameResult
}

func (m *fuzzSeriesModel) redUser() int64 {
	if m.redIsHost {
		return m.host
	}
	return m.guest
}

func (m *fuzzSeriesModel) other(userID int64) int64 {
	if userID == m.host {
		return m.guest
	}
	return m.host
}

func (m *fuzzSeriesModel) ready(userID int64) error {
	if m.state == SeriesFinished {
		return ErrSeriesFinished
	}
	switch userID {
	case m.host:
		m.readyHost = true
	case m.guest:
		m.readyGuest = true
	default:
		return ErrNotParticipant
	}
	if m.state == SeriesCreated && m.readyHost && m.readyGuest {
		m.state = SeriesReady
	}
	return nil
}

func (m *fuzzSeriesModel) record(outcome Outcome) error {
	switch outcome {
	case Draw, RedWins, BlueWins:
	default:
		return ErrInvalidOutcome
	}
	switch m.state {
	case SeriesCreated:
		return ErrNotReady
	case SeriesReady, SeriesInGame:
	case SeriesFinished:
		return ErrSeriesFinished
	}
	m.apply(outcome)
	m.settle()
	return nil
}

func (m *fuzzSeriesModel) forfeit(userID int64) error {
	if m.state == SeriesFinished {
		return ErrSeriesFinished
	}
	if userID != m.host && userID != m.guest {
		return ErrNotParticipant
	}
	for m.played < m.boLen {
		red := m.redUser()
		outcome := RedWins
		if red == userID {
			outcome = BlueWins
		}
		m.synth = append(m.synth, GameResult{
			GameNo: m.played + 1, RedUserID: red, BlueUserID: m.other(red), Outcome: outcome,
		})
		m.apply(outcome)
	}
	m.winner = SideNone
	switch {
	case m.hostWins > m.guestWins:
		m.winner = SideHost
	case m.guestWins > m.hostWins:
		m.winner = SideGuest
	}
	m.state = SeriesFinished
	return nil
}

// apply books one decided game: the winner color banks the win, red passes
// to the decisive loser (a RedWins loss under the current red) and survives
// a BlueWins loss, a draw moves nothing.
func (m *fuzzSeriesModel) apply(outcome Outcome) {
	switch outcome {
	case RedWins:
		if m.redIsHost {
			m.hostWins++
		} else {
			m.guestWins++
		}
		m.redIsHost = !m.redIsHost
	case BlueWins:
		if m.redIsHost {
			m.guestWins++
		} else {
			m.hostWins++
		}
	}
	m.played++
}

func (m *fuzzSeriesModel) settle() {
	need := m.boLen/2 + 1
	switch {
	case m.hostWins >= need:
		m.winner, m.state = SideHost, SeriesFinished
	case m.guestWins >= need:
		m.winner, m.state = SideGuest, SeriesFinished
	case m.played == m.boLen:
		m.state = SeriesFinished
		m.winner = SideNone
		switch {
		case m.hostWins > m.guestWins:
			m.winner = SideHost
		case m.guestWins > m.hostWins:
			m.winner = SideGuest
		}
	default:
		m.state = SeriesInGame
	}
}

// FuzzSeriesDrive replays a random but reproducible operation script from
// construction through Finish and asserts, after every step, the full
// invariant set: played and win bounds, the frozen post-finish state, the
// terminal condition, synthetic-game accounting, the red rotation, and
// error-for-error agreement with the shadow model.
//
// Encoding: data[0] picks boLen from config.SeriesLengths, data[1] picks
// tcIdx from config.TimeControls, then one operation per byte, b%10:
// 0..2 Ready host/guest/outsider, 3..5 RecordResult draw/red/blue, 6
// RecordResult of an Outcome outside the enum, 7..9 Forfeit host/guest/
// outsider.
func FuzzSeriesDrive(f *testing.F) {
	seeds := [][]byte{
		{0, 0, 3, 4, 4},
		{0, 0, 0, 1, 3, 3, 3},
		{0, 0, 4, 7},
		{0, 0, 2, 9, 7},
		{0, 0, 5, 5, 5, 5},
		{1, 1, 0, 1, 4, 4, 4, 4, 4, 4, 4},
		{2, 0, 4, 4, 4},
		{3, 2, 6, 6, 6, 8, 0, 0},
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) < 2 {
			return
		}
		boLen := config.SeriesLengths[int(data[0])%len(config.SeriesLengths)]
		tcIdx := int(data[1]) % len(config.TimeControls)
		s, err := NewSeries(seriesHostID, seriesGuestID, tcIdx, boLen)
		if err != nil {
			t.Fatalf("NewSeries bo %d tc %d: %v", boLen, tcIdx, err)
		}
		m := fuzzSeriesModel{host: seriesHostID, guest: seriesGuestID, boLen: boLen, redIsHost: true}
		for i, b := range data[2:] {
			var got, want error
			switch b % 10 {
			case 0:
				got, want = s.Ready(m.host), m.ready(m.host)
			case 1:
				got, want = s.Ready(m.guest), m.ready(m.guest)
			case 2:
				got, want = s.Ready(seriesOutsiderID), m.ready(seriesOutsiderID)
			case 3:
				got, want = s.RecordResult(Draw), m.record(Draw)
			case 4:
				got, want = s.RecordResult(RedWins), m.record(RedWins)
			case 5:
				got, want = s.RecordResult(BlueWins), m.record(BlueWins)
			case 6:
				oc := Outcome(3 + int(b)%200)
				got, want = s.RecordResult(oc), m.record(oc)
			case 7:
				got, want = s.Forfeit(m.host), m.forfeit(m.host)
			case 8:
				got, want = s.Forfeit(m.guest), m.forfeit(m.guest)
			case 9:
				got, want = s.Forfeit(seriesOutsiderID), m.forfeit(seriesOutsiderID)
			}
			if !errors.Is(got, want) {
				t.Fatalf("op %d (b=%d): err = %v, model wants %v", i, b, got, want)
			}
			hostWins, guestWins := s.Score()
			played := s.GamesPlayed()
			if st := s.State(); st != m.state {
				t.Fatalf("op %d (b=%d): state %v, model %v", i, b, st, m.state)
			}
			if played != m.played || played > boLen {
				t.Fatalf("op %d (b=%d): played %d, model %d of %d", i, b, played, m.played, boLen)
			}
			if hostWins != m.hostWins || guestWins != m.guestWins {
				t.Fatalf("op %d (b=%d): score %d-%d, model %d-%d", i, b, hostWins, guestWins, m.hostWins, m.guestWins)
			}
			if hostWins+guestWins > played {
				t.Fatalf("op %d (b=%d): wins %d-%d exceed %d played", i, b, hostWins, guestWins, played)
			}
			if w := s.Winner(); w != m.winner {
				t.Fatalf("op %d (b=%d): winner %v, model %v", i, b, w, m.winner)
			}
			if synth := s.SyntheticGames(); !slices.Equal(synth, m.synth) {
				t.Fatalf("op %d (b=%d): synthetic %v, model %v", i, b, synth, m.synth)
			}
			red := s.RedUserID()
			if red != m.host && red != m.guest {
				t.Fatalf("op %d (b=%d): red %d is not a participant", i, b, red)
			}
			if red != m.redUser() {
				t.Fatalf("op %d (b=%d): red %d breaks the rotation, model %d", i, b, red, m.redUser())
			}
			if m.state == SeriesFinished {
				if hostWins < s.WinsNeeded() && guestWins < s.WinsNeeded() && played != boLen {
					t.Fatalf("op %d (b=%d): finished at %d-%d of %d without majority or full schedule",
						i, b, hostWins, guestWins, boLen)
				}
				if len(m.synth) > 0 && played != boLen {
					t.Fatalf("op %d (b=%d): forfeit billed %d of %d games", i, b, played, boLen)
				}
				leader := SideNone
				switch {
				case hostWins > guestWins:
					leader = SideHost
				case guestWins > hostWins:
					leader = SideGuest
				}
				if s.Winner() != leader {
					t.Fatalf("op %d (b=%d): winner %v is not the plurality leader of %d-%d",
						i, b, s.Winner(), hostWins, guestWins)
				}
			}
		}
	})
}

// FuzzMovesBlob pins the moves blob codec: two little-endian bytes per
// move, fixed stride, encode total, decode rejecting odd lengths and cells
// outside the board. Every byte pairing is a legal uint16 cell candidate, so
// the input is consumed raw: data[0:2] is move 1, data[2:4] move 2, and so
// on, a trailing odd byte is dropped.
func FuzzMovesBlob(f *testing.F) {
	seeds := [][]byte{
		{},
		{0},
		{0, 0},
		{1, 0, 2, 0, 3, 0},
		{255, 0, 0, 1},
		{255, 255},
		{16, 39, 16, 39, 240, 216},
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data)%2 != 0 {
			data = data[:len(data)-1]
		}
		moves := make([]rules.Move, 0, len(data)/2)
		inBoard := true
		for i := 0; i < len(data); i += 2 {
			mv := rules.Move(data[i]) | rules.Move(data[i+1])<<8
			moves = append(moves, mv)
			if int(mv) >= config.BoardCells {
				inBoard = false
			}
		}
		blob := EncodeMoves(nil, moves)
		if len(blob) != 2*len(moves) {
			t.Fatalf("%d moves encoded into %d bytes", len(moves), len(blob))
		}
		got, err := DecodeMoves(blob)
		if inBoard {
			if err != nil {
				t.Fatalf("in-board moves %v rejected: %v", moves, err)
			}
			if !slices.Equal(got, moves) {
				t.Fatalf("round-trip drift %v -> %v", moves, got)
			}
		} else if err == nil {
			t.Fatalf("out-of-board moves %v decoded without error", moves)
		}
		if len(blob) > 0 {
			if _, err := DecodeMoves(blob[:len(blob)-1]); err == nil {
				t.Fatalf("truncated blob %v decoded without error", blob)
			}
		}
	})
}
