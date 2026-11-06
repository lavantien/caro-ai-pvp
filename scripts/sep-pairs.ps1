[CmdletBinding()]
param(
	[int]$Games = 20,
	[string]$MemLimit = '12GiB',
	[string]$OutRoot = 'playground/arena/out',
	[switch]$Help,
	[switch]$DryRun,
	[switch]$Foreground
)

$ErrorActionPreference = 'Stop'
$repo = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$target = 'sep-pairs'
. (Join-Path $PSScriptRoot 'arena-launch.ps1')

$usage = @'
sep-pairs: detached 2-arm diagnostic separation for the regate3 square
  medium 2+1 collapse, the channels live in .scratch/mediumpostmortem.md
  arm 1 master-noponder versus medium isolates ponder adoption toxicity,
  a recovered fraction versus the regate3 0.500 leg fingers the adoption
  channel
  arm 2 master-nobook versus medium isolates the blue-seat opening
  deficit, a recovered blue fraction fingers the book opening channel
  both arms square geometry, tc index 1 (2+1), $Games games, one block,
  no extension, plain round robin over the 2 selected seats, the per-seat
  tally rows carry the seat split and the ponder adopt counts
  frozen-binary law: runs the existing bin/arena-freeze.exe unchanged, no
  build, no refreeze, the engine state is unchanged since the regate3
  freeze so the comparison stays on one binary
  marker $OutRoot/sep-pairs.status rewritten on every terminal transition:
  running <arm> | all-done | done-with-failures <arms and causes> |
  failed no-freeze sep-pairs
  logs under $OutRoot/sep-pair-<seat>-tc1.log, raw relay and stderr
  beside them, GOMEMLIMIT $MemLimit, CGO_ENABLED=1, GOTMPDIR <repo>/.gotmp
  relaunching resumes: an arm whose tally row already sits in its log is
  complete and skips, a live arena process for an arm is adopted and
  waited on, never double-launched
'@

if ($Help) { Write-Output $usage; exit 0 }

$arms = @(
	@{Name = 'sep-pair-noponder-tc1'; Seat = 'master-noponder'},
	@{Name = 'sep-pair-nobook-tc1'; Seat = 'master-nobook'}
)

function Convert-ToRepoPath([string]$rel) { Join-Path $repo ($rel -replace '/', '\') }

function Write-Status([string]$text) {
	$path = Convert-ToRepoPath "$OutRoot/$target.status"
	New-Item -ItemType Directory -Force (Split-Path $path -Parent) | Out-Null
	Set-Content -Path $path -Value $text
	Write-Output "status: $text"
}

function Get-ArmLog([string]$name) { Convert-ToRepoPath "$OutRoot/$name.log" }

function Test-ArmDone($arm) {
	$log = Get-ArmLog $arm.Name
	if (-not (Test-Path $log)) { return $false }
	$seatRow = Select-String -Path $log -Pattern ("^{0}\s+\d+-\d+-\d+ over \d+, red wins \d+ blue wins \d+" -f [regex]::Escape($arm.Seat)) | Select-Object -Last 1
	return $null -ne $seatRow
}

function Get-LiveArmProcess($arm) {
	Get-CimInstance Win32_Process -Filter "Name LIKE 'arena%'" |
		Where-Object { $_.CommandLine -and $_.CommandLine.Contains($repo) -and $_.CommandLine.Contains("-seats $($arm.Seat),medium") } |
		Select-Object -First 1
}

function Merge-ArmLog([string]$name) {
	$log = Get-ArmLog $name
	$raw = "$log.raw"
	$err = "$log.err"
	if (Test-Path $raw) { Add-Content -Path $log -Value (Get-Content $raw); Remove-Item $raw -Force -Confirm:$false }
	if (Test-Path $err) { Add-Content -Path $log -Value (Get-Content $err); Remove-Item $err -Force -Confirm:$false }
}

if ($DryRun) {
	Write-Output "sep-pairs dryrun: games $Games mem $MemLimit"
	$freeze = Join-Path $repo 'bin\arena-freeze.exe'
	$freezeNote = if (Test-Path $freeze) { "existing freeze $((Get-Item $freeze).Length) bytes, used unchanged" } else { 'MISSING, launch would fail with failed no-freeze' }
	Write-Output "freeze: $freezeNote"
	foreach ($arm in $arms) {
		$state = if (Test-ArmDone $arm) { 'done' } else { 'pending' }
		$live = Get-LiveArmProcess $arm
		$adopt = if ($null -ne $live) { " adopting pid $($live.ProcessId)" } else { '' }
		Write-Output "arm $($arm.Name) state $state$adopt"
	}
	Write-Output "command template per arm:"
	foreach ($arm in $arms) {
		Write-Output ("  GOMEMLIMIT=$MemLimit CGO_ENABLED=1 bin\arena-freeze.exe -geometry square -tc 1 -games $Games -seats $($arm.Seat),medium >> $OutRoot/$($arm.Name).log")
	}
	exit 0
}

if (-not $Foreground) {
	$pass = "-NoProfile -File `"$PSCommandPath`" -Foreground -Games $Games -MemLimit `"$MemLimit`" -OutRoot `"$OutRoot`""
	Start-Process -FilePath pwsh.exe -ArgumentList $pass -WorkingDirectory $repo -WindowStyle Hidden
	Write-Output "sep-pairs detached, logs under $OutRoot"
	exit 0
}

$twins = Get-CimInstance Win32_Process -Filter "Name = 'pwsh.exe'" | Where-Object { $_.ProcessId -ne $PID -and $_.CommandLine -and $_.CommandLine.Contains($PSCommandPath) -and $_.CommandLine.Contains('-Foreground') }
if ($twins) { Write-Output "sep-pairs: foreground instance already live, exiting"; exit 3 }

$arenaFreeze = Join-Path $repo 'bin\arena-freeze.exe'
if (-not (Test-Path $arenaFreeze)) { Write-Status "failed no-freeze $target"; exit 1 }

$gotmp = Join-Path $repo '.gotmp'
New-Item -ItemType Directory -Force $gotmp | Out-Null
$env:GOTMPDIR = $gotmp
$env:CGO_ENABLED = '1'
$env:GOMEMLIMIT = $MemLimit

$failures = @()
foreach ($arm in $arms) {
	if (Test-ArmDone $arm) { Write-Output "arm $($arm.Name) already complete, skipping"; continue }
	Write-Status "running $($arm.Name)"
	$log = Get-ArmLog $arm.Name
	New-Item -ItemType Directory -Force (Split-Path $log -Parent) | Out-Null
	$argList = @('-geometry', 'square', '-tc', '1', '-games', "$Games", '-seats', "$($arm.Seat),medium")
	$live = Get-LiveArmProcess $arm
	if ($null -ne $live) {
		Write-Output "arm $($arm.Name) adopting live arena pid $($live.ProcessId), waiting"
		try { Wait-Process -Id $live.ProcessId -ErrorAction Stop } catch {}
		Merge-ArmLog $arm.Name
		Add-Content -Path $log -Value "$(Get-Date -Format s) arm $($arm.Name) adopted"
	} else {
		Merge-ArmLog $arm.Name
		$raw = "$log.raw"
		$err = "$log.err"
		Set-Content -Path $log -Value "$(Get-Date -Format s) arm $($arm.Name) start: bin\arena-freeze.exe $($argList -join ' ')"
		$code = Invoke-ArenaProcess -FilePath $arenaFreeze -ArgumentList $argList -WorkingDirectory $repo -OutPath $raw -ErrPath $err
		Merge-ArmLog $arm.Name
		Add-Content -Path $log -Value "$(Get-Date -Format s) arm $($arm.Name) exit $code"
		if ($code -ne 0) {
			Write-Status "crashed $($arm.Name) exit $code"
			$failures += "crashed $($arm.Name) exit $code"
			continue
		}
	}
	if (-not (Test-ArmDone $arm)) {
		Write-Status "no-tally $($arm.Name)"
		$failures += "no-tally $($arm.Name)"
		continue
	}
	Write-Output "arm $($arm.Name) done"
}

if ($failures.Count -gt 0) {
	Write-Status "done-with-failures $($failures -join '; ')"
	exit 2
}
Write-Status 'all-done'
exit 0
