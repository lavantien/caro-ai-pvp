package uci

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestUCIHandlerUCI(t *testing.T) {
	var buf bytes.Buffer
	h := NewUCIHandler(nil, &buf)
	h.HandleCommand("uci")
	output := buf.String()
	assert.Contains(t, output, "id name Caro AI")
	assert.Contains(t, output, "uciok")
}

func TestUCIHandlerIsReady(t *testing.T) {
	var buf bytes.Buffer
	h := NewUCIHandler(nil, &buf)
	h.HandleCommand("isready")
	assert.Contains(t, buf.String(), "readyok")
}

func TestUCIHandlerPosition(t *testing.T) {
	var buf bytes.Buffer
	h := NewUCIHandler(nil, &buf)
	h.HandleCommand("position startpos moves aa")
	assert.Equal(t, "red", h.Board().GetPlayerAt(0, 0).String())
}

func TestUCIHandlerNewGame(t *testing.T) {
	var buf bytes.Buffer
	h := NewUCIHandler(nil, &buf)
	h.HandleCommand("position startpos moves aa")
	h.HandleCommand("ucinewgame")
	assert.Equal(t, "none", h.Board().GetPlayerAt(0, 0).String())
}

// waitForBestmove polls until the async search goroutine has answered.
func waitForBestmove(t *testing.T, h *UCIHandler, buf *threadsafeBuffer) string {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if s := buf.String(); strings.Contains(s, "bestmove ") {
			return s
		}
		time.Sleep(10 * time.Millisecond)
	}
	return buf.String()
}

func TestUCIHandlerGoMovetime(t *testing.T) {
	buf := newThreadsafeBuffer()
	h := NewUCIHandler(nil, buf)
	h.HandleCommand("go movetime 2000")
	output := waitForBestmove(t, h, buf)
	assert.Contains(t, output, "bestmove ")
	assert.Contains(t, output, "info ")
}

func TestUCIHandlerGoWtime(t *testing.T) {
	buf := newThreadsafeBuffer()
	h := NewUCIHandler(nil, buf)
	h.HandleCommand("go wtime 20000 btime 20000")
	output := waitForBestmove(t, h, buf)
	assert.Contains(t, output, "bestmove ")
}

func TestUCIHandlerStop(t *testing.T) {
	var buf bytes.Buffer
	h := NewUCIHandler(nil, &buf)
	h.HandleCommand("stop")
	// Should not panic even with no active search
	assert.Empty(t, buf.String())
}

func TestUCIHandlerSetOption(t *testing.T) {
	var buf bytes.Buffer
	h := NewUCIHandler(nil, &buf)
	h.HandleCommand("setoption name Threads value 8")
	// Should not panic or output anything
	assert.Empty(t, buf.String())
}

func TestUCIHandlerQuit(t *testing.T) {
	var buf bytes.Buffer
	h := NewUCIHandler(nil, &buf)
	h.HandleCommand("quit")
}

func TestUCIHandlerEmpty(t *testing.T) {
	var buf bytes.Buffer
	h := NewUCIHandler(nil, &buf)
	h.HandleCommand("")
	assert.Empty(t, buf.String())
}

func TestRunUCILoop(t *testing.T) {
	input := "uci\nisready\nquit\n"
	reader := bytes.NewBufferString(input)
	var buf bytes.Buffer
	h := NewUCIHandler(nil, &buf)
	RunUCILoop(h, reader)
	output := buf.String()
	assert.Contains(t, output, "uciok")
	assert.Contains(t, output, "readyok")
}

func TestRunUCILoopSkipsEmpty(t *testing.T) {
	input := "uci\n\n\nisready\nquit\n"
	reader := bytes.NewBufferString(input)
	var buf bytes.Buffer
	h := NewUCIHandler(nil, &buf)
	RunUCILoop(h, reader)
	output := buf.String()
	assert.Contains(t, output, "uciok")
}
