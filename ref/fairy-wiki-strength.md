# fairy-stockfish wiki: playing strength

source: https://github.com/fairy-stockfish/Fairy-Stockfish/wiki/Playing-strength
fetched: 2026-10-18 via WebFetch

## content as fetched

Strength assessed by head-to-head relative Elo against reference engines (variantfishtest.org links) with statistical error margins where available; entries without margins come from fewer than 100 games. Starred entries mean a stronger NNUE exists but the listed result is classical eval.

Overall: outperforms most chess variant engines except where specialized engines dominate (shogi). Strongest available in janggi, sittuyin, capablanca, and others. Chess: functionally near-identical to official Stockfish but over 2x slowdown from variant overhead costs over 100 Elo. Lichess variants within roughly plus or minus 200 Elo of multi-variant Stockfish without NNUE; with NNUE surpasses all others; weakest atomic -268 and horde -133, strongest extinction +601, placement +749, 3check +278. Shogi: superhuman with NNUE (about 3800 qhapaq), classical eval over 1000 Elo weaker (about 2700). Xiangqi: superhuman with NNUE, a few hundred Elo below the top specialized engines; classical slightly above Elephant Eye and Cyclone 0.55. Janggi: strongest available with NNUE, stronger than pros; pro-level without. Largeboard build costs 50-200 Elo on small-board variants from lower speed and functional differences.

Connect-n coverage: only breakthrough (8x8 beats GameMaster 2.0 by over +400 Elo). Connect four and tic-tac-toe are not mentioned on this page. Endgame tablebases are not mentioned on this page.

NNUE is the gap-closer: nets exist for most popular variants, make play superhuman in xiangqi, shogi, janggi, makruk, and even outperform the classical eval in speed, so the engine uses pure NNUE when available.

## notes

- The measured cost of generality: over 2x slowdown on plain chess, 50-200 Elo for the largeboard build. Rules-as-data is not free; our single-variant Go engine pays nothing comparable.
- No shipped evidence of strong connect-n play: connect four and tic-tac-toe sit outside the strength table entirely. Our game family is outside this engine's demonstrated competence, which is why Pikafish (specialized single-game fork) is the better strength reference and why our handcrafted threat eval plus solvers is not competing with either project's demonstrated domains.
