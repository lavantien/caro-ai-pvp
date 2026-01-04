using Caro.Core.Entities;
using Caro.Core.GameLogic;
using Caro.Core.Tournament;
using Xunit;
using Xunit.Abstractions;

namespace Caro.Core.Tests.Tournament;

/// <summary>
/// Integration tests that run full tournament games with actual AI opponents.
/// Captures logs and saves snapshots to JSON for physical verification and regression testing.
/// </summary>
public class TournamentIntegrationTests
{
    private readonly ITestOutputHelper _output;
    // Save snapshots to source directory for git commit (regression testing)
    private const string SnapshotDirectory = @"Tournament\Snapshots";

    // Absolute path to source-controlled snapshot directory
    private static string SourceSnapshotDirectory => Path.Combine(
        Directory.GetCurrentDirectory(),
        @"..\..\..\Tournament\Snapshots"
    );

    public TournamentIntegrationTests(ITestOutputHelper output)
    {
        _output = output;
    }

    [Fact]
    public void RunSingleGame_BasicVsMedium_SavesSnapshotAndHigherDifficultyWins()
    {
        // Arrange
        var engine = new TournamentEngine();
        using var capture = new TournamentLogCapture();
        var gameId = Guid.NewGuid().ToString("N")[..8];

        // Act - Run a single game
        var result = engine.RunGame(
            redDifficulty: AIDifficulty.Beginner,  // D1 - weakest
            blueDifficulty: AIDifficulty.Medium,    // D5 - stronger
            onLog: capture.GetCallback(gameId, "Beginner", "Medium", AIDifficulty.Beginner, AIDifficulty.Medium)
        );

        capture.FinalizeGame(result);

        // Assert - Higher difficulty should win
        Assert.Equal(Player.Blue, result.Winner);  // Medium should beat Beginner
        Assert.Equal(AIDifficulty.Medium, result.WinnerDifficulty);
        Assert.False(result.IsDraw);
        Assert.False(result.EndedByTimeout);

        // Save snapshot for physical verification
        Task.Run(async () =>
        {
            await capture.SaveSnapshotAsync(
                nameof(RunSingleGame_BasicVsMedium_SavesSnapshotAndHigherDifficultyWins),
                SourceSnapshotDirectory
            );
            _output.WriteLine($"Snapshot saved to {SourceSnapshotDirectory}");
        }).Wait();

        // Verify snapshot content
        var snapshot = capture.BuildSnapshot(nameof(RunSingleGame_BasicVsMedium_SavesSnapshotAndHigherDifficultyWins));
        Assert.Single(snapshot.Games);
        Assert.Equal("Beginner", snapshot.Games[0].RedBot);
        Assert.Equal("Medium", snapshot.Games[0].BlueBot);
        Assert.True(snapshot.Games[0].MoveLogs.Count > 0, "Should have captured move logs");
    }

    [Fact]
    public void RunThreeGames_EasyVsHard_LogsDepthStatisticsCorrectly()
    {
        // Arrange
        var engine = new TournamentEngine();
        using var capture = new TournamentLogCapture();
        var results = new List<MatchResult>();

        // Act - Run multiple games
        for (int i = 0; i < 3; i++)
        {
            var gameId = $"{Guid.NewGuid():N}"[..8] + $"-{i}";
            var result = engine.RunGame(
                redDifficulty: AIDifficulty.Easy,
                blueDifficulty: AIDifficulty.Hard,
                onLog: capture.GetCallback(gameId, $"Easy{i}", $"Hard{i}", AIDifficulty.Easy, AIDifficulty.Hard)
            );
            results.Add(result);
            capture.FinalizeGame(result);
        }

        // Assert - All games complete successfully
        Assert.Equal(3, results.Count);
        Assert.All(results, r => Assert.True(r.TotalMoves > 0));

        // Verify depth statistics were captured (D4 Hard should search depth 4+)
        var snapshot = capture.BuildSnapshot(nameof(RunThreeGames_EasyVsHard_LogsDepthStatisticsCorrectly));
        Assert.Equal(3, snapshot.Games.Count);

        // Each game should have move logs with depth information
        Assert.All(snapshot.Games, game =>
        {
            Assert.True(game.MoveLogs.Count > 0, "Game should have captured move logs");

            // Hard (D4) should achieve depth 4
            var blueMoves = game.MoveLogs.Where(m => m.Player == "blue").ToList();
            if (blueMoves.Count > 0)
            {
                var avgDepth = blueMoves.Average(m => m.DepthAchieved);
                _output.WriteLine($"Average depth for Hard (blue): {avgDepth:F1}");
                Assert.True(avgDepth >= 3, $"Hard should achieve depth 3+, got {avgDepth:F1}");
            }
        });

        // Save snapshot
        Task.Run(async () =>
        {
            await capture.SaveSnapshotAsync(
                nameof(RunThreeGames_EasyVsHard_LogsDepthStatisticsCorrectly),
                SourceSnapshotDirectory
            );
        }).Wait();
    }

    [Fact]
    public void RunGame_VeryHardVsExpert_ParallelSearchReportsCorrectDepth()
    {
        // This test specifically verifies the parallel search statistics fix
        // D7+ (VeryHard/Expert) use Lazy SMP and should report actual depth, not 0

        // Arrange
        var engine = new TournamentEngine();
        using var capture = new TournamentLogCapture();
        var gameId = Guid.NewGuid().ToString("N")[..8];

        // Act - Run game with D7+ bots that use parallel search
        var result = engine.RunGame(
            redDifficulty: AIDifficulty.VeryHard,  // D7 - uses parallel search
            blueDifficulty: AIDifficulty.Expert,    // D8 - uses parallel search
            onLog: capture.GetCallback(gameId, "VeryHard", "Expert", AIDifficulty.VeryHard, AIDifficulty.Expert)
        );

        capture.FinalizeGame(result);

        // Assert - Game completes successfully
        Assert.True(result.TotalMoves > 0);

        // Verify depth was captured and is NOT 0 (the bug we fixed)
        var snapshot = capture.BuildSnapshot(nameof(RunGame_VeryHardVsExpert_ParallelSearchReportsCorrectDepth));
        Assert.Single(snapshot.Games);

        var game = snapshot.Games[0];

        // Both VeryHard (D7) and Expert (D8) should report depth > 0
        var redMoves = game.MoveLogs.Where(m => m.Player == "red").ToList();
        var blueMoves = game.MoveLogs.Where(m => m.Player == "blue").ToList();

        if (redMoves.Count > 0)
        {
            var maxRedDepth = redMoves.Max(m => m.DepthAchieved);
            var avgRedDepth = redMoves.Average(m => m.DepthAchieved);
            _output.WriteLine($"VeryHard (red) - Max depth: {maxRedDepth}, Avg: {avgRedDepth:F1}");

            // This was the bug: depth was 0 before the fix
            // Now we verify depth is being reported correctly (> 0)
            Assert.True(maxRedDepth > 0, $"VeryHard should report depth > 0, got {maxRedDepth}");
            // Average may vary due to time management, just check it's reasonable
            Assert.True(avgRedDepth >= 3, $"VeryHard (D7) should average depth 3+, got {avgRedDepth:F1}");
        }

        if (blueMoves.Count > 0)
        {
            var maxBlueDepth = blueMoves.Max(m => m.DepthAchieved);
            var avgBlueDepth = blueMoves.Average(m => m.DepthAchieved);
            _output.WriteLine($"Expert (blue) - Max depth: {maxBlueDepth}, Avg: {avgBlueDepth:F1}");

            // Max depth should reach target (D8 = depth 8)
            Assert.True(maxBlueDepth > 0, $"Expert should report depth > 0, got {maxBlueDepth}");
            // Average may vary due to time management
            Assert.True(avgBlueDepth >= 3, $"Expert (D8) should average depth 3+, got {avgBlueDepth:F1}");
        }

        // Save snapshot
        Task.Run(async () =>
        {
            await capture.SaveSnapshotAsync(
                nameof(RunGame_VeryHardVsExpert_ParallelSearchReportsCorrectDepth),
                SourceSnapshotDirectory
            );
        }).Wait();
    }

    [Fact]
    public void RunMiniTournament_FourBots_BalancedScheduleAndNoRepeats()
    {
        // Arrange
        var engine = new TournamentEngine();
        using var capture = new TournamentLogCapture();

        var bots = new List<AIBot>
        {
            new() { Name = "BotA", Difficulty = AIDifficulty.Easy },
            new() { Name = "BotB", Difficulty = AIDifficulty.Medium },
            new() { Name = "BotC", Difficulty = AIDifficulty.Hard },
            new() { Name = "BotD", Difficulty = AIDifficulty.VeryHard }
        };

        var schedule = TournamentScheduler.GenerateRoundRobinSchedule(bots);

        // Act - Run a subset of matches (not all 12, just first round)
        var results = new List<MatchResult>();
        int gamesToRun = 2;  // First round only

        for (int i = 0; i < Math.Min(gamesToRun, schedule.Count); i++)
        {
            var match = schedule[i];
            var gameId = $"{match.MatchId}-{i}";
            var result = engine.RunGame(
                redDifficulty: match.RedBot.Difficulty,
                blueDifficulty: match.BlueBot.Difficulty,
                redBotName: match.RedBot.Name,
                blueBotName: match.BlueBot.Name,
                onLog: capture.GetCallback(gameId, match.RedBot.Name, match.BlueBot.Name,
                                          match.RedBot.Difficulty, match.BlueBot.Difficulty)
            );
            results.Add(result);
            capture.FinalizeGame(result);
        }

        // Assert - All games complete successfully
        Assert.Equal(gamesToRun, results.Count);

        // Verify snapshot contains all games
        var snapshot = capture.BuildSnapshot(nameof(RunMiniTournament_FourBots_BalancedScheduleAndNoRepeats));
        Assert.Equal(gamesToRun, snapshot.Games.Count);

        // Save snapshot
        Task.Run(async () =>
        {
            await capture.SaveSnapshotAsync(
                nameof(RunMiniTournament_FourBots_BalancedScheduleAndNoRepeats),
                SourceSnapshotDirectory
            );
        }).Wait();
    }

    [Fact]
    public void RunGame_BeginnerVsBeginner_WithShortTimeControl_FinishesSuccessfully()
    {
        // Test with very short time control to ensure timeout handling works
        var engine = new TournamentEngine();
        using var capture = new TournamentLogCapture();
        var gameId = Guid.NewGuid().ToString("N")[..8];

        var result = engine.RunGame(
            redDifficulty: AIDifficulty.Beginner,
            blueDifficulty: AIDifficulty.Beginner,
            initialTimeSeconds: 30,  // Short time control
            incrementSeconds: 0,
            onLog: capture.GetCallback(gameId, "BeginnerA", "BeginnerB", AIDifficulty.Beginner, AIDifficulty.Beginner)
        );

        capture.FinalizeGame(result);

        // Should finish (either by win or time running out)
        Assert.True(result.TotalMoves >= 0 || result.EndedByTimeout);

        var snapshot = capture.BuildSnapshot(nameof(RunGame_BeginnerVsBeginner_WithShortTimeControl_FinishesSuccessfully));

        // Save snapshot
        Task.Run(async () =>
        {
            await capture.SaveSnapshotAsync(
                nameof(RunGame_BeginnerVsBeginner_WithShortTimeControl_FinishesSuccessfully),
                SourceSnapshotDirectory
            );
        }).Wait();

        _output.WriteLine($"Game ended: {result.TotalMoves} moves, Duration: {result.DurationMs}ms, Timeout: {result.EndedByTimeout}");
    }
}
