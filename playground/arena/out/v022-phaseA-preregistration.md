# v0.22 phase A preregistration: the TT round

Question: does transposition table size separate engine strength at the 10+5 time control, and which re-scale ships? Registered before any phase A game runs, per the inversion research law. Decision rules are fixed in advance and reference measured bands, never thresholds guessed after the fact.

## seats

hard (4 cores, 128 MiB, shipped), hard-tt256m, hard-tt512m, hard-tt1g (same shape at 256 MiB, 512 MiB, 1 GiB), medium (2 cores, 32 MiB, shipped), medium-tt256m (256 MiB), easy (1 core, no table, shipped), easy-tt32m (32 MiB). All seats wire through server.NewBotSearcher, so the round measures the code the rooms drive.

## schedule

8 new pairs at tc 3 (10+5), 12 games each, red alternating per game so each seat holds red 6 times:

1. easy vs easy (noise floor, the first easy 10+5 floor)
2. easy vs easy-tt32m
3. medium vs medium-tt256m
4. hard vs hard-tt256m
5. hard vs hard-tt512m
6. hard vs hard-tt1g
7. hard-tt256m vs hard-tt512m (adjacent step)
8. hard-tt512m vs hard-tt1g (adjacent step)

96 games total, one detached chain, serialized as the machine's only engine-timing batch. The wave plan enumerated these pairs plus a hard,hard floor at 12 games (108) while stating 72 total. Resolution recorded here before launch: the hard,hard 10+5 floor is already banked at 20 games from v0.21 (18 red-first wins, 2 draws, 0 blue-first wins, logs/archive/arena-v021-tc3-floor.log) and that recorded band is the reference, so it is not rerun and the new-game count is 96.

## reference bands

- hard,hard at 10+5: 18 red wins, 2 draws, 0 blue wins over 20 games. Between identical engines at this control the first mover decides, so a paired comparison shows a real strength difference only as blue-side wins by the stronger seat or a red/blue split moving outside this band in both directions.
- easy,easy at 10+5: measured as pair 1 of this round before any comparison pair reports.

## decision rules, fixed in advance

1. If the size differences sit inside the noise band, table size does not separate at 10+5 (consistent with the measured flat 13 to 16 percent table hit rate across oversubscription ratios) and the re-scale ships as footprint and wrap hygiene with the phase C re-gate as the safety net.
2. If a larger table loses outside the noise band, cap at the winning size and record why.
3. easy-tt32m ships only if it beats easy outside the easy,easy noise band AND phase C still shows easy below medium with zero inversions at 2+1 and 3+2. Otherwise easy keeps 0 bytes and the finding is recorded (easy's no-table identity is part of the ladder gap).

Adjacency pairs (256 v 512, 512 v 1g) refine rule 2's cap placement if a larger size wins. Phase B (the config literal change) ships only after this round's verdict. Phase C re-gates the ladder at 2+1 and 3+2 with zero inversions required.

## outputs

Raw pair logs under logs/archive/arena-v022-A-*.log (local per the logs law), the merged table and verdict under playground/arena/out/v022-phaseA-table.md (committed) once the chain completes.
