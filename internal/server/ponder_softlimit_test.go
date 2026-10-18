package server

import (
	"testing"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/engine"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

func TestAssessSoftLimitBoundary(t *testing.T) {
	budget := int64(2 * time.Second)
	limit := int64(config.PonderAdoptFraction * float64(budget))
	if !AssessSoftLimit(limit, budget) {
		t.Errorf("elapsed exactly at the soft limit must cross")
	}
	if AssessSoftLimit(limit-1, budget) {
		t.Errorf("elapsed one nanosecond under the soft limit must not cross")
	}
	if !AssessSoftLimit(limit+1, budget) {
		t.Errorf("elapsed one nanosecond over the soft limit must cross")
	}
	if AssessSoftLimit(0, budget) {
		t.Errorf("zero elapsed must not cross a budget-sized soft limit")
	}
}

func TestAssessSoftLimitTruncationCorners(t *testing.T) {
	if !AssessSoftLimit(1, 3) {
		t.Error("odd budget 3 truncates the 1.5 fraction limit to 1, elapsed 1 must cross")
	}
	if AssessSoftLimit(0, 3) {
		t.Error("odd budget 3 truncates the 1.5 fraction limit to 1, elapsed 0 must not cross")
	}
	if !AssessSoftLimit(0, 1) {
		t.Error("budget 1 truncates the 0.5 fraction limit to 0, elapsed 0 must cross")
	}
}

func TestAdoptPonderConsumesSoftLimitMarker(t *testing.T) {
	mv := rules.Move(mustCellT(t, "H8"))
	budget := int64(2 * time.Second)
	var ring GrantDepthRing
	ring.Append(budget, 10)
	belowRef := stablePonderStats(t, 9, config.PonderStableIters, mv, []int{100, 100, 100})
	if AdoptPonder(belowRef, "", false, ring, budget) {
		t.Error("depth under the reference with the soft limit uncrossed must reject")
	}
	if !AdoptPonder(belowRef, "", true, ring, budget) {
		t.Error("the crossed marker must satisfy clause 1 without depth adequacy")
	}
	atRef := stablePonderStats(t, 10, config.PonderStableIters, mv, []int{100, 100, 100})
	if !AdoptPonder(atRef, "", false, ring, budget) {
		t.Error("depth at the reference must adopt regardless of the marker")
	}
	if !AdoptPonder(engine.SearchStats{}, config.BotLogTagVCF, false, ring, budget) {
		t.Error("the solver exemption must ignore the marker")
	}
}
