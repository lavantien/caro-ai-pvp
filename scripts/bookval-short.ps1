[CmdletBinding()]
param(
	[int]$Games = 20,
	[int]$FloorDirs = 5,
	[int]$FloorMaxBlocks = 3,
	[string]$MemLimit = '12GiB',
	[string]$OutRoot = 'playground/arena/out',
	[string]$Bin = 'bin/arena.exe',
	[switch]$Help,
	[switch]$DryRun,
	[switch]$Foreground
)

$ErrorActionPreference = 'Stop'
$repo = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path

$usage = @'
bookval-short: detached book validation chain, floors then book-vs-nobook pairs
  arms: floor master-nobook at 2+1 and 3+2 ($FloorDirs dirs of $Games games, auto-extends to $FloorMaxBlocks blocks),
        pairs master vs master-nobook at 1+0, 2+1, 3+2 ($Games games each, pairlog-resumable)
  logs and resume records under $OutRoot, stdout per arm appended to <arm>.log
  switches: -DryRun prints the plan, -Foreground runs inline without detaching
  relaunching resumes every arm from its recorded progress
'@

if ($Help) { Write-Output $usage; exit 0 }

function New-FloorArm([string]$Name, [int]$Tc) {
	@{
		Kind = 'floor'; Name = $Name; Tc = $Tc
		Args = "-floor $Name -floorSeat master-nobook -tc $Tc -games $Games -floorDirs $FloorDirs -floorMaxBlocks $FloorMaxBlocks"
	}
}
function New-PairArm([string]$Name, [int]$Tc, [string]$SeatB) {
	@{
		Kind = 'pair'; Name = $Name; Tc = $Tc
		Args = "-tc $Tc -games $Games -seats master,$SeatB -pairOut $OutRoot/$Name.pairlog"
	}
}
$arms = @(
	(New-FloorArm 'bookval-floor-21' 1),
	(New-FloorArm 'bookval-floor-32' 2),
	(New-PairArm 'bookval-pair-10' 0 'master-nobook'),
	(New-PairArm 'bookval-pair-21' 1 'master-nobook'),
	(New-PairArm 'bookval-pair-32' 2 'master-nobook')
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
function Test-FloorDone([string]$name) {
	return (Get-FloorState $name) -eq 'done'
}
function Test-PairDone([string]$name) {
	$pl = Join-Path $repo (($OutRoot + '/' + $name + '.pairlog') -replace '/', '\')
	if (-not (Test-Path $pl)) { return $false }
	return [bool](Select-String -Path $pl -Pattern '^complete \d+ games' -Quiet)
}
function Test-ArmDone($arm) {
	if ($arm.Kind -eq 'floor') { return Test-FloorDone $arm.Name }
	return Test-PairDone $arm.Name
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
	if (-not (Test-ArmDone $arm)) {
		Write-Host "arm $($arm.Name) ran without reaching completion, see $log"
		return 1
	}
	Write-Host "arm $($arm.Name) done"
	return 0
}

if ($DryRun) {
	Write-Output "bookval-short dryrun: games $Games floorDirs $FloorDirs mem $MemLimit"
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
	$pass = @('-NoProfile', '-File', $PSCommandPath, '-Foreground', "-Games", "$Games", "-FloorDirs", "$FloorDirs", "-FloorMaxBlocks", "$FloorMaxBlocks", "-MemLimit", $MemLimit, "-OutRoot", $OutRoot, "-Bin", $Bin)
	Start-Process -FilePath pwsh.exe -ArgumentList $pass -WorkingDirectory $repo -WindowStyle Hidden
	Write-Output "bookval-short detached, logs under $OutRoot"
	exit 0
}

$binAbs = Join-Path $repo ($Bin -replace '/', '\')
if (-not (Test-Path $binAbs)) {
	$env:GOMEMLIMIT = '4GiB'
	$env:CGO_ENABLED = '1'
	go -C (Join-Path $repo 'playground\arena') build -o $binAbs .
	if ($LASTEXITCODE -ne 0) { Write-Output 'arena build failed'; exit 1 }
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
			if ($rc -ne 0) { exit $rc }
		}
		$state = Get-FloorState $arm.Name
		if ($state -eq 'blocked') {
			Write-Output "arm $($arm.Name) floor blocked at the extension cap without covering 0.5"
			exit 2
		}
		if ($state -ne 'done') {
			Write-Output "arm $($arm.Name) floor never reached a covering interval"
			exit 2
		}
		Write-Output "arm $($arm.Name) done"
		continue
	}
	$rc = Invoke-Arm $arm
	if ($rc -ne 0) { exit $rc }
}
Write-Output 'bookval-short complete'
exit 0
