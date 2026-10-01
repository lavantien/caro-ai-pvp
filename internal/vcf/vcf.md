# VCF and VCT solvers

## What

`internal/vcf` answers one question: does the side to move force a win by a
continuous threat sequence. VCF restricts the attacker to moves that create a
four or open four, or win on the spot. VCT widens the set to three and broken
three creations. Everything else, quiet moves, blocks that create no threat,
is out of model. A claimed win is a complete proof tree under the defender
model below, every leaf an attacker placement verified as a real win through
`rules.FastLastMoveWin` at move time.

## Why

The spec mandates dedicated VCF and VCT solvers whose threat sets come from
the exhaustive line pattern tables, never hand written patterns, with
soundness provable by testing against brute force. This ruleset is not
gomoku: overlines never win and a five close blocked at both ends is dead,
which moves threat boundaries. Two consequences drive the design:

1. A simple four with one end already defender blocked has a second full
   defense, the far end: closing it kills the completed five through both
   ends. Gomoku solvers do not have this reply, importing that assumption
   would claim false wins here.
2. A win cell whose completion would overline is not a win cell. Win cell
   sets are probed on the real board only, never read off window classes.

## How

Search: depth first threat space search with iterative deepening on the
forced line length and a refutation memo. Chosen over proof number search:

- The threat tree is narrow by construction, 1 to 4 defender replies per
  attacker move under the model, so the deep narrow trees where best first
  PNS pays off barely exist here.
- PNS holds the whole frontier in memory and needs node pools and child
  indexing; the DFS reuses the board's own Make and Unmake plus per ply
  stacks, which is what makes the zero allocation proof trivial.
- Three valued propagation maps directly onto budget and deadline
  semantics: fail dominates AND nodes, win dominates OR nodes, abort only
  when unresolved, so resource exhaustion can never surface as a win.
- The memo stores only refutations. A stored fail is depth independent by
  construction: a resFail subtree contains no abort anywhere, aborts never
  convert to fails, so the verdict holds at every deepening pass. Wins stay
  unrecorded so the PV is always re-derived and reconstructible.

Deepening runs the forced line length 1, 3, 5, ... so shallow wins surface
before deep refutation work burns the budget. Attacker moves are ordered by
window class rank, then by the number of directions reaching the class
floor, so double threats through one square go first.

Move generation reads the pattern tables with the probe window centered on
each candidate square and looks up the post placement entry, the child index
with the center set to Own, which per the taxonomy's center bit contract is
a sound over approximation of creating that line's threat. Candidates come
from the Chebyshev `SolverCandidateRadius` dilation of the mover's stones:
any completing or threat creating stone sits within line distance four of
existing own stones. Window classes can over call threats; the search
verifies reality at move time, so screen phantoms cost a subtree, never a
claim.

### Defender model

At a defender node with live attacker win cells, the four case:

- If the defender has any win in 1 cell, the line fails: the defender
  fives first.
- Otherwise the defender's replies are exactly the defusing set: each
  attacker win cell, plus the far end of every live winning frame whose
  other end already holds a defender stone, enumerated over all exact-five
  frames through each win cell. A win cell can complete fives in several
  directions at once; a frame with both ends already defender stones is
  dead, never the win witness, and contributes nothing. Every other
  defender move is dominated: it cannot create a five (W_D was empty) and
  cannot defuse (only the win cell and a live frame's far end change a
  completion's blockedness, and defender stones never extend attacker runs
  into overlines), so the attacker answers by completing at a live win
  cell. This part of the model is exact, and the enumeration of frames is
  exhaustive per direction.

At a defender node with no live attacker win cells, the three case, VCT
only:

- The screened three is gated first: some in window conversion must yield
  a win cell that verifies on the real board, else the threat is phantom
  and the line fails without forcing anyone.
- The defender's replies are every counter four (defender moves whose
  centered post placement window reaches Four, which force the attacker
  back), then every empty cell of the threat windows through the last
  move. Counter fours first because they are the tries most likely to
  refute and AND nodes exit on the first refutation.

The three case is the standard threat forcing defender and carries a
documented residual freedom: a defender stone far outside the threat window
that only matters later in the chain is not explored. The oracle fuzz below
exists to hunt exactly this class.

The attacker node in between: win in 1 wins on the spot, a full board is a
draw, two defender win cells fail the line (no block covers both within the
model), one defender win cell forces the attacker to occupy it or close its
far end, and that reply must itself be a threat move of the solver's kind,
which is what keeps the sequence continuous.

## Oracle cross check

`oracle_test.go` holds a test only AND OR minimax over `rules.NaiveBoard`
sharing no code with the solvers, terminals through `WinsThrough`, the array
path the solvers never touch. The criterion per spec: every claimed win of
P plies must be confirmed as a forced win within exactly P plies, since a
sound P ply claim is a P ply strategy and the evaluation is exact.
Refutations are not verified, completeness is not claimed.

Confirmation exactness survives the oracle's cost cuts: a winning placement
always sits at Chebyshev ring 1 of its own color because the other four
cells of the completing five are stones, so depth 1 nodes probe ring 1
only; defender stones at ring 5 of both colors cannot enter any frame that
completes within the horizon, so all such inert replies evaluate once
through a representative; the attacker visits near cells first and carries
the solver's PV as a first try hint, pure ordering that cannot change an
AND OR value. Full width confirmation cost still grows exponentially with
the claim depth, so the soundness harness clips the solver's line length to
`oracleClaimPlyMax` and keeps every claim decidable; production solvers
stay unbounded.

Runs permanently: the committed corpus under `testdata/fuzz` executes with
every `make test`, deterministic random suites cover both regions, and
`go test -fuzz=FuzzSoundness` mutates live. 16x16 claims are replay
verified (legality, no intermediate win, final placement a verified win),
8x8 claims additionally get the full oracle.
