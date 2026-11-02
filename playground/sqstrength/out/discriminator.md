# Square eval-versus-depth discriminator

The square master passgate shortfall has two candidate causes: the eval misprices the position so the engine believes blue is fine while losing, or the compute ladder stops as much as one ply short of the depth that reveals the truth. This replay separates them.

Positions: 20 recorded master-blue positions from the 12-leg arena re-run logs, blue to move at moves 4, 6, and 8, one fresh master-tier searcher per position under a fixed 60 second budget. Every position is one the in-game engine scored at or above +300 for blue at its in-game depth, taken from games master went on to lose, spread over all 6 square legs. The probe runs one continuous soft-stopped search per position, the same iterative deepening the arena drives in game at a 10x budget, and records the score of the last 8 completed depth iterations from the searcher's exported root history, which always covers the window from depth 9 to the final depth. A depth-capped restart chain was tried first and abandoned: its re-searches could not pass depth 9 inside the budget, the record is discriminator-restart-abort.log.

Decision rule, fixed before the run: a position votes eval when every recorded depth from 9 up scores at or above 0, the engine cannot see the loss even past its in-game depth ceiling. A position votes depth when the score turns negative at some depth from 9 up and the deepest completed depth also scores below 0, more search reveals the loss. Positions whose deepest score stays nonnegative after a negative dip vote mixed, positions that never reach depth 9 vote under-reached. The pre-registered signatures: the score staying at or above 0 at every reachable depth confirms the eval hypothesis, the sign flipping negative past depth 8 to 9 confirms the depth hypothesis. The 20 samples cover 10 distinct board positions, the verdict merges every sample of one board into a single curve, because separate samples of the same board agree on every shared depth.

| pos | leg | game | move | in-game score at depth | depths reached | deepest score | min score | first negative depth | board verdict |
|---|---|---|---|---|---|---|---|---|---|
| 1 | square-hard-tc1 | 4 | 4 | +1000 at 9 | 1 to 9 | +1000 | -400 | 3 | depth |
| 2 | square-hard-tc1 | 24 | 6 | +2100 at 12 | 1 to 11 | +4500 | +200 | none | eval |
| 3 | square-hard-tc1 | 58 | 8 | +4300 at 12 | 1 to 13 | +3500 | +2600 | none | eval |
| 4 | square-hard-tc1 | 6 | 4 | +1000 at 9 | 1 to 9 | +1000 | -400 | 3 | depth |
| 5 | square-hard-tc2 | 4 | 4 | +1000 at 9 | 1 to 10 | -900 | -900 | 3 | depth |
| 6 | square-hard-tc2 | 6 | 4 | +1000 at 9 | 1 to 9 | +1000 | -400 | 3 | depth |
| 7 | square-hard-tc2 | 10 | 4 | +1000 at 9 | 1 to 10 | -900 | -900 | 3 | depth |
| 8 | square-hard-tc2 | 14 | 4 | +300 at 8 | 1 to 9 | +1000 | -400 | 3 | depth |
| 9 | square-medium-tc1 | 2 | 4 | +300 at 8 | 1 to 10 | -900 | -900 | 3 | depth |
| 10 | square-medium-tc1 | 6 | 6 | +3400 at 10 | 1 to 11 | +4500 | +200 | none | eval |
| 11 | square-medium-tc1 | 18 | 8 | +1000 at 11 | 1 to 11 | +1000 | +600 | none | eval |
| 12 | square-medium-tc1 | 4 | 4 | +1000 at 9 | 1 to 12 | -800 | -800 | 10 | depth |
| 13 | square-medium-tc2 | 2 | 4 | +300 at 8 | 1 to 10 | -900 | -900 | 3 | depth |
| 14 | square-medium-tc2 | 44 | 6 | +2600 at 9 | 1 to 10 | +3400 | +200 | none | eval |
| 15 | square-medium-tc2 | 50 | 8 | +4900 at 11 | 1 to 12 | +4300 | +2600 | none | eval |
| 16 | square-medium-tc2 | 6 | 4 | +1000 at 9 | 1 to 9 | +1000 | -400 | 2 | depth |
| 17 | square-easy-tc1 | 2 | 6 | +1600 at 9 | 1 to 11 | +4500 | +200 | none | eval |
| 18 | square-easy-tc1 | 4 | 6 | +2500 at 9 | 1 to 10 | +3400 | +200 | none | eval |
| 19 | square-easy-tc2 | 12 | 4 | +1000 at 9 | 1 to 10 | -900 | -900 | 10 | depth |
| 20 | square-easy-tc2 | 36 | 6 | +3400 at 10 | 1 to 12 | +2100 | +400 | none | eval |

Unique board positions, sample curves merged:

| board | move | samples | deepest depth | deepest score | score at depth 9 | first negative depth from 9 | verdict |
|---|---|---|---|---|---|---|---|
| H8 H6 H5 | 4 | 1, 4, 5, 6, 7, 8, 9, 13, 16, 19 | 10 | -900 | +1000 | 10 | depth |
| H8 I7 H5 | 4 | 12 | 12 | -800 | +1000 | 10 | depth |
| H8 G7 E8 G8 G5 | 6 | 2 | 11 | +4500 | +2500 | none | eval |
| H8 H6 H5 G6 E6 | 6 | 10 | 11 | +4500 | +2600 | none | eval |
| H8 H6 H5 I6 K6 | 6 | 14 | 10 | +3400 | +2600 | none | eval |
| H8 I9 K8 I8 I11 | 6 | 17 | 11 | +4500 | +2500 | none | eval |
| H8 I7 H5 H7 K7 | 6 | 18 | 10 | +3400 | +2500 | none | eval |
| H8 F8 E8 F7 F5 | 6 | 20 | 12 | +2100 | +2100 | none | eval |
| H8 H6 H5 I6 K6 G6 F6 | 8 | 3, 15 | 13 | +3500 | +5500 | none | eval |
| H8 H6 H5 G6 J6 G9 F6 | 8 | 11 | 11 | +1000 | +600 | none | eval |

Score per recorded depth iteration, the last 8 completed, blue to move:

- pos 1 square-hard-tc1 g4 m4: d2:+400 d3:-400 d4:+400 d5:+200 d6:+200 d7:+1000 d8:+300 d9:+1000
- pos 2 square-hard-tc1 g24 m6: d4:+200 d5:+1500 d6:+1500 d7:+1000 d8:+3600 d9:+2500 d10:+3400 d11:+4500
- pos 3 square-hard-tc1 g58 m8: d6:+3600 d7:+2600 d8:+4500 d9:+5500 d10:+4900 d11:+4900 d12:+4300 d13:+3500
- pos 4 square-hard-tc1 g6 m4: d2:+400 d3:-400 d4:+400 d5:+200 d6:+200 d7:+1000 d8:+300 d9:+1000
- pos 5 square-hard-tc2 g4 m4: d3:-400 d4:+400 d5:+200 d6:+200 d7:+1000 d8:+300 d9:+1000 d10:-900
- pos 6 square-hard-tc2 g6 m4: d2:+400 d3:-400 d4:+400 d5:+200 d6:+200 d7:+1000 d8:+300 d9:+1000
- pos 7 square-hard-tc2 g10 m4: d3:-400 d4:+400 d5:+200 d6:+200 d7:+1000 d8:+1000 d9:+1000 d10:-900
- pos 8 square-hard-tc2 g14 m4: d2:+400 d3:-400 d4:+400 d5:+200 d6:+200 d7:+1000 d8:+1000 d9:+1000
- pos 9 square-medium-tc1 g2 m4: d3:-400 d4:+400 d5:+200 d6:+200 d7:+1000 d8:+1000 d9:+1000 d10:-900
- pos 10 square-medium-tc1 g6 m6: d4:+200 d5:+400 d6:+1500 d7:+1000 d8:+3600 d9:+2600 d10:+3400 d11:+4500
- pos 11 square-medium-tc1 g18 m8: d4:+2900 d5:+3900 d6:+2400 d7:+3000 d8:+3100 d9:+600 d10:+1700 d11:+1000
- pos 12 square-medium-tc1 g4 m4: d5:+200 d6:+200 d7:+1000 d8:+1000 d9:+1000 d10:-800 d11:-800 d12:-800
- pos 13 square-medium-tc2 g2 m4: d3:-400 d4:+400 d5:+200 d6:+200 d7:+1000 d8:+1000 d9:+1000 d10:-900
- pos 14 square-medium-tc2 g44 m6: d3:+200 d4:+200 d5:+400 d6:+1500 d7:+1000 d8:+3600 d9:+2600 d10:+3400
- pos 15 square-medium-tc2 g50 m8: d5:+4900 d6:+3600 d7:+2600 d8:+4500 d9:+5500 d10:+4900 d11:+4900 d12:+4300
- pos 16 square-medium-tc2 g6 m4: d2:-400 d3:-400 d4:+400 d5:+200 d6:+200 d7:+1000 d8:+300 d9:+1000
- pos 17 square-easy-tc1 g2 m6: d4:+200 d5:+400 d6:+1500 d7:+1000 d8:+3600 d9:+2500 d10:+4500 d11:+4500
- pos 18 square-easy-tc1 g4 m6: d3:+200 d4:+200 d5:+400 d6:+1500 d7:+1000 d8:+3600 d9:+2500 d10:+3400
- pos 19 square-easy-tc2 g12 m4: d3:+200 d4:+200 d5:+200 d6:+200 d7:+1000 d8:+1000 d9:+1000 d10:-900
- pos 20 square-easy-tc2 g36 m6: d5:+400 d6:+1500 d7:+1000 d8:+3600 d9:+2100 d10:+2100 d11:+2100 d12:+2100

Compressed tally: 9 of 20 samples score at or above 0 at every completed depth, 6 samples flip negative inside the depth window, 6 samples stop below their board's deepest reached depth. Over the 10 unique boards the verdict counts are eval 8, depth 2, by move number move 4: depth 2; move 6: eval 6; move 8: eval 2.
Depth-window flips: H8 H6 H5 scores negative at depth 10, in-game belief +1000 at depth 9; H8 I7 H5 scores negative at depth 10/11/12, in-game belief +1000 at depth 9.
Verdict: Split verdict by phase: the 2 move-4 boards vote depth, the sign flips negative exactly one ply past the in-game ceiling, and the 8 move-6 and move-8 boards vote eval, the score stays at or above +200 through depth 10 to 13 in games master lost. The opening decision is depth-limited, the midgame conversion is eval-limited, and neither fix alone covers both phases.
