# smoke21 first arm verdict (2+1)

Outcome of the run registered in smoke21-rescale-ponder-preregistration.md, scored against its four fixed rules. Run: 56 series, 146 games (131 decided, 15 drawn), easy-1/2, medium-1/2, hard-1/2, master-1/2 over the re-scaled sizes at 2+1 bo3, logs/tourny/20261008-223429-full, db/gates-v025.db. The run survived a memory-guard kill at series 28 (the GOGC heap goal doubled the live master TT slices past the machine cap), resumed behind the GOMEMLIMIT tournament bound (5d74d92) with series 28 replayed whole; every master-versus-master game in this evidence ran under that bound.

## rule 1, ladder: FAIL, rescale tag stays blocked

Final standings by rating: master-1 1381, master-2 1168, medium-2 1008, medium-1 997, hard-2 992, easy-1 963, hard-1 788, easy-2 703.

Standings inversions among easy, medium, hard: medium-2 and medium-1 both above hard-2, easy-1 above hard-1. logstats' series-wins-first ordering counts 6 (easy-1 ranks third overall). The clause "every hard above every medium, every medium above every easy" fails.

Aggregate records against the tier directly below: hard versus medium 10-8-2 (wins exceed losses, thin), medium versus easy 11-5-5 (holds). The cross-two-tier record hard versus easy is 9-10-3, hard losing the aggregate.

The anomaly shape: hard-1 collapsed to 788 at 10-21 while hard-2 held 992 at 16-15, the same tier shape 204 rating points apart, and easy-1 won its mutual meet with hard-1 4-1 in games. The v0.19 reference at the old sizes held zero inversions with hard-1 at 1169, so this is a change from that baseline, not a carried-over shape. Per the rule the re-scale tag blocks pending the 3+2 arm and the investigation.

## rule 2, master dataset: recorded, descriptive only

Aggregate game records, both seats pooled: versus easy 14-6-2, versus medium 14-7-0, versus hard 14-3-1, master-1 over master-2 3-2. Red/blue split across all master games: red 31-4-2, blue 16-17-1. Seat records: master-1 red 16-1-1, blue 10-6-0; master-2 red 15-3-1, blue 6-11-1.

Logstats telemetry per master seat set: depth mean 8.6 (median 9), mean n 25.63m, mean nps 7.79m, mean t 2.73 s against mean alloc 4.54 s, tt 16.1 percent. Inputs to the v0.29 re-derivation, no threshold invented here.

## rule 3, ponder health: FAIL on the t property, ponder tag stays blocked

341 [PONDER] m-lines across the run, adoption 155 of 1107 master-1 moves (14.0 percent) and 186 of 1217 master-2 moves (15.3 percent). 336 lines carry t at or near zero with the ponder stat law intact. 5 lines carry t of 17.97 to 29.86 seconds: all in the two master-versus-master series (s28 replay, s29), the only pairing whose ponder funding sits exactly at the machine core budget with both seats searching and pondering at full width, and both series ran under the post-breach GOMEMLIMIT bound.

No illegal moves, no engine panics, no conductor reconciliation failures, no unfinished series, no fold mismatches: the run itself is clean evidence. The t inflation routes to root-causing before any re-run: the candidate mechanisms are ponder stop-join stretch under full-budget thread contention, GC assist stealing CPU near the 6 GiB soft limit, or an adoption-path block. The confound is recorded: no master-versus-master game from before the memory breach survived (series 28's partial games were scrubbed by the resume), so this evidence set cannot separate the GOMEMLIMIT effect from the pairing's inherent contention.

## rule 4, ledger: PASS

Ratings close at exactly 8000, the 8-seat start sum. Games reconcile at 131 wins, 131 losses, 15 draws. Parse ledger: 0 unattributed m-lines, 0 unfinished series, 0 fold mismatches, every unparsable line confined to summary.txt which is not a series log.

## follow-ups

The 3+2 arm completes the re-gate before any rescale tag decision. The hard-tier investigation (hard-1 collapse, hard versus easy aggregate) and the ponder t-inflation root-cause precede or ride with it per the preregistration chain, all serialized engine batches.
