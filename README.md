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

Engine tiers are resource-only shapes of one search. This table is the demand spec: the re-scaled sizes and the ponder column landed in tree behind their gates, the ladder re-gate for the sizes and the arena pair for ponder, and the book column fills at v0.24. The status section records what is shipped today.

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

Tournaments run bot-versus-bot on the exact room surface human matches use: each pairing meets twice with host and guest seats swapped so every participant holds each color equally, the bot series alternating red after every game rather than the loser-takes-red rotation of human play. Participants register under unique names (easy-1 through hard-2 on the default roster) and every log line, verdict, and summary names them. Each run owns a timestamped folder logs/tourny/<timestamp>-<label>/ holding the per-series txt logs (every M-line plus one summary line per finished game) and a summary.txt with the final rating table, series won, and W-L-D per participant. The conductor subscribes before the room can retire, writes every game to the schema v3 tournament tables as one transaction, and reconciles each finished game against the room's authoritative move list, so a lost or duplicated event anywhere fails the run instead of corrupting a record; a replay validator and the room's terminal verdict act as second and third nets. The per-match rating law runs in the tournament's own space from a configurable start rating, standings derive from the game rows so nothing can drift, and the close writes a frozen snapshot. One run holds the machine at a time: the core budget check plus the ongoing-run gate keep concurrent starts from oversubscribing the engines. A run interrupted by any termination shape (SIGINT, a killed process, a machine cut) resumes from its own persisted state behind make tourney-resume: the run's label rides its row (schema v7) so the resume reopens the interrupted run's own log folder, settled series stand as evidence and are never replayed, an interrupted series' partial games are scrubbed and the series replayed whole, and the summary lands once at the finish; the live drive holds the run behind a claimed heartbeat lease on its row (schema v8), so a resume or a hand close refuses while any drive's claim is fresh and takes over only a stale one, a zombie drive's late writes fail the store's claim fence, both arms refuse pre-v8 legacy runs, and both run from the repo root (the log root is cwd-relative); a stalled run nobody wants to continue is abandoned through the page's close form or make tourney-close. Strength evidence of record: the corrected-clock full 2+1 gate on the fixed solver wiring, 30 series and 66 games with an exact zero-sum and zero cross-tier inversions (hard-2 1199, hard-1 1169, both mediums 1138, easies below, hard-1 beating hard-2 in their mutual meet, logs/tourny/20261006-174554-full). The earlier gates and their caveats are tabulated per wave in the status section.

## status

- v0.1 (M0..M4): rules core at 100% with zero surviving mutants, init-computed pattern tables, zero-alloc PVS+ID+TT search, lazy SMP tiers, VCF/VCT solvers with a brute-force soundness oracle.
- v0.2 (M5): increment-safe time manager with one tuned PID gain set per time control, soft-stop wiring with the Windows clock-quantum guard, mutation gate green at 1573/1573 over rules+engine+clock.
- v0.3 (M6a part 1): SQLite WAL store with forward-only self-migration, per-match rating law, series state machine, single-writer mutation queue, room stats pub-sub hub, argon2id login-or-create, zero-alloc M-line emitter.
- v0.4 (M6a part 2): rooms with the live match driver (human and bot play, per-game engine ephemerality, clock-law budgets, M-line telemetry), JSON + SSE transport with guest observability, drain-then-backstop shutdown, completion as one transaction, per-player history, schema v2 indexes, property fuzz for the series and rating laws, blind adversarial pair with all 10 findings fixed.
- v0.5 (M6b part 1): htmx 4 shell. Login-or-create, home with name, rating, W-L-D, level and the live rooms grid, create-room form from the config hub, logout, history list with per-game score lines and playback links.
- v0.6 (M6b part 2): room and playback pages. Server-rendered 256-cell board with mid-stream reload rehydration, SSE push with EventSource fallback, ready and forfeit handshake, tap-to-preview ghosts, tabular clocks ticking client-side, move history, bot M-line log, playback stepping; browser-verified against Implications 1.1, 1.3, 1.4 and 1.5.
- v0.7 (M7 part 1): tournament core and conductor. Bot rooms on the same surface, twice-pair round robin, tournament rating space, schema v3 tables, per-series txt logs, frozen leaderboard snapshot, headless smoke and full drivers, machine-wide run gate; root-fixed the deferred read-then-write BUSY_SNAPSHOT defect with immediate transactions and a 30-parallel regression test.
- v0.8 (M7 part 2): tournament UI and official gates. Setup page, run page with live leaderboard, past runs, stalled-run close; smoke gates green at 1+0 and 3+2, full 2+1 with the ladder holding; adversarial pair's 3 confirmed findings (per-run core budget, replay-net prefix hole, series-seat validation) fixed red-green.
- v0.9 (M8): release candidate. Screenshots, completed diagram set, bot seat naming unified, fuzz corpora committed; make ci 96.0% overall, core 100%.
- v0.10 (MVP reopen, instrumentation): one trace line per finished game in every series log, the conductor dispatch-after-cancel fix, dark PNG diagrams behind make diagrams, the logstats evidence analyzer.
- v0.11 (player-facing record, dark mode): schema v4 game_stats persisting every bot M-line, reserved unloginable AI accounts seating the bots, ratings unmoved on bot seats, history and playback opening bot games; dark mode as the primary presentation, WCAG-checked by playground/darkcontrast.
- v0.12 (gate-log investigation closed): docs/evidence/investigation-2026-10-04.md root-caused the gate anomalies: minute-unit time controls (the gates had run on seconds), the forced-4 restriction plus best-ordered fallback replacing the extension storm, tier solver wiring behind [VCF]/[VCT] tags, swap-host-guest meets with named participants, per-run log folders, and <difficulty>-<roomid> bot naming (schema v5).
- v0.13 (tournament UI close-out): admin-gated creation, public run pages with a live board per series, the home live-run banner, the run-gate close form, horizontal dark diagrams, the 390px dark screenshot recut; same-tier pairings render both instances.
- v0.14 (mutation pipeline hardened, full gate): run-private build caches after the 28 GB shared-cache crashes, a post-run residue check, auto-resuming mutate-full, a CI smoke gate; full gate 1596/1596 with 0 survivors and 75 challenge-audited allowances; the CI race suite closed the CloseStalled drive-lag window.
- v0.15 (run recovery): schema v7 run labels with whole-series replay of interrupted series, headless resume and close arms; schema v8 drive lease fencing second drives, resumes, and closes with stale-claim takeover and zombie-write rejection, pre-v7 runs closed as legacy, summary-before-flip close ordering, runner-sized suite timeouts; adversarially hardened.
- v0.16 (corrected-clock full gate): 30 series and 71 games with an exact zero-sum and a clean parse ledger; honest inversions (medium-2 first at 10/10 series) kept the calibration open; evidence folded into the committed dbs via make db-checkpoint.
- v0.17 (corrected-clock smoke pair): 3+2 holds the tier ladder exactly (hard-2 first at 1308, zero inversions); 1+0 inverts it with 489 fallback-candidate lines; the pre-fix full 2+1 keeps its caveat.
- v0.18 (fast-clock calibration, round 1): the floor-grant gate (adversarially hardened) reran 1+0 with zero starved moves against the first run's 489; hard-2 first at 1296, hard-versus-medium even at 7-7, hard-1 still under both mediums.
- v0.19 (corrected-clock full gate, fixed wiring): 30 series and 66 games, exact zero-sum, clean parse ledger, zero cross-tier inversions (hard-2 1199, hard-1 1169, mediums 1138, easies below); hard-1 taking its mutual meet reads the 1+0 hard-1 gap as fast-clock variance; the spec's inversion check passes on corrected clocks.
- v0.20 (final issueless finished MVP): the inversion research program closed through playground/arena: self-play noise floor nil between identical engines at 1+0 (40 of 40 red wins), solver passes add nothing measurable at 1+0 in any configuration while thread count survives as the only separation, and the ladder holds exactly on 2+1 and 3+2; the engine mutation re-gate closed 1596/1596 with 0 survivors; no tier constant moved.
- v0.21 (10+5 time control): the 4th control ships with the tuning seam (NewGameClockWithGains) and the zero-fill trap closer (every ClockPID row authored non-zero); 10+5 rides the 3+2 gains provisionally on the flat-objective evidence; the 20-game hard,hard floor at 10+5 settled 18 red-first wins, 2 draws, 0 blue-first (the v0.22 TT reference band); the smoke105 tournament settled both red-first series 2-1 (logs/tourny/20261007-164757-smoke105); the clock label re-gated 67/67; a playground-modules build step closes the nested-module CI blind spot; the GPU offload assessment closed as not beneficial.
- v0.22 (TT round skipped by decision): the preregistered arena TT round was terminated before comparison data existed (1 of 96 games run), the shipped sizes stand as a working hypothesis, no tier constant moved; the skip, banked bands, and reopen trigger are recorded in playground/arena/out/v022-phaseA-preregistration.md.
- v0.23 (master tier, admission basics, gate BLOCKED): master (8 threads, 2 GiB, VCF+VCT) landed with migration v9 seating, the 8-seat default roster, and the whole-room core ledger refusing over-budget bot rooms; the preregistered strength gate ran 80 games over 4 pairs and blocked the tier: master missed the separation bar at both controls (12 of 20 at 2+1, 8 of 20 at 3+2 with hard taking the pair and 4 blue-seat wins), the master,master floor held 0 blue-seat wins over 40, and master-novct inverted the top at 11-8; the code stays in tree untagged, the re-derivation round is the standing v0.29 reserve trigger, evidence in playground/arena/out/v023-master-table.md.
- 2026-10-08 tier re-scale (demand-spec sizes in tree): the tier literals moved to the demand spec (easy 32 MiB, medium 128 MiB, hard 1 GiB, master unchanged) pinned by exact-literal and hardware-budget tests, with ponder-aware core accounting holding MachineCores against a master searching while master ponders; ships behind the smoke21 ladder re-gate.
- 2026-10-08 pondering in tree (v0.25 scope pulled forward, book excluded): engine StartPonder/StopPonder over the full worker set with the root-iteration ring, the room ponder lane owning every ponder engine call, the exported four-clause adoption gate with the solver exemption, [PONDER] M-lines through the existing telemetry surface, full-width-or-off admission with the ponder-off fallback, and fuzz corpora over the gate and the grant ring; the blind adversarial pair closed with the ring calibration, lane panic survival, desynced-state rejection, and the redundant adoption clause fixed plus the coverage gaps filled, while the solver-pass room-mutex hold (node-bounded, sub-second, master-only) and the pre-drain core release (transient, dissolved by the v0.26 search-level ledger) are recorded as accepted; the engine+server mutation gate closed 2761/2761 with 0 survivors (2709 killed, 52 challenge-audited allowances, 13 entries re-keyed after the wave's line moves, one pruned because the suite now kills it); the master-noponder arena pair queues behind the ladder re-gate.
- 2026-10-09 smoke21 first arm (2+1, re-gate FAIL, evidence stands): 56 series and 146 games over the re-scaled sizes with the ledger exactly zero-sum and a clean parse ledger; the ladder failed its standings check (both mediums above hard-2, easy-1 above hard-1, hard-1 at 788 versus hard-2's 992, hard versus easy 9-10-3 in games), blocking the re-scale tag pending the 3+2 arm and the hard-tier investigation; the master dataset records 14-6-2 versus easy, 14-7-0 versus medium, 14-3-1 versus hard, master-1 taking the mutual meet 3-2, red 31-4-2 versus blue 16-17-1; ponder adoption ran 14.0 and 15.3 percent per master seat with 341 [PONDER] lines, 336 clean and 5 carrying t of 18 to 30 seconds confined to the master-versus-master pairings where both ponders fund the full core budget, blocking the ponder tag pending stop-join root-cause; the run survived a guard kill at series 28 (the GOGC heap goal doubling the live master TTs past the machine cap), resumed whole-series behind the GOMEMLIMIT tournament bound (5d74d92), verdict scored against the preregistered rules in playground/arena/out/smoke21-2plus1-verdict.md, logs/tourny/20261008-223429-full.
- 2026-10-10 smoke32 (3+2 ladder arm, re-gate FAIL, evidence stands): 56 series and 147 games over the re-scaled sizes with the ledger exactly zero-sum and a clean parse ledger, the whole run under the GOMEMLIMIT bound; the ladder failed its standings check on one inversion (medium-2 at 1101 above hard-1 at 987) while every tier-edge aggregate held as winning (hard versus medium 6-3-1, medium versus easy 6-3-2, hard versus easy 7-2-1), blocking the rescale tag pending the hard-tier investigation; the hard-1 collapse reproduces at 166 points under hard-2 (204 at 2+1) with hard-1 losing its medium aggregate 1-2-1; the master dataset records 10-0-2 versus easy, 7-3-0 versus medium, 5-4-1 versus hard with master-1 at 0-4 against hard and fifth at 953, master-2 taking the mutual meet 3-1, red 25-9-3 versus blue 12-20-3; ponder adoption ran 16.6 and 15.2 percent per master seat with 425 [PONDER] lines, 420 clean and 5 carrying t of 19 to 30 seconds confined to the two master-versus-master series, the same signature as the 2+1 arm now with the memory-breach confound eliminated, blocking the ponder tag pending the stop-join root-cause; one external kill at s31 resumed whole-series per the resume-on-dead law; verdict scored against the preregistered rules in playground/arena/out/smoke32-3plus2-verdict.md, logs/tourny/20261009-101129-smoke32.
- 2026-10-10 smoke10 (1+0 arm, analysis-only, evidence stands): 56 series and 141 games (140 decisive, 1 drawn) over the re-scaled sizes with the ledger exactly zero-sum and a clean parse ledger, the whole run under the GOMEMLIMIT bound with no kills or resumes; 1+0 gates nothing per the preregistration addendum and the recorded shape completes the three-control evidence set over the increment axis (0, 1s, 2s) for the formal investigation: both easies above both mediums with medium-2 last at 795 and the medium-easy series split 4-4, masters first at 1298 and 1184, and the hard-1 collapse inverting at this control (hard-1 1036 against hard-2 1039, taking both mutual series 2-1 after trailing by 204 and 166 points on the increment arms); the master dataset records 13-7-0 versus easy, 14-6-0 versus medium, 12-6-0 versus hard, red 28-5-0 against blue 16-19-0 across master seat-games; ponder adoption ran 12.2 and 14.5 percent per master seat with 218 [PONDER] lines, 214 clean and 4 carrying t of 17 to 23 seconds confined to the two master-versus-master series, the same signature at all three increments; verdict scored in playground/arena/out/smoke10-1plus0-verdict.md, logs/tourny/20261010-052633-smoke10, db/gates-v027.db.
- 2026-10-10 increment axis investigation (formal, closed): the preregistered investigation over the three committed arms ran through a new standing instrument (make crossarm, playground/crossarm, per-seat folds with the integrity ledger at zero bad lines, unfinished series, and fold mismatches; data report in playground/arena/out/crossarm-increment-axis.md) and closed every question without licensing a tier change: the hard-1 collapse decomposes into mutual sets hard-1 won at all three controls (3-1-1, 3-2-1, 4-2-0, a 30 percent two-sided coin), telemetry parity with hard-2 at every arm, and cross-tier victim drift that rules out a large stable defect while the pooled 13-21 against the tiers below stays a 23 percent outcome, the 204 and 166 point gaps being schedule-slot variance amplified by the first-player effect (master red 31-4-2, 25-9-3, 28-5-0); master-1's 1-8 versus hard at 3+2 (clean sub-record 1-6 beside the replayed s31, no persistent-instance mechanism since searchers rebuild per game) sits at single-arm n=1 with a 3+2 repetition preregistered as the discriminator; the 1+0 medium inversion rides within-tier seat noise plus a 3-game easy lean on common upper-tier opposition around a direct edge inside noise at every control (11-5-5, 11-7-4, 10-8-1), with medium-easy depth parity constant across controls so no search-shape story exists; power accounting shows one arm's roughly 20 pooled games per tier edge cannot certify even a 70 percent edge (master over easy 14-6 is 11.5 percent two-sided, 194 games per family needed at 0.60), so the re-gate redesign is 3 color-paired 2+1 repetitions with slot re-randomization plus the 3+2 discriminator, certifying 0.70-regime edges and reserving the 150-game-per-family full gate for the closing ladder, all behind a 100-game identical-pair hard-hard noise floor measured per control in color-paired blocks with the master-master floor waiting on the ponder verdict; corrections recorded: the smoke32 verdict's 113/34 split is truly 130/17 (raw-grep confirmed, 17 blue wins had been counted as draws) with its hand aggregates superseded by the fold (master versus easy 13-6-4, versus hard 8-11-1) while smoke21's aggregates verified exact and smoke10's 3+2 draw citation inherits the correction; the rescale tag stays blocked on power, verdict in playground/arena/out/increment-axis-investigation.md.
- 2026-10-10 retire ordering law (5f15aed, 4bf61b8): two CI-only races in TestCreateBotVsBotRefusesWhenLedgerFull closed under one contract, retire the only writer of over (close the quit channel as the work fence, release the room cores, set over last so terminal-visible implies budget-returned) with the room-internal series-done signal moved to a separate finished flag read by every worker and API gate as over-or-finished, after completeGameLocked and Forfeit had reopened the over-before-release window; CI green on both.
- 2026-10-10 ponder t-inflation root-caused and fixed: the [PONDER] signature from all three smoke arms (14 t-inflated lines, 17-54 s, master-versus-master only) and an amplified probe (make tourney-ponderprobe, master-master bo11 twice-pairing at 1+0, 9 inflated of 60 ponder lines pre-fix, t 29-54 s with the stalled seat's clock committing the elapsed) decomposed to one mechanism: the serial ponder lane stalled inside solverSearcher.StartPonder's VCT pass, which ran under a nil deadline and on quiet boards burned the whole 1<<20 SolverNodeBudget at about 30k nps single-threaded (measured 33-42 s on the event boards via playground/solverpass; real proofs in play span 5-68850 nodes), so the opponent's stop queued behind the burn and the stalled seat's turn absorbed the remainder; per the corrected design the pass now runs off the lane from the mover's move until the ponder owner's next turn, however long: StartPonder dispatches the full-width inward ponder immediately and spawns the solver passes on a private board copy under a pure stop flag (engine.Stoppable, which also replaced smp's private ponder deadline), StopPonder, Close, and Search trip and join the pass, StartPonder retires any prior pass first, and the goroutine recovers panics locally; no wall-clock constant exists on the path, the only bounds are the opponent's actual turn and the solver's universal node budget (the ponder tree itself stays unbounded, only the solver sub-pass is node-capped, and re-arming past exhaustion is an arena-gated strength question), while the pass rides as one unbooked thread beside the owner's booked workers until v0.26 admission accounts it; the blind adversarial pair against the final shape caught and closed the shared-board torn-copy race and the unstoppable-pass-on-restart window (each pinned by a new test); post-fix probe: zero stalls over 149 ponder lines in 19 games with the worst turn t anywhere at 4.71 seconds (a deep normal search) and every adopted ponder answer under a second, against 9 stalls of 60 lines pre-fix at t 29-54 seconds, verdict and instruments in playground/arena/out/ponderscan-three-arms.md, ponderscan-probe-prefix.md, ponderscan-probe-postfix.md; the ponder tag re-gate (master-noponder arena pair) queues with the confound gone.

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
