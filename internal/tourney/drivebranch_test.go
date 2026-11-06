package tourney

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

func TestResumeFailsWhenRunLogDirIsAFile(t *testing.T) {
	ts, _ := newTestStore(t)
	pointLogsAt(t)
	ctx := context.Background()
	run, err := ts.CreateRun(ctx, 0, config.SeriesBO3, config.TournamentStartRating, roster(2), "test", "")
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	target := RunDirName(run.Label, time.Unix(run.CreatedAt, 0))
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("occupied"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := NewConductor(&fakeSource{script: easySweeps}).Resume(ctx, ts, run.ID, 1); err == nil {
		t.Fatal("resume must fail when the run log directory path is a file")
	}
}
