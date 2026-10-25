package server

import "slices"

type moveItemView struct {
	N         int
	Idx       int
	Name      string
	SeatClass string
	Latest    bool
}

func moveItems(names []string) []moveItemView {
	if len(names) == 0 {
		return nil
	}
	out := make([]moveItemView, len(names))
	for i, name := range names {
		out[i] = moveItemView{N: i + 1, Idx: i, Name: name, SeatClass: seatClassOfMove(i),
			Latest: i == len(names)-1}
	}
	return out
}

func newestMoveItems(names []string) []moveItemView {
	items := moveItems(names)
	slices.Reverse(items)
	return items
}

func seatClassOfMove(i int) string {
	if i%2 == 0 {
		return "s-red"
	}
	return "s-blue"
}
