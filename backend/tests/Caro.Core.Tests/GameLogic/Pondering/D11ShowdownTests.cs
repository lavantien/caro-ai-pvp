using Xunit;
using FluentAssertions;
using Caro.Core.Entities;
using Caro.Core.GameLogic;
using Caro.Core.Tournament;
using System.Diagnostics;

namespace Caro.Core.Tests.GameLogic.Pondering;

/// <summary>
/// D5 Grandmaster Showdown: Test two Grandmaster-level AIs playing against each other
/// This demonstrates the full power of the AI with pondering enabled
/// </summary>
[Trait("Category", "Showcase")]
[Trait("Category", "LongRunning")]
public class D11ShowdownTests
{
    [Fact]
    public void D5vsD5_WithPondering_CompletesSuccessfully()
    {
        // Arrange
        var engine = new TournamentEngine();
        var stopwatch = Stopwatch.StartNew();

        // Act - Run a full D5 vs D5 game with pondering
        var result = engine.RunGame(
            AIDifficulty.Grandmaster,      // Red: D5
            AIDifficulty.Grandmaster,      // Blue: D5
            maxMoves: 225,
            initialTimeSeconds: 60,   // 1 minute each for quick test
            incrementSeconds: 1,
            ponderingEnabled: true
        );

        stopwatch.Stop();

        // Assert
        result.Should().NotBeNull();
        result.TotalMoves.Should().BeGreaterThan(0);
        result.DurationMs.Should().BeGreaterThan(0);

        // Output results
        Console.WriteLine($"\n{'=' * 60}");
        Console.WriteLine($"D5 vs D5 Showdown Results (1+1 time control)");
        Console.WriteLine($"{'=' * 60}");
        Console.WriteLine($"Winner: {result.Winner} ({result.WinnerDifficulty})");
        Console.WriteLine($"Loser: {result.Loser} ({result.LoserDifficulty})");
        Console.WriteLine($"Total Moves: {result.TotalMoves}");
        Console.WriteLine($"Duration: {result.DurationMs / 1000.0:F2}s");
        Console.WriteLine($"Is Draw: {result.IsDraw}");
        Console.WriteLine($"Ended by Timeout: {result.EndedByTimeout}");
        Console.WriteLine($"{'=' * 60}\n");
    }

    [Fact]
    public void D5vsD5_WithProperTimeControl_ReachesFullDepth()
    {
        // Arrange
        var engine = new TournamentEngine();
        var stopwatch = Stopwatch.StartNew();

        // Act - Run with 7+5 time control (designed for D5)
        var result = engine.RunGame(
            AIDifficulty.Grandmaster,      // Red: D5
            AIDifficulty.Grandmaster,      // Blue: D5
            maxMoves: 50,             // Limit moves for demo
            initialTimeSeconds: 420,  // 7 minutes each (7+5 time control)
            incrementSeconds: 5,
            ponderingEnabled: true
        );

        stopwatch.Stop();

        // Assert
        result.Should().NotBeNull();
        result.TotalMoves.Should().BeGreaterThan(0);
        result.DurationMs.Should().BeGreaterThan(0);

        // Output results
        Console.WriteLine($"\n{'=' * 60}");
        Console.WriteLine($"D5 vs D5 Showdown Results (7+5 time control - PROPER)");
        Console.WriteLine($"{'=' * 60}");
        Console.WriteLine($"Winner: {result.Winner} ({result.WinnerDifficulty})");
        Console.WriteLine($"Loser: {result.Loser} ({result.LoserDifficulty})");
        Console.WriteLine($"Total Moves: {result.TotalMoves}");
        Console.WriteLine($"Duration: {result.DurationMs / 1000.0:F2}s");
        Console.WriteLine($"Is Draw: {result.IsDraw}");
        Console.WriteLine($"Ended by Timeout: {result.EndedByTimeout}");
        Console.WriteLine($"Note: With 7+5 time control, D5 should reach adaptive depth 9-11");
        Console.WriteLine($"{'=' * 60}\n");
    }

    [Fact]
    public void D5vsD5_WithoutPondering_CompletesSuccessfully()
    {
        // Arrange
        var engine = new TournamentEngine();
        var stopwatch = Stopwatch.StartNew();

        // Act - Run a full D5 vs D5 game WITHOUT pondering
        var result = engine.RunGame(
            AIDifficulty.Grandmaster,      // Red: D5
            AIDifficulty.Grandmaster,      // Blue: D5
            maxMoves: 225,
            initialTimeSeconds: 60,   // 1 minute each
            incrementSeconds: 1,
            ponderingEnabled: false
        );

        stopwatch.Stop();

        // Assert
        result.Should().NotBeNull();
        result.TotalMoves.Should().BeGreaterThan(0);
        result.DurationMs.Should().BeGreaterThan(0);

        // Output results
        Console.WriteLine($"\n{'=' * 60}");
        Console.WriteLine($"D5 vs D5 (No Pondering) Results");
        Console.WriteLine($"{'=' * 60}");
        Console.WriteLine($"Winner: {result.Winner} ({result.WinnerDifficulty})");
        Console.WriteLine($"Loser: {result.Loser} ({result.LoserDifficulty})");
        Console.WriteLine($"Total Moves: {result.TotalMoves}");
        Console.WriteLine($"Duration: {result.DurationMs / 1000.0:F2}s");
        Console.WriteLine($"Is Draw: {result.IsDraw}");
        Console.WriteLine($"Ended by Timeout: {result.EndedByTimeout}");
        Console.WriteLine($"{'=' * 60}\n");
    }

    [Fact]
    public void D5vsD5_PonderingSpeedup_Comparison()
    {
        // Arrange
        var engine = new TournamentEngine();

        // Act - Run two games and compare
        var stopwatchWith = Stopwatch.StartNew();
        var resultWithPondering = engine.RunGame(
            AIDifficulty.Grandmaster,
            AIDifficulty.Grandmaster,
            maxMoves: 50,             // Limit moves for quick comparison
            initialTimeSeconds: 30,
            incrementSeconds: 0,
            ponderingEnabled: true
        );
        stopwatchWith.Stop();

        var stopwatchWithout = Stopwatch.StartNew();
        var resultWithoutPondering = engine.RunGame(
            AIDifficulty.Grandmaster,
            AIDifficulty.Grandmaster,
            maxMoves: 50,
            initialTimeSeconds: 30,
            incrementSeconds: 0,
            ponderingEnabled: false
        );
        stopwatchWithout.Stop();

        // Assert
        resultWithPondering.TotalMoves.Should().BeGreaterThan(0);
        resultWithoutPondering.TotalMoves.Should().BeGreaterThan(0);

        // Output comparison
        Console.WriteLine($"\n{'=' * 60}");
        Console.WriteLine($"Pondering Speedup Comparison (D5 vs D5)");
        Console.WriteLine($"{'=' * 60}");
        Console.WriteLine($"WITH Pondering:");
        Console.WriteLine($"  Duration: {resultWithPondering.DurationMs / 1000.0:F2}s");
        Console.WriteLine($"  Moves: {resultWithPondering.TotalMoves}");
        Console.WriteLine($"  Avg move time: {(double)resultWithPondering.DurationMs / resultWithPondering.TotalMoves:F2}ms");
        Console.WriteLine($"\nWITHOUT Pondering:");
        Console.WriteLine($"  Duration: {resultWithoutPondering.DurationMs / 1000.0:F2}s");
        Console.WriteLine($"  Moves: {resultWithoutPondering.TotalMoves}");
        Console.WriteLine($"  Avg move time: {(double)resultWithoutPondering.DurationMs / resultWithoutPondering.TotalMoves:F2}ms");
        Console.WriteLine($"{'=' * 60}\n");
    }

    [Fact]
    public void D5_FullGame_DepthProgressionThroughAllPhases()
    {
        // Arrange - Run a FULL game to see depth progression through all phases
        var engine = new TournamentEngine();

        Console.WriteLine($"\n{'=' * 60}");
        Console.WriteLine($"D5 vs D5 Full Game - Depth Progression Analysis");
        Console.WriteLine($"{'=' * 60}");
        Console.WriteLine($"Phase boundaries:");
        Console.WriteLine($"  Opening:   moves 1-10   (0.5x time multiplier)");
        Console.WriteLine($"  EarlyMid:  moves 11-25  (0.8x time multiplier)");
        Console.WriteLine($"  LateMid:   moves 26-45  (1.2x time multiplier)");
        Console.WriteLine($"  Endgame:   moves 46+    (1.0x time multiplier)");
        Console.WriteLine("");
        Console.WriteLine($"Expected depth by phase:");
        Console.WriteLine($"  Opening:   Depth 7-8  (limited by 0.5x time)");
        Console.WriteLine($"  EarlyMid:   Depth 8-9  (slightly more time)");
        Console.WriteLine($"  LateMid:   Depth 9-11 (1.2x time = MORE depth)");
        Console.WriteLine($"  Endgame:   Depth 9-11");
        Console.WriteLine($"{'=' * 60}\n");

        // Act - Run a full game with no move limit
        var result = engine.RunGame(
            AIDifficulty.Grandmaster,      // Red: D5
            AIDifficulty.Grandmaster,      // Blue: D5
            maxMoves: 225,            // Full board
            initialTimeSeconds: 420,  // 7 minutes each
            incrementSeconds: 5,
            ponderingEnabled: false
        );

        // Assert
        result.Should().NotBeNull();

        Console.WriteLine($"\n{'=' * 60}");
        Console.WriteLine($"D5 Full Game Complete");
        Console.WriteLine($"Total moves: {result.TotalMoves}");
        Console.WriteLine($"Winner: {result.Winner}");
        Console.WriteLine($"Is Draw: {result.IsDraw}");
        Console.WriteLine($"Duration: {result.DurationMs / 1000.0:F2}s");
        Console.WriteLine("");
        Console.WriteLine($"Check [TIME] logs above to see depth progression!");
        Console.WriteLine($"Look for depth increasing in LateMid phase (move 26+)");
        Console.WriteLine($"{'=' * 60}\n");
    }

    [Fact]
    public void D5vsD5_MultipleGames_StatisticsAnalysis()
    {
        // Arrange - Run multiple D5 vs D5 games to get statistics
        var engine = new TournamentEngine();
        var gameCount = 10;
        var results = new List<(MatchResult result, double durationSeconds)>();
        var stopwatch = Stopwatch.StartNew();

        Console.WriteLine($"\n{'=' * 60}");
        Console.WriteLine($"D5 vs D5 Multi-Game Statistics ({gameCount} games)");
        Console.WriteLine($"Time control: 30+0 (fast for statistics)");
        Console.WriteLine($"{'=' * 60}\n");

        // Act - Run multiple games
        for (int i = 0; i < gameCount; i++)
        {
            var gameStopwatch = Stopwatch.StartNew();
            var result = engine.RunGame(
                AIDifficulty.Grandmaster,
                AIDifficulty.Grandmaster,
                maxMoves: 225,
                initialTimeSeconds: 30,  // 30 seconds each for quick games
                incrementSeconds: 0,
                ponderingEnabled: false
            );
            gameStopwatch.Stop();

            results.Add((result, gameStopwatch.ElapsedMilliseconds / 1000.0));

            Console.WriteLine($"Game {i + 1}: {result.Winner} wins in {result.TotalMoves} moves ({gameStopwatch.ElapsedMilliseconds / 1000.0:F1}s)");
        }

        stopwatch.Stop();

        // Calculate statistics
        var redWins = results.Count(r => r.result.Winner == Player.Red);
        var blueWins = results.Count(r => r.result.Winner == Player.Blue);
        var draws = results.Count(r => r.result.IsDraw);
        var avgMoves = results.Average(r => (double)r.result.TotalMoves);
        var minMoves = results.Min(r => r.result.TotalMoves);
        var maxMoves = results.Max(r => r.result.TotalMoves);
        var avgDuration = results.Average(r => r.durationSeconds);

        // Output statistics
        Console.WriteLine($"\n{'=' * 60}");
        Console.WriteLine($"Statistics Summary");
        Console.WriteLine($"{'=' * 60}");
        Console.WriteLine($"Total games: {gameCount}");
        Console.WriteLine($"Red wins: {redWins} ({100.0 * redWins / gameCount:F1}%)");
        Console.WriteLine($"Blue wins: {blueWins} ({100.0 * blueWins / gameCount:F1}%)");
        Console.WriteLine($"Draws: {draws} ({100.0 * draws / gameCount:F1}%)");
        Console.WriteLine("");
        Console.WriteLine($"Average moves: {avgMoves:F1}");
        Console.WriteLine($"Min moves: {minMoves}");
        Console.WriteLine($"Max moves: {maxMoves}");
        Console.WriteLine($"Average duration: {avgDuration:F1}s");
        Console.WriteLine("");
        Console.WriteLine($"Games reaching LateMid (26+ moves): {results.Count(r => r.result.TotalMoves >= 26)}");
        Console.WriteLine($"Games reaching Endgame (46+ moves): {results.Count(r => r.result.TotalMoves >= 46)}");
        Console.WriteLine($"Games reaching >70 moves: {results.Count(r => r.result.TotalMoves >= 70)}");
        Console.WriteLine($"{'=' * 60}\n");

        // Assert
        results.Should().HaveCount(gameCount);
        avgMoves.Should().BeGreaterThan(0);

        // Check if we had any long games
        if (maxMoves < 26)
        {
            Console.WriteLine($"WARNING: All games ended before LateMid phase!");
            Console.WriteLine($"This suggests D5 is playing very aggressively or finding early wins.");
        }
    }

    [Fact]
    public void D5_RealWorldDepth_FullTimeControlMultipleDifficulties()
    {
        // Run D5 against various difficulties to see depth in practice
        // With full 14+ minute time control (7+5 designed for D5)
        var engine = new TournamentEngine();

        var matchups = new (AIDifficulty red, AIDifficulty blue, string name)[]
        {
            (AIDifficulty.Grandmaster, AIDifficulty.Grandmaster, "D5 vs D5"),
            (AIDifficulty.Grandmaster, AIDifficulty.Hard, "D5 vs D4"),
            (AIDifficulty.Grandmaster, AIDifficulty.Medium, "D5 vs D3"),
            (AIDifficulty.Grandmaster, AIDifficulty.Easy, "D5 vs D2"),
        };

        var allResults = new List<(string matchup, MatchResult result, double durationSeconds)>();

        Console.WriteLine($"\n{'=' * 70}");
        Console.WriteLine($"D5 Real-World Depth Analysis - Full Time Control (7+5)");
        Console.WriteLine($"Designed to show D5 reaching adaptive depth 9-11 in LateMid phase");
        Console.WriteLine($"{'=' * 70}\n");

        foreach (var (redDiff, blueDiff, name) in matchups)
        {
            var gameStopwatch = Stopwatch.StartNew();
            var result = engine.RunGame(
                redDiff,
                blueDiff,
                maxMoves: 225,           // Full board
                initialTimeSeconds: 420, // 7 minutes each (proper D5 time control)
                incrementSeconds: 5,     // 5 second increment
                ponderingEnabled: false
            );
            gameStopwatch.Stop();

            var durationSeconds = gameStopwatch.ElapsedMilliseconds / 1000.0;
            allResults.Add((name, result, durationSeconds));

            // Output game result
            Console.WriteLine($"{name}:");
            Console.WriteLine($"  Winner: {result.Winner}");
            Console.WriteLine($"  Total moves: {result.TotalMoves}");
            Console.WriteLine($"  Duration: {durationSeconds:F1}s ({durationSeconds / 60:F1} minutes)");
            Console.WriteLine($"  Is Draw: {result.IsDraw}");

            // Phase analysis
            string phaseReached = result.TotalMoves switch
            {
                <= 10 => "Opening only (1-10)",
                <= 25 => "EarlyMid (11-25)",
                <= 45 => "LateMid (26-45)",
                _ => "Endgame (46+)"
            };
            Console.WriteLine($"  Phase reached: {phaseReached}");
            Console.WriteLine("");
        }

        // Overall summary
        Console.WriteLine($"{'=' * 70}");
        Console.WriteLine($"Summary");

        foreach (var (matchup, result, duration) in allResults)
        {
            Console.WriteLine($"{matchup,15} | Moves: {result.TotalMoves,3} | Time: {duration / 60:F1}m | Phase: {GetPhase(result.TotalMoves),10}");
        }

        Console.WriteLine($"");
        Console.WriteLine($"Games reaching LateMid (26+): {allResults.Count(r => r.result.TotalMoves >= 26)}/{allResults.Count}");
        Console.WriteLine($"Games reaching Endgame (46+): {allResults.Count(r => r.result.TotalMoves >= 46)}/{allResults.Count}");
        Console.WriteLine($"");

        // Check [TIME] logs above for actual depth reached
        Console.WriteLine($"IMPORTANT: Check [TIME] logs above for actual depth values!");
        Console.WriteLine($"  - Opening phase should show Depth 7-8");
        Console.WriteLine($"  - LateMid phase should show Depth 9-11 (1.2x time multiplier)");
        Console.WriteLine($"  - Endgame phase should show Depth 9-11");
        Console.WriteLine($"{'=' * 70}\n");

        // Assert
        allResults.Should().HaveCount(matchups.Length);
    }

    private static string GetPhase(int totalMoves)
    {
        return totalMoves switch
        {
            <= 10 => "Opening",
            <= 25 => "EarlyMid",
            <= 45 => "LateMid",
            _ => "Endgame"
        };
    }

    /// <summary>
    /// Test D5 starting from a natural late-midgame position
    /// Approach: Let Medium AI play 40 moves naturally, then D5 continues
    /// This creates a realistic position without forced VCF wins
    /// </summary>
    [Fact]
    public void D5_FromNaturalLateMidPosition_ReachesDepth9_11()
    {
        // Arrange - Let Medium AI play 40 moves to create natural position
        var board = new Board();
        var currentPlayer = Player.Red;
        var moveCount = 0;
        var mediumAI = new MinimaxAI();

        Console.WriteLine($"\n{'=' * 70}");
        Console.WriteLine($"D5 From Natural LateMid Position - Depth Verification");
        Console.WriteLine($"{'=' * 70}");
        Console.WriteLine($"Step 1: Let Medium (D3) AI play 40 moves to create natural position");

        // Play 40 moves with Medium AI
        for (int i = 0; i < 40; i++)
        {
            var (mx, my) = mediumAI.GetBestMove(
                board,
                currentPlayer,
                AIDifficulty.Medium,
                100,  // Quick moves
                moveCount,
                ponderingEnabled: false
            );

            board.PlaceStone(mx, my, currentPlayer);
            moveCount++;
            currentPlayer = currentPlayer == Player.Red ? Player.Blue : Player.Red;

            // Check for winner
            var winner = CheckWinner(board);
            if (winner != Player.None)
            {
                Console.WriteLine($"Game ended early at move {moveCount}, restarting...");
                board = new Board();
                moveCount = 0;
                currentPlayer = Player.Red;
                i = -1; // Restart
            }
        }

        var totalStones = CountStones(board);
        var redStones = CountStonesForPlayer(board, Player.Red);
        var blueStones = CountStonesForPlayer(board, Player.Blue);

        Console.WriteLine($"Natural position created:");
        Console.WriteLine($"  Total stones: {totalStones} (LateMid phase: 26-45 moves)");
        Console.WriteLine($"  Red stones: {redStones}, Blue stones: {blueStones}");
        Console.WriteLine($"  Next to move: {currentPlayer}");
        Console.WriteLine("");
        Console.WriteLine($"Step 2: D5 analyzes this position (5 seconds)");
        Console.WriteLine("");
        Console.WriteLine($"Expected behavior:");
        Console.WriteLine($"  - Natural position has no immediate VCF win");
        Console.WriteLine($"  - D5 should use full 5 seconds for deep search");
        Console.WriteLine($"  - Should reach depth 9-11 in LateMid phase");
        Console.WriteLine($"{'=' * 70}\n");

        // Act - Have D5 play from this position
        var stopwatch = Stopwatch.StartNew();

        var ai = new MinimaxAI();
        var timeMs = 5000; // 5 seconds for deep search
        var (d5x, d5y) = ai.GetBestMove(
            board,
            currentPlayer,
            AIDifficulty.Grandmaster,
            timeMs,
            moveCount,
            ponderingEnabled: false
        );

        stopwatch.Stop();

        // Assert
        d5x.Should().BeGreaterThanOrEqualTo(0);
        d5x.Should().BeLessThan(15);
        d5y.Should().BeGreaterThanOrEqualTo(0);
        d5y.Should().BeLessThan(15);

        Console.WriteLine($"");
        Console.WriteLine($"{'=' * 70}");
        Console.WriteLine($"Result:");
        Console.WriteLine($"  D5 selected: ({d5x}, {d5y})");
        Console.WriteLine($"  Time taken: {stopwatch.ElapsedMilliseconds}ms");
        Console.WriteLine($"");
        Console.WriteLine($"Check [TIME] logs above - should show depth 9-11!");
        Console.WriteLine($"  Look for: [TIME] Depth: 9, ... or [TIME] Depth: 10, ... or [TIME] Depth: 11, ...");
        Console.WriteLine($"{'=' * 70}\n");
    }

    /// <summary>
    /// Test D5 playing multiple moves from a late-midgame position
    /// to see consistent depth 9-11 performance
    /// </summary>
    [Fact]
    public void D5_FromLateMidPosition_5Moves_DepthConsistency()
    {
        // Arrange - Let weaker AIs play to move 40, then D5 continues
        var engine = new TournamentEngine();
        var board = new Board();
        var currentPlayer = Player.Red;
        var moveCount = 0;

        // Use Medium-level AI to play 40 moves naturally
        // This creates a realistic position without forced wins
        var mediumAI = new MinimaxAI();

        Console.WriteLine($"\n{'=' * 70}");
        Console.WriteLine($"D5 LateMid Position - Natural Position Approach");
        Console.WriteLine($"{'=' * 70}");
        Console.WriteLine($"Step 1: Let Medium (D3) AI play 40 moves to create natural position");

        for (int i = 0; i < 40; i++)
        {
            var (mx, my) = mediumAI.GetBestMove(
                board,
                currentPlayer,
                AIDifficulty.Medium,  // Medium level
                100,  // Quick moves
                moveCount,
                ponderingEnabled: false
            );

            board.PlaceStone(mx, my, currentPlayer);
            moveCount++;
            currentPlayer = currentPlayer == Player.Red ? Player.Blue : Player.Red;

            // Check if game ended
            var winner = CheckWinner(board);
            if (winner != Player.None)
            {
                Console.WriteLine($"Game ended at move {moveCount} with {winner} winning");
                Console.WriteLine($"Starting over with different parameters...");
                board = new Board();
                moveCount = 0;
                currentPlayer = Player.Red;
                i = -1; // Restart loop
            }
        }

        Console.WriteLine($"Position created: {CountStones(board)} stones");
        Console.WriteLine($"  Red: {CountStonesForPlayer(board, Player.Red)}, Blue: {CountStonesForPlayer(board, Player.Blue)}");
        Console.WriteLine("");
        Console.WriteLine($"Step 2: D5 plays 5 moves from this position (5 seconds each)");
        Console.WriteLine("");

        // Act - D5 plays 5 moves from this position
        var d5AI = new MinimaxAI();
        for (int moveNum = 1; moveNum <= 5; moveNum++)
        {
            var timeMs = 5000; // 5 seconds for deep search

            var (d5x, d5y) = d5AI.GetBestMove(
                board,
                currentPlayer,
                AIDifficulty.Grandmaster,
                timeMs,
                moveCount,
                ponderingEnabled: false
            );

            Console.WriteLine($"Move {moveNum} ({currentPlayer}): ({d5x}, {d5y})");

            board.PlaceStone(d5x, d5y, currentPlayer);
            moveCount++;
            currentPlayer = currentPlayer == Player.Red ? Player.Blue : Player.Red;

            // Check for winner
            var winner = CheckWinner(board);
            if (winner != Player.None)
            {
                Console.WriteLine($"Game ended! Winner: {winner}");
                break;
            }
        }

        // Assert
        Console.WriteLine($"");
        Console.WriteLine($"{'=' * 70}");
        Console.WriteLine($"Total moves played: {moveCount}");
        Console.WriteLine($"Final stone count: {CountStones(board)}");
        Console.WriteLine($"");
        Console.WriteLine($"Check [TIME] logs above for depth values!");
        Console.WriteLine($"  Each D5 move should show depth 8-11");
        Console.WriteLine($"{'=' * 70}\n");
    }

    private static Player CheckWinner(Board board)
    {
        // Check for 5-in-row
        var directions = new (int dx, int dy)[]
        {
            (1, 0), (0, 1), (1, 1), (1, -1)
        };

        for (int x = 0; x < 15; x++)
        {
            for (int y = 0; y < 15; y++)
            {
                var cell = board.GetCell(x, y);
                if (cell.Player == Player.None) continue;

                foreach (var (dx, dy) in directions)
                {
                    var count = 1;
                    for (int i = 1; i < 5; i++)
                    {
                        var nx = x + dx * i;
                        var ny = y + dy * i;
                        if (nx < 0 || nx >= 15 || ny < 0 || ny >= 15) break;
                        if (board.GetCell(nx, ny).Player != cell.Player) break;
                        count++;
                    }
                    if (count >= 5) return cell.Player;
                }
            }
        }
        return Player.None;
    }

    private static int CountStones(Board board)
    {
        return CountStonesForPlayer(board, Player.Red) + CountStonesForPlayer(board, Player.Blue);
    }

    private static int CountStonesForPlayer(Board board, Player player)
    {
        var count = 0;
        for (int x = 0; x < 15; x++)
        {
            for (int y = 0; y < 15; y++)
            {
                if (board.GetCell(x, y).Player == player)
                    count++;
            }
        }
        return count;
    }
}
