package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func main() {
	repo := flag.String("repo", ".", "repo root")
	flag.Parse()
	roots := []string{
		filepath.Join(*repo, arenaDirOut),
		filepath.Join(*repo, arenaDirArchive),
		filepath.Join(*repo, arenaDirScratch),
	}
	files, err := loadArenaFiles(roots)
	if err != nil {
		fmt.Fprintln(os.Stderr, "arena load:", err)
		os.Exit(1)
	}
	reportArena(files)
	reportFloors(files)
	reportOpenings(files)
	cs, err := loadCorpus(filepath.Join(*repo, corpusDir))
	if err != nil {
		fmt.Fprintln(os.Stderr, "corpus load:", err)
		os.Exit(1)
	}
	reportCorpus(cs)
}

type tcAgg struct {
	Games, RedWins, BlueWins, Draws, Unattributed int
}

func reportArena(files []*arenaFile) {
	fmt.Println("== arena files ==")
	byBucket := map[string]map[string]*tcAgg{}
	byFile := map[string]*tcAgg{}
	var decisiveLens []int
	drawMoves := map[int]int{}
	ponder := map[string]*ponderCounts{}
	totals := map[string]int{}
	for _, af := range files {
		fa := &tcAgg{}
		for _, g := range af.Games {
			bucket := bucketOf(g.Red, g.Blue)
			m, ok := byBucket[bucket]
			if !ok {
				m = map[string]*tcAgg{}
				byBucket[bucket] = m
			}
			a, ok := m[g.TC]
			if !ok {
				a = &tcAgg{}
				m[g.TC] = a
			}
			a.Games++
			fa.Games++
			switch g.Winner {
			case "draw":
				a.Draws++
				fa.Draws++
				drawMoves[g.Moves]++
			default:
				if bucket == bucketIdentical {
					a.Unattributed++
					fa.Unattributed++
				} else if g.Winner == g.Red {
					a.RedWins++
					fa.RedWins++
				} else {
					a.BlueWins++
					fa.BlueWins++
				}
				decisiveLens = append(decisiveLens, g.Moves)
			}
		}
		byFile[af.Path] = fa
		for seat, c := range af.Ponder {
			p := ponder[seat]
			if p == nil {
				p = &ponderCounts{}
				ponder[seat] = p
			}
			p.Arms += c.Arms
			p.Adopt += c.Adopt
			p.Reject += c.Reject
			p.Miss += c.Miss
		}
		for _, g := range af.Games {
			totals[g.TC]++
		}
	}
	paths := make([]string, 0, len(byFile))
	for p := range byFile {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		a := byFile[p]
		if a.Unattributed > 0 {
			fmt.Printf("%-52s games %3d decisive %3d unattributed draws %d\n", p, a.Games, a.Unattributed, a.Draws)
			continue
		}
		fmt.Printf("%-52s games %3d red %3d blue %3d draws %d\n", p, a.Games, a.RedWins, a.BlueWins, a.Draws)
	}
	fmt.Println("\n== red-first by bucket and tc (decisive red share, wilson 95%) ==")
	for _, bucket := range []string{bucketIdentical, bucketSameBase, bucketCross} {
		m := byBucket[bucket]
		tcs := make([]string, 0, len(m))
		for tc := range m {
			tcs = append(tcs, tc)
		}
		sort.Strings(tcs)
		sum := tcAgg{}
		for _, tc := range tcs {
			a := m[tc]
			sum.Games += a.Games
			sum.RedWins += a.RedWins
			sum.BlueWins += a.BlueWins
			sum.Draws += a.Draws
			sum.Unattributed += a.Unattributed
			printAgg(bucket, tc, a)
		}
		if len(tcs) > 1 {
			printAgg(bucket, "all", &sum)
		}
	}
	fmt.Println("\n== game length (moves, decisive games) ==")
	sort.Ints(decisiveLens)
	fmt.Printf("n %d min %d median %d p90 %d max %d\n", len(decisiveLens),
		quantile(decisiveLens, 0), quantile(decisiveLens, 0.5), quantile(decisiveLens, 0.9), quantile(decisiveLens, 1))
	moves := make([]int, 0, len(drawMoves))
	for mv := range drawMoves {
		moves = append(moves, mv)
	}
	sort.Ints(moves)
	var drawTot int
	for _, mv := range moves {
		drawTot += drawMoves[mv]
		fmt.Printf("draws at %d moves: %d\n", mv, drawMoves[mv])
	}
	fmt.Printf("draw rate: %d draws over %d games in parsed set\n", drawTot, len(decisiveLens)+drawTot)
	fmt.Println("\n== ponder outcomes per seat (arms adopt reject miss, hit=adopt+reject) ==")
	seats := make([]string, 0, len(ponder))
	for s := range ponder {
		seats = append(seats, s)
	}
	sort.Strings(seats)
	for _, s := range seats {
		c := ponder[s]
		fmt.Printf("%-24s arms %4d adopt %4d reject %4d miss %4d\n", s, c.Arms, c.Adopt, c.Reject, c.Miss)
	}
}

func printAgg(bucket, tc string, a *tcAgg) {
	if a.Unattributed > 0 {
		fmt.Printf("%-10s %-6s games %3d decisive %3d (seat unattributable, labels alias) draws %2d\n",
			bucket, tc, a.Games, a.Unattributed, a.Draws)
		return
	}
	n := a.RedWins + a.BlueWins
	lo, hi := wilson(a.RedWins, n)
	fmt.Printf("%-10s %-6s games %3d red %3d blue %3d draws %2d redshare %6.1f%% [%5.1f, %5.1f]\n",
		bucket, tc, a.Games, a.RedWins, a.BlueWins, a.Draws,
		100*float64(a.RedWins)/float64(max(n, 1)), 100*lo, 100*hi)
}

func reportCorpus(cs *corpusStats) {
	fmt.Println("\n== corpus files ==")
	for _, cf := range cs.Files {
		var plies int
		for _, g := range cf.Games {
			plies += g.Plies
		}
		fmt.Printf("%-48s games %3d plies %5d fingerprint %s\n", cf.Path, len(cf.Games), plies, cf.Fingerprint)
	}
	fmt.Printf("total P lines %d unique keys %d max ply %d\n", cs.TotalP, len(cs.Keys), cs.MaxPly)
	shared2, shared3, crossFile := 0, 0, 0
	for _, k := range sortedKeys(cs.Keys) {
		a := cs.Keys[k]
		if len(a.Games) >= 2 {
			shared2++
		}
		if len(a.Games) >= 3 {
			shared3++
		}
		if len(a.Files) >= 2 {
			crossFile++
		}
	}
	fmt.Printf("keys shared by >=2 games %d (%.1f%% of unique), >=3 games %d, seen in >=2 files %d\n",
		shared2, 100*float64(shared2)/float64(max(len(cs.Keys), 1)), shared3, crossFile)
	fmt.Println("\n== visit structure (steps " + visitSteps + ") ==")
	for _, v := range []int{2, 5, bookVisitFloor} {
		n, single := 0, 0
		var topShares, secondShares []float64
		var secondAtFloor int
		for _, k := range sortedKeys(cs.Keys) {
			a := cs.Keys[k]
			if a.Visits < v {
				continue
			}
			n++
			counts := a.topCounts()
			if len(counts) == 1 {
				single++
				continue
			}
			topShares = append(topShares, float64(counts[0])/float64(a.Visits))
			secondShares = append(secondShares, float64(counts[1])/float64(a.Visits))
			if counts[1] >= bookVisitFloor {
				secondAtFloor++
			}
		}
		line := fmt.Sprintf("v>=%2d keys %5d single-move %5d (%.1f%%)", v, n, single, 100*float64(single)/float64(max(n, 1)))
		if len(topShares) > 0 {
			line += fmt.Sprintf(" multi %4d mean top share %.3f mean second share %.3f", len(topShares), mean(topShares), mean(secondShares))
		}
		if v == bookVisitFloor {
			line += fmt.Sprintf(" second>=floor %d", secondAtFloor)
		}
		fmt.Println(line)
	}
	fmt.Println("\n== game length (plies per corpus game) ==")
	sort.Ints(cs.Plies)
	fmt.Printf("games %d min %d median %d p90 %d max %d\n", len(cs.Plies),
		quantile(cs.Plies, 0), quantile(cs.Plies, 0.5), quantile(cs.Plies, 0.9), quantile(cs.Plies, 1))
	fmt.Println("\n== ply coverage (ply: unique keys, visits) ==")
	var plies []int
	for p := range cs.PlyUniq {
		plies = append(plies, p)
	}
	sort.Ints(plies)
	for _, p := range plies {
		fmt.Printf("ply %2d keys %5d visits %5d\n", p, cs.PlyUniq[p], cs.PlyVisits[p])
	}
}

func mean(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	var s float64
	for _, x := range xs {
		s += x
	}
	return s / float64(len(xs))
}

type openingAgg struct {
	Games, RedWins, BlueWins, Draws int
	TCs, Seqs                       map[string]bool
}

func reportFloors(files []*arenaFile) {
	fmt.Println("\n== identical-engine floors: move-list availability and length diversity ==")
	for _, af := range files {
		var floors []arenaGame
		withSeq := 0
		for _, g := range af.Games {
			if bucketOf(g.Red, g.Blue) != bucketIdentical {
				continue
			}
			floors = append(floors, g)
			if g.HasSeq {
				withSeq++
			}
		}
		if len(floors) == 0 {
			continue
		}
		seen := map[int]bool{}
		var decisive []int
		for _, g := range floors {
			if g.Winner != "draw" {
				decisive = append(decisive, g.Moves)
				seen[g.Moves] = true
			}
		}
		fmt.Printf("%-24s floors %3d with-mlines %3d decisive %3d distinct-lengths %3d length-collisions %3d\n",
			filepath.Base(af.Path), len(floors), withSeq, len(decisive), len(seen), len(decisive)-len(seen))
	}
}

func reportOpenings(files []*arenaFile) {
	fmt.Println("\n== opening decomposition (same-base pairs with move lists), signature = M1 M2 M3 ==")
	byOpening := map[string]*openingAgg{}
	seqSeen := map[string]int{}
	for _, af := range files {
		for _, g := range af.Games {
			if !g.HasSeq || bucketOf(g.Red, g.Blue) != bucketSameBase || g.Opening == "" {
				continue
			}
			a := byOpening[g.Opening]
			if a == nil {
				a = &openingAgg{TCs: map[string]bool{}, Seqs: map[string]bool{}}
				byOpening[g.Opening] = a
			}
			a.Games++
			switch g.Winner {
			case "draw":
				a.Draws++
			case g.Red:
				a.RedWins++
			default:
				a.BlueWins++
			}
			a.TCs[g.TC] = true
			a.Seqs[g.Seq] = true
			seqSeen[g.Seq]++
		}
	}
	openings := make([]string, 0, len(byOpening))
	for k := range byOpening {
		openings = append(openings, k)
	}
	sort.Slice(openings, func(i, j int) bool {
		a, b := byOpening[openings[i]], byOpening[openings[j]]
		if a.Games != b.Games {
			return a.Games > b.Games
		}
		return openings[i] < openings[j]
	})
	totGames, totSeqs := 0, 0
	for _, o := range openings {
		a := byOpening[o]
		totGames += a.Games
		totSeqs += len(a.Seqs)
		tcs := make([]string, 0, len(a.TCs))
		for tc := range a.TCs {
			tcs = append(tcs, tc)
		}
		sort.Strings(tcs)
		fmt.Printf("%-12s games %3d red %3d blue %3d draws %2d tcs %-10s distinct-full-games %3d\n",
			o, a.Games, a.RedWins, a.BlueWins, a.Draws, strings.Join(tcs, ","), len(a.Seqs))
	}
	dupes := 0
	for _, n := range seqSeen {
		if n > 1 {
			dupes += n - 1
		}
	}
	fmt.Printf("openings %d games %d distinct full games %d exact full-game repeats %d\n",
		len(openings), totGames, totSeqs, dupes)
}
