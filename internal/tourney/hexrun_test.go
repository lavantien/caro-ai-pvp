package tourney

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

func TestSeriesHeaderCarriesTheVariant(t *testing.T) {
	red := Participant{Slot: 0, Name: "easy-1", Tier: "easy"}
	blue := Participant{Slot: 1, Name: "medium-1", Tier: "medium"}

	hexDir := t.TempDir()
	g := NewLogs(hexDir)
	if err := g.WriteSeriesHeader(1, 1, 0, config.SeriesBO3, red, blue, "hex"); err != nil {
		t.Fatalf("hex header: %v", err)
	}
	if err := g.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	text := readFirstLog(t, hexDir)
	if !strings.Contains(text, "tc 1+0 bo3 hex\n") {
		t.Errorf("hex header tc line = %q, want the variant token", firstLineWith(text, "tc "))
	}

	plainDir := t.TempDir()
	h := NewLogs(plainDir)
	if err := h.WriteSeriesHeader(1, 1, 0, config.SeriesBO3, red, blue, ""); err != nil {
		t.Fatalf("default header: %v", err)
	}
	if err := h.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	plain := readFirstLog(t, plainDir)
	if line := firstLineWith(plain, "tc "); line != "tc 1+0 bo3" {
		t.Errorf("default header tc line = %q, want no variant token", line)
	}
}

func firstLineWith(text, prefix string) string {
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, prefix) {
			return line
		}
	}
	return ""
}

func readFirstLog(t *testing.T, dir string) string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) == 0 {
		t.Fatalf("log dir %s read = %v entries %d", dir, err, len(entries))
	}
	data, err := os.ReadFile(filepath.Join(dir, entries[0].Name()))
	if err != nil {
		t.Fatalf("log read: %v", err)
	}
	return string(data)
}
