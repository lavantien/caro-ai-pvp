# Performance Statistics

This file contains baseline benchmark results for the Caro AI engine.

> **Note:** These statistics are snapshots in time and will change as the engine evolves. Run `dotnet run -- --baseline-benchmark` in `backend/src/Caro.TournamentRunner` to generate current metrics.

---

## Baseline Benchmark Results (2026-02-25)

Based on 32-game matchups per time control with alternating colors. Higher difficulty consistently beats lower difficulty:

| Matchup | Time | Higher Win | Draw | Lower Win |
|---------|------|------------|------|-----------|
| Easy vs Braindead | Bullet | 14 | 0 | 18 |
| Easy vs Braindead | Blitz | 15 | 0 | 17 |
| Medium vs Braindead | Bullet | 17 | 0 | 15 |
| Medium vs Braindead | Blitz | 20 | 0 | 12 |
| Grandmaster vs Braindead | Bullet | 25 | 0 | 7 |
| Grandmaster vs Braindead | Blitz | 26 | 0 | 6 |
| Hard vs Easy | Bullet | 18 | 0 | 14 |
| Hard vs Easy | Blitz | 17 | 0 | 15 |
| Grandmaster vs Medium | Bullet | 27 | 0 | 5 |
| Grandmaster vs Medium | Blitz | 22 | 0 | 10 |
| Grandmaster vs Hard | Bullet | 30 | 0 | 2 |
| Grandmaster vs Hard | Blitz | 26 | 0 | 6 |

**Critical Expected Behavior:**
- **Braindead should NEVER win against Medium+** - If Braindead wins consistently against Medium or higher, there is a major bug
- Braindead has 10% error rate and minimal search (1 thread, 5% time)
- Medium+ has full-strength search with 3+ threads and 50%+ time
- Any significant Braindead win rate against Medium+ indicates a regression that must be fixed

**Notes:**
- Win rates vary by time control; longer controls allow deeper search
- If lower difficulties are winning against higher ones, check: time allocation, thread assignment, search quality

---

## Performance Baseline

### Effective Branching Factor (EBF)

~2.5 across all matchups and time controls (excellent pruning efficiency)

### First Move Cutoff % (FMC%) Ranges

| Matchup Type | Bullet FMC% Range | Blitz FMC% Range |
|--------------|-------------------|------------------|
| vs Braindead | 38.6% - 58.4% | 58.3% - 67.3% |
| Hard vs Easy | 30.1% | 43.9% |
| Grandmaster vs Medium | 30.8% | 56.5% |
| Grandmaster vs Hard | 39.4% | 61.9% |

### Nodes Per Second (NPS) Ranges

| Difficulty | Bullet NPS | Blitz NPS |
|------------|------------|-----------|
| Easy | 86.8K - 287.1K | 81.0K - 223.9K |
| Medium | 93.6K - 218.0K | 100.7K - 274.2K |
| Hard | 86.8K - 91.7K | 93.9K - 96.3K |
| Grandmaster | 1.8K - 150.5K | 2.2K - 170.9K |

### Move Count Statistics (Mode/Median/Mean)

| Matchup | Time | Mode | Median | Mean |
|---------|------|------|--------|------|
| Easy vs Braindead | Bullet | 29 | 35.0 | 44.2 |
| Easy vs Braindead | Blitz | 21 | 28.5 | 34.5 |
| Grandmaster vs Braindead | Bullet | 13 | 25.0 | 35.5 |
| Grandmaster vs Braindead | Blitz | 23 | 23.5 | 33.0 |
| Grandmaster vs Medium | Bullet | 79 | 58.5 | 57.4 |
| Grandmaster vs Medium | Blitz | 36 | 39.5 | 59.3 |
| Grandmaster vs Hard | Bullet | 23 | 39.5 | 38.8 |
| Grandmaster vs Hard | Blitz | 19 | 35.0 | 46.3 |

### VCF Trigger Summary

| Matchup | Bullet Triggers | Blitz Triggers |
|---------|-----------------|----------------|
| Grandmaster vs Braindead | 4 | 3 |
| Hard vs Easy | 4 | 9 |
| Grandmaster vs Medium | 2 | 1 |
| Grandmaster vs Hard | 7 | 8 |

### Move Type Distribution

| Matchup | Time | Normal | ImmediateWin | ImmediateBlock | ErrorRate |
|---------|------|--------|--------------|----------------|-----------|
| Easy vs Braindead | Bullet | 89.5% | 2.2% | 3.4% | 4.9% |
| Grandmaster vs Braindead | Bullet | 62.7% | 2.6% | 16.7% | 5.7% |
| Hard vs Easy | Bullet | 93.6% | 3.0% | 3.4% | 0.0% |
| Grandmaster vs Hard | Bullet | 75.3% | 1.6% | 11.1% | 0.0% |

---

## Running Benchmarks

```bash
cd backend/src/Caro.TournamentRunner
dotnet run -- --baseline-benchmark
```

This runs 12 standardized matchups (32 games each) across Bullet (60+0) and Blitz (180+2) time controls:

| Matchup | Time Controls |
|---------|---------------|
| Braindead vs Easy | Bullet, Blitz |
| Braindead vs Medium | Bullet, Blitz |
| Braindead vs Grandmaster | Bullet, Blitz |
| Easy vs Hard | Bullet, Blitz |
| Medium vs Grandmaster | Bullet, Blitz |
| Hard vs Grandmaster | Bullet, Blitz |

Output files: `baseline_{bullet|blitz}_{diff1}_{diff2}.txt`

The benchmark generates:
- **Discrete metrics** (Mode/Median/Mean): Move count, Master depth, First Move Cutoff % (FMC%)
- **Continuous metrics** (Median/Mean): NPS, Helper depth, Time used/allocated, TT hit rate, Effective Branching Factor (EBF)
- **VCF trigger details** (game/move, depth, nodes)
- **Move type distribution** (Normal, Book, BookValidated, etc.)
- **Per-difficulty aggregates** for each time control

**Key Metrics:**
- **FMC%**: First Move Cutoff % - measures move ordering quality (>85% = excellent, <60% = needs work)
- **EBF**: Effective Branching Factor - measures pruning efficiency (lower = better, ~2-3 typical for good alpha-beta)
