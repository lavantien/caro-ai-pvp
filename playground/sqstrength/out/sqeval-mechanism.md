# Square midgame conversion mechanism

Evidence base: the 12-leg arena re-run logs under playground/arena/out, the discriminator positions under playground/sqstrength/out, and the branch, signal, and phase probes under playground/sqstrength/out built by playground/sqstrength/decomp.

## 1. The losing line, move by move

Every short square loss as master blue (hard-tc1 g24 21 moves, easy-tc1 g2 21 moves, easy-tc1 g4 21 moves, easy-tc2 g36 21 moves) follows one shape:

1. Blue builds a one-sided ladder on a single file. H6 I6 I7 or G7 G8 G9 or F7 F8 F9, always with one end pre-blocked by a red stone and the other end open.
2. Red blocks the open end with one stone. G10, I5 plus I10, F6 plus K6, E7 plus K7. The entire blue stack of 3 to 4 stones drops to class None, worth 0 in the class table forever after.
3. Red stacks an open parallel file while blue keeps feeding shapes next to its dead stack. Red E7 through E11 in g24, K5 through K9 in g2, G5 through K5 on row 5 in g4, H5 through H9 in g36. Blue never places one stone on the file red converts.
4. The open file becomes an open three, then an open four, then five. Blue sees mate only from about 6 plies out, when the blocking tempo no longer exists.

The long square losses (the slow-bleed class, 74 of 90 across the hard and medium legs) are the same shape with more moves between steps: blue's early scores sit at plus 200 to plus 4900 in positions it loses, the flip to minus 300 happens at blue's 3rd move (median move 6), and the game ends on an opponent VCF kill.

## 2. The blind decision is the post-block continuation

The discriminator positions at moves 6 and 8 cannot separate lost from won by any static feature: three board pairs are literally identical with opposite outcomes. medium-tc2 g44 (lost) and g10 (won) share H8 H6 H5 I6 K6 G6. easy-tc1 g2 (lost) and g8 (won) share H8 I9 K8 I8 I11 I7. easy-tc2 g36 (lost) and g6 (won) share H8 F8 E8 F7 F5 F9. The outcome is decided by the follow-up choices, and the engine makes those choices deterministically bad:

At the first post-block decision (move 10 of g2 and g24), a single-threaded depth-8 search picks the feed move next to the dead stack (J9, F9) at plus 4500 while every containment or file-switch alternative (L7, K10, E10, F8) rates negative. The in-game SMP search reached the same feed at depth 11 plus 6000 with a false VCT mate-in-7 claim (g2 M12). The won twins escaped through search nondeterminism, not through a better evaluation: the same deterministic probe picks J9 on the g8 board too.

At the race decision (g2 M16), blue holds a real four (one win square G9) while red holds an open three K7-K9 with the whole K file empty. The static table prices blue's threat ball at 22700 against red's 13300, so blue plays H9, red answers the forced block G9, and red's open file converts K6 K5 unopposed while blue's ball is spent.

## 3. The unpriced signal

Decomposing the lost-for-blue positions over the window table (playground/sqstrength/out/decomp-l.txt and decomp-w.txt): red scores literally zero class value in every one of them, blue holds 5800 from its ladder windows, and the only structural asymmetry is window cleanliness. A window is clean for a side when it holds at least one of that side's stones and none of the opponent's. Red's ring stones sit in 111 clean windows against blue's boxed cluster in 76 at the g2 M10 board, 70 against 57 at the move-6 boards, 82 against 51 at the move-8 boards. Red's stones are live development, blue's stones are boxed, and the class table prices only blue's local shapes.

The correction prices that asymmetry: eval gains a clean-window term, the count difference times a per-geometry weight, 50 on square and 0 on hex so the passing hex calibration is untouched. It is additive, symmetric between the colors, and incremental through the same per-window updates the evaluator already maintains.

The weight is capped by search stability: a move in open space adds about 36 clean windows, so the per-move-pair eval swing is 36 times the weight. At weight 200 the swing of 7200 destabilizes alpha-beta on quiet boards, hanging the unbounded-depth ponder test and costing about a ply of depth. At 50 the swing of 1800 stays near the existing open-two scale and the ponder test completes in 2 seconds. The move-10 static ordering (file switch above feed) needs a weight above 134 and was used only as calibration evidence, verified at weight 200 and recorded in out/branch-probe-fixed.txt and out/signal-probe.txt.

## 4. Observed effect of the correction

- At move 4 on the H8 H6 H5 board the corrected engine abandons the I6 G6 H7 I7 boxed-ladder family for J6 and F6, spread moves on blue's own open row, which is the strategy the won games used. This holds from weight 40 up.
- On the move-6 board H8 H6 H5 I6 K6, scored plus 2600 in game and plus 200 to plus 4500 at every depth 9 to 13 by the 60-second discriminator, the corrected engine scores minus 750 at depth 6 at weight 50, and at weight 200 it plays F6 at minus 6800 stable across depths 6 to 10. The other four move-6 families and the move-8 family read minus 6200 to minus 7000 at weight 200 (out/phase-probe-fixed.txt). The weight-50 hold on this board is partial: depth 8 returns to plus 1300.
- The move-10 feed choice drops from plus 4500 to plus 2850 at weight 50 and to 0 at weight 200: the boxed ball no longer reads as converting.
