package uci

import (
	"bytes"
	"testing"

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

func TestUCIHandlerGoMovetime(t *testing.T) {
	var buf bytes.Buffer
	h := NewUCIHandler(nil, &buf)
	h.HandleCommand("go movetime 100")
	output := buf.String()
	assert.Contains(t, output, "bestmove ")
	assert.Contains(t, output, "info ")
}

func TestUCIHandlerGoWtime(t *testing.T) {
	var buf bytes.Buffer
	h := NewUCIHandler(nil, &buf)
	h.HandleCommand("go wtime 5000 btime 5000")
	output := buf.String()
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
