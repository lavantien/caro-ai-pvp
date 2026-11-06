package book

import "testing"

func TestNilBookLenIsZero(t *testing.T) {
	var bk *Book
	if got := bk.Len(); got != 0 {
		t.Fatalf("nil book Len = %d, want 0", got)
	}
}
