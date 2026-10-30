$ErrorActionPreference = "Stop"
$repo = Split-Path (Split-Path $PSScriptRoot -Parent) -Parent
$log = Join-Path $PSScriptRoot "proof.txt"
Remove-Item $log -ErrorAction SilentlyContinue
$env:CGO_ENABLED = "1"
$stamp = [guid]::NewGuid().ToString("N")
$exe = Join-Path $env:TEMP "mutate-proof-$stamp.exe"
$allow = Join-Path $env:TEMP "mutate-proof-allow-$stamp.txt"

"go build cmd/mutate" | Tee-Object -FilePath $log -Append
& go build -o $exe ./cmd/mutate
if ($LASTEXITCODE -ne 0) { throw "go build failed with $LASTEXITCODE" }

"=== fresh run, no allowlist ===" | Tee-Object -FilePath $log -Append
Push-Location $PSScriptRoot
try {
	$plain = & $exe -pkgs . -timeout 60s 2>&1 | Out-String
	"exit $LASTEXITCODE" | Tee-Object -FilePath $log -Append
	$plain | Tee-Object -FilePath $log -Append
	foreach ($want in @(
		"greet.go:4:15 replace + with - KILLED",
		"add.go:4:11 replace + with - KILLED",
		" SURVIVED"
	)) {
		if (-not $plain.Contains($want)) { throw "fresh run missing verdict: $want" }
	}
	if ($plain.Contains(" RUN ERROR")) { throw "fresh run voided on the stillborn mutant" }
	if ($plain.Contains(" ALLOWED ")) { throw "fresh run consumed an allow entry" }

	Set-Content -Path $allow -Value "greet.go:4:15 replace + with - # interim compile-fail entry"
	"=== challenge run, allowlist holds the stillborn ===" | Tee-Object -FilePath $log -Append
	$errOut = & $exe -pkgs . -timeout 60s -allow $allow -challenge 2>&1 | Out-String
	"exit $LASTEXITCODE" | Tee-Object -FilePath $log -Append
	$errOut | Tee-Object -FilePath $log -Append
	foreach ($want in @(
		"greet.go:4:15 replace + with - CHALLENGED",
		"add.go:4:11 replace + with - KILLED",
		"no longer equivalent"
	)) {
		if (-not $errOut.Contains($want)) { throw "challenge run missing marker: $want" }
	}
	if ($errOut.Contains(" RUN ERROR")) { throw "challenge run voided on the stillborn mutant" }
} finally {
	Pop-Location
	Remove-Item $exe, $allow -ErrorAction SilentlyContinue
}
"proof done, log at $log"
