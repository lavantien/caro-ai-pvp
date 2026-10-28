# Square passgate collapse investigation pre-registration

Evidence: the 12 master passgate legs under playground/arena/out, hex and square, hard, medium, and easy at tc 1 (2+1) and tc 2 (3+2), plus the archived hex easy tc 1 first read of 20 games. Thresholds 0.90 versus easy, 0.75 versus medium, 0.60 versus hard, master W+D pooled over both seats with block extension to 3 blocks of 20. The square easy tc 2 leg is still running and enters the dataset as provisional rows from its streaming log, never blocking on the live arena. All numbers come from the tidy dataset and query set under playground/gateforensic, whose ledger accounts for every line of every log.

Questions, fixed before reading any fold beyond the gate verdicts:

1. Is the square collapse red-first driven or seat-symmetric, per tier and control?
2. Does master out-search hard and medium on square, or is compute equalized, by depth, nodes, nps, and TT use on untagged search moves?
3. At which ply and score margin do square losses turn, and what did master's best evaluation before the turn reach?
4. Which game phase dissolves the master edge on square versus hex, measured by ahead share, mating share, and clamped score mean per phase?
5. Does the hex easy 2+1 miss share the square mechanism, by the same turning point and phase folds?
6. What is the length and draw profile per geometry and control, including the draw caps?

Analysis plan, all folds over the tidy dataset with duckdb: per-arm and per-seat W+D with Wald 95 percent intervals at z 1.96; per-seat-pair outcome counts; parity folds of depth, nodes, nps, TT, HF, and FH1 per side on untagged moves; game length distributions split by draw and decided; draw counts and caps; turning point classification per master loss, where the turning move is the last move with a non-negative master-perspective score, the pro margin is the maximum master-perspective score at or before that move, and the after state is the first negative move that never recovered, with phases early at or under 20 moves, mid at or under 40, late beyond; ponder adopt, reject, and miss counts per side; loss mechanism classification by who owns the flip move and whether the reply mates immediately. Mate scores decode as base 2^30 minus 16 per ply of distance, mirroring the engine constants, and master perspective negates the mover score on opponent moves.

Honesty rules: report every marginal interval exactly as computed, no post-hoc threshold invention, the provisional leg is labeled in every table it touches and no conclusion rests on it, findings feed investigation follow-ups only and never tier changes directly, and any integrity check failure stops the analysis rather than getting a footnote.
