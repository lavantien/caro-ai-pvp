[CmdletBinding()]
param(
	[int]$Parallel = 4,
	[string]$Timeout = '5m',
	[string]$Log = 'logs/archive/mutate-finalclose-engine-server.log',
	[switch]$Help,
	[switch]$DryRun,
	[switch]$Foreground
)

$ErrorActionPreference = 'Stop'
$repo = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$target = 'mutate-finalclose'
$scope = 'internal/engine,internal/server'

$usage = @'
mutate-finalclose: detached engine plus server mutation re-gate for the final
  close, plan docs/plans/final-close.md step 2, ruling 8
  scope internal/engine and internal/server only, rules and vcf carry their
  2026-10-31 green because only engine and config source and server test
  files changed since
  allowlist .mutate-allow filtered by -allow-scope to the same prefixes so a
  scoped gate skips entries outside its packages
  resumable: a partial prior log replays its KILLED verdicts through
  -resume, a terminal log line means the gate already closed and the run
  skips
  marker .scratch/mutate-finalclose.status: running | all-done exit 0 |
  failed exit <n> | done, status artifact never edited by hand
  per-mutant timeout defaults to 5m so the 86s clean internal/server suite
  fits the derived inner window (outer minus 61s) with load headroom, the
  prior engine-only gates ran at 2m whose inner 59s cannot hold the server
  suite, no GOMEMLIMIT: the tool caps each worker tree itself and the prior
  green gates ran without one, GOTMPDIR <repo>/.gotmp
'@

if ($Help) { Write-Output $usage; exit 0 }

function Write-Status([string]$text) {
	Set-Content -Path (Join-Path $repo '.scratch\mutate-finalclose.status') -Value $text
	Write-Output "status: $text"
}

$fullLog = Join-Path $repo ($Log -replace '/', '\')
$priorKills = 0

if ($DryRun) {
	Write-Output "mutate-finalclose dryrun: parallel $Parallel log $Log"
	if (Test-Path $fullLog) {
		$last = (Get-Content $fullLog | Select-Object -Last 1)
		$killed = (Select-String -Path $fullLog -Pattern ' KILLED$').Count
		Write-Output "prior log present: $killed kills recorded, last line: $last"
	} else {
		Write-Output 'no prior log, fresh run'
	}
	Write-Output "command: go run ./cmd/mutate -pkgs $scope -allow .mutate-allow -allow-scope $scope -parallel $Parallel -timeout $Timeout > $Log"
	exit 0
}

if (-not $Foreground) {
	$pass = "-NoProfile -File `"$PSCommandPath`" -Foreground -Parallel $Parallel -Log `"$Log`""
	Start-Process -FilePath pwsh.exe -ArgumentList $pass -WorkingDirectory $repo -WindowStyle Hidden
	Write-Output "mutate-finalclose detached, log $Log"
	exit 0
}

$twins = Get-CimInstance Win32_Process -Filter "Name = 'pwsh.exe'" | Where-Object { $_.ProcessId -ne $PID -and $_.CommandLine -and $_.CommandLine.Contains($PSCommandPath) -and $_.CommandLine.Contains('-Foreground') }
if ($twins) { Write-Output 'mutate-finalclose: foreground instance already live, exiting'; exit 3 }

$gotmp = Join-Path $repo '.gotmp'
New-Item -ItemType Directory -Force $gotmp | Out-Null
$env:GOTMPDIR = $gotmp
$env:CGO_ENABLED = '1'

$resumeArg = @()
if (Test-Path $fullLog) {
	$done = Get-Content $fullLog -ErrorAction SilentlyContinue | Where-Object { $_ -match '^\S+ mutate: \d+/\d+ run' } | Select-Object -Last 1
	$priorKills = (Select-String -Path $fullLog -Pattern ' KILLED$').Count
	if ($null -ne $done) { Write-Status "all-done prior gate: $done"; exit 0 }
	$resumeArg = @('-resume', $fullLog)
	Write-Output "mutate-finalclose: resuming prior log with $priorKills kills"
} else {
	New-Item -ItemType Directory -Force (Split-Path $fullLog -Parent) | Out-Null
}

Write-Status 'running'
$argList = @('run', './cmd/mutate', '-pkgs', $scope, '-allow', '.mutate-allow', '-allow-scope', $scope, '-parallel', "$Parallel", '-timeout', $Timeout) + $resumeArg
$stamp = "$(Get-Date -Format s) mutate-finalclose start: go $($argList -join ' ')"
Add-Content -Path $fullLog -Value $stamp
& go @argList 2>&1 | ForEach-Object { "$_" | Add-Content -Path $fullLog }
$code = $LASTEXITCODE
Add-Content -Path $fullLog -Value "$(Get-Date -Format s) mutate-finalclose exit $code"
if ($code -eq 0) { Write-Status "all-done exit 0" } else { Write-Status "failed exit $code" }
exit $code
