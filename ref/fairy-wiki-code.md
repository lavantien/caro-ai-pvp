# fairy-stockfish wiki: understanding the code

source: https://github.com/fairy-stockfish/Fairy-Stockfish/wiki/Understanding-the-code
fetched: 2026-10-18 via WebFetch

## content as fetched

Prerequisites: chess engine programming, ideally Stockfish, since this is a fork. Points to Stockfish readme improving-the-code links and the chess programming wiki.

Design philosophy: merges earlier variant forks (makruk, shatranj, seirawan) into one engine. Rule/feature-centric rather than variant-centric (contrast: lichess Stockfish fork). Position representation and move generation handle individual rules independently; a variant is a set of rules (board size, piece types, etc). Costs some performance, enables fast new variants including runtime via config files, and lets search/eval improvements in one variant carry to similar ones; user-defined variants often reach master level.

Architecture map:
- variant.h variant object, enumerates rule options; variant.cpp built-in definitions; variants.ini example config that also documents every attribute; parser.h/cpp config parsing; piece.h/cpp piece movements.
- position.h position object wrapping variant attribute access; position.cpp FEN parsing/generation, legality, do/undo, variant-specific game end criteria; movegen.cpp pseudo-legal generation for all pieces.
- types.h Bitboard (64 or 128 bit), bitboard.h/cpp, magic.h precomputed magics for fast startup without pext.
- uci.h/cpp/ucioption.cpp UCI dialects (UCI, UCCI, USI); xboard.h/cpp CECP; partner.h/cpp partner communication for four-player variants.
- evaluate.cpp central evaluation; psqt.cpp per-variant piece values and PSQT init; pawns.cpp pawn structure and king safety; material.cpp/endgame.cpp imbalances and specific endgames.
- search.cpp tree search; movepick.cpp, tt.cpp, timeman.cpp mostly Stockfish-unchanged with minor variant tweaks. tests/perft.sh CI perft. pyffish.cpp Python binding plus SAN/notation.

Built-in variants concretely in variant.cpp; user variants via variants.ini parsed at runtime, no recompilation.

Search differences: mirrors official Stockfish, but some techniques are modified or skipped in variants where they do not work well as used for chess. Game-ending logic is variant-specific and lives in position.cpp.

## notes

- "Modified or skipped per variant" is the load-bearing sentence for us: null-move, check-based extensions, and mate-distance logic are chess-shaped; a placement game needs its own cuts. Our engine already made the equivalent call (no null move; solver-guided threat search instead).
- Evaluation is inherited Stockfish PSQT/material/pawns adapted per variant: no threat-shape eval exists here, confirming that a connect-family engine must bring its own evaluation ideas; neither project ships one.
