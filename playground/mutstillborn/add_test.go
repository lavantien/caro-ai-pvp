package mutstillborn

import "testing"

func TestAdd(t *testing.T) {
	if Add(2, 3) != 5 {
		t.Fatal("Add(2, 3) = got wrong sum, want 5")
	}
}
