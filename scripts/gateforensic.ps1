[CmdletBinding()]
param(
	[ValidateSet('ingest', 'analyze', 'tests', 'all')]
	[string]$Step = 'all',
	[switch]$NoProvisional,
	[switch]$Help,
	[switch]$DryRun
)

$ErrorActionPreference = 'Stop'
$repo = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$module = Join-Path $repo 'playground\gateforensic'
$outDir = Join-Path $module 'out'
$runLog = Join-Path $outDir 'gateforensic-run.log'

$usage = @'
gateforensic: forensic harness over the master passgate arena logs
  steps: ingest parses every gate leg log under playground/arena/out into
        the tidy dataset under playground/gateforensic/out with a full
        integrity ledger, analyze runs the duckdb query set into
        playground/arena/out/gateforensic-report.md plus integrity checks,
        tests runs parser units, the fuzz battery, and the pipeline e2e
  the square easy tc 2 leg is still live: ingest reads its streaming
        .log.raw under the provisional flag and marks every row provisional,
        it never blocks on the running arena, rerunning after the verdict
        lands switches that leg to its merged log automatically
  -NoProvisional excludes the live leg entirely
  data analysis only, this script never launches any engine or arena binary
'@

if ($Help) { Write-Output $usage; exit 0 }

New-Item -ItemType Directory -Force $outDir | Out-Null

function Write-RunLog([string]$text) {
	"$(Get-Date -Format s) gateforensic $text" | Add-Content -Path $runLog
}

if ($DryRun) {
	Write-Output "gateforensic dryrun: step $Step noProvisional=$NoProvisional module $module"
	exit 0
}

Write-RunLog "start step=$Step noProvisional=$NoProvisional"

function Invoke-PyStep([string]$name, [string[]]$pyArgs) {
	Write-RunLog "$name entry"
	& python -B (Join-Path $module $name'.py') @pyArgs
	if ($LASTEXITCODE -ne 0) {
		Write-RunLog "$name failure exit $LASTEXITCODE"
		Write-Output "gateforensic: $name failed exit $LASTEXITCODE, see $runLog"
		exit $LASTEXITCODE
	}
	Write-RunLog "$name exit ok"
}

$ingestArgs = @()
if ($NoProvisional) { $ingestArgs += '--no-provisional' }

try {
	switch ($Step) {
		'ingest' { Invoke-PyStep 'ingest' $ingestArgs }
		'analyze' { Invoke-PyStep 'analyze' @() }
		'tests' { Invoke-PyStep 'tests' @(); Invoke-PyStep 'ingest' $ingestArgs; Invoke-PyStep 'analyze' @() }
		'all' { Invoke-PyStep 'ingest' $ingestArgs; Invoke-PyStep 'analyze' @() }
	}
} catch {
	Write-RunLog "crash step=$Step error=$($_.Exception.Message)"
	Write-Output "gateforensic: $Step crashed: $($_.Exception.Message)"
	exit 1
}

Write-RunLog "done step=$Step"
Write-Output "gateforensic: $Step complete"
exit 0
