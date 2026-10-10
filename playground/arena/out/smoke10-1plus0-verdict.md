# smoke10 third arm verdict (1+0)

Analysis-only arm per the preregistration addendum: 1+0 gates nothing, the run completes the three-control evidence set over the increment axis (0, 1s, 2s) that the formal investigation reads. Run: 56 series, 141 games (140 decisive, 1 drawn), the 8-seat roster over the re-scaled sizes at 1+0 bo3, logs/tourny/20261010-052633-smoke10, db/gates-v027.db, finished 2026-10-10 09:29:45. The whole run lived under the GOMEMLIMIT tournament bound with no external kills and no resumes.

## standings: recorded, gates nothing at this control

Final standings by rating: master-2 1298, master-1 1184, hard-2 1039, hard-1 1036, easy-1 939, easy-2 863, medium-1 846, medium-2 795.

Both easies finish above both mediums, the strongest sub-master inversion the program has recorded, and medium-2 lands last. logstats counts 6 tier inversions (the four easy-over-medium pairs plus both hards over master-1). The shape matches the v0.17 finding that 1+0 scrambles the ladder and the v0.20 reading of the control as thread-decided: mean nps separates monotonically by tier (master 7.69m, hard 4.32m, medium 2.21m, easy 1.18m) while the sub-master order inverts below it.

Tier-edge game aggregates, higher tier W-L-D: hard versus easy 14-9-0, hard versus medium 14-7-0, medium versus easy 10-8-1, the last thin with the series split 4-4. Series aggregates: hard versus easy 6-2, hard versus medium 7-1, medium versus easy 4-4.

The hard-1 collapse inverts at this control: hard-1 1036 against hard-2 1039 (3 points apart, both at 9 series wins) and hard-1 took both mutual series 2-1 (4-2 in games), where the increment arms measured hard-2 over hard-1 by 204 points at 2+1 and 166 at 3+2. The collapse is increment-dependent: present with increment, absent at 1+0.

Draws nearly vanish: 1 drawn game in 141 against 34 in 147 at 3+2.

## master dataset: recorded, descriptive only

Aggregate game records, both seats pooled: versus easy 13-7-0, versus medium 14-6-0, versus hard 12-6-0, master-2 over master-1 2-0 in series and 4-1 in games. Red/blue split across master seat-games: red 28-5-0, blue 16-19-0. Against the v0.28 pass bars (90, 75, 60 percent W+D versus easy, medium, hard) master's 1+0 record reads 65, 70, 67 percent: the easy edge collapses hardest at the fast clock, consistent with the opening-decides evidence behind the v0.24 book.

Logstats telemetry per master tier set: depth mean 7.9 (median 9), mean n 13.53m, mean nps 7.69m, mean t 1.27 s against mean alloc 1.96 s, tt 18.9 percent.

## ponder health: t signature reproduced a third time, repro queued

218 [PONDER] m-lines, adoption 92 of 753 master-1 moves (12.2 percent) and 126 of 871 master-2 moves (14.5 percent), below the 14 to 17 percent of the increment arms as expected when opponent turns run about a second. 214 lines carry t under 0.1 second with the ponder stat law intact. 4 lines carry t of 17.15 to 22.72 seconds, all inside the two master-versus-master series (s28, s29), the identical confinement both increment arms showed.

The increment axis is now swept: 0, 1s, and 2s increments all reproduce the inflation confined to master-versus-master contention, which eliminates any increment-scale theory. The stop-join and elapsed-nodes-bookkeeping suspects stand alone, and the instrumented engine-level repro with per-job node and timestamp accounting is the next engine work, queued behind the formal investigation.

No illegal moves, no engine panics, no conductor reconciliation failures, no unfinished series, no fold mismatches: the run is clean evidence.

## ledger: PASS

Ratings close at exactly 8000, the 8-seat start sum. Games reconcile at 140 wins, 140 losses, 1 drawn game, 141 games. Parse ledger: 0 unattributed m-lines, 0 unfinished series, 0 fold mismatches, every unparsable line confined to summary.txt which is not a series log.

## follow-ups

The three-control evidence set over the re-scaled sizes is complete and committed (2+1, 3+2, 1+0). The formal investigation over the increment axis begins on it: the hard-tier question (hard-1 collapse present only with increment, master-1's 0-4 versus hard at 3+2, mediums below easies at 1+0), then the ponder stop-join repro, then the tag decisions. All serialized engine batches per the machine law.
