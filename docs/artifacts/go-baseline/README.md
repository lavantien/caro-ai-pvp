# Go engine baselines (pre C#/.NET 10 port)

Historical record of the Go engine, captured 2026-09-01 on branch
`migration/csharp-port` before the Go backend was removed. Kept for
reference only; the numbers live in the artifacts below and are not
acceptance criteria for anything (see DEVELOPMENT.md, "Measurements live
in artifacts, not docs").

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

Probe positions live in `docs/artifacts/uci-probes/positions.json`; the
matching C# runs were probed through `scripts/uci-probe.mjs` with identical
input and are stored in `docs/artifacts/csharp-port/`.
