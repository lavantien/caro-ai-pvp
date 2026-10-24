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
        run via: bin/arena.exe -geometry hex -ladder -tc <i> -games <n>
  arena contract: the binary plays the round-robin and prints exactly one
        final verdict line:
        hex-ladder: DONE <underpowering note naming the per-edge game count>
        hex-ladder: FAILED edge=<lower>-vs-<higher> tc=<i> wd=<x> n=<games>
        the FAILED line names the inverted edge and its played count
        a nonzero exit or a missing verdict line counts as a crash
  marker $OutRoot/hex-ladder.status is rewritten on every terminal transition:
        running <arm> | all-done | failed <arm> <verdict detail> |
        crashed <arm> exit <n> | crashed <arm> exit 0 no-verdict |
        failed no-arena-binary hex-ladder | failed build hex-ladder exit <n>
  logs under $OutRoot/hex-ladder-<arm>.log, poll trace in $OutRoot/hex-ladder.poll.log
  every engine invocation runs under GOMEMLIMIT $MemLimit
  relaunching resumes: DONE arms skip, FAILED arms re-mark their failure,
        pending arms run; an arm whose arena is already live is adopted and
        waited on, never double-launched; a failed arena build is a loud
        failure, never a loop
'@

if ($Help) { Write-Output $usage; exit 0 }

$arms = @()
foreach ($tc in 1, 2) {
	$arms += @{
		Name = "hex-ladder-tc$tc"
		Args = @('-geometry', 'hex', '-ladder', '-tc', "$tc", '-games', "$Games")
		Match = "-ladder -tc $tc"
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

function Remove-ArenaFreeze([string]$pollLog) {
	foreach ($f in @($arenaFreeze, (Join-Path $repo 'bin\arena.exe~'))) {
		if (Test-Path $f) {
			try {
				Remove-Item $f -Force -Confirm:$false -ErrorAction Stop
				"$(Get-Date -Format s) chain end sweep $(Split-Path $f -Leaf)" | Add-Content -Path $pollLog
			} catch {
				"$(Get-Date -Format s) chain end sweep skip locked $(Split-Path $f -Leaf)" | Add-Content -Path $pollLog
			}
		}
	}
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
	} else {
		if (-not (Ensure-Arena $pollLog)) { return "failed build $target" }
		$raw = "$log.raw"
		$err = "$log.err"
		Set-Content -Path $log -Value "$(Get-Date -Format s) arm $($arm.Name) start: bin\arena-freeze.exe $argText"
		Write-Host "arm $($arm.Name) start, log $log"
		$p = Start-Process -FilePath $arenaFreeze -ArgumentList $arm.Args -WorkingDirectory $repo -WindowStyle Hidden -Wait -PassThru -RedirectStandardOutput $raw -RedirectStandardError $err
		if (Test-Path $raw) { Add-Content -Path $log -Value (Get-Content $raw); Remove-Item $raw -Force -Confirm:$false }
		if (Test-Path $err) { Add-Content -Path $log -Value (Get-Content $err); Remove-Item $err -Force -Confirm:$false }
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
	Write-Output "hex-ladder dryrun: games $Games poll ${PollSec}s mem $MemLimit predecessor $Predecessor go '$GoVerdict'"
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

foreach ($arm in $arms) {
	if (Test-ArmDone $arm.Name) { Write-Output "arm $($arm.Name) already complete, skipping"; continue }
	if (Test-ArmFailed $arm.Name) {
		$verdict = Get-ArmVerdict $arm.Name
		Write-Status "failed $($arm.Name) $verdict"
		Remove-ArenaFreeze $pollLog
		exit 2
	}
	Write-Status "running $($arm.Name)"
	$failure = Invoke-Arm $arm $pollLog
	if ($null -ne $failure) { Write-Status $failure; Remove-ArenaFreeze $pollLog; exit 2 }
}
Write-Status 'all-done'
Remove-ArenaFreeze $pollLog
Write-Output 'hex-ladder complete'
exit 0
