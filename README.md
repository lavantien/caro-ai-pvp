# caro-ai-pvp

[![ci](https://github.com/lavantien/caro-ai-pvp/actions/workflows/ci.yml/badge.svg)](https://github.com/lavantien/caro-ai-pvp/actions/workflows/ci.yml) [![global coverage](https://raw.githubusercontent.com/lavantien/caro-ai-pvp/coverage/coverage-global.svg)](https://github.com/lavantien/caro-ai-pvp/actions/workflows/ci.yml) [![core coverage](https://raw.githubusercontent.com/lavantien/caro-ai-pvp/coverage/coverage-core.svg)](https://github.com/lavantien/caro-ai-pvp/actions/workflows/ci.yml)

Mobile-first web arena for a custom 16x16 gomoku variant. Play from a phone browser against a person or an increment-safe bot on server-hosted rooms, watch live rooms as a guest, step through match history with a playback board, and run bot vs bot tournaments on the same room surface with per-move engine telemetry and a decay-scaled rating. The behavior contract is scenario 1 in [first-cause.md](first-cause.md).

## contents

1. [variant ruleset](#variant-ruleset)
2. [ui/ux features](#uiux-features)
3. [screenshots](#screenshots)
4. [grounding](#grounding)
5. [build and verify](#build-and-verify)
6. [repository layout](#repository-layout)
7. [diagrams](#diagrams)
8. [implemented design](#implemented-design)
9. [status](#status)
10. [roadmap v0.21 to v0.30](#roadmap-v021-to-v030)

## variant ruleset

The game is a 16x16 gomoku variant with four rules on top of place-a-stone, alternate-turns play:

- exactly five in a row horizontally, vertically, or diagonally wins
- a line of six or more stones (an overline) never wins
- a line close-blocked at both ends by adjacent opponent stones is dead even when it holds five
- red's second move must sit at Chebyshev distance 3 or more from red's first stone

Red moves first, the first legal five ends the game, a full board draws. Matches run as best-of 3, 5, 7, or 11 games over the 1+0, 2+1, 3+2, and 10+5 time controls, with the loser of a decisive game taking red next and a draw retaining red.

Engine tiers are resource-only shapes of one search. This table is the demand spec: the re-scaled sizes and the ponder column landed in tree behind their gates, the ladder re-gate for the sizes and the arena pair for ponder, and the book column fills at v0.24. The status section summarizes what is shipped today, with the wave-by-wave record in devlog.md.

| tier | threads | tt | vcf | vct | book | ponder |
| --- | --- | --- | --- | --- | --- | --- |
| easy | 1 | 32 MiB | no | no | no | no |
| medium | 2 | 128 MiB | yes | no | no | no |
| hard | 4 | 1 GiB | yes | yes | no | no |
| master | 8 | 2 GiB | yes | yes | yes | yes |

A series log opens with its pairing bookends and one M-line per bot move, closes each game with its verdict and the series with its tally. Real lines from a 10+5 hard-tier series, with the `[BOOK]` line illustrative until v0.24 ships it and the `[PONDER]` line an illustrative render of the adoption surface now live on master:

```
run 1 series 1
pairing hard-1 (hard) vs hard-2 (hard)
tc 10+5 bo3
M1, Red, H8, d=11, n=83.1m, nps=3.69m, ebf=5.2, tt=9%, hf=99%, fh1=90%, s=-300, thr=4, t=22.50, alloc=22.50, pv=H8 H6
M7, Red, F8, d=12, n=79.39m, nps=3.45m, ebf=4.5, tt=10%, hf=98%, fh1=92%, s=+3000, thr=4, t=22.98, alloc=22.98, pv=F8 F5 G8 E8 I8 J8 I7 J6 I6 I5 J7 J4 K3 G4
M15, Red, D8, d=9, n=177.09k, nps=60.67k, ebf=3.8, tt=0%, hf=0%, fh1=0%, s=M9, thr=4, t=2.92, alloc=23.55, [VCT], pv=D8 B10 E8 I8 E5 E9 F5 I5 D5
M31, Red, E9, d=7, n=129, nps=64.54k, ebf=2.0, tt=0%, hf=0%, fh1=0%, s=M7, thr=4, t=0.00, alloc=27.09, [VCF], pv=E9 E12 F10 B10 C7 G11 B6
M37, Red, B6, d=1, n=1, nps=1k, ebf=0.0, tt=0%, hf=0%, fh1=0%, s=M1, thr=4, t=0.00, alloc=36.52, [VCF], pv=B6
M1, Red, H8, d=0, n=0, nps=0, ebf=0.0, tt=0%, hf=0%, fh1=0%, s=+0, thr=4, t=0.00, alloc=22.50, [BOOK], pv=H8
M12, Red, H10, d=16, n=18m, nps=2.8m, ebf=2.0, tt=44%, hf=78%, fh1=91%, s=-25, thr=4, t=0.01, alloc=4.00, [PONDER], pv=H10 I9 J8 K7 J10
game 1: hard-1 (red) beat hard-2 (blue), 37 moves, won by 4
series hard-1 2-1
```

Notation: `d` search depth, `n` nodes, `nps` nodes per second, `ebf` branching factor, `tt` transposition hit rate, `hf` table occupancy, `fh1` first-move fail-high rate, `s` score in thousands from the mover's side or M# for a solver-proven mate in #, `thr` worker threads, `t` actual wall spent on the move, `alloc` the clock's budget draw for the move, `pv` the principal variation. The trailing tag names the producer: empty for a normal search, `[VCF]` and `[VCT]` for threat-solver proofs, `[BOOK]` for an opening-book reply, `[PONDER]` for an answer adopted from a ponder search run on the opponent's clock. A game verdict names the winner with its seats, the move count, and the winning shape (an open four here), and the series line carries the final tally.

Time controls 1+0, 2+1, 3+2, 10+5 each run the same PID-steered budget controller, the first three on bench-tuned gain rows and 10+5 on the 3+2 gains held provisionally: per-move grants spread the spendable remainder over expected moves left plus an increment share, steered against a planned drain trajectory, floored so a drained clock still funds minimum moves and a timeout can never decide a game.

## ui/ux features

The ui/ux table is the living surface inventory, rebuilt against finished engine behavior when the overhaul lands.

| surface | capabilities | access |
| --- | --- | --- |
| login | login-or-create with timing-flat argon2id, seeded demo admin | public |
| home | player stats, live rooms grid, create-room form (time control, best-of, tier), live-run banner | public |
| room | 256-cell live board, rules-legal playable mask, tap ghost, tabular ticking clocks, bot M-line log, move history, ready and forfeit handshake | players; live stream public |
| terminal room | honest terminal state, mid-game reload rehydrates | public |
| history | per-game score lines, turn counts, move previews | signed-in players |
| playback | first, prev, next, last, autoplay stepping through a finished game | the game's two players |
| tournament | admin setup with config-bounded roster, live board per series, frozen leaderboard, stalled-run close | public read, admin drives |
| presentation | dark-first token layer (color ramps, type and space scales, two type voices: human prose and mono machine data), shared card, chip, field and button recipes, amber reserved for live-attention data marks, accent-strong focus rings, 3:1 contrast floor on state-carrying boundaries, reduced-motion respected, 44px touch floor, two-tap select-then-confirm on coarse pointers | all |
| install | PWA manifest with generated icon set, in-shell install action, offline fallback page; the service worker caches the static shell only and never answers api calls, the event stream, or live room and tournament state | all |

## screenshots

The whole app flow as 390px dark mobile captures, primary presentation. The set is staged through playground/shots behind make shots: the production pages over a seeded store and scripted tournament snapshots, with live rooms driven move by move and no search ever running, so captures never contend with the strength gates for the machine. The bot-room M-line log mid-game, the driven-run live boards and the finished leaderboard of a real run land with the v0.28 master pass gate, which requires screenshots from all supported flows on real systems. Login-or-create (an unknown name registers, the seeded admin row holds its name):

![login or create on a phone](docs/screenshots/mobile-login.png)

Logged-in home with the rooms grid, the create-room form (time control, best-of, opponent tier from the config hub), and the banner headlining a live tournament while one is driven:

![live rooms grid on a phone](docs/screenshots/mobile-home-rooms.png)

![home banner while a tournament runs](docs/screenshots/mobile-home-banner.png)

Live room mid-game on the player's side: turn call-out, tabular clocks ticking between syncs, the running move history and the forfeit control:

![live room on a phone](docs/screenshots/mobile-live-room.png)

A finished series holds its verdict on the open page after the room retires:

![terminal room on a phone](docs/screenshots/mobile-terminal-room.png)

Match history with per-game score lines and move previews:

![match history on a phone](docs/screenshots/mobile-history.png)

The playback board stepping through a finished bot game, first to last with autoplay:

![playback on a phone](docs/screenshots/mobile-playback.png)

Tournament setup, admin-only: the config-bounded roster, tiers, time control, best-of, start rating and parallelism, with the run-gate close form for undriven runs:

![tournament setup on a phone](docs/screenshots/mobile-tourney-setup.png)

While the run drives, its page carries a live board per ongoing bot series in the room board's own style scaled into the card (stones in play order, score from red's side, the card links the room's own page), all public including guests:

![tournament live boards on a phone](docs/screenshots/mobile-tourney-liveboards.png)

The finished run page with the frozen leaderboard:

![tournament run page on a phone](docs/screenshots/mobile-tourney-run.png)

The mobile pass audited every surface at 390x844: long room-id and bot-name tokens wrap or clamp instead of overflowing, controls hit the 44px touch floor under pointer:coarse, the board offers only the spaces the rules allow, a finished series holds its verdict on the open page, and the tournament leaderboard scrolls inside its own wrap.

## grounding

- [first-cause.md](first-cause.md): the founding spec, rules, hardware budget, and match scenarios.
- [docs/plans/first-cause-rebuild.md](docs/plans/first-cause-rebuild.md): milestone plan for the rebuild, updated as milestones land.
- ref/: offline chessprogramming.org and gomocup grounding, browser-sourced.

## build and verify

Requires go 1.27.1+, a C toolchain (gcc or llvm/clang), GNU make, with CGO enabled.

```
make doctor          # toolchain check: go >= 1.27.1, CGO, gcc
make ci              # fmt-check, vet, build, race tests, coverage gates
make mutate-smoke    # rules-core mutation gate (runs in CI on every push)
make mutate-full LABEL=vX PARALLEL=6 CHALLENGE=1  # whole gate, auto-resumes host crashes
make mutate-resume LOG=prior-run.log PARALLEL=8 CHALLENGE=1
make bench           # engine, clock, rules benchmarks with allocs
make fuzz            # rules differential fuzz target
make clocktune       # PID gain sweep for one time control, artifact under playground/clocktune/out/
make diagrams        # render the dark PNG diagrams from the mermaid sources
make logstats          # evidence report over the newest logs/tourny run
make db-checkpoint     # fold every db/*.db write-ahead log into its main file
make serve             # serve on the configured port, db under db/
```

Coverage gates: 95% overall, 100% on internal/rules, internal/engine, and internal/clock. The working tree keeps its artifacts placed: databases under db/, tournament logs under logs/tourny/, nothing database- or log-shaped in the root.

## repository layout

```
cmd/caro          entrypoint (ports, firewall)
cmd/covergate     coverage gate over go cover profiles
cmd/mutate        in-house mutation gate (parallel, resume, challenge audit)
internal/config   single constants hub
internal/rules    board, legality, win detection (M1)
internal/pattern  exhaustive line-pattern tables (M2)
internal/engine   search core, SMP tiers (M3)
internal/vcf      VCF/VCT solvers (M4)
internal/clock    increment-safe time manager, per-TC PID (M5)
internal/server   store, auth, series and rating, write queue, rooms, htmx ui (M6)
internal/tourney  round robin, conductor, tournament store and logs (M7)
playground/       git-tracked R&D ground
docs/             rebuild plan, diagram sources and renders, screenshots
```

## diagrams

Search pipeline, one pass per completed iteration. The soft stop is consulted at iteration heads only, and only completed iterations become the answer.

![search pipeline flowchart](docs/diagrams/search-pipeline.png)

Lazy SMP wraps the same pipeline: one persistent locked-thread worker per core, parked between searches, all sharing one lockless direct-mapped transposition table; a worker's proven immediate win halts its siblings through a shared flag folded into their deadline.

Threat-solver soundness harness. The solvers generate only threat moves (fours for VCF, threes and fours for VCT) from the init-computed pattern tables; a test-only brute-force oracle over the independent naive board cross-checks every claimed win, and that fuzz runs in CI permanently.

![threat-solver soundness harness flowchart](docs/diagrams/threat-solver-harness.png)

Series state machine inside every room. Host takes red first; the loser of a decisive game takes red next; a draw retains red; quitting books a loss for every remaining game.

![series state machine](docs/diagrams/series-state-machine.png)

Room data path. Gameplay never blocks on the disk: moves apply in memory, mutations queue to the single SQLite writer, and telemetry fan out to every subscriber.

![room data path flowchart](docs/diagrams/room-data-path.png)

The tournament conductor. Every series rides the same room surface a human match does; the conductor subscribes before the room can retire, reconciles each finished game against the room's authoritative move list, and fails the whole run on any divergence.

![tournament conductor flowchart](docs/diagrams/tournament-conductor.png)

The machine-wide core budget is held by the run gate: one ongoing run at a time, per-run parallelism bounded by the tier cores, and an undriven run (stalled or failed) closed explicitly from its page before another can start. Starting runs and closing them is the seeded admin account's alone; every page, the setup list, the run boards, and the home's live-run banner read public, guest included.

Page surface. The shell and room pages are server-rendered html/template with vendored htmx 4; page routes mount beside the JSON API and win on their patterns, everything else falls to the shell.

![page surface flowchart](docs/diagrams/page-surface.png)

The pure client functions (tap-to-preview selection, the M-line UI filter) ship as byte-pinned twins: the same source is pinned between markers in the served JS and asserted byte-equal by Go tests, so the browser and the test suite cannot drift.

## implemented design

Rules run on bitboards, 4x uint64 per color over 256 cells, with zobrist keys from a splitmix64 stream and an independent naive oracle transcribed from the spec sentence; the differential harnesses swept every 9-cell window per direction in both board regions and the committed fuzz corpus has millions of clean executions. Make and Unmake cost 3.7 ns, win detection on the last move 13.4 ns, zero allocations anywhere in the package. The pattern tables are computed at process start from the rules package's own win predicate, 4^9 entries per direction in about 52 ms, so the tables cannot drift from the rules; lookups cost 0.26 ns. The threat taxonomy (win-in-1, four, open four, three) is derived mechanically from win-in-1 counts because folk renju patterns do not transfer to exact-5-with-overline-nullified rules.

The engine is fail-soft PVS with iterative deepening, a lockless direct-mapped transposition table with mate-score ply adjustment, killer and history ordering, and a selective forced-four defense extension whose nodes expand only cells whose window interaction reaches the four class, so the restriction preserves node value while collapsing the extension storm; no quiescence, no LMR, no null move, and the hot path allocates nothing (0 B/op searches at 1, 2, and 4 workers). Lazy SMP keeps one persistent locked-thread worker per core parked between searches over the shared table, roughly 5 to 6 Mnps at 4 workers. Tiers are resource-only: easy 1 core with 32 MiB and no solvers, medium 2 cores with 128 MiB and VCF, hard 4 cores with 1 GiB and VCF plus VCT, master 8 cores with 2 GiB, VCF plus VCT, and ponder. Ponder dispatches the same full worker set under an external stop with no time bound, Search and Close halt an active ponder before starting, and SearchStats carries a ring of the last completed root iterations (best move and score each) that feeds the adoption gate. The solvers run in front of the tier's standard search, each pass capped at half of what remains of the move grant plus a node budget: a proof ends the move on the solver's line tagged [VCF] or [VCT] with mate-distance scoring, a miss hands the unspent remainder to the untagged search (the compounding cap keeps at least a quarter of the grant), and a grant under SolverMinGrantMs skips the passes whole so a drained clock's move floor funds the search. Solvers search threat moves only, from the pattern tables, and a test-only brute-force alpha-beta oracle over the naive board verifies every claimed win; that soundness fuzz runs in CI permanently.

The time manager grants per-move budgets as feedforward (spendable remainder over expected moves left plus an increment share) steered by a PID gain set per time control against the planned drain trajectory, with 10+5 provisionally riding the 3+2 gains because its tracking objective is structurally flat, clamped so the floor always funds a minimum move: a fully drained 1+0 clock still moves forever and a timeout can never decide a game. Past the expected move count the grant dumps the spendable remainder. The soft stop refuses new iterations past 65% of the grant, treating a zero clock reading as one 16 ms Windows tick.

The server persists to embedded SQLite in WAL mode behind forward-only startup migrations and a single-writer mutation queue (writes block, never drop), each write riding a per-mutation deadline from the config hub so one wedged write cannot stall the queue, and Close unblocking every sender deterministically: a sender blocked on the full queue returns ErrQueueClosed the moment the flag drops, and everything appended before the flag applies before the worker exits. One game completion is one transaction: the game row, both rating events, and the series finish commit or roll back together, so the zero-sum rating ledger cannot tear. Passwords hash with argon2id at 64 MiB and 2 passes, per-user parameters, timing-flat login-or-create. Every match rates at +30*K and -30*K with K = 10^((R_loser-R_winner)/D) and D = 3000 on an upset or 500+2.5*R_loser clamped otherwise, applied per match on pre-match ratings. Each best-of series (bo3/5/7/11 over 1+0, 2+1, 3+2, 10+5) runs as an explicit state machine: host takes red first, the loser takes red next, a draw retains red, quitting books a loss for every remaining game. Bot engines are ephemeral per game and owned by one worker goroutine each; a forfeit or shutdown racing a search discards the stale answer instead of closing the engine under it. Pondering rides one lane goroutine per room that owns every ponder engine call through a cond-signaled command queue: after a ponding seat's own move the room arms the predicted reply from its PV, the lane searches the predicted board on the opponent's clock, and the next turn's stop joins before the seat's own search can start. A hit (the opponent played the predicted move) passes the exported four-clause adoption gate, solver proofs exempt, else depth-or-time adequacy against the seat's grant-history ring plus best-move stability, score stability, and a stable final iteration. An adopted answer lands tagged [PONDER] under the M-line stat law (alloc the budget draw, t the turn wall, nps from ponder nodes over ponder elapsed), any miss or gate failure falls through to the clock-budgeted search over the warm table, and bot-vs-bot rooms where a seat ponders book both tiers' cores or run ponder-off when the machine budget refuses (full width or off, never half). Each tier seats a reserved, unloginable AI account; every bot move stores its rendered M-line as a game_stats row inside the completion transaction; W-L-D and level count bot games while ratings stay pvp-only with zero rating events on any bot seat; history and playback open bot games with the bot seat named <difficulty>-<roomid> (schema v5), and history score lines count the row's players, not stone colors, so red rotation never misattributes a win. The transport is JSON plus SSE with guest observability, subscribe-before-liveness stream setup, and a drain-then-backstop stop path on SIGINT or SIGTERM. Bot telemetry rides the room-keyed pub-sub hub as zero-alloc M-lines.

The mutation gate is in-house, go/parser and AST rewrites only: parallel workers over isolated module copies with serial confirmation of every survivor, resume that replays prior kills after host failures, an equivalence allowlist whose every entry is challenge-audited by running the suite under the allowlisted mutant, and a self-pruning unused-entry alarm. Current state: 1596/1596 mutants over rules, engine, and clock with 0 survivors and 75 challenge-audited allowances.

The UI is server-rendered pages over the same session cookie the API mints: the shell (login-or-create, home with stats and the rooms grid, history with move previews) and the room and playback pages mount beside the JSON transport, page patterns winning over the root catch-all. Dark mode is the primary presentation across every page, one custom-properties palette whose contrast pairs are machine-checked by playground/darkcontrast. A room page rehydrates the whole board server-side so a mid-game reload misses nothing, then lives off the SSE stream (htmx hx-sse first, a plain EventSource taking over when htmx stops reconnecting) with a one-second detail poll as the safety net and a client-side clock tick between syncs. Coarse pointers get a two-tap flow, select then confirm, with a hover ghost for fine pointers. The bot log line is the raw zero-alloc M-line with ebf, hf and fh1 stripped at the page boundary. Playback reads one finished game row straight from the store, visible only to its two players, and steps by toggling stone visibility only. The two pure client functions are byte-pinned twins asserted from Go.

Tournaments run bot-versus-bot on the exact room surface human matches use: each pairing meets twice with host and guest seats swapped so every participant holds each color equally, the bot series alternating red after every game rather than the loser-takes-red rotation of human play. Participants register under unique names (easy-1 through hard-2 on the default roster) and every log line, verdict, and summary names them. Each run owns a timestamped folder logs/tourny/<timestamp>-<label>/ holding the per-series txt logs (every M-line plus one summary line per finished game) and a summary.txt with the final rating table, series won, and W-L-D per participant. The conductor subscribes before the room can retire, writes every game to the schema v3 tournament tables as one transaction, and reconciles each finished game against the room's authoritative move list, so a lost or duplicated event anywhere fails the run instead of corrupting a record; a replay validator and the room's terminal verdict act as second and third nets. The per-match rating law runs in the tournament's own space from a configurable start rating, standings derive from the game rows so nothing can drift, and the close writes a frozen snapshot. One run holds the machine at a time: the core budget check plus the ongoing-run gate keep concurrent starts from oversubscribing the engines. A run interrupted by any termination shape (SIGINT, a killed process, a machine cut) resumes from its own persisted state behind make tourney-resume: the run's label rides its row (schema v7) so the resume reopens the interrupted run's own log folder, settled series stand as evidence and are never replayed, an interrupted series' partial games are scrubbed and the series replayed whole, and the summary lands once at the finish; the live drive holds the run behind a claimed heartbeat lease on its row (schema v8), so a resume or a hand close refuses while any drive's claim is fresh and takes over only a stale one, a zombie drive's late writes fail the store's claim fence, both arms refuse pre-v8 legacy runs, and both run from the repo root (the log root is cwd-relative); a stalled run nobody wants to continue is abandoned through the page's close form or make tourney-close. Strength evidence of record: the corrected-clock full 2+1 gate on the fixed solver wiring, 30 series and 66 games with an exact zero-sum and zero cross-tier inversions (hard-2 1199, hard-1 1169, both mediums 1138, easies below, hard-1 beating hard-2 in their mutual meet, logs/tourny/20261006-174554-full). The earlier gates and their caveats are tabulated per wave in devlog.md.

## status

The rebuild closed at v0.20 with the final issueless MVP. Landed since: v0.21 with the 10+5 control, v0.22 closed by the preregistered skip decision, v0.23 with the master tier and its strength gate BLOCKED (tier untagged), the demand-spec tier re-scale in tree behind the ladder re-gate, and pondering in tree behind its arena pair. The increment-axis investigation, the retire ordering law, and the ponder t-inflation fix closed on evidence with no tier constant moved. Blocked: the rescale tag on the re-gate redesign, the ponder tag on the master-noponder pair, the master tag on the v0.29 re-derivation reserve. The full wave-by-wave record with every evidence pointer lives in [devlog.md](devlog.md).

## roadmap v0.21 to v0.30

v0.21 (10+5 time control) and v0.22 (TT round skipped by preregistered decision) are closed. v0.23 (master tier and admission basics) closed with its strength gate BLOCKED, the tier untagged pending the re-derivation round. The tier table is the demand spec set 2026-10-08. The resource re-scale (easy 32 MiB, medium 128 MiB, hard 1 GiB, superseding the v0.22 skip decision) and the v0.25 pondering scope (book excluded) landed in tree behind their gates, the smoke21 ladder re-gate for the sizes and the master-noponder arena pair for ponder, both serialized behind the re-gate run. Every tier change rides the inversion research law the v0.20 program established: capability perturbation in the arena, self-play noise floors, and a ladder re-gate before any strength-bearing constant ships.

- v0.24 opening book (master-only): a committed hybrid-law book over the first 16 plies, entries backed by solver proofs or deep self-play agreement over a visit floor, built offline in playground and never hand-curated; master probes it, replies land instantly tagged [BOOK], and the 40-of-40 red-first 1+0 floor is the case for it: at fast clocks the opening decides the game.
- v0.25 pondering: master searches the predicted continuation on the opponent's clock and adopts the tree only through the four-clause gated law (depth or time adequacy, best-move stability over recent iterations, no score drop beyond margin, no last-iteration change; solver proofs exempt), re-searching from scratch on a miss with the shared table keeping misses warm; adopted answers land tagged [PONDER]. Ponder budgets are machine time under an external stop, never the seat's own clock.
- v0.26 admission control: seat-aware reservation of threads and table bytes across rooms and ponder lanes under the machine-wide budget.
- v0.27 ui overhaul: the design system pass, PWA install shell, lobby and spectate browsing, player profiles, and the screenshot recapture of every surface against finished behavior.
- v0.28 master pass gate: master complete with the 16-ply optimal opening book, pondering, full VCF/VCT, 8 threads, 2 GiB TT. Pass threshold: master's W+D record across both red and blue seats must hold 60%+ versus hard, 75%+ versus medium, and 90%+ versus easy; anything below is a strength anomaly and reopens the tier behind the inversion research law. Evidence includes screenshots from all supported flows on real systems, including the ui-started tournament mid run with 2 master bots.
- v0.29 reserve: consumed when an arena verdict fails its preregistered rule and demands a re-derivation round, or the runner migration produces the first timing flake.
- v0.30 final close: full gates, the closing ladder round-robin over all 4 time controls with zero inversions at 2+1, 3+2, 10+5 (1+0 recorded as thread-decided), README final state, evidence bundle, tag.

Concurrency and memory: an engine never runs below its tier's shape, no reduced parallelism, no width degradation. The machine-wide core budget (MachineCores in the config hub, sized per machine, a budget decision rather than a hardware reading) holds every search at its full spec, and demand beyond it queues or fails loudly instead of shrinking the engine. Pondering makes master hold its threads through the opponent's turn, so a master-versus-master game wants twice master's thread count: the ledger funds full-width ponder or runs the game ponder-off, never half-width. The tournament's worst-case table footprint at the spec sizes (2x2 GiB master, 2x1 GiB hard, 2x128 MiB medium, 2x32 MiB easy, about 6.3 GiB) stays inside the RAM/4 machine cap the same controller holds.
