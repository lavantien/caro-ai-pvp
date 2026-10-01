# caro-ai-pvp

[![CI](https://placeholder.invalid/ci.svg)](https://placeholder.invalid/ci) [![coverage](https://placeholder.invalid/coverage.svg)](https://placeholder.invalid/coverage)

Badges are placeholders until CI lands at M8.

Mobile-first web arena for a custom 16x16 caro variant: exact continuous five wins, overlines never win, both-ends close-blocked lines are dead, and red's second move needs Chebyshev distance >= 3 from the first. Play from a phone browser against a person or an increment-safe bot on server-hosted rooms, watch live rooms as a guest, step through match history with a playback board, and run bot vs bot tournaments on the same room surface with per-move engine telemetry and a decay-scaled rating. The v0.20 behavior contract is scenario 1 in [first-cause.md](first-cause.md).

## contents

1. [grounding](#grounding)
2. [build and verify](#build-and-verify)
3. [repository layout](#repository-layout)
4. [diagrams](#diagrams)

## grounding

- [first-cause.md](first-cause.md): the founding spec, rules, hardware budget, and v0.20 scenarios.
- [docs/plans/first-cause-rebuild.md](docs/plans/first-cause-rebuild.md): milestone plan for the rebuild, updated as milestones land.
- ref/: offline chessprogramming.org and gomocup grounding, browser-sourced.

## build and verify

Requires go 1.27.1+, gcc, GNU make, with CGO enabled.

```
make doctor   # toolchain check: go >= 1.27.1, CGO, gcc
make ci       # fmt-check, vet, build, race tests, coverage gates
```

Coverage gates: 95% overall, 100% on internal/rules and internal/engine.

## repository layout

```
cmd/caro          entrypoint (ports, firewall)
cmd/covergate     coverage gate over go cover profiles
internal/config   single constants hub
internal/rules    board, legality, win detection (M1)
internal/pattern  exhaustive line-pattern tables (M2)
internal/engine   search core, SMP tiers (M3)
internal/vcf      VCF/VCT solvers (M4)
playground/       git-tracked R&D ground
```

## diagrams

Placeholder until M8: engine search pipeline, threat-solver cross-check harness, server room and series state machines, tournament conductor.
