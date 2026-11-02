[CmdletBinding()]
param(
	[int]$BudgetSec = 60,
	[string]$MemLimit = '12GiB',
	[string]$OutRoot = 'playground/sqstrength/out',
	[switch]$Help,
	[switch]$DryRun,
	[switch]$Foreground
)

$ErrorActionPreference = 'Stop'
$repo = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$target = 'sqdiscrim'

$usage = @'
sqdiscrim: detached square eval-versus-depth discriminator replay
  replays the 20 recorded master-blue positions from
  $OutRoot/discriminator-positions.json through the real master-tier
  searcher, one fresh engine per position, one continuous soft-stopped
  search under a fixed per-position budget of $BudgetSec seconds, the
  per-iteration scores of the last 8 completed depths from the exported
  root history plus the final M-line land in
  $OutRoot/discriminator-trace.log
  progress marker .scratch/fix-sqdiscrim.progress holds n/20, rewritten
  after each finished position and 20/20 at the end, relaunching resumes
  from the marker, launcher output lands in
  $OutRoot/discriminator-launch.log
  every engine invocation runs under GOMEMLIMIT $MemLimit, CGO_ENABLED=1,
  GOTMPDIR <repo>/.gotmp, bin/arena-freeze.exe is never touched
'@

if ($Help) { Write-Output $usage; exit 0 }

function Convert-ToRepoPath([string]$rel) { Join-Path $repo ($rel -replace '/', '\') }

$trace = Convert-ToRepoPath "$OutRoot/discriminator-trace.log"
$positions = Convert-ToRepoPath "$OutRoot/discriminator-positions.json"
$progress = Convert-ToRepoPath '.scratch/fix-sqdiscrim.progress'
$launchLog = Convert-ToRepoPath "$OutRoot/discriminator-launch.log"
$gotmp = Join-Path $repo '.gotmp'
$probeExe = Join-Path $gotmp 'sqdiscrim.exe'
$probePkg = './playground/sqstrength/probe'

if ($DryRun) {
	Write-Output "sqdiscrim dryrun: budget ${BudgetSec}s mem $MemLimit"
	Write-Output "positions $positions"
	Write-Output "trace $trace"
	Write-Output "progress $progress"
	$done = if (Test-Path $progress) { (Get-Content $progress -First 1) } else { '0/20' }
	Write-Output "progress now $done, would resume from it"
	exit 0
}

if (-not $Foreground) {
	$pass = "-NoProfile -File `"$PSCommandPath`" -Foreground -BudgetSec $BudgetSec -MemLimit $MemLimit -OutRoot `"$OutRoot`""
	Start-Process -FilePath pwsh.exe -ArgumentList $pass -WorkingDirectory $repo -WindowStyle Hidden
	Write-Output "sqdiscrim detached, trace $trace, progress $progress"
	exit 0
}

$twins = Get-CimInstance Win32_Process -Filter "Name = 'pwsh.exe'" | Where-Object { $_.ProcessId -ne $PID -and $_.CommandLine -and $_.CommandLine.Contains($PSCommandPath) -and $_.CommandLine.Contains('-Foreground') }
if ($twins) { Write-Output "sqdiscrim: foreground instance already live, exiting"; exit 3 }

if (-not (Test-Path $positions)) { Write-Output "sqdiscrim: missing $positions"; exit 1 }
New-Item -ItemType Directory -Force $gotmp | Out-Null
New-Item -ItemType Directory -Force (Split-Path $trace -Parent) | Out-Null
New-Item -ItemType Directory -Force (Split-Path $progress -Parent) | Out-Null

"$(Get-Date -Format s) sqdiscrim start" | Add-Content -Path $launchLog
$env:GOMEMLIMIT = '4GiB'
$env:CGO_ENABLED = '1'
$env:GOTMPDIR = $gotmp
go -C (Join-Path $repo 'playground\sqstrength\probe') build -o $probeExe .
if ($LASTEXITCODE -ne 0 -or -not (Test-Path $probeExe)) {
	Write-Output "sqdiscrim: probe build failed exit $LASTEXITCODE"
	"$(Get-Date -Format s) build failed exit $LASTEXITCODE" | Add-Content -Path $launchLog
	exit 1
}

$env:GOMEMLIMIT = $MemLimit
& $probeExe -positions $positions -out $trace -progress $progress -budgetSec $BudgetSec *>> $launchLog
$code = $LASTEXITCODE
$done = if (Test-Path $progress) { (Get-Content $progress -First 1) } else { 'missing' }
"$(Get-Date -Format s) sqdiscrim probe exit $code progress $done" | Add-Content -Path $launchLog
Write-Output "sqdiscrim probe exit $code progress $done"
exit $code
