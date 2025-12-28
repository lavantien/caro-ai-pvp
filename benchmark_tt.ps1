# Quick Transposition Table Benchmark
# Runs 10 games to verify performance improvements

Write-Host "╔═══════════════════════════════════════════════════════════════════╗" -ForegroundColor Cyan
Write-Host "║       TRANSPOSITION TABLE PERFORMANCE BENCHMARK                      ║" -ForegroundColor Cyan
Write-Host "╚═══════════════════════════════════════════════════════════════════╝" -ForegroundColor Cyan
Write-Host ""

Write-Host "Building..." -ForegroundColor Yellow
dotnet build ~/dev/caro-claudecode/backend/Caro.Api.sln --verbosity quiet

Write-Host "Running 10 Expert vs Expert games..." -ForegroundColor Green
$stopwatch = [System.Diagnostics.Stopwatch]::StartNew()

pushd ~/dev/caro-claudecode/backend/src/Caro.TournamentRunner/bin/Debug/net10.0
    ./Caro.TournamentRunner.exe --auto 2>&1 | Select-String -Pattern "Expert vs Expert" | Select-Object -First 10
popd

$stopwatch.Stop()

Write-Host ""
Write-Host "╔═══════════════════════════════════════════════════════════════════╗" -ForegroundColor Cyan
Write-Host "║                          BENCHMARK RESULTS                          ║" -ForegroundColor Cyan
Write-Host "╚═══════════════════════════════════════════════════════════════════╝" -ForegroundColor Cyan
Write-Host ""
Write-Host "  Check output above for transposition table statistics:" -ForegroundColor White
Write-Host "    - [AI TT] Hits: X/Y (Z%)" -ForegroundColor Green
Write-Host "    - [AI TT] Table usage: X entries (Y%)" -ForegroundColor Green
Write-Host ""
Write-Host "  Expected: 30-60% hit rate with transposition tables" -ForegroundColor Yellow
Write-Host ""
