# Test Organization

This directory contains tests organized by category and type.

## Running Tests

### Run unit tests (recommended)
```bash
# From backend root - shows each test name and result
dotnet test --logger "console;verbosity=detailed"

# Result: Caro.Core.Tests (469 tests) + Caro.Core.Infrastructure.Tests (64 tests)
# Completes in ~30 seconds
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
| `Caro.Core.Tests` | Unit | Yes | Core game logic (evaluation, detection, LUTs, threats) - 469 tests |
| `Caro.Core.Infrastructure.Tests` | Unit | Yes | Infrastructure (persistence, time utilities) - 64 tests |
| `Caro.Core.IntegrationTests` | Integration | No | AI search (VCF, DF-PN), stress tests, debug tests - 224 tests |
| `Caro.Core.MatchupTests` | Matchup | No | Full game AI matchups and tournaments - 54 tests |
| `Caro.Core.Domain.Tests` | Unit | Yes | Domain entities (Board, Cell, Player, GameState, Position) - 66 tests |
| `Caro.Core.Application.Tests` | Unit | Yes | Application services (DTOs, mappers) - 14 tests |

## Test Organization Notes

Projects with `<IsTestProject>false</IsTestProject>` are NOT run by default `dotnet test`:
- `Caro.Core.IntegrationTests` - Slow AI search tests (224 tests)
- `Caro.Core.MatchupTests` - Full AI matchups (54 tests)

To run these, explicitly target the project file.

## Test Counts

- Unit tests (default): 613 tests (469 Core + 66 Domain + 14 Application + 64 Infrastructure) (~30s)
- Integration tests: 224 tests (opt-in)
- Matchup tests: 54 tests (opt-in)
