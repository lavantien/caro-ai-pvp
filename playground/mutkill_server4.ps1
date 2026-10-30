param(
	[string]$Package = "./internal/server",
	[int]$TimeoutSec = 300
)

$ErrorActionPreference = 'Stop'
Set-Location (Join-Path $PSScriptRoot '..')
$env:CGO_ENABLED = '1'
$env:GOTMPDIR = 'C:\Users\lavantien\gotmp-mutkill'

function Trace-Line($msg) { Write-Host ("[mutkill4 {0}] {1}" -f (Get-Date -Format 'HH:mm:ss'), $msg) }

$mutants = @(
	@{ f = 'internal/server/stats.go'; line = 96; col = 12; len = 2; old = '10'; new = '9'; mode = 'kill'; test = 'TestCompactNumberTable' },
	@{ f = 'internal/server/store.go'; line = 825; col = 56; len = 2; old = '=='; new = '!='; mode = 'kill'; test = 'TestUserWLDsCountsBothSeatOrdersAndDecisiveOutcomes' },
	@{ f = 'internal/server/boardview.go'; line = 73; col = 25; len = 1; old = '1'; new = '0'; mode = 'allow'; test = '' },
	@{ f = 'internal/server/boardview.go'; line = 58; col = 6; len = 16; old = 'g.Contains(flat)'; new = '!g.Contains(flat)'; mode = 'allow'; test = '' },
	@{ f = 'internal/server/store.go'; line = 54; col = 30; len = 5; old = '0o755'; new = '494'; mode = 'allow'; test = '' }
)

$scratch = Join-Path $env:TEMP ("mutkill-server4-" + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Force $scratch | Out-Null
Trace-Line "enter mutants=$($mutants.Count) scratch=$scratch"

$results = New-Object System.Collections.Generic.List[string]
$bad = 0
try {
	$pinned = $mutants | Where-Object { $_.mode -eq 'kill' } | ForEach-Object { $_.test } | Sort-Object -Unique
	foreach ($t in $pinned) {
		Trace-Line "clean pre-run $t"
		& go test -count=1 -short -run "^$t$" $Package 2>&1 | Out-Null
		if ($LASTEXITCODE -ne 0) {
			Trace-Line "FAIL clean tree already fails $t"
			$results.Add("BAD   clean-tree $t")
			$bad++
		}
		else {
			$results.Add("OK    clean-tree $t")
		}
	}
	foreach ($m in $mutants) {
		$id = "{0}:{1}:{2} {3} -> {4}" -f $m.f, $m.line, $m.col, $m.old, $m.new
		Trace-Line "mutant $id mode=$($m.mode)"
		$absOrig = Join-Path (Get-Location) $m.f
		$lines = [System.IO.File]::ReadAllLines($absOrig)
		$l = $lines[$m.line - 1]
		$at = $l.Substring($m.col - 1, $m.len)
		if ($at -cne $m.old) {
			Trace-Line "FAIL column mismatch: got '$at' want '$($m.old)'"
			$results.Add("BAD   $id column-mismatch-got-$at")
			$bad++
			continue
		}
		$mutLine = $l.Substring(0, $m.col - 1) + $m.new + $l.Substring($m.col - 1 + $m.len)
		$lines[$m.line - 1] = $mutLine
		$name = ((Split-Path $m.f -Leaf) -replace '\.go$', '') + ".L$($m.line)C$($m.col).go"
		$copyPath = Join-Path $scratch $name
		[System.IO.File]::WriteAllLines($copyPath, $lines)
		$overlayPath = Join-Path $scratch ($name + ".overlay.json")
		$overlay = @{ Replace = @{ $absOrig = $copyPath } } | ConvertTo-Json -Compress
		[System.IO.File]::WriteAllText($overlayPath, $overlay)
		if ($m.mode -eq 'kill') {
			$out = & go test -count=1 -short -overlay $overlayPath -run "^$($m.test)$" $Package 2>&1
			$firstFail = ($out | Select-String -Pattern '^(--- FAIL|panic:)' | Select-Object -First 1).Line
			if ($LASTEXITCODE -ne 0) {
				Trace-Line "killed: $firstFail"
				$results.Add("KILLED $id ($firstFail)")
			}
			else {
				Trace-Line "SURVIVED under the pinned test"
				$results.Add("BAD   $id survived")
				$bad++
			}
		}
		else {
			& go test -count=1 -short -overlay $overlayPath $Package 2>&1 | Out-Null
			if ($LASTEXITCODE -eq 0) {
				Trace-Line "full short suite green under the mutant"
				$results.Add("ALLOWED $id full-suite-green")
			}
			else {
				Trace-Line "allow candidate died under the full suite"
				$results.Add("BAD   $id died-under-suite")
				$bad++
			}
		}
	}
}
finally {
	Remove-Item -Recurse -Force $scratch -ErrorAction SilentlyContinue
	Trace-Line "scratch purged"
}

$results | ForEach-Object { Write-Output $_ }
Trace-Line "exit bad=$bad"
if ($bad -gt 0) { exit 1 }
exit 0
