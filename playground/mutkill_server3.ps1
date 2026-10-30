param(
	[string]$Package = "./internal/server",
	[int]$TimeoutSec = 300
)

$ErrorActionPreference = 'Stop'
Set-Location (Join-Path $PSScriptRoot '..')
$env:CGO_ENABLED = '1'

function Trace-Line($msg) { Write-Host ("[mutkill {0}] {1}" -f (Get-Date -Format 'HH:mm:ss'), $msg) }

$mutants = @(
	@{ f = 'internal/server/admission.go'; line = 26; col = 55; len = 1; old = '*'; new = '/'; test = 'TestAdmissionConstructorPinsTheConfiguredWait' },
	@{ f = 'internal/server/admission.go'; line = 33; col = 7; len = 2; old = '<='; new = '<'; test = 'TestAdmissionZeroDemandSkipsDepletedLedgers' },
	@{ f = 'internal/server/admission.go'; line = 33; col = 10; len = 1; old = '0'; new = '-1'; test = 'TestAdmissionZeroDemandSkipsDepletedLedgers' },
	@{ f = 'internal/server/admission.go'; line = 40; col = 7; len = 2; old = '<='; new = '<'; test = 'TestAdmissionZeroDemandSkipsDepletedLedgers' },
	@{ f = 'internal/server/admission.go'; line = 40; col = 10; len = 1; old = '0'; new = '-1'; test = 'TestAdmissionZeroDemandSkipsDepletedLedgers' },
	@{ f = 'internal/server/admission.go'; line = 41; col = 10; len = 4; old = 'true'; new = 'false'; test = 'TestAdmissionZeroDemandSkipsDepletedLedgers' },
	@{ f = 'internal/server/admission.go'; line = 82; col = 11; len = 2; old = '<='; new = '<'; test = 'TestAdmissionZeroDemandSkipsDepletedLedgers' },
	@{ f = 'internal/server/admission.go'; line = 82; col = 14; len = 1; old = '0'; new = '-1'; test = 'TestAdmissionZeroDemandSkipsDepletedLedgers' },
	@{ f = 'internal/server/admission.go'; line = 116; col = 11; len = 2; old = '<='; new = '<'; test = 'TestAdmissionZeroDemandSkipsDepletedLedgers' },
	@{ f = 'internal/server/admission.go'; line = 116; col = 14; len = 1; old = '0'; new = '-1'; test = 'TestAdmissionZeroDemandSkipsDepletedLedgers' },
	@{ f = 'internal/server/admission.go'; line = 116; col = 14; len = 1; old = '0'; new = '1'; test = 'TestAdmissionCanAcquireTTRefusesAtZeroFree' },
	@{ f = 'internal/server/admission.go'; line = 43; col = 31; len = 1; old = '0'; new = '1'; test = 'TestAdmissionTryAcquireNeverWaits'; expect = 'allow' },
	@{ f = 'internal/server/admission.go'; line = 50; col = 14; len = 1; old = '0'; new = '1'; test = 'TestAdmissionAcquireTimesOutLoudly'; expect = 'allow' },
	@{ f = 'internal/server/admission.go'; line = 72; col = 7; len = 2; old = '<='; new = '<'; test = 'TestAdmissionZero'; expect = 'allow' },
	@{ f = 'internal/server/admission.go'; line = 72; col = 10; len = 1; old = '0'; new = '-1'; test = 'TestAdmissionZero'; expect = 'allow' },
	@{ f = 'internal/server/admission.go'; line = 95; col = 11; len = 2; old = '<='; new = '<'; test = 'TestAdmissionTTRefuseAndRelease|TestAdmissionZeroDemandSkipsDepletedLedgers'; expect = 'allow' },
	@{ f = 'internal/server/admission.go'; line = 95; col = 14; len = 1; old = '0'; new = '-1'; test = 'TestAdmissionTTRefuseAndRelease|TestAdmissionZeroDemandSkipsDepletedLedgers'; expect = 'allow' },
	@{ f = 'internal/server/availmem_windows.go'; line = 29; col = 5; len = 8; old = 'ret == 0'; new = 'ret != 0'; test = 'TestHostAvailMem' },
	@{ f = 'internal/server/availmem_windows.go'; line = 29; col = 9; len = 2; old = '=='; new = '!='; test = 'TestHostAvailMem' },
	@{ f = 'internal/server/availmem_windows.go'; line = 29; col = 12; len = 1; old = '0'; new = '1'; test = 'TestHostAvailMem' },
	@{ f = 'internal/server/availmem_windows.go'; line = 30; col = 10; len = 1; old = '0'; new = '1'; test = 'TestHostAvailMem' },
	@{ f = 'internal/server/booksearcher.go'; line = 40; col = 17; len = 1; old = '<'; new = '<='; test = 'TestBookSearcherPonderBoundIsExclusiveEvenWhenBooked' },
	@{ f = 'internal/server/booksearcher.go'; line = 90; col = 26; len = 1; old = '0'; new = '1'; test = 'TestMissingBookArtifactTriedListCarriesNoBlankEntries' },
	@{ f = 'internal/server/refaudit.go'; line = 19; col = 26; len = 2; old = '||'; new = '&&'; test = 'TestOpponentRefutesPassesAPlayedCell' },
	@{ f = 'internal/server/refaudit.go'; line = 19; col = 56; len = 2; old = '||'; new = '&&'; test = 'TestOpponentRefutesPassesAPlayedCell' },
	@{ f = 'internal/server/refaudit.go'; line = 19; col = 65; len = 1; old = '<'; new = '<='; test = 'TestOpponentRefutesStillAuditsTheFloorClaim' },
	@{ f = 'internal/server/refaudit.go'; line = 23; col = 11; len = 1; old = '<'; new = '<='; test = 'TestOpponentRefutesAuditsClaimsOfAtLeastOnePly' },
	@{ f = 'internal/server/refaudit.go'; line = 23; col = 13; len = 1; old = '1'; new = '2'; test = 'TestOpponentRefutesAuditsClaimsOfAtLeastOnePly' },
	@{ f = 'internal/server/refaudit.go'; line = 23; col = 13; len = 1; old = '1'; new = '0'; test = 'TestOpponentRefutesAuditsClaimsOfAtLeastOnePly' },
	@{ f = 'internal/server/refaudit.go'; line = 24; col = 10; len = 5; old = 'false'; new = 'true'; test = 'TestOpponentRefutesAuditsClaimsOfAtLeastOnePly' },
	@{ f = 'internal/server/auth.go'; line = 58; col = 29; len = 1; old = '*'; new = '/'; test = 'TestVerifyPassword(FailsClosed|CostEnvelopeBoundaries|UsesStoredPerUserParams)'; expect = 'allow' }
)

$scratch = Join-Path $env:TEMP ("mutkill-server3-" + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Force $scratch | Out-Null
Trace-Line "enter mutants=$($mutants.Count) scratch=$scratch"

$results = New-Object System.Collections.Generic.List[string]
$killed = 0
$survived = 0
$allowed = 0
$aborted = 0
try {
	foreach ($m in $mutants) {
		$want = if ($m.expect -eq 'allow') { 'allow' } else { 'kill' }
		$id = "{0}:{1}:{2} {3} -> {4}" -f $m.f, $m.line, $m.col, $m.old, $m.new
		Trace-Line "mutant $id want=$want test=$($m.test)"
		$absOrig = Join-Path (Get-Location) $m.f
		$lines = [System.IO.File]::ReadAllLines($absOrig)
		$l = $lines[$m.line - 1]
		$at = $l.Substring($m.col - 1, $m.len)
		if ($at -cne $m.old) {
			Trace-Line "FAIL column mismatch: got '$at' want '$($m.old)'"
			$results.Add("ABORT  $id column-mismatch-got-$at")
			$aborted++
			continue
		}
		$mutLine = $l.Substring(0, $m.col - 1) + $m.new + $l.Substring($m.col - 1 + $m.len)
		$lines[$m.line - 1] = $mutLine
		$name = ((Split-Path $m.f -Leaf) -replace '\.go$', '') + ".L$($m.line)C$($m.col).$($m.new -replace '[^A-Za-z0-9]', '').go"
		$copyPath = Join-Path $scratch $name
		[System.IO.File]::WriteAllLines($copyPath, $lines)
		$overlayPath = Join-Path $scratch ($name + ".overlay.json")
		$overlay = @{ Replace = @{ $absOrig = $copyPath } } | ConvertTo-Json -Compress
		[System.IO.File]::WriteAllText($overlayPath, $overlay)
		$testOutput = & go test -count=1 -overlay $overlayPath -run $m.test $Package 2>&1
		$code = $LASTEXITCODE
		$failed = $code -ne 0
		$firstFail = ($testOutput | Select-String -Pattern '^(--- FAIL|panic:)' | Select-Object -First 1).Line
		if ($want -eq 'kill' -and $failed) {
			Trace-Line "killed: $firstFail"
			$results.Add("KILLED $id ($firstFail)")
			$killed++
		}
		elseif ($want -eq 'allow' -and -not $failed) {
			Trace-Line "survives the nearest test, matching the allow proof"
			$results.Add("ALLOWED $id survives=$($m.test)")
			$allowed++
		}
		elseif ($want -eq 'kill') {
			Trace-Line "SURVIVED under the pinned test"
			$results.Add("SURVIV $id")
			$survived++
		}
		else {
			Trace-Line "allow candidate died: $firstFail"
			$results.Add("DIED-ALLOW $id ($firstFail)")
			$survived++
		}
	}
}
finally {
	Remove-Item -Recurse -Force $scratch -ErrorAction SilentlyContinue
	Trace-Line "scratch purged"
}

$results | ForEach-Object { Write-Output $_ }
Trace-Line "exit killed=$killed allowed=$allowed survived=$survived aborted=$aborted"
if ($survived -gt 0 -or $aborted -gt 0) { exit 1 }
exit 0
