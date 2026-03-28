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
            "Caro.Core.Tests",
            "Caro.Core.MatchupTests"
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
    "failsafe" {
        # Run failsafe tests (binary pass/fail, ~15 min)
        Run-Tests -Project "Caro.Core.MatchupTests" -Filter "Category=Failsafe"
    }
    "smoke" {
        # Run smoke tests (quick sanity, ~5 min)
        Run-Tests -Project "Caro.Core.MatchupTests" -Filter "Category=Smoke"
    }
    "performance" {
        # Run thorough performance tests (statistical, hours)
        Run-Tests -Project "Caro.Core.MatchupTests" -Filter "Category=Integration"
    }
    "quick" {
        # Quick smoke test - no build
        Run-Tests -NoBuild
    }
    default {
        Write-Host @"
Usage: .\run-tests.ps1 [command]

Commands:
  (none)       Run unit tests with detailed output (default)
  unit         Run unit tests only
  integration  Run integration tests (AI search, stress)
  matchup      Run matchup tests (all tiers)
  failsafe     Run failsafe tests (binary pass/fail, ~15 min)
  smoke        Run smoke tests (quick sanity, ~5 min)
  performance  Run thorough performance tests (statistical, hours)
  quick        Quick smoke test (no build)

Examples:
  .\run-tests.ps1              # Run unit tests with detailed output
  .\run-tests.ps1 integration  # Run integration tests
  .\run-tests.ps1 failsafe     # Run failsafe tests only
  .\run-tests.ps1 smoke        # Run smoke tests only
  .\run-tests.ps1 quick        # Quick test without rebuild
"@ -ForegroundColor Yellow
        exit 1
    }
}
