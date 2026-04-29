package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPositionIsValid(t *testing.T) {
	tests := []struct {
		name     string
		pos      Position
		expected bool
	}{
		{"origin", Position{X: 0, Y: 0}, true},
		{"center", Position{X: 8, Y: 8}, true},
		{"corner", Position{X: 15, Y: 15}, true},
		{"negative_x", Position{X: -1, Y: 0}, false},
		{"over_y", Position{X: 0, Y: 16}, false},
		{"both_over", Position{X: 16, Y: 16}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.pos.IsValid())
		})
	}
}

func TestPositionOffset(t *testing.T) {
	p := Position{X: 5, Y: 5}
	assert.Equal(t, Position{X: 6, Y: 7}, p.Offset(1, 2))
}
