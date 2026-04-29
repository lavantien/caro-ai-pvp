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
