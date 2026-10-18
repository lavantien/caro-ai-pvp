# fairy-stockfish variant.h

source: https://github.com/fairy-stockfish/Fairy-Stockfish/blob/master/src/variant.h
fetched: 2026-10-18 via zread read_file (full file, verbatim excerpts)
license: GPL-3.0, quoted for study

## facts

Variant struct is the complete ruleset as data: variantTemplate, pocketSize, maxRank/maxFile (board geometry), chess960, twoBoards, pieceValues per phase, customPiece Betza strings, pieceTypes set, pieceToChar/synonyms, startFen, per-piece mobility regions, promotion regions/types/limits/mandatory flags, blastOnCapture, mutuallyImmuneTypes, petrify rules, double/triple step regions, en passant regions/types, full castling geometry (kingside/queenside files, castling rank, king/rook files, rook piece types, oppositeCastling), kingType, checking/dropChecks, mustCapture/mustDrop, piece drops (dropLoop, capturesToHand, firstRankPawnDrops, drop regions, dropNoDoubled, sittuyin rules), gating, walling (Amazons-style), seirawanGating, cambodianMoves, diagonalLines (xiangqi elephants), pass flags, makpongRule, flyingGeneral, soldierPromotionRank, flipEnclosedPieces (Reversi), freeDrops.

Connect-family rules (verbatim fields):
```
int connectN = 0;
PieceSet connectPieceTypes = ~NO_PIECE_SET;
bool connectHorizontal = true;
bool connectVertical = true;
bool connectDiagonal = true;
Bitboard connectRegion1[COLOR_NB] = {};
Bitboard connectRegion2[COLOR_NB] = {};
int connectNxN = 0;
Value connectValue = VALUE_MATE;
```
Plus collinearN (int, default 0), MaterialCounting, adjudicateFullBoard, CountingRule, CastlingRights castlingWins.

Game-end parameters: nMoveRuleTypes/nMoveRule (50-move analog), nFoldRule (3) with nFoldValue and nFoldValueAbsolute, perpetualCheckIllegal, moveRepetitionIllegal, ChasingRule, stalemateValue (+ stalematePieceCount sign multiply), checkmateValue, shogiPawnDropMateIllegal, shatarMateRule, bikjangRule, extinctionValue/claim/pseudoRoyal/dupleCheck/pieceTypes/count, flagPiece/flagRegion/flagPieceCount/flagPieceBlockedWin/flagMove/flagPieceSafe (racing/flag games), checkCounting.

Derived properties computed by conclude(): fastAttacks/fastAttacks2, nnueAlias, nnueKing, nnueDimensions, nnueUsePockets, pieceSquareIndex/pieceHandIndex/kingSquareIndex, nnueMaxPieces, endgameEval, shogiStylePromotions, connectDirections, connectPieceTypesTrimmed.

VariantMap: std::map<string, const Variant*>, init(), parse<DoCheck>(path) for variants.ini, get_keys(). Custom variants loaded at runtime via the `load` UCI command.

## extraction notes

- connectN is the win condition (n in a row), connectNxN appears alongside collinearN for exactly-n semantics; connectRegion1/2 restrict where the connection counts; connectValue defaults to mate. This is the shipped generalization of our exact-five rule: an overline-nullifying variant would use the exactly-n form.
- The whole struct is the maximal expression of rules-as-data: every flag we hardcode in rules/ has a named field here. Our single-variant repo deliberately does not need this, but the naming taxonomy (what deserves to be a rule axis) is a useful audit list.
- nnueKing + kingSquareIndex exist because Stockfish NNUE indexes features by king square; kingless or multi-king variants need a synthetic anchor square. Note for any future net of ours: gomoku has no king, so a placement-pair anchor or a fixed anchor is required.
