package tourney

import (
	"context"
	"strings"
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/server"
)

func TestFinishRunFencesForeignDrives(t *testing.T) {
	ts, _ := newTestStore(t)
	ctx := context.Background()
	run, err := ts.CreateRun(ctx, 1, config.SeriesBO3, config.TournamentStartRating, roster(2), "test", "")
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	err = ts.FinishRun(ctx, run.ID, 100)
	if err == nil || !strings.Contains(err.Error(), "unfinished series") {
		t.Fatalf("FinishRun over open series = %v, want unfinished refusal", err)
	}
	for _, s := range mustSchedule(t, ts.srv, run.ID) {
		for i := 0; i < config.SeriesBO3; i++ {
			mustAppend(t, ts, gameOf(run.ID, s.ID, i, s.RedFirstSlot, s.BlueFirstSlot, server.RedWins))
		}
	}
	other := NewStore(ts.srv)
	if err := other.ClaimDrive(ctx, run.ID, 150); err != nil {
		t.Fatalf("foreign claim: %v", err)
	}
	err = ts.FinishRun(ctx, run.ID, 160)
	if err == nil || !strings.Contains(err.Error(), "taken over") {
		t.Fatalf("FinishRun against a foreign lease = %v, want takeover refusal", err)
	}
	if err := other.ReleaseDrive(ctx, run.ID); err != nil {
		t.Fatalf("foreign release: %v", err)
	}
	if err := ts.ClaimDrive(ctx, run.ID, 200); err != nil {
		t.Fatalf("claim drive: %v", err)
	}
	if err := ts.FinishRun(ctx, run.ID, 300); err != nil {
		t.Fatalf("FinishRun with a lease: %v", err)
	}
	err = ts.FinishRun(ctx, run.ID, 400)
	if err == nil || !strings.Contains(err.Error(), "already") {
		t.Fatalf("FinishRun on a finished run = %v, want already refusal", err)
	}
}

func TestDriveHeartbeatTracksTheLease(t *testing.T) {
	ts, _ := newTestStore(t)
	ctx := context.Background()
	run, err := ts.CreateRun(ctx, 1, config.SeriesBO3, config.TournamentStartRating, roster(2), "test", "")
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	if err := ts.ClaimDrive(ctx, run.ID, 100); err != nil {
		t.Fatalf("claim drive: %v", err)
	}
	alive, err := ts.BeatDrive(ctx, run.ID, 150)
	if err != nil || !alive {
		t.Fatalf("BeatDrive on a held lease = (%t, %v), want true nil", alive, err)
	}
	if err := ts.ReleaseDrive(ctx, run.ID); err != nil {
		t.Fatalf("release drive: %v", err)
	}
	other := NewStore(ts.srv)
	if err := other.ClaimDrive(ctx, run.ID, 200); err != nil {
		t.Fatalf("claim after release failed: %v", err)
	}
	alive, err = ts.BeatDrive(ctx, run.ID, 250)
	if err != nil || alive {
		t.Fatalf("BeatDrive after a foreign claim = (%t, %v), want false nil", alive, err)
	}
}

func TestResetSeriesRefusesSettledSeries(t *testing.T) {
	ts, _ := newTestStore(t)
	ctx := context.Background()
	run, err := ts.CreateRun(ctx, 1, config.SeriesBO3, config.TournamentStartRating, roster(2), "test", "")
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	series := mustSchedule(t, ts.srv, run.ID)
	target := series[0]
	for i := 0; i < config.SeriesBO3; i++ {
		mustAppend(t, ts, gameOf(run.ID, target.ID, i, target.RedFirstSlot, target.BlueFirstSlot, server.RedWins))
	}
	err = ts.ResetSeries(ctx, target.ID)
	if err == nil || !strings.Contains(err.Error(), "settled") {
		t.Fatalf("ResetSeries on a settled series = %v, want settled refusal", err)
	}
	if err := ts.ResetSeries(ctx, 999); err == nil {
		t.Fatal("ResetSeries on an unknown series succeeded")
	}
}

func TestResumeRejectsUnknownRuns(t *testing.T) {
	ts, _ := newTestStore(t)
	c := NewConductor(nil)
	if _, err := c.Resume(context.Background(), ts, 999, 1); err == nil {
		t.Fatal("Resume on an unknown run succeeded")
	}
}

func TestVariantConfiguredMatchesRegistry(t *testing.T) {
	if variantConfigured("no-such-variant") {
		t.Fatal("variantConfigured accepted an unknown variant")
	}
	if !variantConfigured(config.Variants[0].Name) {
		t.Fatalf("variantConfigured rejected %s", config.Variants[0].Name)
	}
}
