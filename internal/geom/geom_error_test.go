package geom

import (
	"errors"
	"strings"
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

func TestNameErrorCarriesTheGeomPrefix(t *testing.T) {
	_, err := Square(config.BoardSize).CellName(-1)
	var ne *NameError
	if !errors.As(err, &ne) {
		t.Fatalf("CellName(-1) = %T, want *NameError", err)
	}
	msg := ne.Error()
	if !strings.HasPrefix(msg, "geom: ") || !strings.Contains(msg, "out of 0..") {
		t.Fatalf("NameError message = %q, want the geom-prefixed range text", msg)
	}
	if _, err := Hex(12).CellName(0); err == nil {
		t.Fatal("hex CellName(0) accepted, want the region error")
	}
}
