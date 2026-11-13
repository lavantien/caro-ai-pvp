//go:build e2e

package tourney

import (
	"context"
	"database/sql"
	"math"
	"path/filepath"
	"testing"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/geom"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
	"github.com/lavantien/caro-ai-pvp/internal/server"
)

func newHexRunStack(t *testing.T) (RoomSource, *Store, *server.Store) {
	t.Helper()
	srv, err := server.Open(filepath.Join(t.TempDir(), "caro.sqlite"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	wq := server.NewWriteQueue(nil)
	hub := server.NewHub()
	rm := server.NewRoomManager(hub, srv, wq)
	rm.SetMemoryProbe(func() (uint64, error) { return math.MaxUint64, nil })
	t.Cleanup(func() {
		rm.Shutdown()
		wq.Close()
		_ = srv.Close()
		hub.Close()
	})
	return RoomSource{RM: rm}, NewStore(srv), srv
}

func TestHexMiniTournamentRecordsVariantAndDecodes(t *testing.T) {
	src, ts, srv := newHexRunStack(t)
	pointLogsAt(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	roster := []Participant{
		{Slot: 0, Name: "hex-easy-a", Tier: config.TierEasy.Name},
		{Slot: 1, Name: "hex-easy-b", Tier: config.TierEasy.Name},
	}
	res, err := NewConductor(src).Run(ctx, ts, roster,
		mustTC(1, 0), config.SeriesBO3, config.TournamentStartRating, 1, "hextest", "hex")
	if err != nil {
		t.Fatalf("hex run: %v", err)
	}

	run, err := ts.Run(ctx, res.Run.ID)
	if err != nil {
		t.Fatalf("reread run: %v", err)
	}
	if run.Variant != "hex" {
		t.Errorf("run variant = %q, want hex", run.Variant)
	}
	if run.Status != RunStateFinished {
		t.Errorf("run status = %q, want finished", run.Status)
	}

	var blobs [][]byte
	err = srv.WithinTx(ctx, func(tx *sql.Tx) error {
		rows, qerr := tx.QueryContext(ctx,
			`SELECT moves FROM tournament_games WHERE run_id = ? ORDER BY id`, res.Run.ID)
		if qerr != nil {
			return qerr
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var blob []byte
			if serr := rows.Scan(&blob); serr != nil {
				return serr
			}
			blobs = append(blobs, blob)
		}
		return rows.Err()
	})
	if err != nil || len(blobs) == 0 {
		t.Fatalf("hex games read = %d blobs err %v", len(blobs), err)
	}
	hexGeom, err := geom.ByName("hex")
	if err != nil {
		t.Fatalf("hex geometry: %v", err)
	}
	for i, blob := range blobs {
		if _, derr := server.DecodeMoves(blob, geom.Default()); derr == nil {
			t.Errorf("game %d decoded under the square geometry, want rejection", i+1)
		}
		moves, derr := server.DecodeMovesForVariant(blob, run.Variant)
		if derr != nil {
			t.Fatalf("game %d hex decode: %v", i+1, derr)
		}
		b := rules.NewBoardFor(hexGeom)
		for j, mv := range moves {
			if !b.IsLegal(rules.Cell(mv)) {
				t.Fatalf("game %d move %d (%d) illegal on the hex board", i+1, j+1, mv)
			}
			b.Make(rules.Cell(mv))
		}
	}
}
