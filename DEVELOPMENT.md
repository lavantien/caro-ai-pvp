# Development conventions

How values enter this codebase. Read this before introducing any number,
string, or threshold into behavior code. The what-and-where of the current
constants lives in README "Runtime configuration" and ENGINE_FEATURES 10.2;
this file states the rules that keep them true.

## The single-home rule

Every value has exactly one home, and behavior code references it; it never
re-states it.

| Runtime | Home |
|---------|------|
| Backend scalars | `backend/src/Caro.Domain/Constants.cs` (nested domain groups) |
| Backend tables | `backend/src/Caro.Domain/Constants.Tables.cs` (directions, difficulty ladder, time controls) |
| Backend startup overrides | `backend/src/Caro.Domain/CaroConfig.cs` (appsettings `Caro` section / `Caro__*` env) |
| Backend host knobs | `ServerConfig` in `Caro.Server/Program.cs` (port, DB path) |
| Frontend | `frontend/src/lib/config/index.ts` (value-domain unions live here) |
| Scripts | `scripts/lib.mjs` (env-overridable ports, URLs, timeouts, tables) |

When you introduce a value: name it in the right hub first, then wire every
consumer to that name. An inline literal in a component, handler, engine
file, or script is a bug even when its value is currently correct, because
the next change edits one copy and not the other.

## Derive, don't restate

If a value is a function of another value, write it as one.

- `Board.LineLength = 2 * Board.WinLength + 1`, `Board.MaxMoves = Board.Size * Board.Size`.
- The `7+5` table entry is seeded from `Constants.TimeControl.Default*`, so the default clock cannot drift from the table.
- `DefaultSearchOptions` reads the top `DifficultyProfiles` entry, so the no-difficulty fallback cannot diverge from L5.
- The frontend UCI default clock reads the `3+2` row of its own `TIME_CONTROLS`.
- Unit conversions go through `Constants.Time.MsPerSecond`, never a bare `1000`.

Two values that must agree are one value plus a reference. A comment saying
"keep in sync with X" is the fallback when a language boundary forces
duplication (see Mirrors below); it is not a substitute inside one runtime.

## String contracts get a home too

Strings that cross a system boundary (JSON API, SQLite, UCI wire, statline)
are contracts, not prose. They live in a constants class: `MoveTypes`,
`EndReasons`. Messages that carry a bound interpolate it
(`difficulty must be {Min}-{Max}`) instead of re-encoding the numbers in
text. Protocol words (`startpos`, `bestmove`) are the documented exception.

## Proportionality and machine independence

Logic must not depend on absolute values that assume a board, a clock, or a
host. Before introducing a value, ask what it scales with and express it as
that ratio:

- **Clock-relative, not absolute:** search budgets are fractions of the
  remaining clock (`TimeFraction`, `TimeManagement` divisors, buffer and
  reserve fractions), so behavior is identical on 1+0 and 15+10. Never a
  fixed "think for N ms".
- **Host-relative, not fixed:** parallelism adapts through the
  `ProfileThreads` modes (fixed one/two for weak levels, host-share with
  reserved cores for strong ones). Never a thread count that presumes a
  specific machine.
- **Board-relative, not magic:** geometry derives from `Board.Size`
  (`LineLength`, `MaxMoves`, `CenterDistScaleBase`, scan spans). Nothing
  outside the hub assumes 16, 256, or 5. Frontend cell sizing derives from
  `GameConfig.boardSize` and the min/max cell relations.
- **Fraction-of-budget, not magic count:** iteration growth bounds, history
  scales, and VCF block fractions are stated as relations to their parent
  budget so they retune when the parent does.

A new threshold that is "just a number" is usually a ratio with its
denominator left implicit. Name the denominator.

## Const versus config

Values that size `stackalloc` buffers or fixed arrays, or encode
cross-system contracts, stay `const` by necessity. Everything game-tunable
flows through `CaroConfig` so operators can move it without a rebuild. When
you add a value, decide which side of that line it is on and say why in the
PR; the boundary itself is documented in README "Runtime configuration".

## Mirrors

Three surfaces cannot share a source across language boundaries and are
mirrored by hand: the time-control and difficulty tables
(`Constants.Tables.cs`, `frontend/src/lib/config/index.ts`,
`scripts/lib.mjs`), the API port 5207 (`ServerConfig`, `ApiConfig.baseUrl`,
`lib.mjs DEFAULT_API_PORT`), and the GC heap limit
(`Constants.Limits.HeapHardLimitBytes` vs
`Caro.Server.runtimeconfig.template.json`). Rules for mirrors:

1. A mirror carries a cross-reference comment naming its siblings, in every
   copy. A mirror without a pointer is a bug.
2. All copies change in the same commit.
3. Mirrors are a last resort; if a shared source is technically possible,
   prefer it (the Playwright config imports `lib.mjs`; the e2e spec and the
   config index derive from their own homes).

## Tests pin, they don't define

Tests may deliberately re-state contract values as assertions; a pin that
fails when the contract changes is the point. Production code never reads
test values, and a pin lives in a test, never in `src`. A new test either
pins the contract on purpose or references the hub; it never creates a
third production copy of a value.

## Measurements live in artifacts, not docs

Performance, strength, and parity numbers (win-rate gates, score-match
rates, node-count bands, nps ratios, acceptance thresholds) never go into
prose docs. A doc may say how to run a measurement (`STATS.md` benchmark
commands); it never states what number counts as pass. Run outputs belong
in the generated artifacts (`docs/artifacts/`, `tournament-summary.json`),
which are regenerated per run. Written thresholds rot as the engine
evolves and turn the docs into false acceptance criteria.

## Review checklist

- New numeric or string literals in behavior code: each one names a hub
  constant or is justified inline.
- Exactly one home per value; derivations reference their source.
- New thresholds expressed as ratios (of clock, host, board, or budget),
  not absolutes.
- Mirrors updated together, cross-reference comments intact.
- Config-vs-const decision stated for anything new.
