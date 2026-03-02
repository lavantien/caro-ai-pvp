# Caro Opening Book Builder

Tool for generating, verifying, and optimizing Caro opening books through self-play and deep search.

## Overview

The BookBuilder implements a three-phase separated pipeline (Actor-Critic pattern):

| Phase | Component | Purpose | Output |
|-------|-----------|---------|--------|
| 1 | Self-Play (Actor) | Generate diverse raw games | `staging.db` |
| 2 | Verification (Critic) | Deep search + VCF verification | `verified.db` |
| 3 | Integration | Merge verified moves to main book | `opening_book.db` |

**All database files are stored in the repository root by default.**

**Quick Start:**
```bash
cd backend/src/Caro.BookBuilder
dotnet run -- --full-pipeline --games 8192 --threads 8
```

---

## CLI Reference

### Separated Pipeline (Recommended)

#### Phase 1: Self-Play Generation
```bash
dotnet run -- --staging <path> [options]
```

| Option | Default | Description |
|--------|---------|-------------|
| `--staging <path>` | `staging.db` | Output staging database path |
| `--games <n>` | 8192 | Number of games to play |
| `--base-time <ms>` | 60000 | Base time per player (1 min) |
| `--increment <ms>` | 0 | Time increment per move |
| `--threads <n>` | CPU cores | Parallel game threads |
| `--buffer <n>` | 4096 | Games before database commit |
| `--max-ply <n>` | 16 | Maximum ply to record |
| `--resume` | - | Continue from existing games in staging DB |

#### Phase 2: Verification
```bash
dotnet run -- --verify-staging <path> [options]
```

| Option | Default | Description |
|--------|---------|-------------|
| `--verify-staging <path>` | (required) | Staging database to verify |
| `--time <ms>` | 4096 | Time per position for deep search (quality-optimized) |
| `--output <path>` | `verified.db` | Output verified database |
| `--threads <n>` | cores/2 | Parallel verification threads |

**Note:** Survival zone positions (ply 8-16) automatically get 8192ms (2x time).

#### Phase 3: Integration
```bash
dotnet run -- --integrate <path> [options]
```

| Option | Default | Description |
|--------|---------|-------------|
| `--integrate <path>` | (required) | Verified database to integrate |
| `--book <path>` | `opening_book.db` | Main book to update |
| `--batch <n>` | 65536 | Batch size for commits |

#### Full Pipeline (Convenience)
```bash
dotnet run -- --full-pipeline [options]
```

Runs all three phases in sequence. Options from all phases apply.

| Option | Default | Description |
|--------|---------|-------------|
| `--games <n>` | 8192 | Self-play games |
| `--base-time <ms>` | 60000 | Base time per player |
| `--verify-time <ms>` | 4096 | Time per position for verification (quality-optimized) |
| `--threads <n>` | CPU cores | Parallel threads |
| `--book <path>` | `opening_book.db` | Final output book |
| `--resume` | - | Continue Phase 1 from existing staging games |

---

### Resume Support

The `--resume` flag allows interrupted self-play generation to continue from where it left off:

```bash
# Start generation
dotnet run -- --staging staging.db --games 10000

# If interrupted, resume (only generates remaining games)
dotnet run -- --staging staging.db --games 10000 --resume

# Also works with full pipeline
dotnet run -- --full-pipeline --games 8192 --resume
```

**Behavior:**
- Checks existing games in staging database
- Calculates remaining games needed to reach target
- Skips generation if target already reached
- Without `--resume`: warns that new games will be added to existing

---

### File Locations

All database files are stored in the **repository root** by default:

| File | Purpose | Created By |
|------|---------|------------|
| `staging.db` | Raw self-play games | Phase 1 (Self-Play) |
| `verified.db` | Verified moves | Phase 2 (Verification) |
| `opening_book.db` | Final opening book | Phase 3 (Integration) |

**Custom paths** can be specified with flags:
```bash
dotnet run -- --staging /custom/path/staging.db --games 8192
dotnet run -- --verify-staging /custom/path/staging.db --output /custom/path/verified.db
dotnet run -- --integrate /custom/path/verified.db --book /custom/path/opening_book.db
```

**Temporary files** (`staging.db`, `verified.db`) are automatically cleaned up after `--full-pipeline` completes.

---

### Binary Format

Compact binary format for faster loading at startup.

```bash
# Export to binary
dotnet run -- --export-binary book.cobook --book opening_book.db

# Import from binary
dotnet run -- --import-binary book.cobook --output imported_book.db

# Validate only (no import)
dotnet run -- --import-binary book.cobook --verify-only
```

**Benefits:**
- ~4x smaller than SQLite
- ~10x faster load time
- Varint encoding for compactness
- xxHash checksum for integrity

---

### SPSA Parameter Tuning

Optimize AI evaluation parameters through self-play.

```bash
dotnet run -- --tune [options]
```

| Option | Default | Description |
|--------|---------|-------------|
| `--iterations <n>` | 50 | SPSA iterations |
| `--games-per-eval <n>` | 256 | Games per evaluation |
| `--preset <name>` | Default | Preset: Default, Aggressive, Conservative |
| `--base-time <ms>` | 10000 | Base time per player |
| `--output <path>` | - | Output JSON file for parameters |

**Examples:**
```bash
# Quick test
dotnet run -- --tune --iterations 3 --games-per-eval 32 --debug

# Standard tuning
dotnet run -- --tune --iterations 50 --games-per-eval 256

# With output file
dotnet run -- --tune --preset Aggressive --output tuned.json
```

**Parameter Bounds:**

| Parameter | Min | Max | Default |
|-----------|-----|-----|---------|
| FiveInRowScore | 50,000 | 200,000 | 100,000 |
| OpenFourScore | 5,000 | 20,000 | 10,000 |
| ClosedFourScore | 500 | 2,000 | 1,000 |
| OpenThreeScore | 500 | 2,000 | 1,000 |
| ClosedThreeScore | 50 | 200 | 100 |
| OpenTwoScore | 50 | 200 | 100 |
| CenterBonus | 25 | 100 | 50 |
| DefenseMultiplier | 1.0 | 3.0 | 1.5 |

---

### Legacy Mode

Traditional book generation (deprecated, use separated pipeline).

```bash
# Traditional generation
dotnet run -- --output book.db --depth 16 --moves 2 [--resume]

# Legacy self-play
dotnet run -- --self-play 100 --time-control 1000 --max-moves 100

# Verify existing book
dotnet run -- --verify-only
```

---

## Thresholds

All thresholds are powers of 2 for statistical significance:

| Threshold | Value | Purpose |
|-----------|-------|---------|
| MinPlayCount | 512 (2^9) | Filters fluke wins |
| MinWinRate | 62.5% (5/8) | Winning line indicator |
| MaxWinRateForLoss | 37.5% | Losing line indicator |
| MinConsensusRate | 81.25% | Self-play vs deep search consensus |
| MaxScoreDelta | 512 (2^9) | Pruning threshold |
| InclusionScoreDelta | 256 | Inclusion range |
| MaxMovesPerPosition | 4 | Variety without bloat |

### Threshold Rationale: Expert Report Compliance

The implementation follows expert recommendations for chess opening books with Caro-specific adaptations:

| Component | Expert Recommendation | Implementation | Status |
|-----------|----------------------|----------------|--------|
| Book Width | ~30cp margin | 256cp margin | ⚠️ Wider (acceptable) |
| Max Moves | 5 per position | 4 per position | ✅ More conservative |
| Time Control | 1+0 or Fixed Nodes | 1+0 default | ✅ Compliant |
| Threading | Parallel single-threaded | Parallel workers | ✅ Compliant |
| Sampling | Softmax + Temperature decay | Softmax + Decay to 0 by ply 24 | ✅ Compliant |

**Why 256cp vs 30cp?**

1. **Game Complexity**: Caro (Gomoku variant) has higher branching factor than chess (~225 vs ~35 legal moves), leading to greater score variance in self-play.

2. **Pipeline Architecture**: The 3-phase Actor-Critic pipeline provides defense in depth:
   - Phase 1 (Actor): Wide inclusion (256cp) captures candidate moves
   - Phase 2 (Critic): Deep search verification filters false positives
   - Phase 3 (Integration): Statistical consensus (81.25%) ensures quality

3. **Powers of 2**: All thresholds use powers of 2 for clean bit operations and statistical significance testing.

4. **Self-Play vs Chess**: Chess engines typically evaluate positions at 30-50cp granularity. Caro positions have higher variance due to the larger board (16x16 vs 8x8).

**Temperature Decay Schedule:**

| Ply Range | Temperature | Phase |
|-----------|-------------|-------|
| 0-11 | 1.8 | High exploration (moves 1-6) |
| 12-23 | 1.0 | Medium exploration (moves 7-12) |
| 24+ | 0.0 | Optimal play (move 13+) |

This matches the expert recommendation: "decay to 0 by move 12" (our ply 24 = move 13).

---

## Examples

```bash
# Full pipeline with 8K games
dotnet run -- --full-pipeline --games 8192 --threads 8

# Individual phases
dotnet run -- --staging staging.db --games 8192
dotnet run -- --verify-staging staging.db --output verified.db
dotnet run -- --integrate verified.db --book opening_book.db

# Quick SPSA test
dotnet run -- --tune --iterations 3 --games-per-eval 32 --debug

# Export for production
dotnet run -- --export-binary book.cobook
```

---

## Architecture

See [ENGINE_FEATURES.md](../../../ENGINE_FEATURES.md) for:
- AI engine architecture (search, evaluation, move ordering)
- BitKey pattern system
- Transposition table design
- Time management

Book-specific architecture:
- **SelfPlayGenerator**: Temperature-based move sampling with Dirichlet noise
- **MoveVerifier**: Deep search verification with VCF solving
- **StagingBookStore**: SQLite-based game recording with buffering
- **InMemoryOpeningBook**: Fast lookup with 8-way symmetry reduction
