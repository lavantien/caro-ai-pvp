# Phantom four restriction lift

Evidence base: the passgate fill logs under playground/arena/out (square-passgates-hard-tc2.log), the turning point tables in gateforensic-report.md, the regate 2 verdicts under arena-regate2.md, and the replay probe under playground/sqstrength/replay. The two replay boards transcribe square-passgates-hard-tc2 games 27 and 36 at their recorded flip moves, captured in the 2026-10-31 fill batch.

## 1. The residual after the boxed ladder three

Square still fails 3 of 6 regate 2 legs. The forensic shape holds: master sits at a mate-for score, the opponent answers once, and the score collapses to mate-against. Classifying the 14 was-mating flips on square hard 3+2 by the tag on master's flip move: 6 carry a VCT solver claim, 6 are untagged search moves, 2 are ponder-adopted answers whose score came from the same search. The 8 search-class flips are one defect, reproduced below.

## 2. The mechanism

The class table reads a 9-cell window centered on each cell, so a window cannot see a stone at offset 5. A four with one end blocked inside the window and the far end blocked just outside classes Four while every completion is dead: game 27, master red holds M10-M13 with hard blue on M8 and M14. Completing at M9 makes a five closed at both ends, which is no win, and every other completion overlines. The windows centered at M9 and M13 each see only one blocker, so fours red counts 2, and both are the same dead four.

The forced-defense restriction and the extension both key on that counter. At every node after a red move the count is positive, so the search restricts blue to cells priced at the four weight: exactly M9. Blue's real resource, the open three K10-L9-M8 whose conversion N7 makes a double-ended four, prices at the three weight and is never generated. The search then believes blue is forced and mates red on paper, master plays the paper line, and hard's solver converts the real kill N7-J11-O6 in 3 plies. The same blindness prices master's own doom when the seats swap, which is why both engines scored the post-M14 board of game 27 as red-mating while hard was winning.

## 3. The correction

A node is forced only when the previous mover holds a verified win-in-1. The evaluator gains liveWin1: it scans the ring for cells whose window value for the questioned side reaches the four weight, then verifies each candidate by placing the stone and calling the real win predicate. The restriction and the extension both require its verdict, so nothing changes at any node holding a live four, where the win cell always passes both the screen and the verification. Nodes holding only phantom fours regain the full candidate set and stop extending. The correction removes a wrongly applied restriction; no tier, geometry, or constant gets weaker, the solvers and the eval weights are untouched, and no new state is maintained: the probe reads the existing per-cell window values and the board.

## 4. Measured effect on the recorded boards

Game 27, board after M34, master red to move. Before: depth 8 picks the recorded I11 at mate-for 1073741616 over 944,687 nodes, and the child search after I11 answers M9 for blue while scoring blue as mated. After: every depth from 4 to 8 scores the I11 child at exactly the mate-against 1073741776 that the game produced, reply N7, and the root picks the J11 block at minus 3550 to minus 5850. The solver confirms the kill in 5 nodes.

Game 36, board after M65, master blue to move. Before: depth 6 and 8 pick the recorded H11 at mate-for 1073741680 over 92 million nodes. After: depth 4 to 8 pick O7, O12, O11, the blocks of the O file kill zone, the H11 child scores mate-against 1073741776 at every depth, and the whole depth 8 search costs 59,930 nodes.

Quiet midgame board, fixed depth. Depth 6 is unchanged in pick and score, E3 at plus 1750, nodes 11,305 to 11,367. Depth 8 moves from E3 plus 2150 over 8,162,102 nodes to J9 plus 2300 over 137,868 nodes: the removed extension storm, not a new cost. The funded ponder depth pin and the rest of the engine suite hold.

## 5. What the lift cannot fix

The 6 VCT claim flips are false solver proofs under the documented three-case defender freedom and stay open until the defender model widens or the oracle fuzz finds a counterexample. The blue-seat opening deficit at 2+1, the depth-limited move-4 boards of the discriminator, and the open-two attraction on the m10 feed board are untouched. The static sum still prices a phantom four window at the full four weight; repricing it is a static lever and the cutsweep census closed that lane.
