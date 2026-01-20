# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.0.2] - 2026-01-20

### Added

- AI Strength Validation Test Suite - Comprehensive statistical validation framework
  - StatisticalAnalyzer with LOS, Elo CI, binomial tests, SPRT
  - MatchupStatistics data model for tracking matchup results
  - 4-phase test suite: Adjacent, Cross-Level, Color Advantage, Round-Robin
  - HTML report generation with CSS styling
  - CLI runner with configurable parameters
  - 38 statistical unit tests (all passing)
- Paired game design with color swapping for fair comparison
- Likelihood of Superiority (LOS) calculation using error function approximation
- Elo difference with 95% confidence intervals (delta method)
- Color advantage detection using binomial test
- Sequential Probability Ratio Test (SPRT) for early termination
- HTML report generation with summary statistics and Elo ranking
- CLI flags: --validate-strength, --quick-validate, --games, --time, --inc, --verbose

### Changed

- PICKUP.md updated with comprehensive documentation of new validation suite
- README.md updated with AI Strength Validation section
- Tournament runner now supports statistical validation modes

### Technical Details

**Statistical Methods:**

- Elo Difference: Standard chess rating system (400-point scale)
- 95% Confidence Intervals: Delta method approximation
- Likelihood of Superiority (LOS): Based on error function
- Binomial Test: For win rate significance (p < 0.05)
- Paired Game Design: Color swapping to neutralize first-move advantage

**Sample Size Guidelines:**

- 25 games per matchup: Preliminary results (default)
- 50 games per matchup: Moderate confidence
- 100+ games per matchup: High confidence

**Test Configuration:**

- Time control: 2+1 (120s + 1s increment) by default
- Pondering: Enabled for D7+
- Color swapping: Every other game

### Files Added

- src/Caro.Core/Tournament/StatisticalAnalyzer.cs
- src/Caro.Core/Tournament/MatchupStatistics.cs
- tests/Caro.Core.Tests/Tournament/StatisticalAnalyzerTests.cs
- tests/Caro.Core.Tests/Tournament/AIStrengthValidationSuite.cs
- src/Caro.TournamentRunner/ReportGenerators/HtmlReportGenerator.cs
- src/Caro.TournamentRunner/AIStrengthTestRunner.cs

[0.0.2]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v0.0.2

## [0.0.1] - 2026-01-20

### Added

- VCF (Victory by Continuous Four) - Tactical solver restricted to Legend (D11) difficulty only
- Minimum depth separation (1-ply gaps between adjacent difficulties)
- Scaled Critical Defense:
  - D9-D11: Full threat detection (four + open three)
  - D7-D8: Basic threat detection (four only)
  - D6 and below: No automatic threat detection
- Time difficulty multipliers for search depth differentiation
- QuickTest verification suite for AI strength ordering
- Comprehensive documentation in PICKUP.md

### Changed

- Time multipliers rebalanced to prevent timeouts:
  - Legend (D11): 1.5x (reduced from 2.0x)
  - Grandmaster (D10): 1.2x
  - Master (D9): 1.0x (baseline)
  - Expert (D8): 0.9x
  - VeryHard (D7): 0.8x
  - Harder (D6): 0.7x
- VCF Defense disabled to prevent reactive play from D11
- VCF threshold logic fixed to prevent lower difficulties from accessing VCF

### Fixed

- D11 timeout issue - was timing out after 10-25 moves due to excessive time allocation
- VCF threshold bug - D6 was incorrectly given VCF access due to threshold comparison logic
- VCF acting as equalizer - when multiple difficulties had VCF, lower ones could beat higher ones
- Minimum depth calculation - now ensures 1-ply separation between adjacent difficulties
- AI strength inversion - all test groups now pass:
  - Legend (D11) beats Grandmaster (D10)
  - Grandmaster (D10) beats Expert (D8)
  - Legend (D11) beats Harder (D6)
  - Harder (D6) beats Medium (D4)

### Technical Details

**Root Cause Analysis:**

The AI strength inversion was caused by multiple factors:

1. **VCF as Equalizer**: VCF is a powerful tactical solver that finds forced wins. When multiple difficulties had access to VCF, it acted as an equalizer rather than a differentiator - lower difficulties with VCF could beat higher difficulties without it.

2. **Time Multipliers Too Aggressive**: D11 was allocated 3.5x time, leading to 26M node searches and timeouts after 10-25 moves.

3. **VCF Defense Causing Reactive Play**: D11 was focusing too much on blocking opponent threats instead of developing its own position.

**Solution:**

- Restricted VCF to Legend (D11) only, making it a unique advantage for the highest difficulty
- Reduced time multipliers to conservative values (1.5x max) while maintaining depth separation
- Disabled VCF Defense entirely, relying on the evaluation function's 2.2x defense multiplier
- Added minimum depth thresholds to ensure 1-ply separation between adjacent difficulties

### Performance Impact

| Difficulty | Time Multiplier | Min Depth | Max Depth (7+5 TC) | VCF Access |
|------------|-----------------|-----------|-------------------|------------|
| Legend (D11) | 1.5x | 8 | 9 | Yes (exclusive) |
| Grandmaster (D10) | 1.2x | 7 | 8 | No |
| Master (D9) | 1.0x | 6 | 7 | No |
| Expert (D8) | 0.9x | 5 | 6 | No |
| VeryHard (D7) | 0.8x | 4 | 5-6 | No |
| Harder (D6) | 0.7x | 4 | 5 | No |

### Known Issues

- Parallel search (Lazy SMP) remains disabled due to architectural issues with shared transposition table
- SIMD evaluator remains disabled due to score inflation causing AI strength inversion
- Time management still uses hardcoded multipliers instead of fully adaptive PID controller

### Breaking Changes

- VCF is no longer available to difficulties below Legend (D11)
- Time allocation is more conservative to prevent timeouts at 7+5 time control
- Minimum depth is now enforced per difficulty, reducing flexibility but ensuring ordering

[0.0.1]: https://github.com/lavantien/caro-ai-pvp/releases/tag/v0.0.1
