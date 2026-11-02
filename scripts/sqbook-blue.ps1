[CmdletBinding()]
param(
	[string]$Tier = 'master',
	[string]$From = 'db/book-tree.txt',
	[string]$TreeLog = 'db/book-tree.txt',
	[string]$Out = 'db/book.carobook',
	[string]$Stats = 'db/book-stats.txt',
	[string[]]$Seeds = @('H8,H6,H5', 'H8,I7,H5'),
	[string]$MemLimit = '8GiB',
	[string]$Log = 'playground/bookgen/out/sqbook-blue.log',
	[string]$Bin = 'bin/bookgen.exe',
	[switch]$Help,
	[switch]$DryRun
)

$ErrorActionPreference = 'Stop'
$repo = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path

$usage = @'
sqbook-blue: extend the square trunk book with blue-side replies
  runs: bin/bookgen.exe -phase branch -tier <tier> -from <trunk log> -tree-log <log> -out <artifact> -stats <stats> <seed lines>
  seeds the branched log from the committed trunk records, analyzes each blue
  target through the critical ladder, and re-emits the artifact so the blue
  seat probes [BOOK] at its early turns
  red trunk entries carry over verbatim, only blue-to-move entries are new
  switches: -DryRun prints the plan
'@

if ($Help) { Write-Output $usage; exit 0 }

$alive = Get-CimInstance Win32_Process -Filter "Name='bookgen.exe'" -ErrorAction SilentlyContinue | Where-Object {
	$_.CommandLine -and $_.CommandLine.Contains('-phase') -and $_.ExecutablePath -and $_.ExecutablePath.StartsWith($repo, [StringComparison]::OrdinalIgnoreCase)
}
if ($alive) {
	Write-Output "sqbook-blue: a bookgen phase process is already live pid $($alive[0].ProcessId), refusing to touch the tree log"
	exit 1
}

$phaseArgs = @('-phase', 'branch', '-tier', $Tier, '-from', $From, '-tree-log', $TreeLog, '-out', $Out, '-stats', $Stats) + $Seeds
$argLine = ($phaseArgs | ForEach-Object { if ($_ -match '\s') { "`"$_`"" } else { $_ } }) -join ' '

if ($DryRun) {
	Write-Output "sqbook-blue dryrun: tier $Tier from $From log $TreeLog out $Out"
	Write-Output "command: $Bin $argLine"
	exit 0
}

$gotmp = Join-Path $repo '.gotmp'
New-Item -ItemType Directory -Force $gotmp | Out-Null
$env:GOTMPDIR = $gotmp
$env:CGO_ENABLED = '1'
$env:GOMEMLIMIT = '4GiB'
$binAbs = Join-Path $repo ($Bin -replace '/', '\')
go -C (Join-Path $repo 'playground\bookgen') build -o $binAbs .
if ($LASTEXITCODE -ne 0) { Write-Output 'sqbook-blue: bookgen build failed'; exit 1 }

$logAbs = Join-Path $repo ($Log -replace '/', '\')
New-Item -ItemType Directory -Force (Split-Path $logAbs -Parent) | Out-Null
Write-Output "sqbook-blue: branch start, log $logAbs"
$raw = "$logAbs.raw"
$err = "$logAbs.err"
$env:GOMEMLIMIT = $MemLimit
$p = Start-Process -FilePath $binAbs -ArgumentList $phaseArgs -WorkingDirectory $repo -WindowStyle Hidden -Wait -PassThru -RedirectStandardOutput $raw -RedirectStandardError $err
Add-Content -Path $logAbs -Value (Get-Content $raw)
Remove-Item $raw -Force -Confirm:$false
if (Test-Path $err) { Add-Content -Path $logAbs -Value (Get-Content $err); Remove-Item $err -Force -Confirm:$false }
if ($p.ExitCode -ne 0) {
	Write-Output "sqbook-blue: branch exited $($p.ExitCode), see $logAbs"
	exit $p.ExitCode
}
Write-Output 'sqbook-blue: branch done'
exit 0
