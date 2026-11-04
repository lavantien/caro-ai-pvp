# Boxed ladder three re-pricing

Evidence base: the 6-leg regate verdicts under playground/arena/out/arena-regate.md, the ladder decomposition under playground/sqstrength/out/decomp-l.txt, and the probes under out/phase-probe-boxed.txt and out/branch-probe-boxed.txt.

## 1. The residual after the clean window term

The regate improved every square leg but hard and medium still sit under their thresholds. At every recorded 3-stone ladder board (decomp-l.txt, all six m6 positions) blue holds 5800 from its ladder windows: 4 windows class Three at 1200 and 2 windows class BrokenThree at 500. Three of the four Three windows see a red stone at distance 2 from the stack end, yet price at the full open-three 1200, the value of a double-ended three that no single block answers. The boxed ladder three converts only through the far side and one red block defuses the whole file, which is exactly how every recorded loss proceeded.

## 2. The correction

A Three-class window whose own run holds an opponent stone within 2 cells of any own stone is boxed: on square its price drops by EvalBoxedCutThree 850 to 350, below the clean BrokenThree 500, because a one-side-engaged three converts worse than a clean broken three. Hex keeps cut 0 so its calibration is untouched by construction. The cut rides the same incremental window pipeline as the class sums: boxedPairs flags both sides at table build, the evaluator maintains a boxed-three count per color through reset and applyWindow, and eval subtracts cut times the mover-minus-opponent count difference beside the clean window term. The clean window term stays exactly as shipped at 50.

The boxed BrokenThree is deliberately not cut. Measured on the quiet midgame board at fixed depth 8: the uncut engine needs 4.03M nodes, cutThree-only at 850 needs 3.96M, while cutting the boxed broken at only 120 explodes the same search to 23.1M nodes and fails the funded ponder depth pin 5 of 5 runs. Adjacent-blocked threes are ordinary forcing shapes that appear in every quiet position, so cutting them thrashes alpha-beta across the whole tree. The Three-class cut at 850 holds the depth-8 count at 3.96M nodes and passes the ponder pin.

## 3. Effect on the recorded boards

At the m6 ladder board H8 I9 K8 I8 I11 I7 the correction re-prices blue's 5800 to 3250: 3 boxed threes drop 850 each, the far-sided I6 window and the two broken windows keep their value. Through the corrected engine the ladder-completion boards now read negative at depth 6 where the in-game engine scored plus 1600 to plus 3400 and entered: H8 I9 K8 I8 I11 completes with I7 at minus 750 and H8 F8 E8 F7 F5 completes with F9 at minus 750 (out/phase-probe-boxed.txt). The hold is depth 6 only, depth 8 and 10 return to plus 1200 to plus 1300 on the same boards, and the m8 continuation board climbs plus 1300 to plus 3200, so the correction moves the entry price without closing the depth horizon on the ladder family. The m2 through m4 boards are unchanged in their picks because they hold no three yet and the boxed cut is inert there, the clean window term carries the early avoidance, and the shipped fixtures TestSearchAvoidsBoxedLadderOpening and TestSearchScoresBoxedLadderBelowEven still pass at the new placement.

## 4. The residual the ladder classes cannot price

The post-block feed decision is not driven by the ladder classes at all. On the m10 g2 board the feed J9 child holds 12 OpenTwo windows worth 1200 against the file switch L7's 4 windows worth 400, no window classed Three or above, no window engaged within distance 2. The attraction is the open-two flood radiating through the dead stack's two lines, which the ladder-class placement cannot touch and which stays as the structural residual for a later rung, together with the m16 race where the static table still prices a real four above an open three.
