# gametheory: applied game theory over existing artifacts

Inputs: 18 arena logs (463 games) plus 5 corpus shards (307 games at snapshot, live files so corpus numbers are floors). Every number reruns via `go run . -repo <repo root>`. Analysis scripts in this directory.

## verdict 1: first-move advantage is real but moderate, roughly 62 percent at same-base strength

Corrected 2026-10-22: the first pass reported a 100 percent red share on identical-engine floors (78 red, 0 blue, 3 draws over 81 games). That reading is a logging artifact, not evidence. In identical-label pairings both seats share one label, the arena's record() (playground/arena/main.go:393-404) compares winner == redLabel, and that is always true, so every decisive floor game was credited to redWins. The floor logs carry no red-versus-blue attribution at all. The same artifact sits inside v023-master-table.md pair 3's "0 blue-seat wins" and its rule-2 floor calibration. Historical evidence files are left as recorded.

What stands:

- same-base capability-perturbed pairs (book, VCT, ponder, weight toggles): 62.1 percent red [55.4, 68.4] over 222 games
- cross-tier pairs 65.4 percent but confounded by seat alternation
- the decomposition in section 5 shows the same-base edge is essentially the H8 H6 H5 line: 65.4 percent [55.8, 73.8] red over 104 decisive games
- floors: 78 decisive of 81, 3 draws, genuinely varied games (section 5), no seat attribution possible

Product conclusion unchanged: the comeback mechanic (loser takes red next, README.md:28) absorbs the edge, and the v0.28 both-seats W+D gate stays mandatory because any single-seat record carries the structural tilt.

## verdict 2: the book law is the only supportable law, mix is dead until diversity changes

Corpus snapshot: 4912 P lines, 1411 unique positions, plies 0-15 exactly (BookMaxPly holds). Transposition sharing 44.4 percent (627 keys in >= 2 games, 576 across >= 2 of the 3 shards): parallel fleet growth concentrates visits more than it adds breadth. Single-move positions 74.8 percent at v >= 2, 65.3 percent at v >= 5, 48.7 percent at the floor v >= 20. A top-2 mix needs second-move evidence: at floor only 20 keys are multi-move and exactly 1 has a second move >= 20 visits. Mixing today would play single-digit-visit moves. If a mix is ever wanted the lever is corpus diversity, not the selection law. Quantified, parked, no law change.

## verdict 3: ponder singleton policy is the right first bandit, the gate is the lever

Model from ponder.go plus the arena driver: state (pre-move position, side, budget, grant-depth ring), action singleton PV[1] with TTMove fallback, reward hit x banked time, adoption behind the 4-clause gate. Pooled arena outcomes: grandmaster 59.4 percent hit / 28.5 percent adopt over 997 arms, grandmaster-nobook 71.5 percent hit, master 68.9 / 29.2, master-nobook 69.2. This brackets the logmine 214k-move reading of 67-73 percent hit with the gate binding. Next lever is gate tuning, not prediction diversity. Evaluating a real bandit later needs two fields the outcome line lacks: predicted versus actual move per arm, and the candidate set at arm time. Parked until the ponder arena pair reopens.

## verdict 4: no draw death

442 decisive of 463, median 46 moves, p90 86. Draws 21 (4.5 percent), every one at the 256-move full board. No repetition or treaty artifact exists in the ruleset. Watch condition: rising full-board draw rate as strength climbs, from the standing arena pairs per release.

## section 5: opening decomposition and floor independence

Openings from M1 M2 M3, possible only on the six bookgate and evalfit logs, since all four identical-floor logs carry zero M-lines (results only):

| opening | games | red | blue | draws | tcs | distinct full games |
| --- | --- | --- | --- | --- | --- | --- |
| H8 H6 H5 | 109 | 68 | 36 | 5 | 2+1, 3+2 | 100 |
| H8 F8 E8 | 5 | 3 | 2 | 0 | 2+1, 3+2 | 5 |
| H8 G7 H5 | 2 | 1 | 1 | 0 | 3+2 | 2 |
| H8 I7 H5 | 2 | 0 | 2 | 0 | 3+2 | 2 |
| H8 H6 J5 | 1 | 1 | 0 | 0 | 2+1 | 1 |
| H8 I9 K8 | 1 | 0 | 1 | 0 | 3+2 | 1 |

Independence verdict on the 81 floor games: genuinely varied, not a fixed deterministic set. Length diversity proves >= 56 of 78 decisive games distinct (a-noise 26/40, v021 13/18, v023-p3 16/19, v022-A 1/1). At most 22 same-file length collisions remain identity-unknown without move lists, 0 proven repeats, and cross-control games cannot be replays. Separately, 9 of 120 same-base games are exact full-move-list repeats, all inside the dominant H8 H6 H5 opening: deterministic convergence happens even under lazy SMP.

The one-line harness fix that closes the artifact class: emit the winner seat (red or blue), not the winner label, in the arena result line.

Provenance: run1-v023 contended logs (24 games) included and flagged, exclusion moves nothing. The .scratch ponder and smoke probes are real games, included.
