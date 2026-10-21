# arena

The strength-inversion experiment harness. Seat labels and flags live in main.go, the round robin runs through `make arena ARGS="-tc 0 -games 12 -seats hard,medium"`.

## floor pools

Same-tier pools for gate floor bands, per the v0.24 regrow preregistration. A pool drives one seat against itself, red alternating per game, writing one result dir per `-floorDirs` block of `-games` games under `-floorOut`, then aggregates every dir in the pool into a Wald 95 percent interval over the color-pair-pooled decisive fraction. The interval must cover 0.5; the artifact says whether to extend by another block, and `-floorMaxBlocks` caps extensions.

Run from the repo root:

```
make arena ARGS="-floor floor-master21 -tc 1 -games 20 -floorDirs 5"
make arena ARGS="-floor floor-master32 -tc 2 -games 20 -floorDirs 5"
```

Aggregate an existing pool without driving games:

```
make arena ARGS="-floor floor-master21 -tc 1 -games 20 -floorDirs 0"
```

Dirs append after existing ones, so an extend rerun reuses the same command with `-floorDirs 5` again. The artifact lands at `playground/arena/out/floor-master21.md` beside the pool dir. The arms read the frac and 95 percent ci row: a book seat whose color-pair-pooled decisive fraction falls below its floor band lower bound fails the gate.

Stdout carries the M-lines for the whole run, so redirect to `playground/arena/out/<name>.log` for the evidence bundle, for example through a detached launch per the standing gate law.
