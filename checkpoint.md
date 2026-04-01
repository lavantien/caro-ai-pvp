# Checkpoint: v2.5.0

## Summary

Extracted ~200 magic numbers from production code across 25 files into centralized config hubs. Backend uses 3 new static constant classes in `Caro.Core.Domain/Configuration/`. Frontend uses 7 new frozen-object config modules in `src/lib/config/`.

## Changes

### Backend - New Config Classes
- `SearchHeuristicConstants.cs` - Threat scores, search bounds, depth controls, time ratios, VCF thresholds
- `TimeConstants.cs` - TimeMonitor, AsyncQueue, UCIProtocol, SearchLogger, Ponderer, DFPN/TSS defaults, HardBound buffers
- `TimeManagementConstants.cs` - Default time controls, PID controller weights, phase thresholds, adaptive scaling, emergency thresholds

### Backend - Updated Files (13)
- MinimaxAI.cs - 30+ replacements (SHC alias)
- ParallelMinimaxSearch.cs - 30+ replacements (SHC, TMC, TC aliases)
- TimeManager.cs - 16+ replacements (TMC alias)
- AdaptiveTimeManager.cs - 30+ replacements (TMC alias)
- TimeBudgetDepthManager.cs - 9 replacements (TMC alias)
- TimeMonitor.cs, AsyncQueue.cs, Ponderer.cs, DFPNSearch.cs, ThreatSpaceSearch.cs
- SearchLogger.cs, UCIProtocol.cs, UCIMockClient.cs

### Frontend - New Config Modules (7)
- apiConfig.ts, audioConfig.ts, e2eConfig.ts, hapticConfig.ts, ratingConfig.ts, uciConfig.ts, uiConfig.ts

### Frontend - Updated Files (12)
- +page.svelte - 6x URL fallbacks replaced with ApiConfig.baseUrl
- Board.svelte, Cell.svelte, WinningLine.svelte - UI dimensions from uiConfig
- Timer.svelte - URL, intervals, thresholds from config
- gameStore.svelte.ts, ratingStore.svelte.ts, uciEngine.ts, boardUtils.ts, sound.ts, haptics.ts
- e2e/game.spec.ts - All timeout constants from e2eConfig

### Fixed
- WinningLine.svelte: wrong default props (boardSize=15, cellSize=40) now use config values

## Verification

| Check | Result |
|-------|--------|
| dotnet build | Pass (0 errors, 0 warnings) |
| dotnet test (229 tests) | Pass |
| svelte-check | Pass (0 errors, 0 warnings) |
| vitest (64 tests) | Pass |
| grep localhost:5207 in src/ | 1 result (apiConfig.ts definition only) |
| grep 64px in src/ | 0 results |

## Version

- Target: v2.5.0
- Previous: v2.4.1
