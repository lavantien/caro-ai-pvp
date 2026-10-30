param(
	[string]$Package = './internal/server',
	[int]$TimeoutSec = 300
)

$ErrorActionPreference = 'Stop'
Set-Location (Join-Path $PSScriptRoot '..')
$env:CGO_ENABLED = '1'
$env:GOTMPDIR = 'C:\Users\lavantien\gotmp-mutkill'
if (-not (Test-Path $env:GOTMPDIR)) {
	New-Item -ItemType Directory -Force $env:GOTMPDIR | Out-Null
}

function Trace-Line($msg) { Write-Host ("[mutkill5 {0}] {1}" -f (Get-Date -Format 'HH:mm:ss'), $msg) }

$kills = @(
	@{ f = 'internal/server/boardview.go'; line = 45; col = 39; len = 1; old = '-'; new = '+'; test = 'TestHexBoardViewGoldenGeometry' },
	@{ f = 'internal/server/boardview.go'; line = 52; col = 61; len = 1; old = '2'; new = '1'; test = 'TestHexBoardViewGoldenGeometry' },
	@{ f = 'internal/server/boardview.go'; line = 88; col = 14; len = 1; old = '*'; new = '/'; test = 'TestHexBoardViewGoldenGeometry' },
	@{ f = 'internal/server/boardview.go'; line = 89; col = 10; len = 1; old = '+'; new = '-'; test = 'TestHexBoardViewGoldenGeometry' },
	@{ f = 'internal/server/store.go'; line = 262; col = 10; len = 1; old = '0'; new = '1'; test = 'TestBotAccountIDFailuresReturnAZeroID' },
	@{ f = 'internal/server/store.go'; line = 520; col = 6; len = 10; old = 'err != nil'; new = '!(err != nil)'; test = 'TestCasualSeatNameClassifiesBotAndHumanSeats' },
	@{ f = 'internal/server/admission.go'; line = 33; col = 7; len = 2; old = '<='; new = '<'; test = 'TestAdmissionZeroDemandSkipsDepletedLedgers' }
)

$allows = @()

$resultsPath = Join-Path $env:TEMP 'mutkill-server5.results'
$done = @{}
if (Test-Path $resultsPath) {
	foreach ($l in [System.IO.File]::ReadAllLines($resultsPath)) {
		if ($l -match '^(OK|KILLED|ALLOWED|NUMERIC|COMPILEFAIL) (\S+)') {
			$done[$Matches[2]] = $true
		}
	}
}
$results = New-Object System.Collections.Generic.List[string]
$bad = 0

function Record($line) {
	$results.Add($line)
	[System.IO.File]::AppendAllText($resultsPath, $line + "`n")
	Write-Output $line
}

function Splice-Overlay($m, $scratch) {
	$absOrig = Join-Path (Get-Location) $m.f
	$lines = [System.IO.File]::ReadAllLines($absOrig)
	$l = $lines[$m.line - 1]
	$at = $l.Substring($m.col - 1, $m.len)
	if ($at -cne $m.old) {
		Record ("BAD   {0}:{1}:{2} column-mismatch-got-{3}" -f $m.f, $m.line, $m.col, $at)
		return $null
	}
	$lines[$m.line - 1] = $l.Substring(0, $m.col - 1) + $m.new + $l.Substring($m.col - 1 + $m.len)
	$name = ((Split-Path $m.f -Leaf) -replace '\.go$', '') + ".L$($m.line)C$($m.col).go"
	$copyPath = Join-Path $scratch $name
	[System.IO.File]::WriteAllLines($copyPath, $lines)
	$overlayPath = Join-Path $scratch ($name + '.overlay.json')
	$overlay = @{ Replace = @{ $absOrig = $copyPath } } | ConvertTo-Json -Compress
	[System.IO.File]::WriteAllText($overlayPath, $overlay)
	return $overlayPath
}

$scratch = Join-Path $env:TEMP ("mutkill-server5-" + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Force $scratch | Out-Null
Trace-Line "enter kills=$($kills.Count) allows=$($allows.Count) scratch=$scratch results=$resultsPath"
try {
	$pinned = $kills | ForEach-Object { $_.test } | Sort-Object -Unique
	foreach ($t in $pinned) {
		$id = "clean-tree-$t"
		if ($done[$id]) { Trace-Line "skip done $id"; continue }
		Trace-Line "clean pre-run $t"
		& go test -count=1 -short -run "^$t$" $Package 2>&1 | Out-Null
		if ($LASTEXITCODE -ne 0) {
			Record "BAD   $id fails-on-clean-tree"
			$bad++
		}
		else {
			Record "OK    $id"
		}
	}
	foreach ($m in $kills) {
		$id = "{0}:{1}:{2}" -f $m.f, $m.line, $m.col
		if ($done["$id#rep1"] -and $done["$id#rep2"]) { Trace-Line "skip done $id"; continue }
		Trace-Line "kill mutant $id $($m.old)->$($m.new) test=$($m.test)"
		$overlayPath = Splice-Overlay $m $scratch
		if (-not $overlayPath) { $bad++; continue }
		$killedReps = 0
		for ($rep = 1; $rep -le 2; $rep++) {
			if ($done["$id#rep$rep"]) { $killedReps++; continue }
			$out = & go test -count=1 -short -overlay $overlayPath -run "^$($m.test)$" $Package 2>&1
			$firstFail = ($out | Select-String -Pattern '^(--- FAIL|panic:)' | Select-Object -First 1).Line
			if ($LASTEXITCODE -ne 0 -and $firstFail) {
				$killedReps++
				Record ("KILLED {0}#rep{1} {2}->{3} ({4})" -f $id, $rep, $m.old, $m.new, $firstFail)
			}
			else {
				Record ("BAD   {0}#rep{1} no-fail-signature exit={2}" -f $id, $rep, $LASTEXITCODE)
			}
		}
		if ($killedReps -ne 2) { $bad++ }
	}
	foreach ($m in $allows) {
		$id = "{0}:{1}:{2}" -f $m.f, $m.line, $m.col
		if ($done[$id]) { Trace-Line "skip done $id"; continue }
		Trace-Line "allow mutant $id $($m.old)->$($m.new) full short suite"
		$overlayPath = Splice-Overlay $m $scratch
		if (-not $overlayPath) { $bad++; continue }
		& go test -count=1 -short -overlay $overlayPath $Package 2>&1 | Out-Null
		if ($LASTEXITCODE -eq 0) {
			Record ("ALLOWED {0} {1}->{2} full-suite-green" -f $id, $m.old, $m.new)
		}
		else {
			Record ("BAD   {0} died-under-full-suite" -f $id)
			$bad++
		}
	}
	$buildId = 'clean-build'
	if (-not $done[$buildId]) {
		Trace-Line 'roster clean build check'
		& go build ./internal/server ./internal/geom 2>&1 | Out-Null
		if ($LASTEXITCODE -ne 0) {
			Record 'BAD   clean-build-fails'
			$bad++
		}
		else {
			Record 'OK    clean-build'
		}
	}
	$sites = (& go -C playground run ./opcols ../internal/server ../internal/geom) |
		Where-Object { $_ -match '^(\S+\.go):(\d+):(\d+)$' } |
		ForEach-Object { $_ -replace '^\.\./', '' }
	if (-not $sites -or @($sites).Count -eq 0) {
		Record 'BAD   roster-no-sites'
		$bad++
	}
	else {
		Trace-Line "roster add sites=$(@($sites).Count)"
	}
	foreach ($site in @($sites)) {
		$id = "site-$site"
		if ($done[$id]) { continue }
		$parts = $site -split ':'
		$m = @{ f = $parts[0]; line = [int]$parts[1]; col = [int]$parts[2]; len = 1; old = '+'; new = '-' }
		$pkg = './internal/server'
		if ($m.f -like 'internal/geom/*') { $pkg = './internal/geom' }
		$overlayPath = Splice-Overlay $m $scratch
		if (-not $overlayPath) { $bad++; continue }
		$out = & go build -overlay $overlayPath $pkg 2>&1
		if ($LASTEXITCODE -eq 0) {
			Record "NUMERIC $site"
		}
		else {
			$err = ($out | Select-String -Pattern '\.go:\d+:\d+' | Select-Object -First 1).Line
			Record ("COMPILEFAIL $site :: $err")
		}
	}
}
finally {
	Remove-Item -Recurse -Force $scratch -ErrorAction SilentlyContinue
	Trace-Line 'scratch purged'
}

Trace-Line "exit bad=$bad"
if ($bad -gt 0) { exit 1 }
exit 0
