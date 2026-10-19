param(
	[int]$Shards = 2,
	[int]$ShardGames = 55,
	[string]$ShardCorpus = 'book-corpus-b.txt',
	[int]$LegacyGames = 300,
	[string]$MemLimit = '1536MiB',
	[switch]$VerifyOnly
)

$ErrorActionPreference = 'Stop'
$repo = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$bin = Join-Path $repo 'bin\bookgen.exe'
$outRoot = 'playground/bookgen/out'
$members = @(
	@{ Name = 'legacy'; Corpus = "$outRoot/book-corpus.txt"; Games = $LegacyGames; Log = 'corpus-native' }
)
foreach ($i in 2..(1 + $Shards)) {
	$members += @{ Name = "inst$i"; Corpus = "$outRoot/inst$i/$ShardCorpus"; Games = $ShardGames; Log = 'corpus' }
}

function Test-MemberAlive([string]$corpus) {
	$procs = Get-CimInstance Win32_Process -Filter "Name='bookgen.exe'" -ErrorAction SilentlyContinue
	foreach ($p in $procs) {
		if ($p.CommandLine -and $p.CommandLine.Contains($corpus) -and $p.ExecutablePath -and $p.ExecutablePath.StartsWith($repo, [StringComparison]::OrdinalIgnoreCase)) {
			return $p.ProcessId
		}
	}
	return 0
}

if (-not (Test-Path $bin)) {
	if ($VerifyOnly) {
		Write-Output "missing $bin"
		exit 1
	}
	$env:GOMEMLIMIT = '4GiB'
	$env:CGO_ENABLED = '1'
	go -C (Join-Path $repo 'playground\bookgen') build -o $bin .
	if ($LASTEXITCODE -ne 0) {
		Write-Output "build failed"
		exit 1
	}
}

$env:GOMEMLIMIT = $MemLimit
$failed = 0
foreach ($m in $members) {
	$corpusAbs = Join-Path $repo ($m.Corpus -replace '/', '\')
	$dirAbs = Split-Path $corpusAbs -Parent
	New-Item -ItemType Directory -Force $dirAbs | Out-Null
	$pidAlive = Test-MemberAlive $m.Corpus
	if ($pidAlive -gt 0) {
		Write-Output "$($m.Name) alive pid=$pidAlive"
		continue
	}
	if ($VerifyOnly) {
		Write-Output "$($m.Name) dead"
		$failed++
		continue
	}
	$lock = "$corpusAbs.corpus.lock"
	if (Test-Path $lock) {
		$age = (Get-Date) - (Get-Item $lock).LastWriteTime
		$wait = [TimeSpan]::FromSeconds(35) - $age
		if ($wait.TotalSeconds -gt 0 -and $wait.TotalSeconds -le 40) {
			Write-Output "$($m.Name) waiting stale lock window $([int]$wait.TotalSeconds)s"
			Start-Sleep -Seconds ([int][Math]::Ceiling($wait.TotalSeconds))
		}
	}
	Start-Process -FilePath $bin `
		-ArgumentList '-phase', 'corpus', '-games', "$($m.Games)", '-corpus', $m.Corpus `
		-WorkingDirectory $repo `
		-RedirectStandardOutput (Join-Path $dirAbs "$($m.Log).log") `
		-RedirectStandardError (Join-Path $dirAbs "$($m.Log).err") `
		-WindowStyle Hidden
	Write-Output "$($m.Name) launched"
}

Start-Sleep -Seconds 5
foreach ($m in $members) {
	$pidAlive = Test-MemberAlive $m.Corpus
	if ($pidAlive -gt 0) {
		Write-Output "$($m.Name) confirmed pid=$pidAlive"
	} else {
		Write-Output "$($m.Name) not running"
		$failed++
	}
}
exit (($failed -gt 0) ? 1 : 0)
