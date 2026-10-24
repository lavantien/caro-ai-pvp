# Original v2 evidence: smoke10-v2 landed at 0270dc0, smoke32-v2 at 20ac301 (docs/evidence/analysis-2026-10-04.md).
$ErrorActionPreference = 'Continue'
Set-Location (Join-Path $PSScriptRoot '..\..\..')
New-Item -ItemType Directory -Force db, docs\evidence\tourney\smoke10-v2, docs\evidence\tourney\smoke32-v2 | Out-Null
go run ./cmd/run tourney-smoke-10 ARGS="--db db/evidence-smoke10-v2.db" *> docs\evidence\tourney\smoke10-v2\leaderboard.txt
go run ./cmd/run tourney-smoke-32 ARGS="--db db/evidence-smoke32-v2.db" *> docs\evidence\tourney\smoke32-v2\leaderboard.txt
