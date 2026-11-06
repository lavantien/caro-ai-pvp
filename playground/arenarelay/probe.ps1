[CmdletBinding()]
param(
	[int]$Lines = 5000,
	[int]$BurstLines = 20000,
	[switch]$Help
)

$ErrorActionPreference = 'Stop'
$repo = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
. (Join-Path $repo 'scripts\arena-launch.ps1')

$usage = @'
arenarelay probe: proves the arena arm relay keeps draining the child stdout
  and stderr while the child runs, delivers every line, propagates the exit
  code, and survives a hostile concurrent reader of the output file
  scenarios: old control via Start-Process redirect, new helper, new helper
  under burst load with an external reader loop hammering the raw file,
  new helper under a slow drip asserting the raw file grows live mid-run
'@

if ($Help) { Write-Output $usage; exit 0 }

$failures = @()
function Assert([bool]$cond, [string]$msg) {
	if ($cond) { Write-Output "pass: $msg" } else { $script:failures += $msg; Write-Output "FAIL: $msg" }
}

$work = Join-Path $repo '.gotmp\relayprobe'
if (Test-Path $work) { Remove-Item $work -Recurse -Force -Confirm:$false }
New-Item -ItemType Directory -Force $work | Out-Null
$producer = Join-Path $work 'producer.exe'
$env:CGO_ENABLED = '1'
go -C (Join-Path $repo 'playground\arenarelay') build -o $producer .
if ($LASTEXITCODE -ne 0) { throw 'producer build failed' }

$oldRaw = Join-Path $work 'old.raw'
$oldErr = Join-Path $work 'old.err'
$oldProgress = Join-Path $work 'old.progress'
$sw = [System.Diagnostics.Stopwatch]::StartNew()
$p = Start-Process -FilePath $producer -ArgumentList @('-lines', "$Lines", '-every', '1ms', '-exit', '7', '-progress', $oldProgress) -WorkingDirectory $repo -WindowStyle Hidden -Wait -PassThru -RedirectStandardOutput $oldRaw -RedirectStandardError $oldErr
$oldWall = $sw.ElapsedMilliseconds
$oldCount = (Get-Content $oldRaw | Measure-Object -Line).Lines
Assert ($p.ExitCode -eq 7) "old control propagates exit code, got $($p.ExitCode)"
Assert ($oldCount -eq $Lines) "old control delivers $Lines lines, got $oldCount"
Write-Output ("note: old control wall {0} ms" -f $oldWall)

$newRaw = Join-Path $work 'new.raw'
$newErr = Join-Path $work 'new.err'
$newProgress = Join-Path $work 'new.progress'
$sw.Restart()
$code = Invoke-ArenaProcess -FilePath $producer -ArgumentList @('-lines', "$Lines", '-every', '1ms', '-stderrEvery', '50', '-exit', '7', '-progress', $newProgress) -WorkingDirectory $repo -OutPath $newRaw -ErrPath $newErr
$newWall = $sw.ElapsedMilliseconds
$newCount = (Get-Content $newRaw | Measure-Object -Line).Lines
$errCount = (Get-Content $newErr | Measure-Object -Line).Lines
Assert ($code -eq 7) "new helper propagates exit code, got $code"
Assert ($newCount -eq $Lines) "new helper delivers $Lines stdout lines, got $newCount"
Assert ($errCount -eq [math]::Floor($Lines / 50)) "new helper delivers stderr lines, got $errCount"
Assert (($newWall - $oldWall) -lt 15000) "new helper wall within 15 s of old control, delta $($newWall - $oldWall) ms"

$burstRaw = Join-Path $work 'burst.raw'
$burstErr = Join-Path $work 'burst.err'
$burstProgress = Join-Path $work 'burst.progress'
$reader = Start-Job { param($path) for ($i = 0; $i -lt 250; $i++) { if (Test-Path $path) { Get-Content -Tail 5 $path -ErrorAction SilentlyContinue | Out-Null; Start-Sleep -Milliseconds 20 } } } -ArgumentList $burstRaw
$sw.Restart()
$code = Invoke-ArenaProcess -FilePath $producer -ArgumentList @('-lines', "$BurstLines", '-every', '0ms', '-exit', '3', '-progress', $burstProgress) -WorkingDirectory $repo -OutPath $burstRaw -ErrPath $burstErr
$burstWall = $sw.ElapsedMilliseconds
$reader | Wait-Job | Out-Null
$reader | Remove-Job -Force -Confirm:$false
$burstCount = (Get-Content $burstRaw | Measure-Object -Line).Lines
Assert ($code -eq 3) "burst under hostile reader propagates exit code, got $code"
Assert ($burstCount -eq $BurstLines) "burst under hostile reader delivers $BurstLines lines, got $burstCount"
Assert ($burstWall -lt 60000) "burst under hostile reader completes under 60 s, took $burstWall ms"

$dripRaw = Join-Path $work 'drip.raw'
$dripErr = Join-Path $work 'drip.err'
$dripRunner = Start-Job { param($repo, $producer, $raw, $err) . (Join-Path $repo 'scripts\arena-launch.ps1'); Invoke-ArenaProcess -FilePath $producer -ArgumentList @('-lines', '20', '-every', '500ms', '-exit', '0') -WorkingDirectory $repo -OutPath $raw -ErrPath $err } -ArgumentList $repo, $producer, $dripRaw, $dripErr
Start-Sleep -Milliseconds 4200
$midCount = if (Test-Path $dripRaw) { (Get-Content $dripRaw | Measure-Object -Line).Lines } else { 0 }
$dripRunner | Wait-Job | Out-Null
$dripCode = Receive-Job $dripRunner
$dripRunner | Remove-Job -Force -Confirm:$false
$dripCount = (Get-Content $dripRaw | Measure-Object -Line).Lines
Assert ($midCount -ge 1 -and $midCount -le 19) "drip raw grows live mid-run, sampled $midCount at 4.2 s"
Assert ($dripCount -eq 20) "drip delivers all 20 lines, got $dripCount"
Assert ($dripCode -eq 0) "drip propagates exit code, got $dripCode"

Remove-Item $work -Recurse -Force -Confirm:$false
if ($failures.Count -gt 0) {
	Write-Output ("probe FAILED, {0} assertions" -f $failures.Count)
	exit 1
}
Write-Output 'probe passed'
exit 0
