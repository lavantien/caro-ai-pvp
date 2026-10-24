[CmdletBinding()]
param(
	[int]$Games = 40,
	[int]$PollSec = 60,
	[string]$MemLimit = '12GiB',
	[string]$OutRoot = 'playground/arena/out',
	[string]$Predecessor = 'hex-passgates',
	[string]$GoVerdict = 'all-done',
	[switch]$Help,
	[switch]$DryRun,
	[switch]$Foreground
)

$ErrorActionPreference = 'Stop'
$repo = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$target = 'hex-ladder'
$verdictPrefix = 'hex-ladder:'

$usage = @'
hex-ladder: detached hex ladder inversion chain, easy/medium/hard/master round-robin
  waits for $GoVerdict inside $OutRoot/$Predecessor.status before starting
  gate law: zero tier inversions among adjacent and non-adjacent pairs at the
        played counts, the deliberate underpowering ($Games games per edge at
        tc index 1 (2+1) and 2 (3+2) only) is recorded in the verdict line
  arms: full round-robin at tc index 1 and 2, one arm per time control,
        run via: make hex-ladder TC=<i> GAMES=<n>
  make contract: the target plays the round-robin and prints exactly one
        final verdict line:
        hex-ladder: DONE <underpowering note naming the per-edge game count>
        hex-ladder: FAILED edge=<lower>-vs-<higher> tc=<i> wd=<x> n=<games>
        the FAILED line names the inverted edge and its played count
        a nonzero exit or a missing verdict line counts as a crash
  marker $OutRoot/hex-ladder.status is rewritten on every terminal transition:
        running <arm> | all-done | failed <arm> <verdict detail> |
        crashed <arm> exit <n> | crashed <arm> exit 0 no-verdict |
        failed no-make-target hex-ladder
  logs under $OutRoot/hex-ladder-<arm>.log, poll trace in $OutRoot/hex-ladder.poll.log
  every engine invocation runs under GOMEMLIMIT $MemLimit
  relaunching resumes: DONE arms skip, FAILED arms re-mark their failure,
        pending arms run; a missing make target is a loud failure, never a loop
'@

if ($Help) { Write-Output $usage; exit 0 }

$arms = @()
foreach ($tc in 1, 2) {
	$arms += @{ Name = "hex-ladder-tc$tc"; Vars = "TC=$tc GAMES=$Games" }
}

function Convert-ToRepoPath([string]$rel) { Join-Path $repo ($rel -replace '/', '\') }

function Get-ArmVerdict([string]$name) {
	$log = Convert-ToRepoPath "$OutRoot/$name.log"
	if (-not (Test-Path $log)) { return $null }
	$line = Select-String -Path $log -Pattern ('^' + [regex]::Escape($verdictPrefix) + ' (DONE|FAILED.*)$') | Select-Object -Last 1
	if ($null -eq $line) { return $null }
	return $line.Line
}

function Test-ArmDone([string]$name) {
	$v = Get-ArmVerdict $name
	return $null -ne $v -and $v -eq "$verdictPrefix DONE"
}

function Test-ArmFailed([string]$name) {
	$v = Get-ArmVerdict $name
	return $null -ne $v -and $v -like "$verdictPrefix FAILED*"
}

function Write-Status([string]$text) {
	$path = Convert-ToRepoPath "$OutRoot/$target.status"
	New-Item -ItemType Directory -Force (Split-Path $path -Parent) | Out-Null
	Set-Content -Path $path -Value $text
	Write-Output "status: $text"
}

function Invoke-Arm($arm) {
	$log = Convert-ToRepoPath "$OutRoot/$($arm.Name).log"
	New-Item -ItemType Directory -Force (Split-Path $log -Parent) | Out-Null
	$cmd = "set GOMEMLIMIT=$MemLimit&& make $target $($arm.Vars) >>`"$log`" 2>&1"
	Write-Host "arm $($arm.Name) start, log $log"
	$p = Start-Process -FilePath cmd.exe -ArgumentList '/c', $cmd -WorkingDirectory $repo -WindowStyle Hidden -Wait -PassThru
	if ($p.ExitCode -ne 0) {
		Write-Host "arm $($arm.Name) exited $($p.ExitCode), see $log"
		return "crashed $($arm.Name) exit $($p.ExitCode)"
	}
	$verdict = Get-ArmVerdict $arm.Name
	if ($null -eq $verdict) {
		Write-Host "arm $($arm.Name) produced no verdict line, see $log"
		return "crashed $($arm.Name) exit 0 no-verdict"
	}
	if (-not (Test-ArmDone $arm.Name)) {
		return "failed $($arm.Name) $verdict"
	}
	Write-Host "arm $($arm.Name) done"
	return $null
}

if ($DryRun) {
	Write-Output "hex-ladder dryrun: games $Games poll ${PollSec}s mem $MemLimit predecessor $Predecessor go '$GoVerdict'"
	foreach ($arm in $arms) {
		$state = if (Test-ArmDone $arm.Name) { 'done' } elseif (Test-ArmFailed $arm.Name) { 'failed' } else { 'pending' }
		Write-Output "arm $($arm.Name) state $state"
	}
	Write-Output "command template per arm:"
	foreach ($arm in $arms) {
		Write-Output "  set GOMEMLIMIT=$MemLimit&& make $target $($arm.Vars)"
	}
	Write-Output "probe before the first arm: make -n $target"
	exit 0
}

if (-not $Foreground) {
	$pass = "-NoProfile -File `"$PSCommandPath`" -Foreground -Games $Games -PollSec $PollSec -MemLimit $MemLimit -OutRoot `"$OutRoot`" -Predecessor `"$Predecessor`" -GoVerdict `"$GoVerdict`""
	Start-Process -FilePath pwsh.exe -ArgumentList $pass -WorkingDirectory $repo -WindowStyle Hidden
	Write-Output "hex-ladder detached, logs under $OutRoot"
	exit 0
}

$predStatus = Convert-ToRepoPath "$OutRoot/$Predecessor.status"
$pollLog = Convert-ToRepoPath "$OutRoot/$target.poll.log"
New-Item -ItemType Directory -Force (Split-Path $pollLog -Parent) | Out-Null
"$(Get-Date -Format s) hex-ladder start, waiting for '$GoVerdict' in $Predecessor.status" | Add-Content -Path $pollLog
while ($true) {
	if ((Test-Path $predStatus) -and ((Get-Content $predStatus -Raw) -match [regex]::Escape($GoVerdict))) { break }
	"$(Get-Date -Format s) waiting for '$GoVerdict' in $Predecessor.status" | Add-Content -Path $pollLog
	Start-Sleep -Seconds $PollSec
}
"$(Get-Date -Format s) $Predecessor.status reached '$GoVerdict'" | Add-Content -Path $pollLog

$global:LASTEXITCODE = 0
$probeOk = $true
try { & make -n $target *> $null } catch { $probeOk = $false }
if (-not $probeOk -or $LASTEXITCODE -ne 0) { Write-Status "failed no-make-target $target"; exit 2 }

foreach ($arm in $arms) {
	if (Test-ArmDone $arm.Name) { Write-Output "arm $($arm.Name) already complete, skipping"; continue }
	if (Test-ArmFailed $arm.Name) {
		$verdict = Get-ArmVerdict $arm.Name
		Write-Status "failed $($arm.Name) $verdict"
		exit 2
	}
	Write-Status "running $($arm.Name)"
	$failure = Invoke-Arm $arm
	if ($null -ne $failure) { Write-Status $failure; exit 2 }
}
Write-Status 'all-done'
Write-Output 'hex-ladder complete'
exit 0
