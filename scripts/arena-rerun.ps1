[CmdletBinding()]
param(
	[int]$Games = 20,
	[int]$PollSec = 60,
	[string]$MemLimit = '12GiB',
	[string]$OutRoot = 'playground/arena/out',
	[switch]$Help,
	[switch]$DryRun,
	[switch]$Foreground
)

$ErrorActionPreference = 'Stop'
$repo = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$target = 'arena-rerun'

$usage = @'
arena-rerun: detached 12-leg master pass gate re-run after the solver audit fixes
  arms: square then hex, hard, medium, easy at tc index 1 (2+1) and 2 (3+2),
        12 legs total, $Games games per block, the table this batch emits is
        the v0.24 close evidence per playground/refaudit/design.md
  gate law: master W+D aggregated across both seats must reach 0.90 versus
        easy, 0.75 versus medium, 0.60 versus hard, with block extension:
        a wald interval at gateZ over the pooled read decides, lower bound
        at or above the threshold passes, upper bound below it fails, a
        straddle extends one block, 3 blocks max, then the point rule
  frozen-binary law: the batch freezes its own HEAD, a stale
        bin/arena-freeze.exe is removed before the first leg whenever no leg
        of this batch holds a verdict and no leg arena is live, every leg
        runs the freeze copy so all games play on one identical build, a
        resume with verdicts present keeps the existing freeze untouched
  marker $OutRoot/arena-rerun.status is rewritten on every terminal
        transition: running <leg> | all-done | done-with-failures <legs and
        verdicts> | crashed <leg> exit <n> | crashed <leg> exit 0
        no-verdict | failed no-arena-binary arena-rerun | failed build
        arena-rerun exit <n> | failed freeze-copy arena-rerun
  summary $OutRoot/arena-rerun.md lands at the terminal transition, one row
        per leg in the square-collapse investigation table format, legs
        without a verdict render as MISSING so the table never hides a hole
  the pre-registered response if square legs still fail: engine fixes only
        inside the capability-removal ladder under the never-degrade law,
        the reference-model tier redesign stays a post-close product option
  logs under $OutRoot/arena-rerun-<geometry>-<opp>-tc<i>.log, poll trace in
        $OutRoot/arena-rerun.poll.log, every engine invocation runs under
        GOMEMLIMIT $MemLimit
  relaunching resumes: DONE legs skip, FAILED legs are recorded and the
        batch continues so every table cell gets its evidence, a leg whose
        arena is already live is adopted and waited on, never
        double-launched; adopted legs merge their redirected output into
        the leg log before the verdict check; a failed arena build is a
        loud failure, never a loop
'@

if ($Help) { Write-Output $usage; exit 0 }

$tcLabels = @{1 = '2+1'; 2 = '3+2'}
$arms = @()
foreach ($geo in 'square', 'hex') {
	foreach ($opp in 'hard', 'medium', 'easy') {
		foreach ($tc in 1, 2) {
			$arms += @{
				Name = "$target-$geo-$opp-tc$tc"
				Geo = $geo
				Opp = $opp
				Tc = $tc
				Args = @('-geometry', $geo, '-passgate', $opp, '-tc', "$tc", '-games', "$Games", '-gateMaxBlocks', '3')
				Match = "-geometry $geo -passgate $opp -tc $tc"
			}
		}
	}
}

function Convert-ToRepoPath([string]$rel) { Join-Path $repo ($rel -replace '/', '\') }

function Get-ArmVerdict($arm) {
	$log = Convert-ToRepoPath "$OutRoot/$($arm.Name).log"
	if (-not (Test-Path $log)) { return $null }
	$line = Select-String -Path $log -Pattern ("^$($arm.Geo)-passgates: (DONE|FAILED.*)$") | Select-Object -Last 1
	if ($null -eq $line) { return $null }
	return $line.Line
}

function Test-ArmDone($arm) {
	$v = Get-ArmVerdict $arm
	return $null -ne $v -and $v -eq "$($arm.Geo)-passgates: DONE"
}

function Test-ArmFailed($arm) {
	$v = Get-ArmVerdict $arm
	return $null -ne $v -and $v -like "$($arm.Geo)-passgates: FAILED*"
}

function Write-Status([string]$text) {
	$path = Convert-ToRepoPath "$OutRoot/$target.status"
	New-Item -ItemType Directory -Force (Split-Path $path -Parent) | Out-Null
	Set-Content -Path $path -Value $text
	Write-Output "status: $text"
}

function Get-LiveArmProcess([string]$match) {
	Get-CimInstance Win32_Process -Filter "Name LIKE 'arena%'" |
		Where-Object { $_.CommandLine -and $_.CommandLine.Contains($repo) -and $_.CommandLine.Contains($match) } |
		Select-Object -First 1
}

function Get-LiveBatchArm {
	foreach ($arm in $arms) {
		$live = Get-LiveArmProcess $arm.Match
		if ($null -ne $live) { return $live }
	}
	return $null
}

$arenaBin = Join-Path $repo 'bin\arena.exe'
$arenaFreeze = Join-Path $repo 'bin\arena-freeze.exe'
$script:arenaBuilt = $false
function Ensure-Arena([string]$pollLog) {
	if ($script:arenaBuilt) { return $true }
	$env:GOMEMLIMIT = '4GiB'
	$env:CGO_ENABLED = '1'
	go -C (Join-Path $repo 'playground\arena') build -o $arenaBin .
	if ($LASTEXITCODE -ne 0) { Write-Status "failed build $target exit $LASTEXITCODE"; "$(Get-Date -Format s) arena build failed exit $LASTEXITCODE" | Add-Content -Path $pollLog; return $false }
	if (-not (Test-Path $arenaBin)) { Write-Status "failed no-arena-binary $target"; "$(Get-Date -Format s) arena binary missing after build" | Add-Content -Path $pollLog; return $false }
	if (-not (Test-Path $arenaFreeze)) {
		Copy-Item -Path $arenaBin -Destination $arenaFreeze -Force
		if ((Test-Path $arenaFreeze) -and ((Get-Item $arenaBin).Length -eq (Get-Item $arenaFreeze).Length)) {
			"$(Get-Date -Format s) arena frozen: bin\arena-freeze.exe $((Get-Item $arenaFreeze).Length) bytes" | Add-Content -Path $pollLog
		} else {
			if (Test-Path $arenaFreeze) { Remove-Item $arenaFreeze -Force -Confirm:$false }
			Write-Status "failed freeze-copy $target"
			"$(Get-Date -Format s) freeze copy size mismatch" | Add-Content -Path $pollLog
			return $false
		}
	}
	$env:GOMEMLIMIT = $MemLimit
	$script:arenaBuilt = $true
	return $true
}

function Merge-ArmLog([string]$name) {
	$log = Convert-ToRepoPath "$OutRoot/$name.log"
	$raw = "$log.raw"
	$err = "$log.err"
	if (Test-Path $raw) { Add-Content -Path $log -Value (Get-Content $raw); Remove-Item $raw -Force -Confirm:$false }
	if (Test-Path $err) { Add-Content -Path $log -Value (Get-Content $err); Remove-Item $err -Force -Confirm:$false }
}

function Invoke-Arm($arm, $pollLog) {
	$log = Convert-ToRepoPath "$OutRoot/$($arm.Name).log"
	New-Item -ItemType Directory -Force (Split-Path $log -Parent) | Out-Null
	$argText = $arm.Args -join ' '
	$live = Get-LiveArmProcess $arm.Match
	if ($null -ne $live) {
		Write-Host "arm $($arm.Name) adopting live arena pid $($live.ProcessId), waiting"
		"$(Get-Date -Format s) arm $($arm.Name) adopt pid $($live.ProcessId)" | Add-Content -Path $pollLog
		try { Wait-Process -Id $live.ProcessId -ErrorAction Stop } catch {}
		Merge-ArmLog $arm.Name
		Add-Content -Path $log -Value "$(Get-Date -Format s) arm $($arm.Name) adopted"
	} else {
		Merge-ArmLog $arm.Name
		$verdict = Get-ArmVerdict $arm
		if ($null -ne $verdict) {
			if (Test-ArmDone $arm) { Write-Host "arm $($arm.Name) complete via harvested log"; return $null }
			return "failed $($arm.Name) $verdict"
		}
		if (-not (Ensure-Arena $pollLog)) { return "failed build $target" }
		$raw = "$log.raw"
		$err = "$log.err"
		Set-Content -Path $log -Value "$(Get-Date -Format s) arm $($arm.Name) start: bin\arena-freeze.exe $argText"
		Write-Host "arm $($arm.Name) start, log $log"
		$p = Start-Process -FilePath $arenaFreeze -ArgumentList $arm.Args -WorkingDirectory $repo -WindowStyle Hidden -Wait -PassThru -RedirectStandardOutput $raw -RedirectStandardError $err
		Merge-ArmLog $arm.Name
		Add-Content -Path $log -Value "$(Get-Date -Format s) arm $($arm.Name) exit $($p.ExitCode)"
		if ($p.ExitCode -ne 0) {
			Write-Host "arm $($arm.Name) exited $($p.ExitCode), see $log"
			return "crashed $($arm.Name) exit $($p.ExitCode)"
		}
	}
	$verdict = Get-ArmVerdict $arm
	if ($null -eq $verdict) {
		Write-Host "arm $($arm.Name) produced no verdict line, see $log"
		return "crashed $($arm.Name) exit 0 no-verdict"
	}
	if (-not (Test-ArmDone $arm)) {
		return "failed $($arm.Name) $verdict"
	}
	Write-Host "arm $($arm.Name) done"
	return $null
}

function Get-ArmRow($arm) {
	$label = "$($arm.Geo) $($arm.Opp) $($tcLabels[$arm.Tc])"
	$log = Convert-ToRepoPath "$OutRoot/$($arm.Name).log"
	$verdict = Get-ArmVerdict $arm
	$extN = ''
	$extWd = ''
	$extLb = ''
	$extUb = ''
	if (Test-Path $log) {
		foreach ($line in (Get-Content $log)) {
			if ($line -match '^extend: block \d+/\d+ n=(\d+) wd=([\d.]+) lb=([\d.]+) ub=([\d.]+)$') {
				$extN = $matches[1]; $extWd = $matches[2]; $extLb = $matches[3]; $extUb = $matches[4]
			}
		}
	}
	if ($null -eq $verdict) { return @{Done = $false; Row = "| $label | MISSING | | | | | |" } }
	if (Test-ArmDone $arm) { return @{Done = $true; Row = "| $label | PASS | $extN | $extWd | $extLb | $extUb | |" } }
	if ($verdict -match 'wd=([\d.]+) threshold=([\d.]+) n=(\d+) lb=([\d.]+) ub=([\d.]+) cap=(true|false)') {
		return @{Done = $false; Row = "| $label | FAIL | $($matches[3]) | $($matches[1]) | $($matches[4]) | $($matches[5]) | $($matches[6]) |" }
	}
	return @{Done = $false; Row = "| $label | FAIL | $extN | $extWd | $extLb | $extUb | |" }
}

function Write-Summary {
	$rows = @()
	$passed = 0
	foreach ($arm in $arms) {
		$r = Get-ArmRow $arm
		if ($r.Done) { $passed++ }
		$rows += $r.Row
	}
	$freezeNote = if (Test-Path $arenaFreeze) { "one frozen binary bin\arena-freeze.exe $((Get-Item $arenaFreeze).Length) bytes" } else { 'no frozen binary on disk' }
	$lines = @(
		'# Arena 12-leg re-run',
		'',
		"Master pass gates over both seats after the solver audit fixes, $freezeNote. Thresholds: hard 0.60, medium 0.75, easy 0.90, wald interval at gateZ 1.96 with block extension, then the point rule.",
		'',
		'| leg | verdict | n | wd | lb | ub | cap |',
		'|---|---|---|---|---|---|---|'
	) + $rows + @(
		'',
		"$passed of 12 legs pass. A still-failing square leg gets engine fixes only inside the capability-removal ladder under the never-degrade law, the reference-model tier redesign stays a post-close product option."
	)
	$path = Convert-ToRepoPath "$OutRoot/$target.md"
	Set-Content -Path $path -Value ($lines -join "`r`n")
	Write-Output "summary: $path"
}

if ($DryRun) {
	Write-Output "arena-rerun dryrun: games $Games poll ${PollSec}s mem $MemLimit"
	$started = $false
	foreach ($arm in $arms) {
		$state = if (Test-ArmDone $arm) { 'done' } elseif (Test-ArmFailed $arm) { 'failed' } else { 'pending' }
		if ($state -ne 'pending') { $started = $true }
		$live = Get-LiveArmProcess $arm.Match
		$adopt = if ($null -ne $live) { " adopting pid $($live.ProcessId)" } else { '' }
		Write-Output "arm $($arm.Name) state $state$adopt"
	}
	$freezePlan = if ($started -or -not (Test-Path $arenaFreeze)) { 'build and freeze a fresh bin\arena-freeze.exe' } else { "remove the stale bin\arena-freeze.exe $((Get-Item $arenaFreeze).Length) bytes, then build and freeze fresh" }
	Write-Output "freeze plan: $freezePlan"
	Write-Output "command template per arm:"
	foreach ($arm in $arms) {
		Write-Output "  GOMEMLIMIT=$MemLimit bin\arena-freeze.exe $($arm.Args -join ' ') >> $OutRoot/$($arm.Name).log"
	}
	Write-Output "probe before the first arm: go -C playground\arena build -o bin\arena.exe . then freeze to bin\arena-freeze.exe"
	Write-Output "summary at the terminal transition: $OutRoot/$target.md"
	exit 0
}

if (-not $Foreground) {
	$pass = "-NoProfile -File `"$PSCommandPath`" -Foreground -Games $Games -PollSec $PollSec -MemLimit $MemLimit -OutRoot `"$OutRoot`""
	Start-Process -FilePath pwsh.exe -ArgumentList $pass -WorkingDirectory $repo -WindowStyle Hidden
	Write-Output "arena-rerun detached, logs under $OutRoot"
	exit 0
}

$twins = Get-CimInstance Win32_Process -Filter "Name = 'pwsh.exe'" | Where-Object { $_.ProcessId -ne $PID -and $_.CommandLine -and $_.CommandLine.Contains($PSCommandPath) -and $_.CommandLine.Contains('-Foreground') }
if ($twins) { Write-Output "arena-rerun: foreground instance already live, exiting"; exit 3 }

$pollLog = Convert-ToRepoPath "$OutRoot/$target.poll.log"
New-Item -ItemType Directory -Force (Split-Path $pollLog -Parent) | Out-Null
"$(Get-Date -Format s) arena-rerun start" | Add-Content -Path $pollLog

$batchStarted = $false
foreach ($arm in $arms) {
	if ((Test-ArmDone $arm) -or (Test-ArmFailed $arm)) { $batchStarted = $true; break }
}
if (-not $batchStarted -and (Test-Path $arenaFreeze) -and $null -eq (Get-LiveBatchArm)) {
	Remove-Item -Path $arenaFreeze -Force -Confirm:$false
	"$(Get-Date -Format s) stale freeze removed, this batch freezes its own HEAD" | Add-Content -Path $pollLog
}

$failures = @()
foreach ($arm in $arms) {
	if (Test-ArmDone $arm) { Write-Output "arm $($arm.Name) already complete, skipping"; continue }
	if (Test-ArmFailed $arm) {
		$verdict = Get-ArmVerdict $arm
		Write-Output "arm $($arm.Name) already failed, recording"
		$failures += "$($arm.Name) $verdict"
		continue
	}
	Write-Status "running $($arm.Name)"
	$failure = Invoke-Arm $arm $pollLog
	if ($null -ne $failure) {
		Write-Status $failure
		$failures += $failure
		continue
	}
}
if ($failures.Count -gt 0) {
	Write-Status "done-with-failures $($failures -join '; ')"
	Write-Summary
	Write-Output "arena-rerun complete with $($failures.Count) failed arms"
	exit 2
}
Write-Status 'all-done'
Write-Summary
Write-Output 'arena-rerun complete'
exit 0
