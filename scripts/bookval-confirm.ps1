[CmdletBinding()]
param(
	[int]$Games = 20,
	[string]$MemLimit = '12GiB',
	[string]$OutRoot = 'playground/arena/out',
	[string]$Bin = 'bin/arena.exe',
	[switch]$Help,
	[switch]$DryRun,
	[switch]$Foreground
)

$ErrorActionPreference = 'Stop'
$repo = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$name = 'bookval-confirm-105'

$usage = @'
bookval-confirm: detached 10+5 book confirmation pair, master vs master-nobook
  arm: arena -tc 3 -games <n> -seats master,master-nobook -pairOut <out>/<name>.pairlog
  logs and resume record under the out root, stdout appended to <name>.log
  switches: -DryRun prints the plan, -Foreground runs inline without detaching
  relaunching resumes from the pairlog's recorded games
'@

if ($Help) { Write-Output $usage; exit 0 }

$args_ = "-tc 3 -games $Games -seats master,master-nobook -pairOut $OutRoot/$name.pairlog"

function Test-PairDone([string]$pairlog) {
	$pl = Join-Path $repo (($OutRoot + '/' + $pairlog) -replace '/', '\')
	if (-not (Test-Path $pl)) { return $false }
	return [bool](Select-String -Path $pl -Pattern '^complete \d+ games' -Quiet)
}

function Invoke-Arm([string]$armName, [string]$armArgs) {
	$log = Join-Path $repo (($OutRoot + '/' + $armName + '.log') -replace '/', '\')
	New-Item -ItemType Directory -Force (Split-Path $log -Parent) | Out-Null
	$binAbs = Join-Path $repo ($Bin -replace '/', '\')
	$cmd = "set GOMEMLIMIT=$MemLimit&& `"$binAbs`" $armArgs >>`"$log`" 2>&1"
	if ($DryRun) {
		Write-Host "dryrun arm ${armName}: $cmd"
		return 0
	}
	Write-Host "arm $armName start, log $log"
	$p = Start-Process -FilePath cmd.exe -ArgumentList '/c', $cmd -WorkingDirectory $repo -WindowStyle Hidden -Wait -PassThru
	if ($p.ExitCode -ne 0) {
		Write-Host "arm $armName exited $($p.ExitCode), see $log"
		return $p.ExitCode
	}
	if (-not (Test-PairDone "$armName.pairlog")) {
		Write-Host "arm $armName ran without reaching completion, see $log"
		return 1
	}
	Write-Host "arm $armName done"
	return 0
}

if ($DryRun) {
	Write-Output "bookval-confirm dryrun: games $Games mem $MemLimit"
	$state = if (Test-PairDone "$name.pairlog") { 'done' } else { 'pending' }
	Write-Output "arm $name state $state"
	Write-Output "command template: bin/arena.exe $args_"
	exit 0
}

if (-not $Foreground) {
	$pass = @('-NoProfile', '-File', $PSCommandPath, '-Foreground', '-Games', "$Games", '-MemLimit', $MemLimit, '-OutRoot', $OutRoot, '-Bin', $Bin)
	Start-Process -FilePath pwsh.exe -ArgumentList $pass -WorkingDirectory $repo -WindowStyle Hidden
	Write-Output "bookval-confirm detached, logs under $OutRoot"
	exit 0
}

$binAbs = Join-Path $repo ($Bin -replace '/', '\')
if (-not (Test-Path $binAbs)) {
	$env:GOMEMLIMIT = '4GiB'
	$env:CGO_ENABLED = '1'
	go -C (Join-Path $repo 'playground\arena') build -o $binAbs .
	if ($LASTEXITCODE -ne 0) { Write-Output 'arena build failed'; exit 1 }
}

if (Test-PairDone "$name.pairlog") {
	Write-Output "arm $name already complete, skipping"
	exit 0
}
$rc = Invoke-Arm $name $args_
if ($rc -ne 0) { exit $rc }
Write-Output 'bookval-confirm complete'
exit 0
