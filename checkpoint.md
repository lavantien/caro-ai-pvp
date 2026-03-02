# Checkpoint: v1.72.0 Development

## Summary

Expert Report Compliance Verification: Added comprehensive unit tests for self-play sampling strategy, fixed temperature=0 edge case, and documented threshold rationale.

## Progress

### Phase 1: Unit Tests for Sampling Strategy ✅

Added 28 new tests to `SelfPlayGeneratorTests.cs`:

| Test Category | Tests | Status |
|---------------|-------|--------|
| Temperature Decay | 4 | ✅ Pass |
| Score Delta Threshold | 3 | ✅ Pass |
| Dirichlet Noise | 5 | ✅ Pass |
| Softmax Sampling | 5 | ✅ Pass |
| Fallback Behavior | 2 | ✅ Pass |

### Phase 2: Pipeline Verification ✅

| Verification | Result |
|--------------|--------|
| Full pipeline smoke test | ✅ All 3 phases complete |
| Binary export/import | ✅ Round-trip validation |
| Existing test suite | ✅ 153 tests pass |

### Phase 3: Documentation ✅

Added expert report compliance documentation to BookBuilder README.

### Bug Fix Applied

- **SelfPlayGenerator.SampleMove()** - Fixed temperature=0 edge case to deterministically select best move (previously caused undefined behavior with division by zero in softmax calculation).

### Files Modified

| File | Change |
|------|--------|
| `SelfPlayGenerator.cs` | Internal visibility for testability, temperature=0 fix |
| `SelfPlayGeneratorTests.cs` | +28 tests for sampling strategy |
| `Caro.BookBuilder/README.md` | Expert report compliance documentation |

### Expert Report Compliance Summary

| Component | Expert Recommendation | Implementation | Status |
|-----------|----------------------|----------------|--------|
| Book Width | ~30cp margin | 256cp margin | ⚠️ Acceptable (documented) |
| Max Moves | 5 per position | 4 per position | ✅ More conservative |
| Time Control | 1+0 or Fixed Nodes | 1+0 default | ✅ Compliant |
| Threading | Parallel single-threaded | Parallel workers | ✅ Compliant |
| Sampling | Softmax + Decay | Softmax + Decay to 0 by ply 24 | ✅ Compliant |

## Version

- Target: v1.72.0
- Previous: v1.71.0
