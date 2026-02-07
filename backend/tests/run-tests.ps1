#!/usr/bin/env pwsh
# Test running script for Caro backend

$ErrorActionPreference = "Stop"

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$backendDir = Split-Path -Parent $scriptDir

# Standard logger format for test output
$logger = "console;verbosity=detailed"

function Run-Tests {
    param(
        [string]$Filter = "",
        [string]$Project = "",
        [switch]$NoBuild
    )

    $args = @()
    if ($NoBuild) { $args += "--no-build" }
    $args += "--logger", "`"$logger`""
    if ($Filter) { $args += "--filter", "`"$Filter`"" }

    if ($Project) {
        Push-Location (Join-Path $backendDir "tests\$Project")
        dotnet test @args
        Pop-Location
    } else {
        Push-Location (Join-Path $backendDir "tests")
        # Run only unit test projects (IsTestProject=true)
        $unitTestProjects = @(
            "Caro.Core.Infrastructure.Tests",
            "Caro.Core.Tests"
        )
        foreach ($proj in $unitTestProjects) {
            if (Test-Path $proj) {
                Push-Location $proj
                Write-Host "Running tests in $proj..." -ForegroundColor Cyan
                dotnet test @args
                Pop-Location
            }
        }
        Pop-Location
    }
}

# Main entry point
$command = $args[0]
$remainingArgs = $args[1..$args.Length]

switch ($command) {
    "" {
        # Default: run unit tests with detailed output
        Run-Tests
    }
    "unit" {
        Run-Tests
    }
    "integration" {
        # Run integration tests
        Run-Tests -Project "Caro.Core.IntegrationTests"
    }
    "matchup" {
        # Run matchup tests
        Run-Tests -Project "Caro.Core.MatchupTests"
    }
    "quick" {
        # Quick smoke test - no build
        Run-Tests -NoBuild
    }
    default {
        Write-Host @"
Usage: .\run-tests.ps1 [command]

Commands:
  (none)     Run unit tests with detailed output (default)
  unit       Run unit tests only
  integration Run integration tests (AI search, stress)
  matchup     Run matchup tests
  quick       Quick smoke test (no build)

Examples:
  .\run-tests.ps1              # Run unit tests with detailed output
  .\run-tests.ps1 integration  # Run integration tests
  .\run-tests.ps1 quick        # Quick test without rebuild
"@ -ForegroundColor Yellow
        exit 1
    }
}
