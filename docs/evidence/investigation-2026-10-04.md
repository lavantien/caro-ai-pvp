# investigation: 2026-10-04 log findings

Inputs: the 1+0 smoke10 gate (docs/evidence/tourney/smoke10) and the 3+2 smoke32 gate (docs/evidence/tourney/smoke32), both run before the fixes below. Every claim cites the log line or the code that produced it.

## finding 1: the time controls were seconds, the spec means minutes

The spec's 1+0, 2+1, 3+2 name minutes of initial bank. config carried `TimeControls = {{InitialSec: 1}, {InitialSec: 2, IncrementSec: 1}, {InitialSec: 3, IncrementSec: 2}}` (config.go before 27c9fe4), so every gate ran on a 1-3 second bank. The clock law then reproduced every logged alloc exactly:

- 3+2 move 0: spendable (3000ms - 120ms) / 30 moves + 0.5 * 2000ms = 1096ms, logged `alloc=1.10` (smoke32 s5 M1). The rising 1.10 to 1.79 ladder over 13 rounds is the bank growing off unspent increment (spend ~1.4s, +2.0s per move) while movesLeft shrinks 30 to 17, plus the PID correction (error ~10s against a clamped trajectory, correction capped at 0.25 of feedforward).
- 1+0 move 0: (1000 - 120) / 30 = 29ms, logged `alloc=0.03` (smoke10 s9 M1), declining to 0.01 as the 1s bank drains.

So the time manager itself was never in scramble mode: it faithfully spread a bank that was 60x too small. The law is homogeneous in milliseconds, so the minute fix (TimeControl.InitialMin, 27c9fe4) needs no PID retune; the 3+2 move 0 grant becomes (180000 - 120)/30 + 1000 = 6996ms and 1+0 becomes ~2s per move.

## finding 2: the threat extension widened instead of narrowed, so depth collapsed to d=0

The forced-defense extension fires whenever the opponent of the mover holds a live four (search.go `fours[b.Side^1] > 0`, depth++), but move generation still offered every ring candidate (~100+ cells midgame). Every non-blocking move leaves the four alive, so each child extends again, up to SearchExtensionMaxPly=16: a single depth-1 iteration burned 1.4m-5.7m nodes midgame:

- smoke32 s9 M22: `d=0, n=5.7m ... t=1.68, alloc=1.68, pv=` — 5.7 million nodes, zero completed plies, empty PV.
- smoke10 s9 game 1 M38-M63: both seats at d=0 with n=12k-125k on 30ms grants.
- Depth trajectories (probe of smoke32 s5): 8, 8, 6, 7, 5, 6, 4, 5, 4, 3, 3, 2, 2, 1, 1, 1, 1, 2, 1 through move 26 — collapse regardless of tier.

This is the spec violation, not a tuning issue: first-cause.md says "selective threat extension (only expanding forced 4-blocks, open 3s, ...)". The fix (f4b68ad) restricts candidates at a four-facing node to cells whose centered window interaction reaches the Four class weight, a superset of own win-in-1 cells and the opponent's block cells (both center a Four-or-above window), so the node value is preserved — every pinned mutkill craft kept its move, score, and PV while node counts fell (mid d3 1047 to 578, mate3 d4 3058 to 2032).

## finding 3: the d=0 fallback played the first cell of the A1 scan

With no completed iteration, Search returned fallbackMove = the first candidate in raw enumeration order (search.go before f4b68ad), which scans from A1: the smoke10 s9 d=0 zone played J5, J8, A9, B9, F9, C7, C5, D11, D6, B8, B6 — top-left drift, identical policy for every tier. Once both seats hit it, games became fallback-vs-fallback coin flips, which is the mechanism behind easy beating hard (finding 4). f4b68ad also made the fallback the best-ordered candidate; under a live four the restriction leaves exactly the forced answers.

d=0 is not solver output: no package outside tests imported internal/vcf before 3fc4b73, so easy held no VCF/VCT by spec and hard's solvers never ran at all. Both tiers fell to d=0 through the same plain-search starvation.

## finding 4: why easy beat hard (s9, s22, s26; medium-2 over hard-1)

Three independent equalizers, all now closed:

1. The seconds-units bank starved every tier into findings 2-3, so "hard with full gears" never searched: its logged depth collapsed alongside easy's.
2. The extension storm hits all tiers equally (it is in the shared search), so hard's 4 threads bought nodes, not depth: ebf saturated at the 32.0 clamp (stats.go ebfMilli hi=32000 milliunits) on both seats from the early midgame.
3. The solvers never ran, so the tier table's VCF/VCT capabilities were fiction (see finding 5).

The inversion appears for easy as red and as blue, so no red advantage is involved: the win condition (exact five at move n-1, trace-tagged "won by 4") was always legal; the illegal part was who got to search.

## finding 5: the [VCF]/[VCT] tags never fired because the solvers were never wired

config defined the tags and the M-line format carried the slot, but match.go hardcoded `mLineTag = ""` and nothing called vcf.Solve outside tests. 3fc4b73 wires the tier solvers in front of the standard search: VCF's four-only tree first, VCT's superset second, each under half the grant and SolverNodeBudget nodes; a proof ends the move on the solver PV with the tag, stats mapped onto the search shape (depth = plies, score = M<plies> on the engine lattice, matching the spec's vct-hit golden), a miss leaves the untagged search the remainder. Medium runs VCF only, hard both, easy neither, per Implication 2.1.

## finding 6: cosmetic telemetry defects

- `nps=211000m` (smoke10 s9 M65): npsReport floored a zero elapsed reading to 1ns, so 211 nodes reported as 2.11e11 nps. The floor is now 1ms.
- `hf=0%` throughout is honest: 30ms-1.7s searches never filled the tables (medium 32MB from 27c9fe4, hard 128MB).
- ebf=32.0 is the saturation value, a true "tree exploded" signal, kept as the clamp.

## finding 7: seat and naming language

"Blue won" then "guest won the series" was meaningless because bot series rotated red by the PvP loser-takes-red law while first-cause specifies alternating red for benchmarks. Fixed end to end: NewBotSeries alternates red after every game, the conductor's fold mirrors it (so across a twice-paired round robin each participant's red count balances), game and series log lines name participants, every run owns logs/tourny/<timestamp>-<label>/ with a summary.txt rating table, and the pvp bot renders as <difficulty>-<roomid> (schema v5 games.bot_name).

## verification: same position, same grant, after the fixes

playground/probe replays smoke32 s9 game 1 to a move and searches the position under the logged grant on both tiers (commit e0ca4d5, at f4b68ad):

| position | logged (before) | easy after | hard after |
| --- | --- | --- | --- |
| move 21 (1.68s) | blue d=0 n=5.7m, red d=2 | d=2 n=2.0m pv=G4... | d=2 n=8.5m pv=G4... |
| move 10 (1.35s) | d=3 / d=3 | d=9 n=2.4m | d=10 n=10.1m |
| move 4 (1.17s) | d=8 / d=8 | d=7 | d=12 |

At the starved 1.68s scrape the deepest midgame position still only reaches d=2: that is what a 3+2 bank of 3 seconds buys. The rerun gates at true minutes (60/120/180) are the actual strength measurement; their evidence lands under docs/evidence/tourney/ as each gate completes.

## fixes landed

| commit | fix |
| --- | --- |
| 27c9fe4 | minutes time controls, medium 32MB / hard 128MB TT |
| f4b68ad | selective forced-4 restriction, best-ordered fallback, nps floor |
| 3fc4b73 | tier solver wiring with [VCF]/[VCT] tags |
| f03cb4b | bot-series red alternation, named verdicts, per-run log folders, summary file |
| b8853cc | pvp bot named <difficulty>-<roomid> everywhere |
| 4e5e847 | databases under db/, logs under logs/, root cleaned |
