package mutateproof

import "testing"

func TestFlag(t *testing.T) {
	if !Flag() {
		t.Fatal("Flag must be true")
	}
}
