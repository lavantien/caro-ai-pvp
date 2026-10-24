# hex bench anomaly analysis

Static analysis of the v0.29 bench artifacts, no engine run, ahead of the quiet window re-bench. Reads bench-baseline-square.txt (pre-P1 capture), bench-p1-square.txt, bench-p3-square.txt, and bench-hexagon.txt under this directory. The P3 and hexagon captures carry headers recording arena=True at 46 to 55 percent load, so every ratio below is read against the canary rows those artifacts define.

## readings

Canary inflation at P3 against the baseline minimums: Lookup 0.3879 over 0.3043 ns = 1.27x, MakeUnmake 4.034 over 3.526 ns = 1.14x. The load band is 1.1 to 1.45x.

| label | baseline min | P1 min | P3 min | P3 ratio | over canary | class |
| --- | --- | --- | --- | --- | --- | --- |
| engine EvalReset | 16425 ns | 36317 ns | 29857 ns | 1.82x | ~1.4x | structural |
| engine Generate | 184.7 ns | 347.8 ns | 380.4 ns | 2.06x | ~1.6x | structural |
| pattern Index | 15.89 ns | 38.82 ns | 24.25 ns | 1.53x | ~1.2x | structural, small |
| rules LegalMoves | 242.0 ns | 595.6 ns | 535.5 ns | 2.21x | ~1.7x | structural, outside the original three |
| rules FastLastMoveWin | 15.11 ns | 28.41 ns | 26.54 ns | 1.76x | ~1.4x | mixed |
| engine EvalMakeUnmake | 321.9 ns | 756.8 ns | 436.6 ns | 1.36x | ~1.1x | load artifact |
| rules Wins | 533.4 ns | 576.7 ns | 345.3 ns | 0.65x | faster | quiet read settles it |
| pattern Init | 52.10 ms | 14.53 ms | 15.17 ms | 0.29x | faster | structural win, table 4 to 1 |

The split is informative but not conclusive: load inflation hits memory-bound code harder than the L1-bound canaries, so the split fits both stories. The quiet window separates them.

## hex EvalReset bytes are a measurement artifact

BenchmarkHexEvalReset reads 8, 18, and 21 B/op beside 0 allocs/op. The byte total per round is constant: 8 times 45036 = 360288, 18 times 21717 = 390906, 21 times 18393 = 386253, all within truncation distance of one constant ~389 KB. That constant is the benchmark setup:

| setup allocation | bytes |
| --- | --- |
| Engine moves [64][518]uint16 (search.go:16) | 66304 |
| Engine scores [64][518]int (search.go:17) | 265216 |
| Engine history, pv, killers, pvLen (search.go:18-21) | 17384 |
| evaluator win, wpair, cell (eval.go:26-29) | 18648 |
| Engine rank arrays (search.go:34-37) | 10360 |
| Engine total | ~378 KB |
| Board, zobrist, hex Geometry, ttTable | ~11 KB |

Mechanism: internal/engine/hex_bench_test.go:41-47 uses the legacy `for range b.N` shape, so the setup inside the benchmark function lands in every measured runN window and divides by the final N. internal/engine/bench_test.go:73-79 uses `b.Loop`, which starts the timer at the first loop call and excludes setup, which is why the identical square bench reads 0 B/op. The rules benches also use the legacy shape (rules/bench_test.go:38,45) but their setup is ~2 KB against multi-million final N, so they print 0. EvalReset is the only family where per-op cost is tens of microseconds, driving final N to 18 to 45 thousand and exposing the setup bytes. About 30 setup allocations over 18 to 45 thousand ops truncate to the printed 0 allocs/op.

Engine code is clean: reset (eval.go:46-69) and pattern.Index (window.go:28-64) contain no allocation site, and the square pin TestZeroAllocsEvalReset (allocs_test.go:57-63) holds 0. Ruled out: background-goroutine attribution, since New(0) starts nothing (search.go:50-64, the ponder lane at search.go:121 never runs here) and a wall-clock-rate source would scale the byte total with round duration, but the total is constant while ns/op rose 36 to 63. Ruled out: GC and stack growth, neither counts into the MemStats TotalAlloc that -benchmem reads.

Quiet window actions: convert the hex bench files to `b.Loop`, add hex zero-alloc pins beside the square set (TestZeroAllocsEvalResetHex, TestZeroAllocsGenerateHex, hex has none today, allocs_test.go:44-72), re-capture, expect 0 B/op and 0 allocs/op.

## square per-op causes

The pre-P1 code at commit 2c08ec7 folded every board dimension into compile-time constants. The unified code reads them through the geometry:

pattern.Index: g.Cells range check (window.go:29), g.Col slice load (window.go:34), Dirs slice element per call (window.go:33), g.Cols compare per window position (window.go:44), g.Rows times g.Stride per position (window.go:46). The old version compared against BoardSize and BoardStride constants with constant wordBits, so bounds-check elimination and constant folding applied. This is the main EvalReset and Index cause, plus the reset loop bound moving from the constant PatternDirections to the Dirs slice length (eval.go:51) and the evaluator clear doubling from 9.2 to 18.6 KB per call from the 518-cell array width (eval.go:48).

Generate: the square dilate branch already landed during P1 (bitboard.go:53-77), keeping the shift shape the plan names as the fallback, but stride and clear masks now load from the geometry at runtime (bitboard.go:60-73) instead of package constants, and fill adds the Region AND and Geom.Words reloads per pass (movegen.go:35-36). The opening-constraint path calls the kind-switched Distance (movegen.go:41, geom.go:207-219).

LegalMoves and FastLastMoveWin: same class, the region walk reads Geom.Words and Region words (legality.go:39-44) and the win walk reads Stride and the Dirs slice per step (win.go:106-126), where the old code used constants.

Counter-signal worth recording: pattern Init improved 3.4x under the same load from the table unification, so the artifact family resolves structural change in both directions and the geometry-indirection cost is a plausible but unproven residual until the quiet capture.

## quiet window decision tree

Rules fixed before the capture, no rerun-until-green:

1. Capture conditions: no engine batch on the machine, load under 10 percent, count=5, minimum per label decides. Artifact bench-quiet-square.txt with a header recording load, canary minimums, and count.
2. Canary validity gate: Lookup min at most 0.36 ns and MakeUnmake min at most 4.1 ns, both 1.15x baseline. Failing this voids the window, no decision, reschedule.
3. Branch A, load artifact: EvalReset at most 18.9 µs, Generate at most 212 ns, Index at most 18.3 ns, LegalMoves at most 278 ns, FastLastMoveWin at most 17.4 ns, EvalMakeUnmake at most 370 ns. Action: record the quiet numbers in the README engine table, close the item.
4. Branch B, bounded structural cost: any label between 1.15 and 1.5x baseline. Action: accept and record the honest per-geometry cost note, no code change in v0.29. The hex gates measured strength with this cost already inside the binary, so the tag is not blocked. Re-measure at the v0.30 close.
5. Branch C, material regression: any label above 1.5x baseline, EvalReset over 24.6 µs, Generate over 277 ns, Index over 23.8 ns. Action: design the square specialization keyed off geom.Kind (constant-stride Index and fill paths, precomputed square masks) and bench it in the same window, but land it after the v0.29 tag as a strength-bearing patch carrying its own arena crossarm evidence per the inversion research law, plus a per-geometry bench pair. The tagged v0.29 keeps the measured acceptable cost and its gates stay valid.
6. Hex riders on every branch: the b.Loop conversion, the hex zero-alloc pins, and a fresh hex capture whose 0 B/op expectation is now a checked prediction, not a hope.

## required evidence per branch

Branch A: the quiet artifact alone. Branch B: the quiet artifact plus a README cost note. Branch C: the quiet artifact, the specialization diff with its bench pair, and the arena evidence plan recorded before any pair runs.
