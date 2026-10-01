# Pattern taxonomy

Threat classes for one line window, derived mechanically from the rules win predicate. No hand-written patterns: every class below is a closed-form predicate over window states, and the generator evaluates it only through `rules.Board.Make` and `rules.Board.FastLastMoveWin`. Each definition names the machine check that verifies it.

## Window universe

A window is a sequence of `config.PatternWindowLen = 2*WinLength-1 = 9` states, one per cell, each from `{PatternStateEmpty, PatternStateOwn, PatternStateOpp, PatternStateOff}`. The packed index is base 4, position 0 in the two lowest bits, `4^9 = 262144` entries per direction.

A window is realizable iff its non-Off positions form one contiguous run, that is Off appears only as a prefix, only as a suffix, or not at all. Every window read off a real board line through an on-region center cell is realizable: a straight line intersects a rectangle (full 16x16 region or the 8x8 cross-check region) in one interval, and the center cell sits inside it. Realizable windows per direction: `1 (all Off) + sum over run length L = 1..9 of 3^L * (10-L) = 44272`. Machine check: `TestIndexMatchesManual` decodes every window extracted from seeded random boards in both regions and asserts the contiguity invariant plus a non-Off center, and `TestRealizableCount` pins the absolute count against an independent contiguity scan.

Non-realizable entries (interior Off) hold the zero entry and are unreachable at lookup.

## Window world semantics

An entry describes the 9 window cells as the whole world:

- Off is a wall: never playable, never holds a stone, never blocks, terminates runs.
- Empty is playable, Own and Opp hold the mover's and the opponent's stones.
- No cells exist outside the window. A run touching the window edge has an unblocked end, exactly like a run touching a board wall.

The generator emulates a window by mapping position i to board cell `(anchor + (i-4)*dir)` with the anchor at the board center, placing stones only at Own and Opp positions through `rules.Board.Make`, and probing each Empty position with a Make, `FastLastMoveWin(Red, cell)`, Unmake. Out-of-window cells stay empty, which reproduces wall semantics exactly: runs stop at the window edge (no stones beyond it) and blocking requires Opp stones (empty never blocks). `FastLastMoveWin` scans all 4 directions but the placed stones are collinear along the window direction only, so no cross-direction run of 5 can appear. Machine check: `TestGeneratorTruthExhaustive` compares the full win-in-1 mask of every realizable entry against an independent `rules.NaiveBoard` recomputation using `WinsThrough`.

Consumer note: on a longer real line the 2 cells immediately outside the window are invisible to a single entry, so an edge-touching exact 5 can classify as win-in-1 while the full board disagrees (an outside Own stone makes it an overline, or an outside Opp stone together with an inside Opp stone closes both ends). Win1-bit soundness is scoped to the CENTER position only: a real win through the window's center cell implies the center bit is set (sound over-approximation), while off-center bits are hints that can be false negatives when the deciding stones lie outside the window. Exact win decisions remain `rules.FastLastMoveWin` on the real board. Eval tolerates the window-local view by summing overlapping windows, VCF and VCT verify candidate wins at move time and must center their probe windows on each candidate square.

## Definitions

Let `place(W, i)` be W with position i set to Own, defined only for `W[i] = Empty`.

`win1(W, i)` holds when the mover placing Own at position i wins through that stone, evaluated by the rules win predicate: a maximal run of exactly `WinLength` Own stones through i whose two immediate in-window neighbors are not both Opp. Two consequences fall out of the predicate itself, they are not extra rules:

- Overline exclusion: a placement whose maximal run through i has length 6 or more never satisfies win1, by construction of the exact-5 rule.
- Off positions never satisfy win1: placement is undefined there.

`W1(W) = { i : W[i] = Empty and win1(W, i) }` and `n1(W) = |W1(W)|`. The entry stores W1 as a bitmask over the 9 positions.

Threat classes, first match wins:

- OpenFour: `n1(W) >= 2`. Two or more immediate wins, a double threat the opponent cannot answer.
- Four: `n1(W) = 1`. Exactly one immediate win square.
- Three: some i with `n1(place(W, i))` classified OpenFour. One move reaches a double threat.
- BrokenThree: some i with `n1(place(W, i)) = 1`. One move reaches a Four.
- OpenTwo: some i with `place(W, i)` classified Three. One move reaches a Three.
- None: otherwise.

Forcing depth orders precedence: OpenFour wins now unstoppably, Four wins on the next move at one square, Three wins in two own moves if unaddressed, BrokenThree in three, OpenTwo in four. A window with one child reaching OpenFour and another reaching Four classifies Three, the stronger.

The plan defines three as a state reaching a double-threat state with one own stone, the existential reading used here. The stricter exactly-one-placement reading is rejected: the canonical double-ended open three `..XXX..` has 2 placements reaching OpenFour, exactly-one would exclude it.

Two pinned consequences:

- A window already holding a live own exact-5 run classifies None: every extension through it overlines, so n1 = 0. Search terminates on wins before eval, no information is lost.
- A blocked two such as `OXX.....` classifies None: the Opp end removes every OpenFour-reaching descendant. Classes measure reachability of a double threat, the M3 weights decide what each class is worth.

Machine check: `TestTaxonomyWitnesses` constructs one explicit window per class and asserts both the table class byte and an independent naive recomputation of the whole classification chain, and `TestOverlineExclusion` pins the overline-exclusion cases (`.OOOOO.` extensions, `OO.OOO` gap fill, a live and a fully blocked existing five).

## Direction freeness

The entry depends only on the 9 states, not on the direction: the rules win predicate treats all 4 directions identically and the emulation is collinear, so all 4 per-direction tables coincide as functions of the packed index. Transposition of the board maps a horizontal window to the vertical window with identical position order (identity permutation), fixes the main diagonal, and maps the anti-diagonal to itself with position order reversed (permutation `i -> 8-i`). Reversal invariance holds for every direction because the win predicate is invariant under reversing a line, with the win-in-1 mask bits reversed alongside. Machine checks: `TestTranspositionHorizontalVertical`, `TestDiagonalsUnderTransposition`, `TestAllDirectionsEqual`, `TestReversalInvariance`.
