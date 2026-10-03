# Corrected-clock evidence rerun chain (2026-10-04): the pre-fix smoke gates
# ran on second-unit banks, so their strength conclusions are void (see
# docs/evidence/investigation-2026-10-04.md). This script reruns both gates
# sequentially at real-minute clocks, each into its own db and evidence dir.
# Detached launch:
#   Start-Process pwsh -WindowStyle Hidden -ArgumentList '-NoProfile','-File','docs/evidence/tourney/rerun-v2.ps1'
$ErrorActionPreference = 'Continue'
Set-Location (Join-Path $PSScriptRoot '..\..\..')
New-Item -ItemType Directory -Force db, docs\evidence\tourney\smoke10-v2, docs\evidence\tourney\smoke32-v2 | Out-Null
make tourney-smoke-10 ARGS="--db db/evidence-smoke10-v2.db" *> docs\evidence\tourney\smoke10-v2\leaderboard.txt
make tourney-smoke-32 ARGS="--db db/evidence-smoke32-v2.db" *> docs\evidence\tourney\smoke32-v2\leaderboard.txt
