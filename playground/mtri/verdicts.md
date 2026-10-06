# Mutation survivor triage, Oct 2026

Ground truth: every verdict below was executed against the current suite via
playground/mutkill-scratch/mtri (cmd/mutate's exact splice grammar, gate
command `go test -count=1 -short ./pkg`, serial, GOMAXPROCS=4). KILL means
the named test was watched failing under the applied mutant and passing on
clean code. Full logs: playground/mutkill-scratch/{verify.log,verify2.log,redcheck.log,allowspot.log}.

Corrections to the two crashed-run survivor lists: both r1-only sightings
(movegen 49:67 0 with 1, search 266:11 0 with 1) are already killed by the
current suite; the runs that saw them survive predate it. My own first
verification pass also misreported 15 keys as killed (a compile error in my
scratch sweep file failed every build); verify2.log is the corrected rerun
and the table below stands on it.

## Verdicts

| survivor | verdict |
| --- | --- |
| internal/clock/clock.go:86:24 replace 0 with -1 | ALLOW (re-anchored 85:24, clamp-collapse proof, prior 5.4M-pair sweep) |
| internal/clock/clock.go:86:24 replace 0 with 1 | ALLOW (re-anchored 85:24, same proof) |
| internal/engine/bitboard.go:79:10 replace 0 with 1 | ALLOW (new: paired with constrained=false, fill never reads anchor; 0 with -1 sibling auto-killed, uint16 constant overflow) |
| internal/engine/movegen.go:34:17 replace 0 with 1 | KILL TestMutKillEmptyBoardFastPathSlot |
| internal/engine/movegen.go:34:22 replace 0 with -1 | KILL TestMutKillEmptyBoardFastPathSlot |
| internal/engine/movegen.go:34:22 replace 0 with 1 | KILL TestMutKillEmptyBoardFastPathSlot |
| internal/engine/movegen.go:40:15 replace 0 with -1 | ALLOW (new: static is a sum of weights >= 0, static < -1 never fires) |
| internal/engine/movegen.go:40:15 replace 0 with 1 | ALLOW (new: requireLive=true already excludes static == 0; fours path overwrites initializer) |
| internal/engine/movegen.go:46:66 replace 0 with -1 | ALLOW (new: same two facts on the second fill) |
| internal/engine/movegen.go:46:66 replace 0 with 1 | ALLOW (new: same) |
| internal/engine/movegen.go:49:67 replace 0 with -1 | ALLOW (new: reached only with every candidate static == 0; static < -1 unreachable) |
| internal/engine/movegen.go:49:67 replace 0 with 1 | KILL already (TestSearchOpeningRuleSecondMove, TestCandidateSetCoversAllWinInOneCells, TestGenerateRespectsOpeningRule, TestMutKillMate2Depth3Pins, +6 more) |
| internal/engine/movegen.go:105:15 replace 1 with 0 | ALLOW (re-anchored 86:15: j==i self-compare under strict > is vacuous) |
| internal/engine/search.go:212:11 replace 0 with -1 | KILL TestMutKillStoppedRootReturn |
| internal/engine/search.go:212:11 replace 0 with 1 | KILL TestMutKillStoppedRootReturn |
| internal/engine/search.go:236:32 replace <= with < | KILL TestMutKillNodeCheckCadence |
| internal/engine/search.go:236:35 replace 0 with -1 | KILL TestMutKillNodeCheckCadence |
| internal/engine/search.go:236:35 replace 0 with 1 | KILL TestMutKillNodeCheckCadence |
| internal/engine/search.go:243:10 replace 0 with -1 | KILL TestMutKillStoppedEntryReturn |
| internal/engine/search.go:243:10 replace 0 with 1 | KILL TestMutKillStoppedEntryReturn |
| internal/engine/search.go:266:11 replace 0 with -1 | ALLOW (re-anchored 262:11: m == -1 means miss, Move(-1)=65535 equals no cell, ttm feeds only equality) |
| internal/engine/search.go:266:11 replace 0 with 1 | KILL already (TestMutKillTTMoveZeroAdopted; r1 sighting stale) |
| internal/engine/search.go:272:43 replace 1 with 2 | KILL TestMutKillEmptyCandidateSentinel |
| internal/engine/search.go:299:11 replace 0 with -1 | KILL TestMutKillStoppedLoopReturn |
| internal/engine/search.go:299:11 replace 0 with 1 | KILL TestMutKillStoppedLoopReturn |
| internal/engine/stats.go:41:15 replace < with <= | ALLOW (re-anchored 40:15: at exactly 1ms the clamp assigns 1ms over 1ms) |
| internal/engine/stats.go:56:25 replace 1 with 2 | ALLOW (re-anchored 55:25; see disproven hypotheses) |
| internal/engine/stats.go:74:11 replace > with >= | ALLOW (re-anchored 73:11; see disproven hypotheses) |

The lead's kill hypotheses for the last two are wrong, verified two ways:

- stats 74:11: divergence needs term == floor(2^62/bm) exactly. Exhaustive
  scan bm in [1000,32000] x 400 rounds: 0 hits. The bm=2048 claim fails
  because the term ladder is not 2^i: t6 = floor(65.536) = 65, and the
  ladder runs above 2^k from round 6 on, crossing the 2^51 boundary
  strictly; geoSum(2048,51)=geoSum(2048,52)=geoSum(2048,53)=2^62 either way.
- stats 56:25: the mutant mid floor((lo+hi+2)/2) equals the clean mid or
  exactly one above it and never exceeds hi, so both rules preserve
  P(lo) and !P(hi+1) with strict progress and converge to the same fixed
  point. Sweep: nodes 2..2^22 fully x 11 depths plus log-spaced samples to
  2^48, 0 diffs (~4.8M pairs).

DEAD-WRITE findings: none new. The old movegen 25:17/25:22 "dead write"
entries are replaced by kills (the slot write is observable through a
package-internal sentinel prefill, so pinning beats allowing). The 3
previously flagged dead writes in negamax remain deleted code, nothing to do.

## Re-anchor mapping (stale entries, old line -> current line)

| old entry | new | disposition |
| --- | --- | --- |
| clock.go:85:24 x2 | 86:24 x2 | kept, proof line refs refreshed (85->86, 87->88, 89->90, 92->93, Commit 74-82 -> 75-83, config_test 258 -> 85,378) |
| movegen.go:25:17 | 34:17 | dropped: killed by TestMutKillEmptyBoardFastPathSlot |
| movegen.go:25:22 x2 | 34:22 x2 | dropped: killed by same |
| movegen.go:86:15 | 105:15 | kept, same proof |
| stats.go:40:15 | 41:15 | kept, boundary value restated as 1ms |
| stats.go:40:17 | (none) | dropped: the mutant vanished, the literal became time.Millisecond |
| stats.go:55:25 | 56:25 | kept, proof strengthened with the invariant argument + sweep |
| stats.go:73:11 | 74:11 | kept, scan re-verified this session |
| search.go:208:11 x2 | 212:11 x2 | dropped: killed by TestMutKillStoppedRootReturn |
| search.go:239:10 | 243:10 | dropped: killed by TestMutKillStoppedEntryReturn |
| search.go:262:11 | 266:11 | kept, orderScore citation 70 -> 89 |
| search.go:268:43 | 272:43 | dropped: killed by TestMutKillEmptyCandidateSentinel (the old proof's "generate never returns 0" is false on the degenerate 1-red-stone red-to-move board, where the sentinel is observable; the pin now cements -(M+1)) |
| search.go:295:11 x2 | 299:11 x2 | dropped: killed by TestMutKillStoppedLoopReturn |
| tt.go:162:12 | (same line) | dropped: killed by TestZeroAllocsSearch. Masked-kill discovery: the entry matched, so non-challenge gates consumed it without ever running the suite; the equivalence proof predates the engine rewrite. Recommend one challenge-mode gate run to flush any remaining masked entries. |

Entry count: 79 -> 74 (10 dropped as killed/vanished, 6 added for the new
equivalences at movegen 40:15/46:66/49:67 and bitboard 79:10). All 74 match
live mutants in the current tree (mtri anchor: 74 matched, 0 unmatched).
Spot-checked survivors under the current suite: search 24:34/25:13/140:4,
bitboard 81:23 x2/85:16/86:12, tt 157:11/181:7, smp 262:14/346:4/357:4,
eval 87:17 x2, rules win 14:28: all still survive, no other masked kills
found in the sample.

## Files

- killers.go.txt: paste as internal/engine/mutkill_triage_test.go. Green on
  clean code (full engine suite, 13.5s), red under each of its 13 target
  mutants (redcheck.log).
- allowlist.txt: replacement .mutate-allow content.
- mtri/ (source) + mtri.exe: the rebuilt apply-by-key harness
  (playground/mtri from the Oct 2 session no longer exists; this one embeds
  cmd/mutate's collect verbatim and mutates only the work/ copy).
- verify.sh/verify2.sh/redcheck.sh/allowspot.sh + *.log: evidence, rerunnable.

## Reading log

read internal/engine/mutkill_search_test.go 1-691 - killer conventions, countdown deadline, helpers
read cmd/mutate/mutate.go 1-229 - splice grammar replicated in mtri
read cmd/mutate/run.go 1-313 - gate test command, verdict and allow consumption semantics
read cmd/mutate/discover.go 1-61 - GoFiles-only mutation targets
read cmd/mutate/allow.go 1-95 - entry format, unused-allow failure
read internal/engine/movegen.go 1-132 - survivor sites 34/40/46/49/105
read internal/engine/search.go 1-327 - survivor sites 212/236/243/266/272/299 and their callers
read internal/engine/stats.go 1-95 - survivor sites 41/56/74
read internal/engine/bitboard.go 1-92 - survivor site 79, openingAnchor
read internal/clock/clock.go 1-94 - survivor site 86, law
read internal/engine/eval.go 1-120 - weight provenance for the static >= 0 proofs
grep internal/engine/smp.go 95-105,255-300,330-360,398-406 - stopped-discard callers, allow entry anchors
read internal/engine/mutkill_rest_test.go 1-135 - existing geoSum/EBF pins
grep internal/engine/mutkill_smptt_test.go test names - existing smp/tt coverage
read internal/engine/engine_test.go 1-40 + grep helpers - place/mustCell/midgameBoard
grep internal/config/config.go constants - weights, node interval, mate lattice
grep internal/config/config_test.go 85-98,254-262,378 - floor assertions cited by clock proofs
read internal/engine/tt.go 105-185 - probe miss lines for the 266:11 citation
grep internal/rules/legality.go,naive.go,win.go regions - allow entry anchors
