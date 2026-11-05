package server

import (
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

func TestNewBotSearcherForBuildsEngine(t *testing.T) {
	for _, variant := range config.Variants {
		s := NewBotSearcherFor(variant.Name, config.TierEasy)
		if s == nil {
			t.Fatalf("NewBotSearcherFor(%s) = nil", variant.Name)
		}
	}
}
