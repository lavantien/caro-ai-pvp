$ErrorActionPreference = "Stop"
$log = Join-Path $PSScriptRoot "sigs.txt"
Remove-Item $log -ErrorAction SilentlyContinue
$work = Join-Path $env:TEMP ("mutstillborn-sigs-" + [guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Force $work | Out-Null
$env:CGO_ENABLED = "1"

function Write-Module($name, $files) {
	$dir = Join-Path $work $name
	New-Item -ItemType Directory -Force $dir | Out-Null
	foreach ($k in $files.Keys) {
		$target = Join-Path $dir $k
		New-Item -ItemType Directory -Force (Split-Path $target -Parent) | Out-Null
		Set-Content -Path $target -Value $files[$k] -NoNewline
	}
	return $dir
}

function Invoke-Probe($title, $dir, $pkg) {
	"=== $title ===" | Tee-Object -FilePath $log -Append
	Push-Location $dir
	try {
		$output = & go test -count=1 $pkg 2>&1 | Out-String
		"exit $LASTEXITCODE" | Tee-Object -FilePath $log -Append
		$output | Tee-Object -FilePath $log -Append
	} finally {
		Pop-Location
	}
}

"go version:" | Tee-Object -FilePath $log -Append
(& go version) | Tee-Object -FilePath $log -Append

$still = Write-Module "sigstill" @{
	"go.mod"        = "module sigstill`n`ngo 1.27`n"
	"greet.go"      = "package sigstill`n`nfunc Greet(name string) string {`n`treturn `"hi `" - name`n}`n"
	"add.go"        = "package sigstill`n`nfunc Add(a, b int) int {`n`treturn a + b`n}`n"
	"add_test.go"   = "package sigstill`n`nimport `"testing`"`n`nfunc TestAdd(t *testing.T) {`n`tif Add(2, 3) != 5 {`n`t`tt.Fatal(`"wrong sum`")`n`t}`n}`n"
}
Invoke-Probe "stillborn string subtraction in the package under test" $still "."

$dep = Write-Module "sigdep" @{
	"go.mod"       = "module sigdep`n`ngo 1.27`n"
	"dep/den.go"   = "package dep`n`nconst X = `"a`" - `"b`"`n"
	"use/use.go"   = "package use`n`nimport `"sigdep/dep`"`n`nfunc F() string { return dep.X }`n"
	"use_test.go"  = "package use`n`nimport `"testing`"`n`nfunc TestF(t *testing.T) { _ = F() }`n"
}
Invoke-Probe "dependency package fails to compile" $dep "./use"

$testfail = Write-Module "sigtestfile" @{
	"go.mod"       = "module sigtestfile`n`ngo 1.27`n"
	"den.go"       = "package sigtestfile`n`nconst Den = 0`n"
	"den_test.go"  = "package sigtestfile`n`nvar sink = 3 / Den`n"
}
Invoke-Probe "mutated package constant breaks the test file compile" $testfail "."

$vet = Write-Module "sigvet" @{
	"go.mod"       = "module sigvet`n`ngo 1.27`n"
	"vet.go"       = "package sigvet`n`nimport `"fmt`"`n`nfunc F() { fmt.Printf(`"%d`", `"x`") }`n"
	"vet_test.go"  = "package sigvet`n`nimport `"testing`"`n`nfunc TestF(t *testing.T) { F() }`n"
}
Invoke-Probe "vet rejection in the package under test" $vet "."

Remove-Item -Recurse -Force $work
"probe done, log at $log"
