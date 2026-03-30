# Performance Statistics

This file previously contained baseline benchmark results for a multi-difficulty bot system (Braindead/Easy/Medium/Hard/Grandmaster/Experimental) and tournament mode. These features were removed. The engine now runs at full strength only.

To benchmark the engine, use the UCI Mock Client for engine self-play:

```bash
cd backend/src/Caro.UCIMockClient && dotnet run -- --games 4 --time 180 --inc 2
```
