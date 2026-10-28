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
$target = 'passgates-fill'
$verdictPattern = '^(hex|square)-passgates: (DONE|FAILED.*)$'
$verdictDonePattern = '^(hex|square)-passgates: DONE$'

$usage = @'
passgates-fill: detached fill batch for the missing master pass gate legs
  arms: hex easy pair (the tc 1 leg reruns under block extension, attempt 1
        archived) plus the full square set, hard, medium, easy at tc index
        1 (2+1) and 2 (3+2), 8 arms total, $Games games per block
  gate law: master W+D aggregated across both seats must reach 0.90 versus
        easy, 0.75 versus medium, 0.60 versus hard, with block extension:
        a wald interval at gateZ over the pooled read decides, lower bound
        at or above the threshold passes, upper bound below it fails, a
        straddle extends one block, 3 blocks max, then the point rule
  frozen-binary law: the batch builds bin/arena.exe as a compile probe,
        freezes to bin/arena-freeze.exe only if missing, every arm runs the
        freeze copy so all games play on one identical build
  marker $OutRoot/passgates-fill.status is rewritten on every terminal
        transition: running <arm> | all-done | failed <arm> <verdict detail> |
        crashed <arm> exit <n> | crashed <arm> exit 0 no-verdict |
        failed no-arena-binary passgates-fill | failed build passgates-fill exit <n>
  the marker never feeds the ladder chain: hex-passgates.status is neither
        read nor written, so the hex-ladder poller stays dormant
  logs under $OutRoot/<arm>.log, every engine invocation runs under
        GOMEMLIMIT $MemLimit
  relaunching resumes: DONE arms skip, FAILED arms are recorded and the batch
        continues to the next leg so every table cell gets its evidence, an
        arm whose arena is already live is adopted and waited on, never
        double-launched; adopted and dead-predecessor arms merge their
        redirected output into the arm log before the verdict check; a failed
        arena build is a loud failure, never a loop; the terminal marker is
        all-done, or done-with-failures naming every failed arm, exit 2
'@

if ($Help) { Write-Output $usage; exit 0 }

$arms = @()
foreach ($tc in 1, 2) {
	$arms += @{
		Name = "hex-passgates-easy-tc$tc"
		Args = @('-geometry', 'hex', '-passgate', 'easy', '-tc', "$tc", '-games', "$Games", '-gateMaxBlocks', '3')
		Match = "-geometry hex -passgate easy -tc $tc"
	}
}
foreach ($opp in 'hard', 'medium', 'easy') {
	foreach ($tc in 1, 2) {
		$arms += @{
			Name = "square-passgates-$opp-tc$tc"
			Args = @('-geometry', 'square', '-passgate', $opp, '-tc', "$tc", '-games', "$Games", '-gateMaxBlocks', '3')
			Match = "-geometry square -passgate $opp -tc $tc"
		}
	}
}

function Convert-ToRepoPath([string]$rel) { Join-Path $repo ($rel -replace '/', '\') }

function Get-ArmVerdict([string]$name) {
	$log = Convert-ToRepoPath "$OutRoot/$name.log"
	if (-not (Test-Path $log)) { return $null }
	$line = Select-String -Path $log -Pattern $verdictPattern | Select-Object -Last 1
	if ($null -eq $line) { return $null }
	return $line.Line
}

function Test-ArmDone([string]$name) {
	$v = Get-ArmVerdict $name
	return $null -ne $v -and $v -match $verdictDonePattern
}

function Test-ArmFailed([string]$name) {
	$v = Get-ArmVerdict $name
	return $null -ne $v -and $v -match '^(hex|square)-passgates: FAILED'
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
		$verdict = Get-ArmVerdict $arm.Name
		if ($null -ne $verdict) {
			if (Test-ArmDone $arm.Name) { Write-Host "arm $($arm.Name) complete via harvested log"; return $null }
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
	Write-Output "passgates-fill dryrun: games $Games poll ${PollSec}s mem $MemLimit"
	foreach ($arm in $arms) {
		$state = if (Test-ArmDone $arm.Name) { 'done' } elseif (Test-ArmFailed $arm.Name) { 'failed' } else { 'pending' }
		$live = Get-LiveArmProcess $arm.Match
		$adopt = if ($null -ne $live) { " adopting pid $($live.ProcessId)" } else { '' }
		Write-Output "arm $($arm.Name) state $state$adopt"
	}
	Write-Output "command template per arm:"
	foreach ($arm in $arms) {
		Write-Output "  GOMEMLIMIT=$MemLimit bin\arena-freeze.exe $($arm.Args -join ' ') >> $OutRoot/$($arm.Name).log"
	}
	Write-Output "probe before the first arm: go -C playground\arena build -o bin\arena.exe . then freeze to bin\arena-freeze.exe"
	exit 0
}

if (-not $Foreground) {
	$pass = "-NoProfile -File `"$PSCommandPath`" -Foreground -Games $Games -PollSec $PollSec -MemLimit $MemLimit -OutRoot `"$OutRoot`""
	Start-Process -FilePath pwsh.exe -ArgumentList $pass -WorkingDirectory $repo -WindowStyle Hidden
	Write-Output "passgates-fill detached, logs under $OutRoot"
	exit 0
}

$twins = Get-CimInstance Win32_Process -Filter "Name = 'pwsh.exe'" | Where-Object { $_.ProcessId -ne $PID -and $_.CommandLine -and $_.CommandLine.Contains($PSCommandPath) -and $_.CommandLine.Contains('-Foreground') }
if ($twins) { Write-Output "passgates-fill: foreground instance already live, exiting"; exit 3 }

$pollLog = Convert-ToRepoPath "$OutRoot/$target.poll.log"
New-Item -ItemType Directory -Force (Split-Path $pollLog -Parent) | Out-Null
"$(Get-Date -Format s) passgates-fill start" | Add-Content -Path $pollLog

$failures = @()
foreach ($arm in $arms) {
	if (Test-ArmDone $arm.Name) { Write-Output "arm $($arm.Name) already complete, skipping"; continue }
	if (Test-ArmFailed $arm.Name) {
		$verdict = Get-ArmVerdict $arm.Name
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
	Write-Output "passgates-fill complete with $($failures.Count) failed arms"
	exit 2
}
Write-Status 'all-done'
Write-Output 'passgates-fill complete'
exit 0
