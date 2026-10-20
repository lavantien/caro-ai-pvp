package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

func fuzzStoneKey(cell int) [config.BookKeyBytes]byte {
	b := rules.NewBoard()
	b.Make(rules.Cell(cell))
	return keyOf(b)
}

func FuzzLoadCorpus(f *testing.F) {
	k0 := fuzzStoneKey(0)
	fp := corpusFingerprint(config.TierEasy.Name, 0, 1)
	seeds := [][]byte{
		nil,
		[]byte(fp),
		[]byte(fp + "\n"),
		[]byte(fp + "\nG 0\n"),
		[]byte(fp + "\nG 0\nP " + hexKey(k0[:]) + " 7"),
		[]byte(fp + "\nG 0\nP " + hexKey(k0[:]) + " 7\nE 1 0\n"),
		[]byte(fp + "\nG 0\nE 2 0\n"),
		[]byte(fp + "\n\n\n"),
		[]byte(fp + "\nQ 1\n"),
		[]byte("G 0\n"),
		[]byte("F corpus v3 tier=easy tc=0 games=1\n"),
		[]byte("F corpus v1 tier=easy tc=x games=1\n"),
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > config.FuzzInputMaxBytes {
			return
		}
		path := filepath.Join(t.TempDir(), "c.txt")
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatalf("write corpus: %v", err)
		}
		sc, err := loadCorpus(path)
		if err != nil {
			return
		}
		if sc.clean < 0 || sc.clean > int64(len(data)) {
			t.Fatalf("clean offset %d outside the %d-byte file", sc.clean, len(data))
		}
		if len(data) > 0 && !bytes.Contains(data, []byte{'\n'}) {
			if sc.fp != "" || sc.clean != 0 || !sc.torn {
				t.Fatalf("newline-free file scanned as fp %q clean %d torn %t, want no clean data", sc.fp, sc.clean, sc.torn)
			}
		}
		if sc.games < 0 || sc.illegal < 0 {
			t.Fatalf("negative counts in %+v", sc)
		}
		again, err := loadCorpus(path)
		if err != nil {
			t.Fatalf("reread failed after a clean scan: %v", err)
		}
		if again.games != sc.games || again.clean != sc.clean || again.torn != sc.torn || again.fp != sc.fp || again.illegal != sc.illegal || len(again.recs) != len(sc.recs) {
			t.Fatalf("nondeterministic scan: %+v then %+v", sc, again)
		}
	})
}

func FuzzReadOverlay(f *testing.F) {
	k := fuzzStoneKey(4)
	fp := overlayFingerprint(corpusFingerprint(config.TierEasy.Name, 0, 1), config.BookVisitFloor, 0)
	seeds := [][]byte{
		nil,
		[]byte(fp),
		[]byte(fp + "\n"),
		[]byte(fp + "\nP " + hexKey(k[:]) + " 11 42 vcf\n"),
		[]byte(fp + "\nP " + hexKey(k[:]) + " 11 42 vcf"),
		[]byte(fp + "\nN " + hexKey(k[:]) + "\n"),
		[]byte(fp + "\nP " + hexKey(k[:]) + " 11 42 vct\nP " + hexKey(k[:]) + " 9 9 vcf\n"),
		[]byte(fp + "\nP " + hexKey(k[:]) + " 11 x vcf\n"),
		[]byte(fp + "\nP " + hexKey(k[:]) + " 11 42 vx\n"),
		[]byte(fp + "\nQ x\n"),
		[]byte("F overlay v3 floor=1 budget=1\n"),
		[]byte("P " + hexKey(k[:]) + " 11 42 vcf\n"),
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > config.FuzzInputMaxBytes {
			return
		}
		path := filepath.Join(t.TempDir(), "o.txt")
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatalf("write overlay: %v", err)
		}
		sc, err := readOverlay(path)
		if err != nil {
			return
		}
		if sc.clean < 0 || sc.clean > int64(len(data)) {
			t.Fatalf("clean offset %d outside the %d-byte file", sc.clean, len(data))
		}
		if len(data) > 0 && !bytes.Contains(data, []byte{'\n'}) {
			if sc.fp != "" || sc.clean != 0 || !sc.torn {
				t.Fatalf("newline-free file scanned as fp %q clean %d torn %t, want no clean data", sc.fp, sc.clean, sc.torn)
			}
		}
		if strings.ContainsRune(sc.fp, '\n') {
			t.Fatalf("fingerprint %q spans a newline", sc.fp)
		}
		again, err := readOverlay(path)
		if err != nil {
			t.Fatalf("reread failed after a clean scan: %v", err)
		}
		if again.clean != sc.clean || again.torn != sc.torn || again.fp != sc.fp || len(again.done) != len(sc.done) || len(again.proofs) != len(sc.proofs) {
			t.Fatalf("nondeterministic scan: %+v then %+v", sc, again)
		}
	})
}
