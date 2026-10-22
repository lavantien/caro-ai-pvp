[CmdletBinding()]
param(
	[string]$Tier = 'master',
	[string]$Corpus = 'playground/bookgen/out/book-corpus.txt',
	[switch]$Reverify,
	[string]$MemLimit = '12GiB',
	[string]$Log = 'playground/bookgen/out/tree-run.log',
	[string]$Bin = 'bin/bookgen.exe',
	[switch]$Help,
	[switch]$DryRun,
	[switch]$Foreground
)

$ErrorActionPreference = 'Stop'
$repo = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path

$usage = @'
booktree-run: detached fixed-budget tree expansion book build
  runs: bin/bookgen.exe -phase tree -tier <tier> [corpus paths] [-reverify]
  the builder resumes itself from its expansion log, relaunching this script
  when no build process is alive continues the recorded tree
  stdout telemetry appended to the log
  switches: -DryRun prints the plan, -Foreground runs inline without detaching
'@

if ($Help) { Write-Output $usage; exit 0 }

function Test-TreeAlive {
	$procs = Get-CimInstance Win32_Process -Filter "Name='bookgen.exe'" -ErrorAction SilentlyContinue
	foreach ($p in $procs) {
		if ($p.CommandLine -and $p.CommandLine.Contains('-phase tree') -and $p.ExecutablePath -and $p.ExecutablePath.StartsWith($repo, [StringComparison]::OrdinalIgnoreCase)) {
			return $p.ProcessId
		}
	}
	return 0
}

$phaseArgs = @('-phase', 'tree', '-tier', $Tier)
if (Test-Path (Join-Path $repo ($Corpus -replace '/', '\'))) {
	$phaseArgs += $Corpus
}
if ($Reverify) { $phaseArgs += '-reverify' }
$argLine = ($phaseArgs | ForEach-Object { if ($_ -match '\s') { "`"$_`"" } else { $_ } }) -join ' '

function Invoke-Build {
	$logAbs = Join-Path $repo ($Log -replace '/', '\')
	New-Item -ItemType Directory -Force (Split-Path $logAbs -Parent) | Out-Null
	$binAbs = Join-Path $repo ($Bin -replace '/', '\')
	$cmd = "set GOMEMLIMIT=$MemLimit&& `"$binAbs`" $argLine >>`"$logAbs`" 2>&1"
	if ($DryRun) {
		Write-Host "dryrun build: $cmd"
		return 0
	}
	Write-Host "build start, log $logAbs"
	$p = Start-Process -FilePath cmd.exe -ArgumentList '/c', $cmd -WorkingDirectory $repo -WindowStyle Hidden -Wait -PassThru
	if ($p.ExitCode -ne 0) {
		Write-Host "build exited $($p.ExitCode), see $logAbs"
		return $p.ExitCode
	}
	Write-Host 'build done'
	return 0
}

if ($DryRun) {
	Write-Output "booktree-run dryrun: tier $Tier corpus $Corpus reverify $Reverify mem $MemLimit"
	$alive = Test-TreeAlive
	Write-Output "build process alive pid $alive"
	Invoke-Build | Out-Null
	Write-Output "command template: bin/bookgen.exe $argLine"
	exit 0
}

if (-not $Foreground) {
	$pass = @('-NoProfile', '-File', $PSCommandPath, '-Foreground', '-Tier', $Tier, '-Corpus', $Corpus, '-MemLimit', $MemLimit, '-Log', $Log, '-Bin', $Bin)
	if ($Reverify) { $pass += '-Reverify' }
	Start-Process -FilePath pwsh.exe -ArgumentList $pass -WorkingDirectory $repo -WindowStyle Hidden
	Write-Output "booktree-run detached, log $Log"
	exit 0
}

$alive = Test-TreeAlive
if ($alive -gt 0) {
	Write-Output "build already alive pid $alive, nothing to do"
	exit 0
}

$binAbs = Join-Path $repo ($Bin -replace '/', '\')
if (-not (Test-Path $binAbs)) {
	$env:GOMEMLIMIT = '4GiB'
	$env:CGO_ENABLED = '1'
	go -C (Join-Path $repo 'playground\bookgen') build -o $binAbs .
	if ($LASTEXITCODE -ne 0) { Write-Output 'bookgen build failed'; exit 1 }
}

$rc = Invoke-Build
if ($rc -ne 0) { exit $rc }
exit 0
