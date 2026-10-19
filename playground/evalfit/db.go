package main

import (
	"database/sql"
	"encoding/binary"
	"fmt"
	"path/filepath"
	"sort"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
	_ "modernc.org/sqlite"
)

type dbSeries struct {
	Key   string
	Games []parsedGame
}

type dbLoad struct {
	Series []dbSeries
}

func readDSN(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	return "file:" + filepath.ToSlash(abs) + "?mode=ro"
}

func decodeMoves(blob []byte, gameID int64) ([]gameMove, error) {
	if len(blob)%2 != 0 {
		return nil, fmt.Errorf("game %d moves blob holds %d bytes, not uint16 cells", gameID, len(blob))
	}
	moves := make([]gameMove, 0, len(blob)/2)
	color := rules.Red
	for i := 0; i < len(blob); i += 2 {
		cell := binary.LittleEndian.Uint16(blob[i:])
		if int(cell) >= config.BoardCells {
			return nil, fmt.Errorf("game %d move %d cell %d outside the %d-cell board", gameID, len(moves)+1, cell, config.BoardCells)
		}
		moves = append(moves, gameMove{Color: color, Cell: rules.Cell(cell)})
		color = color.Opponent()
	}
	return moves, nil
}

func outcomeOfColumn(outcome string, gameID int64) (gameOutcome, error) {
	switch outcome {
	case "red":
		return outcomeRedWin, nil
	case "blue":
		return outcomeBlueWin, nil
	case "draw":
		return outcomeDraw, nil
	}
	return 0, fmt.Errorf("game %d outcome %q is neither red, blue, nor draw", gameID, outcome)
}

func loadDBGames(path string, tiers map[string]bool) (dbLoad, error) {
	db, err := sql.Open("sqlite", readDSN(path))
	if err != nil {
		return dbLoad{}, fmt.Errorf("evalfit: open %s: %w", path, err)
	}
	defer db.Close()
	rows, err := db.Query(`
		SELECT g.id, g.series_id, g.outcome, g.moves,
			pr.name, pr.tier, pb.name, pb.tier
		FROM tournament_games g
		JOIN tournament_participants pr ON pr.run_id = g.run_id AND pr.slot = g.red_slot
		JOIN tournament_participants pb ON pb.run_id = g.run_id AND pb.slot = g.blue_slot
		ORDER BY g.series_id, g.idx_in_series, g.id`)
	if err != nil {
		return dbLoad{}, fmt.Errorf("evalfit: query games in %s: %w", path, err)
	}
	defer rows.Close()
	out := dbLoad{}
	var cur *dbSeries
	for rows.Next() {
		var gameID, seriesID int64
		var outcome string
		var blob []byte
		var redName, redTier, blueName, blueTier string
		if err := rows.Scan(&gameID, &seriesID, &outcome, &blob, &redName, &redTier, &blueName, &blueTier); err != nil {
			return dbLoad{}, fmt.Errorf("evalfit: scan game in %s: %w", path, err)
		}
		if len(tiers) > 0 && !(tiers[redTier] && tiers[blueTier]) {
			continue
		}
		oc, err := outcomeOfColumn(outcome, gameID)
		if err != nil {
			return dbLoad{}, fmt.Errorf("evalfit: %s: %w", path, err)
		}
		moves, err := decodeMoves(blob, gameID)
		if err != nil {
			return dbLoad{}, fmt.Errorf("evalfit: %s: %w", path, err)
		}
		if cur == nil || cur.Key != seriesKey(path, seriesID) {
			out.Series = append(out.Series, dbSeries{Key: seriesKey(path, seriesID)})
			cur = &out.Series[len(out.Series)-1]
		}
		cur.Games = append(cur.Games, parsedGame{
			Red:     seat{Name: redName, Tier: redTier},
			Blue:    seat{Name: blueName, Tier: blueTier},
			Outcome: oc,
			Moves:   moves,
		})
	}
	if err := rows.Err(); err != nil {
		return dbLoad{}, fmt.Errorf("evalfit: iterate games in %s: %w", path, err)
	}
	return out, nil
}

func seriesKey(path string, seriesID int64) string {
	return fmt.Sprintf("%s#%d", path, seriesID)
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
