# Go engine baselines (pre C#/.NET 10 port)

Recorded 2026-09-01 on branch `migration/csharp-port` before the Go backend was
removed, so the C# port can be checked for strength and speed parity against the
engine it replaces.

- Machine: 12th Gen Intel Core i7-12700F, 20 logical processors
- Go: go1.27.0 windows/amd64 (module pinned go 1.26)
- Engine: HEAD of the port start (`ac34bb1` difficulty ladder recalibration)

## Contents

| File | What it is |
|------|------------|
| `go-test-full.txt` | `CGO_ENABLED=1 go test -race -count=1 ./...`, all packages green |
| `tournament-L1L5.txt` / `-summary.json` | L1 vs L5, 3+2, 20 games, seed 20260821 (copied from the repo-root artifacts of the 2026-08-21 run) |
| `tournament-L3L4.txt` / `-summary.json` | L3 vs L4, 3+2, 10 games, seed 20260821, fresh run on 2026-09-01 |
| `uci-parity.txt` | Fixed-position UCI probes: 1 thread, hash 256, skill 5, `go depth 4` and `go depth 8` |
| `uci-speed.txt` | Depth-9 full searches on `midgame-quiet`, 3 runs, single thread |

Probe positions live in `docs/artifacts/uci-probes/positions.json`; both engines
must be probed through `scripts/uci-probe.mjs` with identical input.

## Headline numbers

- L1 vs L5 (20 games): L1 5, L5 16, draws 1, errored 0, avg 50.45 moves, 135.6 s/game
- L3 vs L4 (10 games): L3 3, L4 7, draws 0, errored 0, avg 43 moves, 61.8 s/game
- UCI speed: depth 9 on `midgame-quiet` = 4,119,518 nodes, ~73K nps single
  thread; node count identical across 3 runs (deterministic)

## Parity acceptance for the C# port

- `uci-probe.mjs --mode parity`: bestmove and score cp must match exactly for
  every position and depth (single thread, fixed depth, fresh hash); nodes
  within roughly 15% (sort tie-order may differ).
- `uci-probe.mjs --mode speed`: nps at least 0.5x the Go single-thread figure
  (~37K), stretch 0.8x. Below 0.4x triggers the optimization backlog before
  merge.
- Seeded tournament at the same configs: 0 errored games, and the stronger
  level must win the matchup clearly (L5 >= 14/20 vs L1, L4 >= 6/10 vs L3)
  with `[VCF]` statlines still appearing in L4/L5 endgames.
