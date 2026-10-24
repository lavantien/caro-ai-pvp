[CmdletBinding()]
param(
	[int]$Games = 20,
	[string]$EvalWeights = '97,122,512,513,2077',
	[string]$EvalBase = 'master',
	[string]$MemLimit = '12GiB',
	[string]$OutRoot = 'playground/arena/out',
	[string]$Bin = 'bin/arena.exe',
	[switch]$Help,
	[switch]$DryRun,
	[switch]$Foreground
)

$ErrorActionPreference = 'Stop'
$repo = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$fittedSeat = "$EvalBase-w$(($EvalWeights -replace ',', '.'))"
$regArgs = "-evalweights $EvalWeights -evalbase $EvalBase"

$usage = @'
evaladopt-arms: detached evalweights adoption chain, fitted-vs-scaffold pairs only
  arms: pairs master vs $fittedSeat at 2+1 and 3+2 ($Games games each,
        pairlog-resumable), floors cut from weight adoption by owner order
        2026-10-24
  every arm carries $regArgs so the derived seat exists in the registry
  logs, artifacts, and the evaladopt-arms.status marker under $OutRoot
  relaunching resumes every arm from its recorded progress
'@

if ($Help) { Write-Output $usage; exit 0 }

function New-PairArm([string]$Name, [int]$Tc) {
	@{
		Name = $Name; Tc = $Tc
		Args = "$regArgs -tc $Tc -games $Games -seats $EvalBase,$fittedSeat -pairOut $OutRoot/$Name.pairlog"
	}
}
$arms = @(
	(New-PairArm 'evalpair-21' 1),
	(New-PairArm 'evalpair-32' 2)
)

function Test-ArmDone([string]$name) {
	$pl = Join-Path $repo (($OutRoot + '/' + $name + '.pairlog') -replace '/', '\')
	if (-not (Test-Path $pl)) { return $false }
	return [bool](Select-String -Path $pl -Pattern '^complete \d+ games' -Quiet)
}

function Write-Status([string]$text) {
	$path = Join-Path $repo (($OutRoot + '/evaladopt-arms.status') -replace '/', '\')
	New-Item -ItemType Directory -Force (Split-Path $path -Parent) | Out-Null
	Set-Content -Path $path -Value $text
	Write-Output "status: $text"
}

function Invoke-Arm($arm) {
	$log = Join-Path $repo (($OutRoot + '/' + $arm.Name + '.log') -replace '/', '\')
	New-Item -ItemType Directory -Force (Split-Path $log -Parent) | Out-Null
	$binAbs = Join-Path $repo ($Bin -replace '/', '\')
	$cmd = "set GOMEMLIMIT=$MemLimit&& `"$binAbs`" $($arm.Args) >>`"$log`" 2>&1"
	if ($DryRun) {
		Write-Host "dryrun arm $($arm.Name): $cmd"
		return 0
	}
	Write-Host "arm $($arm.Name) start, log $log"
	$p = Start-Process -FilePath cmd.exe -ArgumentList '/c', $cmd -WorkingDirectory $repo -WindowStyle Hidden -Wait -PassThru
	if ($p.ExitCode -ne 0) {
		Write-Host "arm $($arm.Name) exited $($p.ExitCode), see $log"
		return $p.ExitCode
	}
	if (-not (Test-ArmDone $arm.Name)) {
		Write-Host "arm $($arm.Name) ran without reaching completion, see $log"
		return 1
	}
	Write-Host "arm $($arm.Name) done"
	return 0
}

if ($DryRun) {
	Write-Output "evaladopt-arms dryrun: games $Games weights $EvalWeights base $EvalBase mem $MemLimit"
	foreach ($arm in $arms) {
		$state = if (Test-ArmDone $arm.Name) { 'done' } else { 'pending' }
		Write-Output "arm $($arm.Name) state $state"
	}
	Write-Output "command template per arm:"
	foreach ($arm in $arms) {
		Write-Output "  bin/arena.exe $($arm.Args)"
	}
	exit 0
}

if (-not $Foreground) {
	$pass = @('-NoProfile', '-File', $PSCommandPath, '-Foreground', "-Games", "$Games", "-EvalWeights", $EvalWeights, "-EvalBase", $EvalBase, "-MemLimit", $MemLimit, "-OutRoot", $OutRoot, "-Bin", $Bin)
	Start-Process -FilePath pwsh.exe -ArgumentList $pass -WorkingDirectory $repo -WindowStyle Hidden
	Write-Output "evaladopt-arms detached, logs under $OutRoot"
	exit 0
}

$binAbs = Join-Path $repo ($Bin -replace '/', '\')
if (-not (Test-Path $binAbs)) {
	$env:GOMEMLIMIT = '4GiB'
	$env:CGO_ENABLED = '1'
	go -C (Join-Path $repo 'playground\arena') build -o $binAbs .
	if ($LASTEXITCODE -ne 0) { Write-Status 'crashed build exit 1'; exit 1 }
}

foreach ($arm in $arms) {
	if (Test-ArmDone $arm.Name) {
		Write-Output "arm $($arm.Name) already complete, skipping"
		continue
	}
	$rc = Invoke-Arm $arm
	if ($rc -ne 0) { Write-Status "crashed $($arm.Name) exit $rc"; exit $rc }
}
Write-Status 'all-done'
Write-Output 'evaladopt-arms complete'
exit 0
