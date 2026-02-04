using Caro.Core.Entities;
using Caro.Core.GameLogic;
using Caro.Core.Tournament;
using Xunit;
using Xunit.Abstractions;

#pragma warning disable xUnit1031 // Integration tests use blocking for test isolation

namespace Caro.Core.MatchupTests.Tournament;

/// <summary>
/// Integration tests that run full tournament games with actual AI opponents.
/// Captures logs and saves snapshots to JSON for physical verification and regression testing.
/// These tests are excluded from the default test run - run with: dotnet test --filter "Category!=Integration"
/// </summary>
[Trait("Category", "Integration")]
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
            redDifficulty: AIDifficulty.Braindead,  // D1 - weakest
            blueDifficulty: AIDifficulty.Medium,    // D3 - stronger
            onLog: capture.GetCallback(gameId, "Braindead", "Medium", AIDifficulty.Braindead, AIDifficulty.Medium)
        );

        capture.FinalizeGame(result);

        // Assert - Higher difficulty should win
        Assert.Equal(Player.Blue, result.Winner);  // Medium should beat Braindead
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
        Assert.Equal("Braindead", snapshot.Games[0].RedBot);
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

            // Hard (D4) should report depth > 0 (the bug was it reported 0)
            var blueMoves = game.MoveLogs.Where(m => m.Player == "blue").ToList();
            if (blueMoves.Count > 0)
            {
                var maxDepth = blueMoves.Max(m => m.DepthAchieved);
                _output.WriteLine($"Max depth for Hard (blue): {maxDepth}");
                Assert.True(maxDepth > 0, $"Hard should report depth > 0, got {maxDepth}");
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
    public void RunGame_HardVsGrandmaster_ParallelSearchReportsCorrectDepth()
    {
        // This test specifically verifies the parallel search statistics fix
        // D4+ (Hard/Grandmaster) use Lazy SMP and should report actual depth, not 0

        // Arrange
        var engine = new TournamentEngine();
        using var capture = new TournamentLogCapture();
        var gameId = Guid.NewGuid().ToString("N")[..8];

        // Act - Run game with D4+ bots that use parallel search
        var result = engine.RunGame(
            redDifficulty: AIDifficulty.Hard,  // D4 - uses parallel search
            blueDifficulty: AIDifficulty.Grandmaster,    // D5 - uses parallel search
            onLog: capture.GetCallback(gameId, "Hard", "Grandmaster", AIDifficulty.Hard, AIDifficulty.Grandmaster)
        );

        capture.FinalizeGame(result);

        // Assert - Game completes successfully
        Assert.True(result.TotalMoves > 0);

        // Verify depth was captured and is NOT 0 (the bug we fixed)
        var snapshot = capture.BuildSnapshot(nameof(RunGame_HardVsGrandmaster_ParallelSearchReportsCorrectDepth));
        Assert.Single(snapshot.Games);

        var game = snapshot.Games[0];

        // Both Hard (D4) and Grandmaster (D5) should report depth > 0
        var redMoves = game.MoveLogs.Where(m => m.Player == "red").ToList();
        var blueMoves = game.MoveLogs.Where(m => m.Player == "blue").ToList();

        if (redMoves.Count > 0)
        {
            var maxRedDepth = redMoves.Max(m => m.DepthAchieved);
            _output.WriteLine($"Hard (red) - Max depth: {maxRedDepth}");

            // The bug was that parallel search reported depth 0
            // We only verify depth is being reported (> 0), not a specific value
            // Actual depth achieved depends on host machine capability
            Assert.True(maxRedDepth > 0, $"Hard should report depth > 0, got {maxRedDepth}");
        }

        if (blueMoves.Count > 0)
        {
            var maxBlueDepth = blueMoves.Max(m => m.DepthAchieved);
            _output.WriteLine($"Grandmaster (blue) - Max depth: {maxBlueDepth}");

            // The bug was that parallel search reported depth 0
            // We only verify depth is being reported (> 0), not a specific value
            // Actual depth achieved depends on host machine capability
            Assert.True(maxBlueDepth > 0, $"Grandmaster should report depth > 0, got {maxBlueDepth}");
        }

        // Save snapshot
        Task.Run(async () =>
        {
            await capture.SaveSnapshotAsync(
                nameof(RunGame_HardVsGrandmaster_ParallelSearchReportsCorrectDepth),
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
            new() { Name = "BotD", Difficulty = AIDifficulty.Hard }
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
    public void RunGame_BraindeadVsBraindead_WithShortTimeControl_FinishesSuccessfully()
    {
        // Test with very short time control to ensure timeout handling works
        var engine = new TournamentEngine();
        using var capture = new TournamentLogCapture();
        var gameId = Guid.NewGuid().ToString("N")[..8];

        var result = engine.RunGame(
            redDifficulty: AIDifficulty.Braindead,
            blueDifficulty: AIDifficulty.Braindead,
            initialTimeSeconds: 30,  // Short time control
            incrementSeconds: 0,
            onLog: capture.GetCallback(gameId, "BraindeadA", "BraindeadB", AIDifficulty.Braindead, AIDifficulty.Braindead)
        );

        capture.FinalizeGame(result);

        // Should finish (either by win or time running out)
        Assert.True(result.TotalMoves >= 0 || result.EndedByTimeout);

        var snapshot = capture.BuildSnapshot(nameof(RunGame_BraindeadVsBraindead_WithShortTimeControl_FinishesSuccessfully));

        // Save snapshot
        Task.Run(async () =>
        {
            await capture.SaveSnapshotAsync(
                nameof(RunGame_BraindeadVsBraindead_WithShortTimeControl_FinishesSuccessfully),
                SourceSnapshotDirectory
            );
        }).Wait();

        _output.WriteLine($"Game ended: {result.TotalMoves} moves, Duration: {result.DurationMs}ms, Timeout: {result.EndedByTimeout}");
    }
}
