package main

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

type mergeOpts struct {
	shards  []string
	outPath string
	tier    string
	tc      int
	games   int
}

type shardOrigin struct {
	path string
	idx  int
}

var corpusIdentityFields = [...]string{"tier", "tc", "games"}

func shardFingerprintError(path, fp, want string) error {
	for _, key := range corpusIdentityFields {
		got, ok := fingerprintField(fp, key)
		expected, _ := fingerprintField(want, key)
		if !ok {
			return fmt.Errorf("bookgen: merge shard %s fingerprint misses %s=, want %s=%s under %q", path, key, key, expected, want)
		}
		if key == "games" {
			shardGames, err := strconv.Atoi(got)
			if err != nil {
				return fmt.Errorf("bookgen: merge shard %s fingerprint games=%s is not a number", path, got)
			}
			target, _ := strconv.Atoi(expected)
			if shardGames > target {
				return fmt.Errorf("bookgen: merge shard %s fingerprint games=%d exceeds the merge target %d, wrong-generation shard", path, shardGames, target)
			}
			continue
		}
		if got != expected {
			return fmt.Errorf("bookgen: merge shard %s fingerprint %s=%s, want %s=%s under %q", path, key, got, key, expected, want)
		}
	}
	return nil
}

func shardHeld(path string) error {
	lock := lockPath(path, "corpus")
	fi, err := os.Stat(lock)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("bookgen: merge shard lock %s: %w", lock, err)
	}
	if time.Since(fi.ModTime()) < time.Duration(config.BookLockStaleMs)*time.Millisecond {
		holder, _ := os.ReadFile(lock)
		return fmt.Errorf("bookgen: merge shard %s is held by running pid %s, merge refused", path, string(holder))
	}
	return nil
}

func sameFilePath(a, b string) bool {
	ab, aerr := filepath.Abs(a)
	bb, berr := filepath.Abs(b)
	if aerr != nil || berr != nil {
		return filepath.Clean(a) == filepath.Clean(b)
	}
	return strings.EqualFold(ab, bb)
}

func runMerge(o mergeOpts, progress io.Writer) error {
	want := corpusFingerprint(o.tier, o.tc, o.games)
	type shardScan struct {
		path string
		cf   corpusFile
	}
	scans := make([]shardScan, 0, len(o.shards))
	for _, shard := range o.shards {
		if sameFilePath(shard, o.outPath) {
			return fmt.Errorf("bookgen: merge output %s is also an input shard", o.outPath)
		}
		if err := shardHeld(shard); err != nil {
			return err
		}
		cf, err := parseCorpusFile(shard)
		if err != nil {
			return err
		}
		if cf.fp == "" {
			return fmt.Errorf("bookgen: merge shard %s holds no corpus fingerprint, regenerate it with -phase corpus", shard)
		}
		if cf.torn {
			return fmt.Errorf("bookgen: merge shard %s ends torn, let its member finish or repair it", shard)
		}
		if err := shardFingerprintError(shard, cf.fp, want); err != nil {
			return err
		}
		scans = append(scans, shardScan{path: shard, cf: cf})
		fmt.Fprintf(progress, "shard %s: %d games\n", shard, cf.games)
	}
	seen := make(map[[sha256.Size]byte]shardOrigin)
	kept := make([]gameBlock, 0, len(scans))
	raw := 0
	dups := 0
	for i := range scans {
		for j, blk := range scans[i].cf.blocks {
			raw++
			if origin, ok := seen[blk.sig]; ok {
				dups++
				fmt.Fprintf(progress, "duplicate game skipped: %s game %d repeats %s game %d\n", scans[i].path, j, origin.path, origin.idx)
				continue
			}
			seen[blk.sig] = shardOrigin{path: scans[i].path, idx: j}
			kept = append(kept, blk)
		}
	}
	if len(kept) < o.games {
		return fmt.Errorf("bookgen: merge kept %d of %d games across %d shards, below the -games floor %d", len(kept), raw, len(scans), o.games)
	}
	var out bytes.Buffer
	fmt.Fprintf(&out, "%s\n", want)
	for i, blk := range kept {
		fmt.Fprintf(&out, "G %d\n", i)
		out.Write(blockBody(blk))
	}
	if err := writeFileAtomic(o.outPath, out.Bytes()); err != nil {
		return err
	}
	fmt.Fprintf(progress, "merge done: %d games from %d shards (%d duplicates skipped) at %s\n", len(kept), len(scans), dups, o.outPath)
	return nil
}
