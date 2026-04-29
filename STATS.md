# Performance Statistics

The engine supports 5 difficulty levels (L1 Novice through L5 Grandmaster). L5 = full strength.

To benchmark the engine at full strength (L5), use the tournament script:

```bash
node scripts/run-tournament.mjs --games 4 --red 5 --blue 5 --tc 3+2
```

To compare difficulty levels head-to-head:

```bash
node scripts/simulate-match.mjs --red 5 --blue 1 --tc 3+2 --json
```
