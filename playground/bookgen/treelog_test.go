package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

func treeLogPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "book-tree.txt")
}

func TestTreeFingerprintRefusesDrift(t *testing.T) {
	path := treeLogPath(t)
	p := paramsFromConfig(config.TierMaster.Name)
	fp := p.fingerprint(8)
	clean, err := writeSidecarHeader(path, fp)
	if err != nil {
		t.Fatal(err)
	}
	tl, err := parseTreeLog(path)
	if err != nil {
		t.Fatal(err)
	}
	if tl.fp != fp {
		t.Fatalf("parsed fingerprint %q, want %q", tl.fp, fp)
	}
	if _, err := appendTreeLine(path, clean, formatNoProof([config.BookKeyBytes]byte{1}, 5)); err != nil {
		t.Fatal(err)
	}
	other := paramsFromConfig(config.TierGrandmaster.Name)
	if _, err := parseTreeLog(path); err == nil && other.fingerprint(16) != fp {
		t.Log("distinct fingerprints parse independently")
	}
	mismatch := paramsFromConfig(config.TierMaster.Name)
	mismatch.leaf += 1
	if mismatch.fingerprint(8) == fp {
		t.Fatal("leaf budget drift must change the fingerprint")
	}
	mismatch = paramsFromConfig(config.TierMaster.Name)
	mismatch.ladderGate += 1
	if mismatch.fingerprint(8) == fp {
		t.Fatal("ladder gate drift must change the fingerprint")
	}
	if p.fingerprint(16) == fp {
		t.Fatal("ply bound drift must change the fingerprint")
	}
}

func TestTreeLogRoundTripAllLineKinds(t *testing.T) {
	path := treeLogPath(t)
	p := paramsFromConfig(config.TierMaster.Name)
	clean, err := writeSidecarHeader(path, p.fingerprint(8))
	if err != nil {
		t.Fatal(err)
	}
	k1 := canonicalBoardKey(boardWithMoves(t, rules.Cell(119), rules.Cell(72)))
	k2 := canonicalBoardKey(boardWithMoves(t, rules.Cell(119), rules.Cell(72), rules.Cell(0)))
	ex := treeExpansion{ply: 2, own: true, bestGap: 111, children: []childRec{{move: 5, child: k2}}}
	clean, err = appendTreeLine(path, clean, formatExpansion(k1, ex))
	if err != nil {
		t.Fatal(err)
	}
	clean, err = appendTreeLine(path, clean, formatLeaf(k2, leafRec{budget: 1024, score: -35, best: 9, nodes: 4096}))
	if err != nil {
		t.Fatal(err)
	}
	clean, err = appendTreeLine(path, clean, formatSolver(k2, 12, "vcf", 777, 3))
	if err != nil {
		t.Fatal(err)
	}
	clean, err = appendTreeLine(path, clean, formatNoProof(k1, 55))
	if err != nil {
		t.Fatal(err)
	}
	clean, err = appendTreeLine(path, clean, formatFreeze(k2, "win", 1<<28))
	if err != nil {
		t.Fatal(err)
	}
	_, err = appendTreeLine(path, clean, formatReverify(reverifyRec{key: k1, smp: 40, delta: 12, flip: true}))
	if err != nil {
		t.Fatal(err)
	}
	tl, err := parseTreeLog(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := tl.expansions[k1]; got == nil || got.ply != 2 || !got.own || got.bestGap != 111 || len(got.children) != 1 || got.children[0].move != 5 || got.children[0].child != k2 {
		t.Errorf("expansion round trip = %+v", got)
	}
	if got := tl.analyses[k2]; len(got) != 1 || got[0].budget != 1024 || got[0].score != -35 || got[0].best != 9 || got[0].nodes != 4096 {
		t.Errorf("leaf round trip = %+v", got)
	}
	if got := tl.solver[k2]; got.move != 12 || got.kind != "vcf" || got.pvLen != 3 {
		t.Errorf("solver round trip = %+v", got)
	}
	if _, ok := tl.solverDone[k1]; !ok {
		t.Error("no-proof done marker lost")
	}
	if got := tl.frozen[k2]; got.reason != "win" || got.score != 1<<28 {
		t.Errorf("freeze round trip = %+v", got)
	}
	if len(tl.reverify) != 1 || !tl.reverify[0].flip || tl.reverify[0].delta != 12 {
		t.Errorf("reverify round trip = %+v", tl.reverify)
	}
	if tl.spent != 4096+777+55 {
		t.Errorf("logged spend = %d, want %d", tl.spent, 4096+777+55)
	}
}

func TestTreeLogTornTailTruncatesToLastCompleteLine(t *testing.T) {
	path := treeLogPath(t)
	p := paramsFromConfig(config.TierMaster.Name)
	clean, err := writeSidecarHeader(path, p.fingerprint(8))
	if err != nil {
		t.Fatal(err)
	}
	k := [config.BookKeyBytes]byte{7}
	clean, err = appendTreeLine(path, clean, formatNoProof(k, 5))
	if err != nil {
		t.Fatal(err)
	}
	torn := formatNoProof([config.BookKeyBytes]byte{9}, 6)
	f, err := os.OpenFile(path, os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteAt([]byte(torn[:len(torn)-3]), clean); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	tl, err := parseTreeLog(path)
	if err != nil {
		t.Fatal(err)
	}
	if !tl.torn {
		t.Error("torn tail must be flagged")
	}
	if _, ok := tl.solverDone[[config.BookKeyBytes]byte{9}]; ok {
		t.Error("partial line must not survive the parse")
	}
	if _, ok := tl.solverDone[k]; !ok {
		t.Error("complete line before the tear must survive")
	}
	if tl.clean != clean {
		t.Errorf("clean offset = %d, want %d", tl.clean, clean)
	}
}

func TestTreeLogRejectsMalformedLines(t *testing.T) {
	cases := []string{
		"Q something\n",
		"X deadbeef\n",
		"X " + strings.Repeat("00", 64) + " 2 own -1 5:" + strings.Repeat("00", 64) + "\n",
		"X " + strings.Repeat("00", 64) + " 2 sideways 5:" + strings.Repeat("00", 64) + "\n",
		"L " + strings.Repeat("00", 64) + " 0 1 2 3\n",
		"F " + strings.Repeat("00", 64) + " mystery 1\n",
		"S " + strings.Repeat("00", 64) + " xyz 1 2\n",
	}
	for _, line := range cases {
		path := treeLogPath(t)
		p := paramsFromConfig(config.TierMaster.Name)
		clean, err := writeSidecarHeader(path, p.fingerprint(8))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := appendTreeLine(path, clean, line); err != nil {
			t.Fatal(err)
		}
		if _, err := parseTreeLog(path); err == nil {
			t.Errorf("line %q must be rejected", strings.TrimSpace(line))
		}
	}
}

func TestTreeLogMissingFileIsEmpty(t *testing.T) {
	tl, err := parseTreeLog(filepath.Join(t.TempDir(), "absent-tree.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if tl.fp != "" || len(tl.expansions) != 0 {
		t.Error("absent tree log must parse as empty")
	}
}
