package server

import (
	"slices"
	"testing"
)

func TestMovePairs(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   []string
		want []MovePair
	}{
		{"nil", nil, nil},
		{"empty", []string{}, nil},
		{"single odd tail", []string{"D4"}, []MovePair{{N: 1, Red: "D4", RedLatest: true}}},
		{"one full pair", []string{"D4", "P16"}, []MovePair{{N: 1, Red: "D4", Blue: "P16", BlueLatest: true}}},
		{"odd tail after pairs", []string{"D4", "P16", "H8"}, []MovePair{
			{N: 1, Red: "D4", Blue: "P16"},
			{N: 2, Red: "H8", RedLatest: true},
		}},
		{"numbering climbs and only the final blue is latest", []string{"D4", "P16", "H8", "P12", "D5", "P8"}, []MovePair{
			{N: 1, Red: "D4", Blue: "P16"},
			{N: 2, Red: "H8", Blue: "P12"},
			{N: 3, Red: "D5", Blue: "P8", BlueLatest: true},
		}},
		{"final red latest on odd count", []string{"D4", "P16", "H8", "P12", "D5"}, []MovePair{
			{N: 1, Red: "D4", Blue: "P16"},
			{N: 2, Red: "H8", Blue: "P12"},
			{N: 3, Red: "D5", RedLatest: true},
		}},
	} {
		if got := movePairs(tc.in); !slices.Equal(got, tc.want) {
			t.Errorf("%s: movePairs(%v) = %+v, want %+v", tc.name, tc.in, got, tc.want)
		}
	}
}
