# Test Organization

This directory contains tests organized by category and type.

## Running Tests

### Run unit tests (recommended)
```bash
# From backend root - shows each test name and result
dotnet test --logger "console;verbosity=detailed"

# Result: Caro.Core.Tests (330 tests) + Caro.Core.Infrastructure.Tests (48 tests)
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
| `Caro.Core.Tests` | Unit | Yes | Core game logic (evaluation, detection, LUTs, threats) |
| `Caro.Core.Infrastructure.Tests` | Unit | Yes | Infrastructure (persistence, time utilities) |
| `Caro.Core.IntegrationTests` | Integration | No | AI search (VCF, DF-PN), stress tests, debug tests |
| `Caro.Core.MatchupTests` | Matchup | No | Full game AI matchups and tournaments |
| `Caro.Core.Domain.Tests` | Unit | No | Domain entities (outdated, needs API updates) |
| `Caro.Core.Application.Tests` | Unit | No | Application services (outdated, needs API updates) |

## Test Organization Notes

Projects with `<IsTestProject>false</IsTestProject>` are NOT run by default `dotnet test`:
- `Caro.Core.IntegrationTests` - Slow AI search tests moved here
- `Caro.Core.MatchupTests` - Full AI matchups
- `Caro.Core.Domain.Tests` - Outdated
- `Caro.Core.Application.Tests` - Outdated

To run these, explicitly target the project file.

## Test Counts

- Unit tests (default): 378 tests (~30s)
- Integration tests: 100+ tests (opt-in)
- Matchup tests: 50+ tests (opt-in)
