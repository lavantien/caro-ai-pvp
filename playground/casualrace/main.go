package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

const tailScan = 4096

type roundResult struct {
	blocks int
	m2     int
	m4     int
	want   int
}

func main() {
	procs := flag.Int("procs", 2, "GOMAXPROCS forced during the hammer")
	instances := flag.Int("instances", 8, "concurrent writer instances on one root")
	per := flag.Int("per", 250, "games each instance writes per round")
	rounds := flag.Int("rounds", 10, "fresh-root rounds")
	pad := flag.Int("pad", 0, "filler bytes appended to every block to widen the in-flight window")
	flag.Parse()
	old := runtime.GOMAXPROCS(*procs)
	defer runtime.GOMAXPROCS(old)
	fmt.Printf("casualrace: entry procs=%d instances=%d per=%d rounds=%d pad=%d\n", *procs, *instances, *per, *rounds, *pad)
	failures := 0
	for round := 1; round <= *rounds; round++ {
		started := time.Now()
		res, err := hammerRound(*instances, *per, *pad)
		if err != nil {
			fmt.Printf("casualrace: round %d failure: %v\n", round, err)
			failures++
			continue
		}
		verdict := "ok"
		if res.blocks != res.want || res.m2 != res.want || res.m4 != res.want {
			verdict = fmt.Sprintf("LOSS blocks=%d/%d m2=%d/%d m4=%d/%d",
				res.blocks, res.want, res.m2, res.want, res.m4, res.want)
			failures++
		}
		fmt.Printf("casualrace: round %d exit %s (%s)\n", round, verdict, time.Since(started).Round(time.Millisecond))
	}
	fmt.Printf("casualrace: exit rounds=%d failures=%d\n", *rounds, failures)
	if failures > 0 {
		os.Exit(1)
	}
}

func hammerRound(instances, per, pad int) (roundResult, error) {
	root, err := os.MkdirTemp("", "casualrace")
	if err != nil {
		return roundResult{}, fmt.Errorf("round root: %w", err)
	}
	defer func() { _ = os.RemoveAll(root) }()
	path := filepath.Join(root, "s7_alice-vs-easy-ab12.txt")
	var wg sync.WaitGroup
	for i := range instances {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for g := range per {
				block := blockFor(i*per+g+1, pad)
				if err := recover(path); err != nil {
					fmt.Printf("casualrace: instance %d game %d recover failure: %v\n", i, g+1, err)
					return
				}
				if err := appendBlock(path, block); err != nil {
					fmt.Printf("casualrace: instance %d game %d append failure: %v\n", i, g+1, err)
					return
				}
			}
		}(i)
	}
	wg.Wait()
	body, err := os.ReadFile(path)
	if err != nil {
		return roundResult{}, fmt.Errorf("round read: %w", err)
	}
	want := instances * per
	return roundResult{
		blocks: strings.Count(string(body), "game "),
		m2:     strings.Count(string(body), "M2, Blue, P12, d=9"),
		m4:     strings.Count(string(body), "M4, Blue, P8, d=9"),
		want:   want,
	}, nil
}

func blockFor(gameNo, pad int) []byte {
	var b strings.Builder
	b.WriteString("game " + strconv.Itoa(gameNo) + " room ab12 mode vs bot\n")
	b.WriteString("pairing alice vs easy-ab12\n")
	b.WriteString("tc 3+2 bo3 outcome draw\n")
	b.WriteString("M2, Blue, P12, d=9\n")
	b.WriteString("M4, Blue, P8, d=9\n")
	if pad > 0 {
		b.WriteString(strings.Repeat("x", pad))
		b.WriteByte('\n')
	}
	b.WriteByte('\n')
	return []byte(b.String())
}

func recover(path string) error {
	f, err := os.OpenFile(path, os.O_RDWR, 0o644)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	size := info.Size()
	if size == 0 {
		return nil
	}
	scan := int64(tailScan)
	if scan > size {
		scan = size
	}
	buf := make([]byte, scan)
	if _, err := f.ReadAt(buf, size-scan); err != nil && err != io.EOF {
		return err
	}
	if buf[len(buf)-1] == '\n' {
		return nil
	}
	cut := int64(-1)
	for i := len(buf) - 2; i >= 0; i-- {
		if buf[i] == '\n' {
			cut = size - scan + int64(i) + 1
			break
		}
	}
	if cut < 0 {
		cut = size - scan
	}
	return f.Truncate(cut)
}

func appendBlock(path string, block []byte) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	_, err = f.Write(block)
	return err
}
