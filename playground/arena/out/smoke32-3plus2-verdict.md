# smoke32 second arm verdict (3+2)

Outcome of the 3+2 arm registered in smoke21-rescale-ponder-preregistration.md, scored against the same four fixed rules. Run: 56 series, 147 games (113 decided, 34 drawn), the 8-seat roster over the re-scaled sizes at 3+2 bo3, logs/tourny/20261009-101129-smoke32, db/gates-v026.db. The whole run lived under the GOMEMLIMIT tournament bound: the launch target carried it from the first game. One external kill at 2026-10-09 20:19 mid s31 (master-1 versus hard-2, no panic, no event-log entry, memory never breached) was resumed per the resume-on-dead law with s31 replayed whole, and the resume survived two launch faults (a console Ctrl+C kill, a make quoting failure) before the hidden-detached launch carried it to the finish.

## rule 1, ladder: FAIL, rescale tag stays blocked

Final standings by rating: master-2 1212, hard-2 1153, medium-2 1101, hard-1 987, master-1 953, medium-1 950, easy-2 928, easy-1 716.

The standings clause fails on one inversion: medium-2 at 1101 finishes above hard-1 at 987. Every easy seat sits below every medium seat, hard-2 at 1153 sits above both mediums. logstats' series-wins-first ordering counts the same shape (medium-2 second with 9 series wins against hard-2's 8).

Aggregate records against the tier directly below: hard versus medium 6-3-1, medium versus easy 6-3-2, both winning. The cross-two-tier record hard versus easy is 7-2-1, hard winning the aggregate, where the 2+1 arm had hard losing it at 9-10-3.

The anomaly shape repeats: hard-1 collapsed to 987 at 16-15-6 while hard-2 held 1153 at 19-14-3, 166 rating points apart at the same tier shape, and hard-1 lost its medium aggregate 1-2-1 while hard-2 took its own 5-1. The 2+1 arm measured the same gap at 204 points (788 versus 992). Two arms, two controls, the hard-1 collapse reproduces, so the hard-tier investigation stands ahead of any rescale tag.

## rule 2, master dataset: recorded, descriptive only

Aggregate game records, both seats pooled: versus easy 10-0-2, versus medium 7-3-0, versus hard 5-4-1, master-2 over master-1 3-1. The hard edge splits: master-2 versus hard 5-0-1 while master-1 versus hard went 0-4-0, and master-1 finished fifth at 953 under both hard seats. Red/blue split across all master games: red 25-9-3, blue 12-20-3. Seat records: master-2 took 24 wins at 1212, master-1 took 13 at 953.

Logstats telemetry per master seat set: depth mean 8.7 (median 8), mean n 34.16m, mean nps 7.26m, mean t 3.97 s against mean alloc 6.70 s, tt 18.3 percent. Inputs to the v0.29 re-derivation, no threshold invented here.

## rule 3, ponder health: FAIL on the t property, ponder tag stays blocked, confound resolved

425 [PONDER] m-lines across the run, adoption 229 of 1377 master-1 moves (16.6 percent) and 196 of 1289 master-2 moves (15.2 percent). 420 lines carry t under 0.1 second with the ponder stat law intact. 5 lines carry t of 19.00 to 30.20 seconds, all inside the two master-versus-master series (s28, s29).

The 2+1 arm carried the same signature, 5 inflated lines at 18 to 30 seconds confined to its two master-versus-master series, but with the confound that its own master-versus-master evidence all postdated the memory breach. This arm removes it: every game ran under GOMEMLIMIT from launch, no breach occurred, and the inflation still appears in exactly the one pairing where both seats search and ponder at the full machine core budget. The GOMEMLIMIT and memory-breach theories are eliminated. The surviving suspects stand, now two-arm replicated: a stop that returns without joining the running job across intervening turn starts, or elapsed-nodes bookkeeping that survives a job boundary on the same engine. The instrumented engine-level repro with per-job node and timestamp accounting decides it, queued behind the arms.

No illegal moves, no engine panics, no conductor reconciliation failures, no unfinished series, no fold mismatches: the run itself is clean evidence.

## rule 4, ledger: PASS

Ratings close at exactly 8000, the 8-seat start sum. Games reconcile at 113 wins, 113 losses, 34 draws, 147 games. Parse ledger: 0 unattributed m-lines, 0 unfinished series, 0 fold mismatches, every unparsable line confined to summary.txt which is not a series log.

## follow-ups

smoke10 (1+0, the zero-increment arm, analysis-only) launched on the freed machine, logs/tourny/20261010-052633-smoke10, db/gates-v027.db. The hard-tier investigation (hard-1 collapse at two controls, master-1's 0-4 against hard feeding the same question) and the ponder stop-join repro ride behind it per the preregistered chain, all serialized engine batches.
