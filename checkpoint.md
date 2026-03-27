# Checkpoint: v1.76.0

## Summary

Removed the entire opening book system (~20K lines across all layers). The system was unfeasible — generation took too long with no reliable quality assurance.

## Changes

### Opening Book Removal
- Deleted BookBuilder project (SPSA tuning, self-play, verification)
- Removed all BookServices (14 files: generation, lookup, validation, canonicalization, binary import/export)
- Removed infrastructure persistence (SQLite, staging, file stores)
- Removed MinimaxAI opening book integration
- Removed UCI `Use Opening Book` option and frontend client method
- Removed BookGeneration difficulty level
- Removed ~130 test files and test helpers

### Bug Fixes
- PositionTests: corrected board size assumption (18→15 for 16x16)
- UCIMoveNotation: updated 32x32 references to 16x16

### Documentation
- Removed SPSA and book references from README, ENGINE_FEATURES
- Updated move notation from 32x32 to 16x16
- Removed stale test counts from CSHARP_ONBOARDING

## Verification

| Check | Result |
|-------|--------|
| dotnet build (solution) | Pass |
| Caro.Core.Tests (566) | Pass |
| Caro.Core.Domain.Tests (52) | Pass |
| Caro.Core.Application.Tests (14) | Pass |
| Caro.Core.Infrastructure.Tests (48) | Pass |
| Caro.Core.IntegrationTests | Pass |
| Caro.Core.MatchupTests | Pass |
| Frontend build | Pass |
| Grep for book references (non-CHANGELOG) | Zero |

## Version

- Target: v1.76.0
- Previous: v1.75.0
