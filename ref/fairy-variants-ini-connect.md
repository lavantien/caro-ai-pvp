# fairy-stockfish variants.ini connect-family definitions

source: https://github.com/fairy-stockfish/Fairy-Stockfish/blob/master/src/variants.ini
fetched: 2026-10-18 via zread read_file (full file persisted in session tool results, excerpts below verbatim)
license: GPL-3.0, quoted for study

## verbatim excerpts

Tic-tac-toe:
```
[tictactoe]
maxRank = 3
maxFile = 3
immobile = p
startFen = 3/3/3[PPPPPpppp] w - - 0 1
pieceDrops = true
doubleStep = false
castling = false
stalemateValue = draw
immobilityIllegal = false
connectN = 3
```

Connect four (section start, gravity via enclosingDrop = top):
```
[cfour]
maxRank = 6
maxFile = 7
immobile = p
startFen = 7/7/7/7/7/7[PPPPPPPPPPPPPPPPPPPPPppppppppppppppppppppp] w - - 0 1
pieceDrops = true
enclosingDrop = top
doubleStep = false
```

Teeko/picaria (tight square win alongside line win):
```
[teeko:picaria]
maxRank = 5
maxFile = 5
connectN = 4
connectNxN = 2
customPiece1 = p:mK
startFen = 5/5/5/5/5[PPPPpppp] w - - 0 1
```

Documented parameter semantics:
```
# connectNxN: connect a tight NxN square for win [int] (default: 0)
# collinearN: arrange N pieces collinearly (other squares can be between pieces) [int] (default: 0)
# connectValue: result in case of connect [Value] (default: win)
```

## notes

- Placement games are modeled as drop games: immobile pawns in a pocket, board-only FEN, connectN as the win condition, enclosingDrop = top for gravity. Win detection is generic (position.cpp is_game_end), not a line scanner special case beyond the connect rules.
- connectNxN and collinearN show the win-condition axes they found worth parameterizing: tight square, line with gaps allowed. Our rule set needs none of these knobs, but the exact-five plus overline-nullified plus blocked-both-ends-dead triad is stricter than anything expressible here, which is the structural reason this engine family cannot host our game even at 16x16 over their 12x10 cap.
- Their connectN accepts an overline (n or more in a row semantics by default). The exactly-n semantics our rules require has no shipped variant example, reinforcing that our rules package owns this logic correctly.
