[CmdletBinding()]
param(
	[int]$Games = 20,
	[int]$Tc = 1,
	[string]$EvalWeights = '',
	[string]$MemLimit = '12GiB',
	[string]$OutRoot = 'playground/arena/out',
	[string]$Bin = 'bin/arena.exe',
	[switch]$Help,
	[switch]$DryRun,
	[switch]$Foreground
)

$ErrorActionPreference = 'Stop'
$repo = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$name = 'bookval-cross-21'

$usage = @'
bookval-cross: detached cross-check arm under perturbed settings
  arm 1: arena -tc <tc> -games <n> -seats master,master-cross -pairOut <out>/<name>.pairlog
         master-cross = halved cores and tt, book kept
  arm 2 (only with -EvalWeights a,b,c,d,e): arena -evalweights a,b,c,d,e -evalbase master
         -tc <tc> -games <n>, the perturbed-eval pair against the master base
  switches: -DryRun prints the plan, -Foreground runs inline without detaching
  relaunching resumes every arm from its pairlog
'@

if ($Help) { Write-Output $usage; exit 0 }

if ($EvalWeights -ne '' -and ($EvalWeights.Split(',').Count -ne 5)) {
	Write-Output 'bookval-cross: -EvalWeights wants 5 comma-separated integers'
	exit 1
}

$arms = @(
	@{ Name = $name; Args = "-tc $Tc -games $Games -seats master,master-cross -pairOut $OutRoot/$name.pairlog" }
)
if ($EvalWeights -ne '') {
	$arms += @{ Name = 'bookval-cross-weights'; Args = "-evalweights $EvalWeights -evalbase master -tc $Tc -games $Games -pairOut $OutRoot/bookval-cross-weights.pairlog" }
}

function Test-PairDone([string]$pairlog) {
	$pl = Join-Path $repo (($OutRoot + '/' + $pairlog) -replace '/', '\')
	if (-not (Test-Path $pl)) { return $false }
	return [bool](Select-String -Path $pl -Pattern '^complete \d+ games' -Quiet)
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
	if (-not (Test-PairDone "$($arm.Name).pairlog")) {
		Write-Output "arm $($arm.Name) ran without reaching completion, see $log"
		return 1
	}
	Write-Output "arm $($arm.Name) done"
	return 0
}

if ($DryRun) {
	Write-Output "bookval-cross dryrun: games $Games tc $Tc weights '$EvalWeights' mem $MemLimit"
	foreach ($arm in $arms) {
		$state = if (Test-PairDone "$($arm.Name).pairlog") { 'done' } else { 'pending' }
		Write-Output "arm $($arm.Name) state $state"
		Write-Output "  bin/arena.exe $($arm.Args)"
	}
	exit 0
}

if (-not $Foreground) {
	$pass = @('-NoProfile', '-File', $PSCommandPath, '-Foreground', '-Games', "$Games", '-Tc', "$Tc", '-MemLimit', $MemLimit, '-OutRoot', $OutRoot, '-Bin', $Bin)
	if ($EvalWeights -ne '') { $pass += @('-EvalWeights', $EvalWeights) }
	Start-Process -FilePath pwsh.exe -ArgumentList $pass -WorkingDirectory $repo -WindowStyle Hidden
	Write-Output "bookval-cross detached, logs under $OutRoot"
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
	if (Test-PairDone "$($arm.Name).pairlog") {
		Write-Output "arm $($arm.Name) already complete, skipping"
		continue
	}
	$rc = Invoke-Arm $arm
	if ($rc -ne 0) { exit $rc }
}
Write-Output 'bookval-cross complete'
exit 0
