# Quick script to check tournament progress
$outputFile = "backend\src\Caro.TournamentRunner\tournament_output.txt"

if (Test-Path $outputFile) {
    $content = Get-Content $outputFile -Raw

    # Extract latest progress
    if ($content -match "\[(\d+)/(\d+)\]\s+([\d.]+)%\s+-\s+(\w+)\s+vs\s+(\w+):") {
        $current = $matches[1]
        $total = $matches[2]
        $percent = $matches[3]
        $red = $matches[4]
        $blue = $matches[5]

        Write-Host "Tournament Progress: $current/$total ($percent%)" -ForegroundColor Cyan
        Write-Host "Current Match: $red vs $blue" -ForegroundColor Yellow
    }

    # Count completed games
    $games = ([regex]::Matches($content, "\[\d+/90\]")).Count
    Write-Host "`nGames Completed: $games" -ForegroundColor Green

    # Show last few results
    Write-Host "`nRecent Results:" -ForegroundColor Magenta
    $content -split "`n" | Select-String "→" | Select-Object -Last 3 | ForEach-Object {
        Write-Host "  $_" -ForegroundColor Gray
    }
} else {
    Write-Host "Tournament output file not found!" -ForegroundColor Red
}
