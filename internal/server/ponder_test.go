package server

import (
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

func TestGrantDepthRingEmptyHasNoReference(t *testing.T) {
	var ring GrantDepthRing
	if ref, ok := ring.ReferenceDepth(1_000_000); ok || ref != 0 {
		t.Fatalf("empty ring reference = (%d, %t), want (0, false)", ref, ok)
	}
}

func TestGrantDepthRingExactGrantCompares(t *testing.T) {
	var ring GrantDepthRing
	ring.Append(1_000_000, 12)
	if ref, ok := ring.ReferenceDepth(1_000_000); !ok || ref != 12 {
		t.Fatalf("same-grant reference = (%d, %t), want (12, true)", ref, ok)
	}
}

func TestGrantDepthRingBandBoundaries(t *testing.T) {
	var ring GrantDepthRing
	ring.Append(500, 5)
	ring.Append(2000, 7)
	ring.Append(499, 9)
	ring.Append(2001, 11)
	if ref, ok := ring.ReferenceDepth(1000); !ok || ref != 7 {
		t.Fatalf("band reference = (%d, %t), want the max in-band depth (7, true)", ref, ok)
	}
	if _, ok := ring.ReferenceDepth(501); !ok {
		t.Error("grant 501 lost every comparable: 500 sits in [250.5, 1002]")
	}
	if _, ok := ring.ReferenceDepth(1999); !ok {
		t.Error("grant 1999 lost every comparable: 2000 sits in [999.5, 3998]")
	}
}

func TestGrantDepthRingEvictsOldestAtCapacity(t *testing.T) {
	var ring GrantDepthRing
	ring.Append(100, 42)
	for range config.PonderDepthHistory {
		ring.Append(100, 10)
	}
	if ref, ok := ring.ReferenceDepth(100); !ok || ref != 10 {
		t.Fatalf("reference after the wrap = (%d, %t), want the oldest entry evicted so (10, true)", ref, ok)
	}
}

func TestGrantDepthRingZeroGrantBand(t *testing.T) {
	var ring GrantDepthRing
	ring.Append(0, 4)
	ring.Append(1, 9)
	if ref, ok := ring.ReferenceDepth(0); !ok || ref != 4 {
		t.Fatalf("zero-grant reference = (%d, %t), want only the zero-grant entry (4, true)", ref, ok)
	}
}
