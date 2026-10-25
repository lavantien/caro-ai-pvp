[CmdletBinding()]
param(
	[int]$Games = 20,
	[int]$PollSec = 60,
	[string]$MemLimit = '12GiB',
	[string]$OutRoot = 'playground/arena/out',
	[string]$Predecessor = 'evaladopt-terminal',
	[string]$GoVerdict = 're-registered 2026-10-24',
	[switch]$Help,
	[switch]$DryRun,
	[switch]$Foreground
)

$ErrorActionPreference = 'Stop'
$repo = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$target = 'hex-noise'
$verdictPrefix = 'hex-noise:'

$usage = @'
hex-noise: detached hex noise floor chain, master vs master on the hex board
  waits for $GoVerdict inside $OutRoot/$Predecessor.status before starting
  arms: hex noise floor at tc index 1 (2+1) and 2 (3+2), $Games games each,
        run via: bin/arena.exe -geometry hex -floor hex-noise-tc<i> -floorSeat
        master-nobook -tc <i> -games <n> -floorDirs 5 -floorBlockDirs 5
        -floorMaxBlocks 3 -floorVerdict hex-noise
  arena contract: the binary plays the games, computes the Wald CI band on the
        observed W+D, and prints exactly one final verdict line:
        hex-noise: DONE
        hex-noise: FAILED <reason naming what failed to measure>
        a nonzero exit or a missing verdict line counts as a crash
  marker $OutRoot/hex-noise.status is rewritten on every terminal transition:
        running <arm> | all-done | failed <arm> <verdict detail> |
        crashed <arm> exit <n> | crashed <arm> exit 0 no-verdict |
        failed no-arena-binary hex-noise | failed build hex-noise exit <n>
  logs under $OutRoot/hex-noise-<arm>.log, poll trace in $OutRoot/hex-noise.poll.log
  every engine invocation runs under GOMEMLIMIT $MemLimit
  relaunching resumes: DONE arms skip, FAILED arms re-mark their failure,
        pending arms run; an arm whose arena is already live is adopted and
        waited on, never double-launched; adopted and dead-predecessor arms
        merge their redirected output into the arm log before the verdict
        check; a failed arena build is a loud failure, never a loop
'@

if ($Help) { Write-Output $usage; exit 0 }

$arenaArgs = @{
	1 = @('-geometry', 'hex', '-floor', 'hex-noise-tc1', '-floorSeat', 'master-nobook', '-tc', '1', '-games', "$Games", '-floorDirs', '5', '-floorBlockDirs', '5', '-floorMaxBlocks', '3', '-floorVerdict', 'hex-noise')
	2 = @('-geometry', 'hex', '-floor', 'hex-noise-tc2', '-floorSeat', 'master-nobook', '-tc', '2', '-games', "$Games", '-floorDirs', '5', '-floorBlockDirs', '5', '-floorMaxBlocks', '3', '-floorVerdict', 'hex-noise')
}

$arms = @()
foreach ($tc in 1, 2) {
	$arms += @{
		Name = "hex-noise-tc$tc"
		Args = $arenaArgs[$tc]
		Match = "-floor hex-noise-tc$tc"
	}
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
	Write-Output "hex-noise dryrun: games $Games poll ${PollSec}s mem $MemLimit predecessor $Predecessor go '$GoVerdict'"
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
	$pass = "-NoProfile -File `"$PSCommandPath`" -Foreground -Games $Games -PollSec $PollSec -MemLimit $MemLimit -OutRoot `"$OutRoot`" -Predecessor `"$Predecessor`" -GoVerdict `"$GoVerdict`""
	Start-Process -FilePath pwsh.exe -ArgumentList $pass -WorkingDirectory $repo -WindowStyle Hidden
	Write-Output "hex-noise detached, logs under $OutRoot"
	exit 0
}

$twins = Get-CimInstance Win32_Process -Filter "Name = 'pwsh.exe'" | Where-Object { $_.ProcessId -ne $PID -and $_.CommandLine -and $_.CommandLine.Contains($PSCommandPath) -and $_.CommandLine.Contains('-Foreground') }
if ($twins) { Write-Output "hex-noise: foreground instance already live, exiting"; exit 3 }

$predStatus = Convert-ToRepoPath "$OutRoot/$Predecessor.status"
$pollLog = Convert-ToRepoPath "$OutRoot/$target.poll.log"
New-Item -ItemType Directory -Force (Split-Path $pollLog -Parent) | Out-Null
"$(Get-Date -Format s) hex-noise start, waiting for '$GoVerdict' in $Predecessor.status" | Add-Content -Path $pollLog
while ($true) {
	if ((Test-Path $predStatus) -and ((Get-Content $predStatus -Raw) -match [regex]::Escape($GoVerdict))) { break }
	"$(Get-Date -Format s) waiting for '$GoVerdict' in $Predecessor.status" | Add-Content -Path $pollLog
	Start-Sleep -Seconds $PollSec
}
"$(Get-Date -Format s) $Predecessor.status reached '$GoVerdict'" | Add-Content -Path $pollLog

foreach ($arm in $arms) {
	if (Test-ArmDone $arm.Name) { Write-Output "arm $($arm.Name) already complete, skipping"; continue }
	if (Test-ArmFailed $arm.Name) {
		$verdict = Get-ArmVerdict $arm.Name
		Write-Status "failed $($arm.Name) $verdict"
		exit 2
	}
	Write-Status "running $($arm.Name)"
	$failure = Invoke-Arm $arm $pollLog
	if ($null -ne $failure) { Write-Status $failure; exit 2 }
}
Write-Status 'all-done'
Write-Output 'hex-noise complete'
exit 0
