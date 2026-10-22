[CmdletBinding()]
param(
	[int]$Games = 20,
	[int]$FloorDirs = 5,
	[int]$FloorMaxBlocks = 3,
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
evaladopt-arms: detached evalweights adoption chain, floors then fitted-vs-scaffold pairs
  arms: floor $fittedSeat at 2+1 and 3+2, floor master at 2+1 and 3+2
        ($FloorDirs dirs of $Games games each, auto-extends to $FloorMaxBlocks blocks),
        pairs master vs $fittedSeat at 2+1 and 3+2 ($Games games each, pairlog-resumable)
  every arm carries $regArgs so the derived seat exists in the registry
  logs, artifacts, and the evaladopt-arms.status marker under $OutRoot
  relaunching resumes every arm from its recorded progress
'@

if ($Help) { Write-Output $usage; exit 0 }

function New-FloorArm([string]$Name, [int]$Tc, [string]$Seat, [string]$Extra) {
	$pre = if ($Extra) { "$Extra " } else { '' }
	@{
		Kind = 'floor'; Name = $Name; Tc = $Tc; Seat = $Seat
		Args = "${pre}-floor $Name -floorSeat $Seat -tc $Tc -games $Games -floorDirs $FloorDirs -floorMaxBlocks $FloorMaxBlocks"
	}
}
function New-PairArm([string]$Name, [int]$Tc) {
	@{
		Kind = 'pair'; Name = $Name; Tc = $Tc
		Args = "$regArgs -tc $Tc -games $Games -seats $EvalBase,$fittedSeat -pairOut $OutRoot/$Name.pairlog"
	}
}
$arms = @(
	(New-FloorArm 'evalfloor-fitted-21' 1 $fittedSeat $regArgs),
	(New-FloorArm 'evalfloor-fitted-32' 2 $fittedSeat $regArgs),
	(New-FloorArm 'evalfloor-scaffold-21' 1 $EvalBase ''),
	(New-FloorArm 'evalfloor-scaffold-32' 2 $EvalBase ''),
	(New-PairArm 'evalpair-21' 1),
	(New-PairArm 'evalpair-32' 2)
)

function Get-FloorState([string]$name) {
	$md = Join-Path $repo (($OutRoot + '/' + $name + '.md') -replace '/', '\')
	if (-not (Test-Path $md)) { return 'absent' }
	$line = Select-String -Path $md -Pattern '^extend: ' | Select-Object -First 1
	if ($null -eq $line) { return 'absent' }
	if ($line.Line -match 'not needed') { return 'done' }
	if ($line.Line -match 'blocked') { return 'blocked' }
	return 'extend'
}
function Test-PairDone([string]$name) {
	$pl = Join-Path $repo (($OutRoot + '/' + $name + '.pairlog') -replace '/', '\')
	if (-not (Test-Path $pl)) { return $false }
	return [bool](Select-String -Path $pl -Pattern '^complete \d+ games' -Quiet)
}
function Test-ArmDone($arm) {
	if ($arm.Kind -eq 'floor') { return (Get-FloorState $arm.Name) -eq 'done' }
	return Test-PairDone $arm.Name
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
		Write-Output "dryrun arm $($arm.Name): $cmd"
		return 0
	}
	Write-Output "arm $($arm.Name) start, log $log"
	$p = Start-Process -FilePath cmd.exe -ArgumentList '/c', $cmd -WorkingDirectory $repo -WindowStyle Hidden -Wait -PassThru
	if ($p.ExitCode -ne 0) {
		Write-Output "arm $($arm.Name) exited $($p.ExitCode), see $log"
		return $p.ExitCode
	}
	if (-not (Test-ArmDone $arm)) {
		Write-Output "arm $($arm.Name) ran without reaching completion, see $log"
		return 1
	}
	Write-Output "arm $($arm.Name) done"
	return 0
}

if ($DryRun) {
	Write-Output "evaladopt-arms dryrun: games $Games floorDirs $FloorDirs weights $EvalWeights base $EvalBase mem $MemLimit"
	foreach ($arm in $arms) {
		$state = if (Test-ArmDone $arm) { 'done' } else { 'pending' }
		Write-Output "arm $($arm.Name) state $state"
	}
	Write-Output "command template per arm:"
	foreach ($arm in $arms) {
		Write-Output "  bin/arena.exe $($arm.Args)"
	}
	exit 0
}

if (-not $Foreground) {
	$pass = @('-NoProfile', '-File', $PSCommandPath, '-Foreground', "-Games", "$Games", "-FloorDirs", "$FloorDirs", "-FloorMaxBlocks", "$FloorMaxBlocks", "-EvalWeights", $EvalWeights, "-EvalBase", $EvalBase, "-MemLimit", $MemLimit, "-OutRoot", $OutRoot, "-Bin", $Bin)
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
	if (Test-ArmDone $arm) {
		Write-Output "arm $($arm.Name) already complete, skipping"
		continue
	}
	if ($arm.Kind -eq 'floor') {
		$state = ''
		for ($attempt = 1; $attempt -le $FloorMaxBlocks; $attempt++) {
			$state = Get-FloorState $arm.Name
			if ($state -eq 'done') { break }
			if ($state -eq 'blocked') { break }
			$rc = Invoke-Arm $arm
			if ($rc -ne 0) { Write-Status "crashed $($arm.Name) exit $rc"; exit $rc }
		}
		$state = Get-FloorState $arm.Name
		if ($state -eq 'blocked') {
			Write-Output "arm $($arm.Name) floor blocked at the extension cap without covering 0.5"
			Write-Status "floor-failed $($arm.Name)"
			exit 2
		}
		if ($state -ne 'done') {
			Write-Output "arm $($arm.Name) floor never reached a covering interval"
			Write-Status "floor-failed $($arm.Name)"
			exit 2
		}
		Write-Output "arm $($arm.Name) done"
		continue
	}
	$rc = Invoke-Arm $arm
	if ($rc -ne 0) { Write-Status "crashed $($arm.Name) exit $rc"; exit $rc }
}
Write-Status 'all-done'
Write-Output 'evaladopt-arms complete'
exit 0
