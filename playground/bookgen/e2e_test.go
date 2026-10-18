package main

import (
	"bufio"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/book"
	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

func TestSeatTierResolvesAndClearsTheBook(t *testing.T) {
	m, err := seatTier(config.TierMaster.Name)
	if err != nil {
		t.Fatalf("seatTier master: %v", err)
	}
	if m.Book {
		t.Errorf("corpus seat keeps the book flag, want the artifact out of the corpus loop")
	}
	if m.Cores != config.TierMaster.Cores || m.TTBytes != config.TierMaster.TTBytes || m.VCF != config.TierMaster.VCF || m.VCT != config.TierMaster.VCT {
		t.Errorf("corpus seat perturbs the master shape: %+v over %+v", m, config.TierMaster)
	}
	if _, err := seatTier("no-such-tier"); err == nil {
		t.Errorf("seatTier accepted an unknown tier")
	}
}

func TestEndToEndSmokeChain(t *testing.T) {
	if testing.Short() {
		t.Skip("the smoke chain drives two real engine games")
	}
	tier, err := seatTier(config.TierEasy.Name)
	if err != nil {
		t.Fatalf("seatTier easy: %v", err)
	}
	dir := t.TempDir()
	corpus := filepath.Join(dir, "c.txt")
	var mlines bytes.Buffer
	play := func(idx, redSeat int) gameResult {
		return playCorpusGame(tier, 0, idx, redSeat, &mlines)
	}
	if err := runCorpus(corpusOpts{path: corpus, tier: config.TierEasy.Name, tc: 0, games: 2}, play, io.Discard); err != nil {
		t.Fatalf("runCorpus: %v", err)
	}
	sc, err := loadCorpus(corpus)
	if err != nil {
		t.Fatalf("loadCorpus: %v", err)
	}
	if sc.games != 2 || sc.torn {
		t.Fatalf("corpus scan = %+v, want 2 complete games", sc)
	}
	if len(sc.recs) < 2 || len(sc.recs) > 2*config.BookMaxPly {
		t.Fatalf("corpus records = %d, want between one and %d", len(sc.recs), 2*config.BookMaxPly)
	}
	if !bytes.Contains(mlines.Bytes(), []byte("M1, Red, ")) {
		t.Errorf("engine moves emitted no M-lines, buffer holds %d bytes", mlines.Len())
	}
	overlay := filepath.Join(dir, "o.txt")
	if err := overlayRun(overlayOpts{corpusPath: corpus, outPath: overlay, floor: 1, budget: 1 << 12}, solverProber(1<<12), io.Discard); err != nil {
		t.Fatalf("overlayRun: %v", err)
	}
	out := filepath.Join(dir, "b.carobook")
	stats := filepath.Join(dir, "s.txt")
	st, err := emitRun(emitOpts{
		corpusPath: corpus, overlayPath: overlay, outPath: out, statsPath: stats,
		floor: 1, created: "2026-10-18T00:00:00Z",
	}, io.Discard)
	if err != nil {
		t.Fatalf("emitRun: %v", err)
	}
	if st.Entries < 1 {
		t.Fatalf("smoke emitted %d entries, want at least the root position", st.Entries)
	}
	f, err := os.Open(out)
	if err != nil {
		t.Fatalf("open artifact: %v", err)
	}
	defer func() { _ = f.Close() }()
	bk, err := book.Load(f)
	if err != nil {
		t.Fatalf("book.Load rejected the smoke artifact: %v", err)
	}
	if bk.Len() != st.Entries {
		t.Fatalf("artifact holds %d entries, emit reported %d", bk.Len(), st.Entries)
	}
	counts := make(map[[config.BookKeyBytes]byte]map[rules.Move]int)
	for _, rec := range sc.recs {
		if counts[rec.key] == nil {
			counts[rec.key] = make(map[rules.Move]int)
		}
		counts[rec.key][rec.move]++
	}
	proofs := make(map[[config.BookKeyBytes]byte]rules.Move)
	pf, err := os.Open(overlay)
	if err != nil {
		t.Fatalf("open overlay for the expected moves: %v", err)
	}
	scan := bufio.NewScanner(pf)
	for scan.Scan() {
		fields := strings.Fields(scan.Text())
		if len(fields) != 5 || fields[0] != "P" {
			continue
		}
		k, err := keyFromHex(fields[1])
		if err != nil {
			t.Fatalf("overlay key in the expected-move scan: %v", err)
		}
		mv, err := strconv.Atoi(fields[2])
		if err != nil || mv < 0 || mv >= config.BoardCells {
			t.Fatalf("overlay move %q in the expected-move scan: %v", fields[2], err)
		}
		proofs[k] = rules.Move(mv)
	}
	if err := scan.Err(); err != nil {
		t.Fatalf("scan overlay: %v", err)
	}
	_ = pf.Close()
	if st.Entries != len(counts) {
		t.Fatalf("emit reported %d entries over %d distinct corpus positions at floor 1", st.Entries, len(counts))
	}
	for key, per := range counts {
		want, bestN := 0, -1
		for mv, n := range per {
			if n > bestN || n == bestN && int(mv) < want {
				want, bestN = int(mv), n
			}
		}
		if p, ok := proofs[key]; ok {
			want = int(p)
		}
		b, err := boardFromKey(key)
		if err != nil {
			t.Fatalf("boardFromKey on a corpus record: %v", err)
		}
		mv, ok := bk.Probe(b)
		if !ok {
			t.Fatalf("floor-passing key %s missing from the artifact", hexKey(key[:]))
		}
		if int(mv) != want {
			t.Fatalf("key %s probed move %d, want the emitted move %d", hexKey(key[:]), mv, want)
		}
		if mv >= config.BoardCells || b.At(rules.Cell(mv)) != rules.Empty {
			t.Fatalf("emitted move %d is off the board or occupied in its position", mv)
		}
	}
	if len(proofs) != st.Solver {
		t.Fatalf("overlay holds %d proofs but emit counted %d solver entries at floor 1", len(proofs), st.Solver)
	}
	if _, err := os.Stat(stats); err != nil {
		t.Fatalf("stats artifact missing beside the book: %v", err)
	}
}
