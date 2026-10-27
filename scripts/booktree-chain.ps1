[CmdletBinding()]
param(
	[int]$PollSec = 120,
	[string]$OutRoot = 'playground/arena/out',
	[string]$Predecessor = 'passgates-fill',
	[switch]$Help,
	[switch]$DryRun,
	[switch]$Foreground
)

$ErrorActionPreference = 'Stop'
$repo = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path

$usage = @'
booktree-chain: detached queue gate for the tree build
  waits for the fill batch terminal marker, all-done or done-with-failures,
  in $OutRoot/$Predecessor.status, then hands the machine to
  scripts/booktree-run.ps1 -Foreground inline
  both terminal markers mean the fill batch released the machine, failed legs
  stay recorded in their logs for the ruling, the tree build never waits on
  verdicts
  marker states running <arm> | failed <arm> <detail> | crashed <arm> exit <n>
  are transient, the fill batch continues past failed arms, the chain ignores
  them
  poll trace in $OutRoot/booktree-chain.poll.log
  relaunching is safe: a live twin exits 3, an already-live tree build makes
  booktree-run itself a no-op
'@

if ($Help) { Write-Output $usage; exit 0 }

function Convert-ToRepoPath([string]$rel) { Join-Path $repo ($rel -replace '/', '\') }

if ($DryRun) {
	$status = Convert-ToRepoPath "$OutRoot/$Predecessor.status"
	$marker = if (Test-Path $status) { (Get-Content $status -Raw).Trim() } else { 'absent' }
	$terminal = $marker -match '^(all-done|done-with-failures)'
	$alive = Get-CimInstance Win32_Process -Filter "Name='bookgen.exe'" | Where-Object { $_.CommandLine -and $_.CommandLine.Contains('-phase tree') -and $_.ExecutablePath -and $_.ExecutablePath.StartsWith($repo, [StringComparison]::OrdinalIgnoreCase) }
	Write-Output "booktree-chain dryrun: poll ${PollSec}s predecessor $Predecessor"
	Write-Output "marker '$marker' terminal $terminal tree-alive $($alive.Count)"
	exit 0
}

if (-not $Foreground) {
	$pass = "-NoProfile -File `"$PSCommandPath`" -Foreground -PollSec $PollSec -OutRoot `"$OutRoot`" -Predecessor `"$Predecessor`""
	Start-Process -FilePath pwsh.exe -ArgumentList $pass -WorkingDirectory $repo -WindowStyle Hidden
	Write-Output 'booktree-chain detached'
	exit 0
}

$twins = Get-CimInstance Win32_Process -Filter "Name = 'pwsh.exe'" | Where-Object { $_.ProcessId -ne $PID -and $_.CommandLine -and $_.CommandLine.Contains($PSCommandPath) -and $_.CommandLine.Contains('-Foreground') }
if ($twins) { Write-Output 'booktree-chain: foreground instance already live, exiting'; exit 3 }

$predStatus = Convert-ToRepoPath "$OutRoot/$Predecessor.status"
$pollLog = Convert-ToRepoPath "$OutRoot/booktree-chain.poll.log"
New-Item -ItemType Directory -Force (Split-Path $pollLog -Parent) | Out-Null
"$(Get-Date -Format s) booktree-chain start, waiting for fill terminal marker" | Add-Content -Path $pollLog
while ($true) {
	if ((Test-Path $predStatus) -and ((Get-Content $predStatus -Raw) -match '^(all-done|done-with-failures)')) { break }
	$state = if (Test-Path $predStatus) { (Get-Content $predStatus -Raw).Trim() } else { 'absent' }
	"$(Get-Date -Format s) waiting, marker '$state'" | Add-Content -Path $pollLog
	Start-Sleep -Seconds $PollSec
}
"$(Get-Date -Format s) predecessor terminal, starting booktree-run" | Add-Content -Path $pollLog
& (Join-Path $PSScriptRoot 'booktree-run.ps1') -Foreground
$rc = $LASTEXITCODE
"$(Get-Date -Format s) booktree-run exit $rc" | Add-Content -Path $pollLog
exit $rc
