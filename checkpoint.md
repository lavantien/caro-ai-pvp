# Checkpoint: v1.77.0

## Summary

Fixed 12 UCI integration gaps where parsed options/parameters were stored but never forwarded to the core MinimaxAI engine. Unified AI engines by replacing StatelessSearchEngine with MinimaxAI in AIService. Removed dead abstraction layer.

## Changes

### UCI Integration Fixes
- Fixed moveNumber (hardcoded 0 -> computed from board) so Open Rule fires via UCI
- Forwarded increment (winc/binc) to MinimaxAI time management
- Exposed real search score in UCI info output (was hardcoded 0)
- Applied Hash option on `ucinewgame` (resize TT at runtime)
- Unified version string to single UCIEngineOptions.EngineVersion constant
- Forwarded Threads count to MinimaxAI (was used as boolean only)
- Forwarded go depth/nodes/movetime search limits to MinimaxAI
- Fixed Program.cs WebSocket handler brace mismatch

### MinimaxAI Enhancements
- Added optional parameters: incrementSeconds, threadCount, maxDepth, maxNodes, maxTimeMs
- Added ResizeTranspositionTable method (clear and rebuild)
- Removed arbitrary depth 6 cap in iterative deepening

### AI Unification
- Replaced StatelessSearchEngine with MinimaxAI in AIService
- Updated AIServiceTests for new dependency
- Deleted IUCIProtocolHandler dead interface

### Documentation
- Fixed Hash default, column notation in ENGINE_FEATURES.md
- Replaced StatelessSearchEngine example with MinimaxAI in CSHARP_ONBOARDING.md
- Fixed board size refs (19->16) and test counts in CSHARP_ONBOARDING.md
- Fixed UCIMoveNotation comment (0-7 -> 0-3)

## Verification

| Check | Result |
|-------|--------|
| dotnet build (solution) | Pass |
| Caro.Core.Tests (566) | Pass |
| Caro.Core.Domain.Tests (52) | Pass |
| Caro.Core.Application.Tests (14) | Pass |
| Caro.Core.Infrastructure.Tests (48) | Pass |

## Version

- Target: v1.77.0
- Previous: v1.76.0
