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
  launches: bin/arena.exe <regArgs> -tc <i> -games <n> -seats master,<fitted>
        -pairOut $OutRoot/<arm>.pairlog, under GOMEMLIMIT $MemLimit
  logs, artifacts, and the evaladopt-arms.status marker under $OutRoot
  relaunching resumes: complete pairlogs skip, partial pairlogs continue from
        their recorded games; an arm whose arena is already live is adopted
        and waited on, never double-launched
'@

if ($Help) { Write-Output $usage; exit 0 }

function New-PairArm([string]$Name, [int]$Tc) {
	@{
		Name = $Name; Tc = $Tc
		Args = "$regArgs -tc $Tc -games $Games -seats $EvalBase,$fittedSeat -pairOut $OutRoot/$Name.pairlog"
		Match = "$OutRoot/$Name.pairlog"
	}
}
$arms = @(
	(New-PairArm 'evalpair-21' 1),
	(New-PairArm 'evalpair-32' 2)
)

function Convert-ToRepoPath([string]$rel) { Join-Path $repo ($rel -replace '/', '\') }

function Test-ArmDone([string]$name) {
	$pl = Convert-ToRepoPath "$OutRoot/$name.pairlog"
	if (-not (Test-Path $pl)) { return $false }
	return [bool](Select-String -Path $pl -Pattern '^complete \d+ games' -Quiet)
}

function Write-Status([string]$text) {
	$path = Convert-ToRepoPath "$OutRoot/evaladopt-arms.status"
	New-Item -ItemType Directory -Force (Split-Path $path -Parent) | Out-Null
	Set-Content -Path $path -Value $text
	Write-Output "status: $text"
}

function Get-LiveArmProcess([string]$match) {
	Get-CimInstance Win32_Process -Filter "Name LIKE 'arena%'" |
		Where-Object { $_.CommandLine -and $_.CommandLine.Contains($repo) -and $_.CommandLine.Contains($match) } |
		Select-Object -First 1
}

function Invoke-Arm($arm) {
	$log = Convert-ToRepoPath "$OutRoot/$($arm.Name).log"
	New-Item -ItemType Directory -Force (Split-Path $log -Parent) | Out-Null
	if ($DryRun) {
		Write-Host "dryrun arm $($arm.Name): GOMEMLIMIT=$MemLimit `"$Bin`" $($arm.Args)"
		return 0
	}
	$binAbs = Convert-ToRepoPath $Bin
	$live = Get-LiveArmProcess $arm.Match
	if ($null -ne $live) {
		Write-Host "arm $($arm.Name) adopting live arena pid $($live.ProcessId), waiting"
		try { Wait-Process -Id $live.ProcessId -ErrorAction Stop } catch {}
	} else {
		$raw = "$log.raw"
		$err = "$log.err"
		Set-Content -Path $log -Value "$(Get-Date -Format s) arm $($arm.Name) start: bin\arena-freeze.exe $($arm.Args)"
		Write-Host "arm $($arm.Name) start, log $log"
		$p = Start-Process -FilePath $arenaFreeze -ArgumentList $arm.Args -WorkingDirectory $repo -WindowStyle Hidden -Wait -PassThru -RedirectStandardOutput $raw -RedirectStandardError $err
		if (Test-Path $raw) { Add-Content -Path $log -Value (Get-Content $raw); Remove-Item $raw -Force -Confirm:$false }
		if (Test-Path $err) { Add-Content -Path $log -Value (Get-Content $err); Remove-Item $err -Force -Confirm:$false }
		Add-Content -Path $log -Value "$(Get-Date -Format s) arm $($arm.Name) exit $($p.ExitCode)"
		if ($p.ExitCode -ne 0) {
			Write-Host "arm $($arm.Name) exited $($p.ExitCode), see $log"
			return $p.ExitCode
		}
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
		$live = Get-LiveArmProcess $arm.Match
		$adopt = if ($null -ne $live) { " adopting pid $($live.ProcessId)" } else { '' }
		Write-Output "arm $($arm.Name) state $state$adopt"
	}
	Write-Output "command template per arm:"
	foreach ($arm in $arms) {
		Write-Output "  GOMEMLIMIT=$MemLimit bin\arena-freeze.exe $($arm.Args)"
	}
	exit 0
}

if (-not $Foreground) {
	$pass = @('-NoProfile', '-File', $PSCommandPath, '-Foreground', "-Games", "$Games", "-EvalWeights", $EvalWeights, "-EvalBase", $EvalBase, "-MemLimit", $MemLimit, "-OutRoot", $OutRoot, "-Bin", $Bin)
	Start-Process -FilePath pwsh.exe -ArgumentList $pass -WorkingDirectory $repo -WindowStyle Hidden
	Write-Output "evaladopt-arms detached, logs under $OutRoot"
	exit 0
}

$binAbs = Convert-ToRepoPath $Bin
$arenaFreeze = Join-Path $repo 'bin\arena-freeze.exe'
$chainLog = Convert-ToRepoPath "$OutRoot/evaladopt-arms.log"
New-Item -ItemType Directory -Force (Split-Path $chainLog -Parent) | Out-Null
if (-not (Test-Path $binAbs)) {
	$env:GOMEMLIMIT = '4GiB'
	$env:CGO_ENABLED = '1'
	go -C (Join-Path $repo 'playground\arena') build -o $binAbs .
	if ($LASTEXITCODE -ne 0) { Write-Status 'crashed build exit 1'; exit 1 }
}
if (-not (Test-Path $arenaFreeze)) {
	Copy-Item -Path $binAbs -Destination $arenaFreeze -Force
	if (-not (Test-Path $arenaFreeze) -or ((Get-Item $binAbs).Length -ne (Get-Item $arenaFreeze).Length)) {
		if (Test-Path $arenaFreeze) { Remove-Item $arenaFreeze -Force -Confirm:$false }
		Write-Status 'crashed freeze-copy size mismatch'
		"$(Get-Date -Format s) freeze copy size mismatch" | Add-Content -Path $chainLog
		exit 1
	}
	"$(Get-Date -Format s) arena frozen: bin\arena-freeze.exe $((Get-Item $arenaFreeze).Length) bytes" | Add-Content -Path $chainLog
}
$env:GOMEMLIMIT = $MemLimit

function Remove-ArenaFreeze {
	foreach ($f in @($arenaFreeze, (Join-Path $repo 'bin\arena.exe~'))) {
		if (Test-Path $f) {
			try {
				Remove-Item $f -Force -Confirm:$false -ErrorAction Stop
				"$(Get-Date -Format s) chain end sweep $(Split-Path $f -Leaf)" | Add-Content -Path $chainLog
			} catch {
				"$(Get-Date -Format s) chain end sweep skip locked $(Split-Path $f -Leaf)" | Add-Content -Path $chainLog
			}
		}
	}
}

foreach ($arm in $arms) {
	if (Test-ArmDone $arm.Name) {
		Write-Output "arm $($arm.Name) already complete, skipping"
		continue
	}
	$rc = Invoke-Arm $arm
	if ($rc -ne 0) { Write-Status "crashed $($arm.Name) exit $rc"; Remove-ArenaFreeze; exit $rc }
}
Write-Status 'all-done'
Remove-ArenaFreeze
Write-Output 'evaladopt-arms complete'
exit 0
