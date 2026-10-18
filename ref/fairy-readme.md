# fairy-stockfish readme

source: https://github.com/fairy-stockfish/Fairy-Stockfish/blob/master/README.md
fetched: 2026-10-18 via zread read_file (full file)
license: GPL-3.0, quoted for study

## facts

- Chess variant engine derived from Stockfish, designed for fairy chess variant support and easy extensibility with user-defined rules (variants.ini + wiki Variant-configuration page).
- Protocols: UCI, UCCI, USI, UCI-cyclone, CECP/XBoard, for GUI compatibility.
- Goal: large variety of chess-like games with the full Stockfish search; playing strength very high in almost all supported variants (wiki Playing-strength page).
- Largeboard build supports boards up to 12x16 (zread PSQT page: LARGEBOARDS -> SQUARE_NB 120, SQUARE_BITS 7).
- Optional NNUE parameter files improve strength for many variants (fairy-stockfish.github.io/nnue list); dedicated NNUE releases for Xiangqi, Janggi, Makruk.
- Bindings: pyffish (Python, pychess-variants backend), ffish.js (JavaScript via emscripten); WASM port for in-browser analysis.
- Related games supported (connect family relevant): Cfour (Connect Four), Tic-tac-toe, Breakthrough, Ataxx, Clobber, Amazons, Flipersi/Flipello (Reversi/Othello), Fox and Hounds, Isolation, Joust, Snailtrail. No gomoku in the shipped list; connect-family rules exist via connectN parameters (see variant.h notes).
- Tests: tests/ has perft.sh, instrumented.sh, protocol.sh, regression.sh, reprosearch.sh, signature.sh, plus js tests and committed PGN fixtures.

## extraction notes

- The engine is the library: pyffish/ffish.js expose movegen, SAN, FEN, and game-end checks for every variant. Rules-as-data plus bindings is the extensibility story.
- No opening-book facility appears anywhere in the README; books are GUI-side in this family (matches Stockfish lineage). Verify on FAQ page before stating in second-cause.md.
