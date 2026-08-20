package uci

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"caro-ai-pvp/internal/domain"
	"caro-ai-pvp/internal/engine"

	"github.com/stretchr/testify/assert"
)

func TestNotationRoundTripAllCells(t *testing.T) {
	for x := range domain.BoardSize {
		for y := range domain.BoardSize {
			s := MoveToString(x, y)
			px, py, ok := ParseMove(s)
			assert.True(t, ok, "ParseMove(%s) failed", s)
			assert.Equal(t, x, px, "x mismatch for %s", s)
			assert.Equal(t, y, py, "y mismatch for %s", s)
		}
	}
}

// The search must not block the command loop: stop arrives while the engine
// is thinking and bestmove comes back promptly. movetime is deliberately huge
// because the time manager only spends a fraction of the remaining clock.
func TestStopInterruptsActiveSearch(t *testing.T) {
	buf := newThreadsafeBuffer()
	h := NewUCIHandler(nil, buf)
	h.HandleCommand("go movetime 600000")

	time.Sleep(300 * time.Millisecond)
	assert.False(t, strings.Contains(buf.String(), "bestmove"),
		"precondition: search should still be running before stop")

	start := time.Now()
	h.HandleCommand("stop")

	for time.Now().Before(start.Add(2*time.Second)) && !strings.Contains(buf.String(), "bestmove") {
		time.Sleep(10 * time.Millisecond)
	}
	assert.True(t, strings.Contains(buf.String(), "bestmove"), "stop must produce bestmove")
	assert.Less(t, time.Since(start), 2*time.Second, "stop must interrupt promptly")
}

func TestPositionRejectsBadMove(t *testing.T) {
	var buf bytes.Buffer
	h := NewUCIHandler(nil, &buf)
	h.HandleCommand("position startpos moves aa zz bb")

	assert.Contains(t, buf.String(), "info string error", "bad move must be reported")
	assert.Equal(t, "none", h.Board().GetPlayerAt(0, 0).String(),
		"a command with an invalid move must not be partially applied")
}

func TestParseGoOptionsClocksBySide(t *testing.T) {
	args := []string{"wtime", "1000", "btime", "200000", "winc", "3000", "binc", "0"}
	opts := parseGoOptions(args, domain.PlayerRed, engine.SearchOptions{})
	assert.Equal(t, int64(1000), opts.TimeRemainingMs, "red must use wtime")
	assert.Equal(t, int64(3000), opts.IncrementMs, "red must use winc")

	opts = parseGoOptions(args, domain.PlayerBlue, engine.SearchOptions{})
	assert.Equal(t, int64(200000), opts.TimeRemainingMs, "blue must use btime")
	assert.Equal(t, int64(0), opts.IncrementMs, "blue must use binc")
}

func TestParseGoOptionsMovetimeAndDepth(t *testing.T) {
	opts := parseGoOptions([]string{"movetime", "500"}, domain.PlayerRed, engine.SearchOptions{})
	assert.Equal(t, int64(500), opts.TimeRemainingMs)

	opts = parseGoOptions([]string{"depth", "6"}, domain.PlayerRed, engine.SearchOptions{})
	assert.Equal(t, 6, opts.MaxDepth)
}

func TestSkillLevelChangesStrengthProfile(t *testing.T) {
	var buf bytes.Buffer
	h := NewUCIHandler(nil, &buf)
	h.HandleCommand("setoption name Skill Level value 2")
	assert.Equal(t, 2, h.SkillLevel(), "skill option must be stored")

	profile := engine.GetDifficultyProfile(2)
	assert.Equal(t, profile.MaxDepth, h.skillSearchOptions().MaxDepth,
		"search options must honor the configured skill level")
}

// threadsafeBuffer allows the search goroutine to write while the test reads.
type threadsafeBuffer struct {
	mu  chan struct{}
	buf bytes.Buffer
}

func (b *threadsafeBuffer) Write(p []byte) (int, error) {
	<-b.mu
	n, err := b.buf.Write(p)
	b.mu <- struct{}{}
	return n, err
}

func (b *threadsafeBuffer) String() string {
	<-b.mu
	s := b.buf.String()
	b.mu <- struct{}{}
	return s
}

func newThreadsafeBuffer() *threadsafeBuffer {
	b := &threadsafeBuffer{mu: make(chan struct{}, 1)}
	b.mu <- struct{}{}
	return b
}
