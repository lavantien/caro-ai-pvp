# Test Organization

This directory contains tests organized by category and type.

## Running Tests

### Run unit tests (recommended)
```bash
# From backend root - shows each test name and result
dotnet test --logger "console;verbosity=detailed"

# Result: Caro.Core.Tests + Caro.Core.Infrastructure.Tests
# Completes in ~2 minutes
```

### Run integration tests (AI search, stress tests)
```bash
# Must specify the project explicitly since IsTestProject=false
dotnet test tests/Caro.Core.IntegrationTests/Caro.Core.IntegrationTests.csproj --logger "console;verbosity=detailed"
```

### Run matchup tests
```bash
# Must specify the project explicitly since IsTestProject=false
dotnet test tests/Caro.Core.MatchupTests/Caro.Core.MatchupTests.csproj --logger "console;verbosity=detailed"
```

### Quick smoke test (no build)
```bash
dotnet test --no-build --logger "console;verbosity=detailed"
```

## Test Projects

| Project | Type | Default? | Description |
|---------|------|----------|-------------|
| `Caro.Core.Tests` | Unit | Yes | Core game logic (evaluation, detection, LUTs, threats) |
| `Caro.Core.Infrastructure.Tests` | Unit | Yes | Infrastructure (persistence, time utilities) |
| `Caro.Core.IntegrationTests` | Integration | No | AI search (VCF, DF-PN), stress tests, debug tests |
| `Caro.Core.MatchupTests` | Matchup | No | Full game AI matchups and tournaments |
| `Caro.Core.Domain.Tests` | Unit | Yes | Domain entities (Board, Cell, Player, GameState, Position) |
| `Caro.Core.Application.Tests` | Unit | Yes | Application services (DTOs, mappers) |

## Test Organization Notes

Projects with `<IsTestProject>false</IsTestProject>` are NOT run by default `dotnet test`:
- `Caro.Core.IntegrationTests` - Slow AI search tests
- `Caro.Core.MatchupTests` - Full AI matchups

To run these, explicitly target the project file.
