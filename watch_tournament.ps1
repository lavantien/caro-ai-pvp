# Real-time Tournament Progress Monitor - 8-Bot Edition
# Run this script to watch the tournament live

$outputFile = "backend/src/Caro.TournamentRunner/bin/Release/net10.0/tournament_8bots.txt"

Write-Host "╔════════════════════════════════════════════════════════════════════════════╗" -ForegroundColor Cyan
Write-Host "║          🏁 AI TOURNAMENT LIVE MONITOR - 8 BOTS COMPETING                ║" -ForegroundColor Cyan
Write-Host "╚════════════════════════════════════════════════════════════════════════════╝" -ForegroundColor Cyan
Write-Host ""
Write-Host "  8 Bots (all starting at 600 ELO):" -ForegroundColor White
Write-Host "    • Rookie Alpha & Bravo (Easy, depth 1)" -ForegroundColor Gray
Write-Host "    • Casual Alpha & Bravo (Medium, depth 2)" -ForegroundColor Cyan
Write-Host "    • Skilled Alpha & Bravo (Hard, depth 3)" -ForegroundColor Magenta
Write-Host "    • Master Alpha & Bravo (Expert, depth 5)" -ForegroundColor Yellow
Write-Host ""

if (!(Test-Path $outputFile)) {
    Write-Host "❌ Tournament output file not found!" -ForegroundColor Red
    Write-Host "   Make sure the tournament is running..." -ForegroundColor Yellow
    exit 1
}

# Get file size to track new content
$lastSize = (Get-Item $outputFile).Length

Write-Host "📊 Watching for progress updates..." -ForegroundColor Green
Write-Host "   Press Ctrl+C to stop monitoring" -ForegroundColor Gray
Write-Host ""

while ($true) {
    Start-Sleep -Seconds 2

    if (!(Test-Path $outputFile)) { break }

    $currentSize = (Get-Item $outputFile).Length

    # Read new content if file grew
    if ($currentSize -gt $lastSize) {
        # Get last few lines
        $content = Get-Content $outputFile -Tail 30

        # Find latest progress (auto-detect total games)
        $latestProgress = $content | Select-String "\[\d+/\d+\]" | Select-Object -Last 1

        if ($latestProgress) {
            Clear-Host
            Write-Host "╔════════════════════════════════════════════════════════════════════════════╗" -ForegroundColor Cyan
            Write-Host "║          🏁 AI TOURNAMENT LIVE MONITOR - 8 BOTS COMPETING                ║" -ForegroundColor Cyan
            Write-Host "╚════════════════════════════════════════════════════════════════════════════╝" -ForegroundColor Cyan
            Write-Host ""

            # Parse progress (handles any total game count)
            if ($latestProgress.Line -match "\[(\d+)/(\d+)\]\s+([\d.]+)%\s+-\s+(\w+)\s+vs\s+(\w+):\s*(.+)") {
                $current = $matches[1]
                $total = $matches[2]
                $percent = $matches[3]
                $red = $matches[4]
                $blue = $matches[5]
                $result = $matches[6].Trim()

                # Progress bar
                $barLength = 50
                $filled = [math]::Floor($barLength * ([double]$percent / 100))
                $empty = $barLength - $filled
                $bar = "█" * $filled + "░" * $empty

                Write-Host "  Progress: [$bar] $percent%" -ForegroundColor Green
                Write-Host "  Game: $current/$total" -ForegroundColor Cyan
                Write-Host "  Matchup: $red vs $blue" -ForegroundColor Yellow
                Write-Host "  Result: $result" -ForegroundColor Magenta
                Write-Host ""
            }

            # Show AI debug info if available
            $aiDebug = $content | Select-String "\[AI DEBUG\]" | Select-Object -Last 1
            if ($aiDebug) {
                Write-Host "  🤖 AI Status:" -ForegroundColor DarkGray
                Write-Host "     $($aiDebug.Line.Trim())" -ForegroundColor DarkGray
                Write-Host ""
            }

            # Show recent results
            Write-Host "  Recent Results:" -ForegroundColor White
            Write-Host "  ───────────────" -ForegroundColor Gray

            $content | Select-String "→" | Select-Object -Last 4 | ForEach-Object {
                $line = $_.Line.Trim()
                # Handle wins
                if ($line -match "→ (\w+) \((\w+)\) defeated (\w+) \((\w+)\)") {
                    $winnerDiff = $matches[1]
                    $winner = $matches[2]
                    $loserDiff = $matches[3]
                    $loser = $matches[4]

                    $color = if ($winnerDiff -eq "Easy") { "Gray" }
                            elseif ($winnerDiff -eq "Medium") { "Cyan" }
                            elseif ($winnerDiff -eq "Hard") { "Magenta" }
                            elseif ($winnerDiff -eq "Expert") { "Yellow" }
                            else { "White" }

                    Write-Host "    ▶ $winnerDiff ($winner) beat $loserDiff ($loser)" -ForegroundColor $color
                }
                # Handle draws
                elseif ($line -match "→ Draw - (\w+) vs (\w+)") {
                    $diff1 = $matches[1]
                    $diff2 = $matches[2]
                    Write-Host "    ▸ Draw - $diff1 vs $diff2" -ForegroundColor DarkGray
                }

                if ($line -match "Moves: (\d+), Time: ([\d.]+)s, Avg Move: ([\d.]+)ms") {
                    $moves = $matches[1]
                    $time = $matches[2]
                    $avgMove = $matches[3]
                    Write-Host "       📊 $moves moves, ${time}s, ${avgMove}ms avg" -ForegroundColor DarkGray
                }
            }

            # Show timeouts if any
            $timeouts = $content | Select-String "TIMEOUT" | Select-Object -Last 2
            if ($timeouts) {
                Write-Host ""
                Write-Host "  Recent Timeouts:" -ForegroundColor Red
                Write-Host "  ────────────────" -ForegroundColor Gray
                $timeouts | ForEach-Object {
                    Write-Host "    ⏰ $($_.Line.Trim())" -ForegroundColor DarkRed
                }
            }

            Write-Host ""
            Write-Host "  Monitoring tournament_8bots.txt..." -ForegroundColor DarkGray
            Write-Host ""
            $lastSize = $currentSize
        }
    }
}
