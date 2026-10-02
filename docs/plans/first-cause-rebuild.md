# Caro AI PvP first-cause rebuild plan

## Progress

- M0a done: scaffold, config hub, covergate, firewall, CI chain, Makefile (2026-10-01)
- M0b done: ref/chessprog (21 pages), ref/gomocup (docs + 8 papers) (2026-10-01)
- M1 done incl gates: internal/rules at 100.0% statements, race clean, zero-alloc pinned (Make+Unmake 3.7 ns, FastLastMoveWin 13.4 ns), 24M fuzz execs clean, adversarial pair review findings fixed, mutation gate 497/497 with 0 survivors and 21 proven-equivalent allows (2026-10-01)
- M2 done incl adversarial pair review: internal/pattern tables generated from rules predicates, init 51.7 ms, Lookup 0.26 ns, 100.0% coverage, both reviewers verified all 44272 realizable entries per direction against independent oracles with zero divergences, findings fixed (2026-10-01)
- M3a done incl adversarial pair review: internal/engine PVS+ID+TT at 100.0% statements, race clean, Search() 0 B/op 0 allocs/op (1s bench: depth 5, 1.63 Mnps, ebf 17, fh1 94). Both reviewers converged on one real defect (TT mate ply adjust used bare ply instead of EvalMateScoreStep units, live across searches sharing a TT) plus latent guards, all fixed in ae28190. Deviation: threat extension triggers on forced four-blocks only, open-3 extension deferred to M7 strength tuning as an explosion risk (2026-10-01)
- M3b done incl adversarial pair review: lazy SMP persistent parked pool, lockless shared TT, tier wiring, engine at 100.0% statements, 0 allocs/op searches at 1/2/4 workers (5.0-6.0 Mnps at 4). Both reviewers converged on: Close racing pool start leaked workers silently (lifecycle now one mutex, Close joins, dispatch rechecks closed), runWG.Done now deferred, dead jobDL dropped, quit-branch drains queued tokens, npsReport floors sub-ns elapsed in both drivers. Fixed in fde5459 (2026-10-01)
- M4 done incl adversarial pair review: VCF/VCT threat-space solvers with FAIL-only memo at 100.0% statements, race clean, fuzz corpus committed. Reviewer A broke soundness: walkFive sampled the first exact-five frame per win cell, a dead first frame masked a live frame's far-end defense and both kinds claimed a 5-ply win the full-width oracle refutes; fixed by fiveFrames enumerating every direction, forcedBlock multi-frame far ends, defends resized to BoardCells (58397c8). Reviewer B supplied the reachable 110-counter-four panic repro (pinned in 562c4b4, overflow itself closed by the resize) and the oracle cut-2 depth bound, now asserted (2026-10-01)
- In flight: run close, mutation gate over rules+engine, then clean-clone make ci
- Run 1 closed (2026-10-02): clean-clone make ci green; mutation gate over rules+engine green at 1483/1483 run, 1402 killed, 0 survived, 81 allowed, after triaging 232 first-run survivors into 169 killer assertions (mutkill_{rest,smptt,search}_test.go plus two stale-row killers), 60 equivalences, one subsumed allow pruned, and deleting two dead cutoff-pv writes. Gate tooling gained -resume (make mutate-resume LOG=file) after two host failures; operational rule recorded: the gate runs on an exclusive machine, verdict flips under concurrent load were all environmental. M0..M4 complete, tag v0.1 cut
- M5 done incl adversarial pair review: internal/clock GameClock law (feedforward spendable/movesLeft + increment share, drain-trajectory target, PID correction clamped to a fraction of feedforward, floor wins over reserve so a drained 1+0 clock still funds minimum moves, Commit panics on negative elapsed) with per-time-control PID gains in the config hub, 100.0% statements, race clean, 30-game x 128-move sim over 6 cost models with integer-ns bound recompute, live engine+clock wiring pinned by BenchmarkLiveGameClock1plus0. Soft stop consults at iteration heads in Search() and every SMP worker (SearchDepth exempt by contract) with the Windows clock-quantum guard: a zero elapsed reading after a banked iteration counts as one SearchClockQuantumMs tick. Adversarial pair findings all fixed. Mutation gate over rules+engine+clock green at 1573/1573 run, 1494 killed, 0 survived, 79 allowed, every allowance challenge-audited (suite run under the allowlisted mutant, serial resolution, zero demotions); 37 first-run survivors triaged into 12 killer tests (guard panic-message pins, movesLeft spread, draw-case pv row clears, head-guard and soft-stop deadline-consult counts) and 25 proven equivalences (clamp collapse, dead writes, unreachable sentinels and ties). Gate gained a parallel runner (make mutate PARALLEL=N CHALLENGE=1, 8 workers measured 2.49x with zero verdict flips, serial survivor and challenge confirmation, temp-isolate hygiene, resume-safety under interruption), itself cleared by a blind adversarial pair (canceled confirmation can no longer forge kills; killed-allowance alarm now challenge-verified). Tag v0.2 cut (2026-10-02)

## Context

Repo nuked (commits `2e339c3`, `8e3a9b0`), tracked files are only `first-cause.md` and `typos.toml`, tree clean on main, remote `github.com/lavantien/caro-ai-pvp`. Everything rebuilds bottom up: rules core, engine, solvers, then (next run) server, UI, tournaments. No archival, prior-codebase memories invalidated.

Verified environment (2026-10-01): Go 1.27.1 (user-updated, spec floor 1.27.1+) with CGO_ENABLED=1, gcc 15.2.0, GNU Make 4.4.1, golangci-lint 2.14.0, staticcheck, benchstat, docker 29.8.0, node 26.3.0. htmx 4.0.0 shipped 2026-08-28 (matches spec "HTMX 4+").

## Rule formalization

Board 16x16, A1..P16. Bitboards: 256 cells = 4x uint64 per color, bit = row*16+col, stride always 16, playable region = mask from config (full board default, 8x8 region rows 0..7 cols 0..7 for cross-check). A1 = row 0 col 0, rendered top-left, consistent everywhere. Out-of-region cells can never hold stones so they behave exactly like walls.

Win, formalized before any engine code:
- Color C wins if and only if some maximal run of exactly 5 contiguous C stones exists in one of 4 directions where NOT both immediate neighbors along that line hold opponent stones. Walls and out-of-region cells never block.
- Overlines (6+) never win, no sub-window of 5 inside an overline counts.
- Exact 5 with one end blocked and the other a wall: win. Both ends blocked by stones: dead.

Opening rule: red's 2nd move needs Chebyshev distance >= 3 from red's 1st (max(|dx|,|dy|) >= 3, diagonals included). Red O first, blue X second. PvP series: loser takes red. Bot matches: alternate red equally. Draw if and only if the board is full, no other draw condition.

## Non-goals for v0.20, each with a design seam so later plug-in stays cheap

- Ponder: search state separability, PV access, and time manager hooks designed now, implementation after the core engine.
- R&D items (A*/priority queue, ant colony, genetic algorithm): live in `playground/` as organized, git-tracked R&D ground (per-topic subdirs), off the MVP critical path, evaluated against the engine through config tiers.
- DuckDB past v0.20: SQLite schema stays analytics-friendly and the WAL write path stays isolated behind the message/worker queue so the analytics reader can swap in later.
- Forgot password: credential table stays minimal, argon2id parameters in config.
- Fraud detection: match and rating event log retained, no detection logic.
- External hosting: server binds and ports stay config-driven.
- Archival of the old repo: none.

## Architecture

```
caro-ai-pvp/
  ref/{chessprog,gomocup}/  offline grounding, browser-sourced (spec-mandated)
  docs/plans/                 this rebuild plan sourced into the repo, updated as milestones land
  playground/                 organized R&D ground, git-tracked, per-topic subdirs
  internal/config/            single constants hub, everything lives here
  internal/rules/             board, moves, legality, win, zobrist, coords, naive oracle
  internal/pattern/           exhaustive line-pattern tables, computed at init
  internal/engine/            eval, movegen, search (PVS+ID+TT+ordering), smp, deadline
  internal/vcf/               VCF/VCT solvers (threat-space + PNS), test-only cross-check oracle
  internal/clock/             increment-safe time manager
  internal/server/            sqlite WAL, write queue, auth, rooms, series, rating, stats pipeline, htmx ui
  internal/tourney/           round robin, smoke series, leaderboard
  cmd/caro/                   entrypoint
  scripts/firewall.ps1        netsh wrapper for the uncommon ports
  Makefile, .github/workflows/ci.yml, README.md
```

Config hub day one: board (Size, Stride, Cells, WordsPerColor, WinLength, OpeningChebyshevMin, CrossCheckSize), zobrist Seed with splitmix64 stream, time controls (1+0, 2+1, 3+2), series (bo3/5/7/11), tiers (Easy 1 core no TT no solvers, Medium 2 cores 256MiB TT + VCF, Hard 4 cores 1GiB TT + VCF + VCT), resource caps (half machine, TournamentParallel 2), uncommon ports, rating constants (Delta 30, DivisorFull 3000, DecayBase 500, DecaySlope 2.5, DecayMin 500, DecayMax 3000, StartRating 0), eval (MilliUnit 1000, mate bounds, pattern weights at M2), search (MaxPly, MaxMovesPerPly, NodeCheckInterval, SoftStopFraction, SafetyMarginMs, MinMoveTimeMs), bot log M-line template, quality gates (95 overall, 100 core). Config tests assert against spec literals, port uniqueness, tier bounds within caps.

## Milestones

M0a scaffold, first commits
- go.mod (module github.com/lavantien/caro-ai-pvp, go 1.27.1), .gitignore (bin, coverage out, *.db, no playground exclusion: it is tracked R&D ground), Makefile, README skeleton (TOC, badge placeholders, diagrams section), CI ci.yml (push+PR, ubuntu-latest, setup-go from go.mod, `make ci`, coverage artifact), internal/config hub, cmd/caro (ports + firewall subcommands), scripts/firewall.ps1, docs/plans/ carrying this plan sourced from the planning file.
- Makefile targets: doctor (go >= 1.27.1, CGO, gcc), fmt, fmt-check, lint (go vet, stdlib), vet, build, test, test-race, cover, bench, mutate, fuzz, run, migrate, firewall, tidy, ci. All go invocations prefixed CGO_ENABLED=1.
- Tooling policy: testing and tracing build on stdlib and official golang.org/x extensions only, zero 3rd party. Fuzzing is built-in testing.F, tracing is runtime/pprof + runtime/trace + expvar, coverage gates parse `go tool cover -func` output.
- Mutation testing: no 3rd party tools. The spec mandates mutation on the core, so build an in-house stdlib-only pipeline (go/parser + go/ast rewrites: operator swaps, boundary deltas, constant tweaks) targeting internal/rules first, then engine core, wired as `make mutate`. Built at M1 when the first target exists, kept under 1000 SLOC.
- Record protocol deviation: SQLite is embedded via CGO, no server for docker compose to run, migrations are startup self-migration (version table + embedded SQL) wrapped by `make migrate`.

M0b ref sourcing, parallel background track, gate = M3 start
- Browser-save (chrome-devtools or playwright, curl gets Cloudflare-blocked) to ref/chessprog/: chessprogramming.org basics + principal topics, full Stockfish page with selected features minus NNUE. ref/gomocup/: developer docs + recommended sources, educational only, their templates and APIs are not followed. Timeboxed docs commits.

M1 rules core, internal/rules
- Files: coords.go (cell to name codec A1..P16), board.go (Board: Red, Blue [4]uint64, Side, preallocated undo stack [256], Region + Full masks, Make, Unmake, At, LegalMoves into caller buffer), win.go (multi-word shift scans with cross-word carries, row-wrap masks for horizontal, exactly5 = 5-run starts minus 6-run starts and back-extensions, both-ends-block nullification, plus FastLastMoveWin checking only the 4 lines through the last stone), naive.go (oracle: array board, direct transcription of the spec sentence, independent reasoning, parameterized size), zobrist.go ([2][256]uint64 from config Seed via splitmix64 + side key, incremental), legality.go (empty + in region + opening rule via red popcount == 1).
- Test suites: named spec-clause cases (exact 5 open, one end blocked, both blocked, wall+blocked, wall+wall, overline 6, sub-5 inside 6, own-stone-beyond, overline in one direction plus exact 5 in another, diagonals, 8x8 variants), exhaustive 4^9 window sweep per direction bitboard-vs-naive, props (make/unmake round trip, hash restore, random play terminates), native fuzz differential with committed seed corpus, AllocsPerRun zero-alloc assertions in plain tests plus benchmem benches.
- TDD sequence, one atomic commit per green step: naive oracle from spec, coords, board+zobrist, win (naive cases then bitboard until differential + exhaustive pass), legality, props+fuzz, zero-alloc, mutation clean.
- Acceptance: `make ci` green on clean clone, internal/rules coverage exactly 100%, zero surviving mutants, 0 B/op 0 allocs/op on all rules benches, -race clean, stdlib + math/bits + math/rand/v2 only, every file under 1000 SLOC.

M2 pattern tables, internal/pattern
- Formal threat taxonomy artifact first: classes defined mechanically from the win predicate. Win-in-1 cell = empty cell whose occupation by the mover wins in that line. Four = exactly 1 win-in-1 cell in the line state, open four = 2+, three = a state reaching a double-threat state with one own stone, lower classes derived analogously. Overline-completing extensions are never win-in-1 by construction, which is exactly the spec's non-standard nuance.
- Tables computed at init from the rules package's own win predicate, no go:generate codegen: single source of truth, rules and tables can never drift, init cost is tens of ms. 4 states per cell (empty, own, opp, off-board), 4^9 = 262144 entries per direction, the off-board state expresses edge-distance cases (walls never block yet are never playable). Entry: 9-bit win-in-1 mask plus derived class byte for eval. Property tests: H/V and diagonal pairs equal up to index permutation, table-vs-naive differential harness.
- Go 1.27 generics where they cut duplication: one generic table type and one generic window-enumeration routine shared across all 4 directions, no extra abstraction layers beyond that.
- Bitboard line extraction via shearing for diagonals, row-parallel masks.

M3a single-threaded search core, internal/engine
- Integer eval in milliunits from pattern class weights, mate-in-N bounds, no floats in the hot loop.
- Candidate movegen, alpha-beta/PVS, iterative deepening, TT (Zobrist keyed, direct-mapped, 16-byte entries, depth/generation replacement, mate-score ply adjustment in one encode/decode pair), history heuristic, killer moves, selective threat extension (forced 4-blocks, open 3s). No quiescence, no piece-square, no LMR, no null-move.
- Zero-alloc, proven before SMP exists: MoveStack [MaxPly][MaxMoves]Move with index offsets, no fmt in the engine path (strconv.AppendInt into preallocated buffers for PV and log line), no defer in hot functions, AllocsPerRun tests in CI, benchmem gate, periodic `-gcflags="-m"` audit.
- Deadline interface defined here (fixed-budget stub), consumed later by solvers and the clock.

M3b SMP + tiers, internal/engine/smp
- Lazy SMP: GOMAXPROCS(N) + LockOSThread workers, shared lockless TT, every access through sync/atomic on packed words (key^signature + data), TT strictly per instance, never shared across instances. -race in CI and every later smoke run.
- Adversarial note: zobrist does not hash the region, identical stones+side hash identically across full and 8x8 boards. Never share or reuse a TT across board kinds (cross-check runs get fresh instances only).
- Tier wiring from config: Easy, Medium, Hard. Ponder interface stub present.

M4 VCF/VCT solvers, internal/vcf
- Threat-space + proof-number search generating only threat moves (fours for VCF, threes+fours for VCT) strictly from M2 tables, consuming the M3a deadline interface.
- Cross-check oracle: test-only brute-force alpha-beta over the naive array board sharing no code with the engine, random positions with controllable density plus 8x8 region mode. Soundness-only criterion per spec: every claimed win must verify, refutations ignored. Fuzz runs in CI permanently.

M5 time manager, internal/clock
- Budget formula entirely in config (fraction of remaining + increment - reserve, floor MinMoveTimeMs), deadline checks every NodeCheckInterval nodes and at iteration boundaries, only completed iterations are candidates, legal fallback move computed before search starts, server hands an absolute deadline already reduced by SafetyMarginMs. Tiers differ only by resources so relative strength is hardware independent. Timeout is never an outcome: win, loss, or draw by full board only.

M6a server backend, internal/server
- SQLite WAL + custom message/worker queue for writes, startup self-migration, argon2id auth (username, password, login/create), rating per spec formula, W-L-D + level (series won), series state machine first-class (loser-takes-red, bo progression, quit mid-series = loss for remaining games), rooms lifecycle, realtime stats pipeline (pub-sub) emitting the M-line format.
M6b UI + e2e, acceptance = Implications 1.1, 1.3, 1.4
- Mobile-first htmx 4 UI (pin exact 4.0.x, hx-sse for push with POST for moves, SSE spike first, EventSource fallback), ghost stone, invalid-move rejection, large clocks, move history, bot logs, playback board. Implication 1.5 stats line rendered from the pipeline.

M7 tournament, internal/tourney, acceptance = Implication 1.2
- Round robin twice-pair, same room surface, 2 parallel matches, logs to DB + local txt, leaderboard. Smoke series 3+2 bo3 then 1+0 bo3 (6 matchups each, fix and repeat until clean), then full UI tournament: 6 bots, 2 per tier, start 1000, 2+1 bo3, strength-inversion check.

M8 README and polish
- TOC, coverage + CI badges, 3 screenshots (live bot vs bot room, logged-in home with room grid, match history with playback board), diagrams.

## Design decisions

- Pattern tables init-computed from the rules win predicate, not codegen. What: generator lives in code. Why: single source of truth, zero drift, mutation and coverage exercise the generator itself. Where: internal/pattern at startup. When: M2.
- Threat taxonomy derived mechanically from win-in-1 counts. Why: folk renju terms do not transfer to exact-5-with-overline-nullified rules, hand-written patterns are banned by the spec.
- Region masks instead of a parameterized board size. Why: the 8x8 cross-check reuses identical bitboard code, walls and out-of-region behave identically (no stones possible), stride 16 keeps shifts uniform.
- Zero-alloc proven in M3a before SMP. Why: allocation attribution across goroutines is ambiguous, escape analysis is single-threaded truth.
- In-house stdlib-only mutation pipeline instead of any 3rd party mutator. Why: testing and tracing stay on stdlib and official extensions, the core mutation surface is small (rules + engine), go/parser + go/ast covers it.
- Embedded SQLite startup self-migration wrapped by `make migrate`. Why: docker compose has no server to run for an in-process CGO database, deviation recorded here per protocol.

## First execution scope: M0 through M4 (user-confirmed)

One run delivers the complete engine: M0a scaffold, M0b refs (parallel, gated on M3 start), M1 rules core, M2 pattern tables, M3a zero-alloc search, M3b SMP + tiers, M4 solvers with soundness cross-checks. M5..M8 wait.

Marathon mitigation: each milestone is independently green, verified, and committed before the next starts, so an interrupted run leaves durable progress. If time forces a cut, stopping after M3 still yields a usable engine.

Quota gate (user-mandated, active whenever work is fanned out): a session-only recurring job fires every 10 minutes and reads the 5-hour window quota percentage. At 97% all parallel work stops and development pauses completely, resuming only when the quota window resets. Never durable, never skipped while fan-out is active.

Commit sequence (conventional, atomic, no attribution lines): 1 chore: scaffold go.mod gitignore Makefile CI doctor. 2 feat: config hub constants and tests. 3 docs: readme skeleton. 4 feat: firewall script and ports subcommand. 5 test: naive rules oracle from spec. 6 feat: coordinate codec. 7 feat: board bitboards make unmake zobrist. 8 feat: bitboard win detection with exhaustive differential tests. 9 feat: legality with opening rule. 10 test: property and fuzz suites. 11 perf: zero-alloc assertions and benchmarks. 12 chore: mutation target and clean run. Then M2, M3a, M3b, M4 in their own atomic sequences, docs commits interleaved from the sourcing track.

## Verification

Per milestone, in order through make targets, committing at each green step: feature-specific tests, fmt-check, lint, vet, build, full unit suite, test-race, cover (95 overall, 100 core gates), bench where applicable. Then 2 independent adversarial reviews attack the change, neither seeing the other, fix every confirmed finding, re-run the chain. Final gate: clean clone `make ci` green. Zero-alloc is asserted by AllocsPerRun tests in CI, not just benchmarks.

## Open confirmation before M6 (next run)

Rating formula reading: per game, winner +30*K, loser -30*K, K = 10^((R_loser - R_winner)/D), D = 3000 when winner is lower or equal rated, else D = min(3000, max(500, 500 + 2.5*R_loser)). The band table equals this closed form evaluated at band starts. Per-game recompute of K recommended over series-start snapshot. Confirm with user before implementing.
