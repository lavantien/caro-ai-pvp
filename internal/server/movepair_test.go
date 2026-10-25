package server

import (
	"slices"
	"testing"
)

func TestMoveItems(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   []string
		want []moveItemView
	}{
		{"nil", nil, nil},
		{"empty", []string{}, nil},
		{"single red move", []string{"D4"}, []moveItemView{
			{N: 1, Idx: 0, Name: "D4", SeatClass: "s-red", Latest: true},
		}},
		{"seats alternate and only the final move is latest", []string{"D4", "P16", "H8"}, []moveItemView{
			{N: 1, Idx: 0, Name: "D4", SeatClass: "s-red"},
			{N: 2, Idx: 1, Name: "P16", SeatClass: "s-blue"},
			{N: 3, Idx: 2, Name: "H8", SeatClass: "s-red", Latest: true},
		}},
	} {
		if got := moveItems(tc.in); !slices.Equal(got, tc.want) {
			t.Errorf("%s: moveItems(%v) = %+v, want %+v", tc.name, tc.in, got, tc.want)
		}
	}
}

func TestNewestMoveItemsLeadWithTheLastMove(t *testing.T) {
	if got := newestMoveItems(nil); got != nil {
		t.Errorf("newestMoveItems(nil) = %+v, want nil", got)
	}
	got := newestMoveItems([]string{"D4", "P16", "H8"})
	want := []moveItemView{
		{N: 3, Idx: 2, Name: "H8", SeatClass: "s-red", Latest: true},
		{N: 2, Idx: 1, Name: "P16", SeatClass: "s-blue"},
		{N: 1, Idx: 0, Name: "D4", SeatClass: "s-red"},
	}
	if !slices.Equal(got, want) {
		t.Errorf("newestMoveItems = %+v, want %+v", got, want)
	}
}
