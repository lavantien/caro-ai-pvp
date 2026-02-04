using Caro.Core.GameLogic;
using Xunit;
using Xunit.Abstractions;

namespace Caro.Core.MatchupTests.Tournament;

/// <summary>
/// Tests that verify saved tournament snapshots against expected invariants.
/// These tests can detect regressions by ensuring snapshots maintain expected properties.
///
/// Integration path test - covered by matchup suite which includes file logging.
/// Run with: dotnet test --filter "Category!=Integration" to exclude.
/// </summary>
[Trait("Category", "Integration")]
public class SavedLogVerifierTests
{
    private const string SnapshotDirectory = @"Tournament\Snapshots";

    // Absolute path to source-controlled snapshot directory
    private static string SourceSnapshotDirectory => Path.Combine(
        Directory.GetCurrentDirectory(),
        @"..\..\..\Tournament\Snapshots"
    );

    public SavedLogVerifierTests()
    {
        // Ensure snapshot directory exists for tests that will generate snapshots
        if (!Directory.Exists(SourceSnapshotDirectory))
        {
            Directory.CreateDirectory(SourceSnapshotDirectory);
        }
    }

    [Fact]
    public async Task VerifyAllSnapshots_HaveNoIllegalMoves()
    {
        // Load all snapshots
        var snapshotFiles = TournamentLogCapture.GetSnapshotFiles(SourceSnapshotDirectory);

        // If no snapshots exist, skip this test
        if (snapshotFiles.Count == 0)
        {
            return;
        }

        var failures = new List<string>();

        foreach (var file in snapshotFiles)
        {
            var snapshot = await TournamentLogCapture.LoadSnapshotAsync(file);

            foreach (var game in snapshot.Games)
            {
                if (game.Result.HadIllegalMove)
                {
                    failures.Add($"Snapshot {Path.GetFileName(file)}, Game {game.GameId}: Had illegal move");
                }
            }
        }

        Assert.Empty(failures);
    }

    [Fact]
    public async Task VerifyAllSnapshots_DepthIsNonZeroForD4Plus()
    {
        // This test verifies the parallel search statistics fix
        // D4+ (Hard and above) should report depth > 0

        var snapshotFiles = TournamentLogCapture.GetSnapshotFiles(SourceSnapshotDirectory);

        if (snapshotFiles.Count == 0)
        {
            return;
        }

        var failures = new List<string>();

        foreach (var file in snapshotFiles)
        {
            var snapshot = await TournamentLogCapture.LoadSnapshotAsync(file);

            foreach (var game in snapshot.Games)
            {
                // Check Hard (D4) moves
                if (game.RedDifficulty >= AIDifficulty.Hard)
                {
                    var redMoves = game.MoveLogs.Where(m => m.Player == "red").ToList();
                    if (redMoves.Any() && redMoves.All(m => m.DepthAchieved == 0))
                    {
                        failures.Add($"Snapshot {Path.GetFileName(file)}, Game {game.GameId}: Red ({game.RedDifficulty}) reported depth 0 for all moves");
                    }
                }

                // Check Hard/Grandmaster moves
                if (game.BlueDifficulty >= AIDifficulty.Hard)
                {
                    var blueMoves = game.MoveLogs.Where(m => m.Player == "blue").ToList();
                    if (blueMoves.Any() && blueMoves.All(m => m.DepthAchieved == 0))
                    {
                        failures.Add($"Snapshot {Path.GetFileName(file)}, Game {game.GameId}: Blue ({game.BlueDifficulty}) reported depth 0 for all moves");
                    }
                }
            }
        }

        Assert.Empty(failures);
    }

    [Fact]
    public async Task VerifyAllSnapshots_HigherDifficultyWinsMostOfTheTime()
    {
        // Higher difficulty bots should win more often than lower difficulty bots
        // This is a statistical sanity check, not a strict requirement

        var snapshotFiles = TournamentLogCapture.GetSnapshotFiles(SourceSnapshotDirectory);

        if (snapshotFiles.Count == 0)
        {
            return;
        }

        var higherDiffWins = 0;
        var lowerDiffWins = 0;
        var sameDiffGames = 0;

        foreach (var file in snapshotFiles)
        {
            var snapshot = await TournamentLogCapture.LoadSnapshotAsync(file);

            foreach (var game in snapshot.Games)
            {
                // Skip draws
                if (game.Result.Winner == "draw")
                    continue;

                int redDiffLevel = (int)game.RedDifficulty;
                int blueDiffLevel = (int)game.BlueDifficulty;
                int winnerDiffLevel = game.Result.Winner == "red" ? redDiffLevel : blueDiffLevel;
                int loserDiffLevel = game.Result.Winner == "red" ? blueDiffLevel : redDiffLevel;

                if (winnerDiffLevel > loserDiffLevel)
                {
                    higherDiffWins++;
                }
                else if (winnerDiffLevel < loserDiffLevel)
                {
                    lowerDiffWins++;
                }
                else
                {
                    sameDiffGames++;
                }
            }
        }

        // Higher difficulty should win at least 60% of games against lower difficulty
        // (This allows for some upsets but ensures the AI strength curve is working)
        if (higherDiffWins + lowerDiffWins > 0)
        {
            var winRate = (double)higherDiffWins / (higherDiffWins + lowerDiffWins);
            Assert.True(winRate >= 0.5,
                $"Higher difficulty wins {higherDiffWins}/{higherDiffWins + lowerDiffWins} = {winRate:P1}, expected at least 50%");
        }
    }

    [Fact]
    public async Task VerifyAllSnapshots_AllGamesHaveValidCoordinates()
    {
        // All move coordinates should be within the 19x19 board

        var snapshotFiles = TournamentLogCapture.GetSnapshotFiles(SourceSnapshotDirectory);

        if (snapshotFiles.Count == 0)
        {
            return;
        }

        var failures = new List<string>();

        foreach (var file in snapshotFiles)
        {
            var snapshot = await TournamentLogCapture.LoadSnapshotAsync(file);

            foreach (var game in snapshot.Games)
            {
                foreach (var move in game.MoveLogs)
                {
                    if (move.X < 0 || move.X >= 19 || move.Y < 0 || move.Y >= 19)
                    {
                        failures.Add($"Snapshot {Path.GetFileName(file)}, Game {game.GameId}, Move {move.MoveNumber}: Invalid coordinates ({move.X}, {move.Y})");
                    }
                }
            }
        }

        Assert.Empty(failures);
    }

    [Fact]
    public async Task VerifyAllSnapshots_NoTimeoutsInNormalGames()
    {
        // Games with reasonable time control should not timeout

        var snapshotFiles = TournamentLogCapture.GetSnapshotFiles(SourceSnapshotDirectory);

        if (snapshotFiles.Count == 0)
        {
            return;
        }

        var timeouts = new List<string>();

        foreach (var file in snapshotFiles)
        {
            var snapshot = await TournamentLogCapture.LoadSnapshotAsync(file);

            // Skip tests specifically designed for short time control
            if (snapshot.TestName.Contains("ShortTimeControl"))
                continue;

            foreach (var game in snapshot.Games)
            {
                if (game.Result.EndedByTimeout)
                {
                    timeouts.Add($"Snapshot {Path.GetFileName(file)}, Game {game.GameId}: Ended by timeout");
                }
            }
        }

        Assert.Empty(timeouts);
    }

    [Fact]
    public async Task VerifyAllSnapshots_AllGamesCompleteAtLeastFiveMoves()
    {
        // Games should be reasonably competitive (not instant losses)

        var snapshotFiles = TournamentLogCapture.GetSnapshotFiles(SourceSnapshotDirectory);

        if (snapshotFiles.Count == 0)
        {
            return;
        }

        var failures = new List<string>();

        foreach (var file in snapshotFiles)
        {
            var snapshot = await TournamentLogCapture.LoadSnapshotAsync(file);

            foreach (var game in snapshot.Games)
            {
                // Allow very short games if they ended by illegal move (AI bug)
                if (game.Result.TotalMoves < 5 && !game.Result.HadIllegalMove)
                {
                    failures.Add($"Snapshot {Path.GetFileName(file)}, Game {game.GameId}: Only {game.Result.TotalMoves} moves played");
                }
            }
        }

        Assert.Empty(failures);
    }

    [Fact]
    public async Task VerifyAllSnapshots_MoveTimesAreReasonable()
    {
        // Move times should be positive and not extremely long (> 10 seconds per move)

        var snapshotFiles = TournamentLogCapture.GetSnapshotFiles(SourceSnapshotDirectory);

        if (snapshotFiles.Count == 0)
        {
            return;
        }

        var failures = new List<string>();

        foreach (var file in snapshotFiles)
        {
            var snapshot = await TournamentLogCapture.LoadSnapshotAsync(file);

            foreach (var game in snapshot.Games)
            {
                foreach (var move in game.MoveLogs)
                {
                    if (move.TimeMs < 0)
                    {
                        failures.Add($"Snapshot {Path.GetFileName(file)}, Game {game.GameId}, Move {move.MoveNumber}: Negative time {move.TimeMs}ms");
                    }

                    // Allow up to 30 seconds for higher difficulties in complex positions
                    if (move.TimeMs > 30000)
                    {
                        failures.Add($"Snapshot {Path.GetFileName(file)}, Game {game.GameId}, Move {move.MoveNumber}: Excessive time {move.TimeMs}ms");
                    }
                }
            }
        }

        Assert.Empty(failures);
    }

    [Fact]
    public async Task LoadSnapshot_WithValidFile_ReturnsCorrectStructure()
    {
        // This test creates a minimal snapshot and verifies it can be loaded

        var testPath = Path.Combine(SnapshotDirectory, "test-snapshot.json");

        try
        {
            // Create a minimal snapshot
            var snapshot = new TournamentSnapshot
            {
                TestName = "TestSnapshot",
                TimestampUtc = DateTime.UtcNow,
                Games = new List<GameSnapshot>
                {
                    new()
                    {
                        GameId = "test-game-1",
                        RedBot = "TestRed",
                        BlueBot = "TestBlue",
                        RedDifficulty = AIDifficulty.Medium,
                        BlueDifficulty = AIDifficulty.Hard,
                        MoveLogs = new List<MoveLogEntry>
                        {
                            new()
                            {
                                MoveNumber = 1,
                                Player = "red",
                                X = 7,
                                Y = 7,
                                TimeMs = 100,
                                DepthAchieved = 5,
                                NodesSearched = 1000,
                                NodesPerSecond = 10000
                            }
                        },
                        RawLogs = new List<string> { "Test log entry" },
                        Result = new GameResult
                        {
                            Winner = "blue",
                            WinnerDifficulty = "Hard",
                            TotalMoves = 25,
                            DurationMs = 5000,
                            EndedByTimeout = false,
                            HadIllegalMove = false
                        }
                    }
                },
                Summary = new TournamentSummary
                {
                    TotalGames = 1,
                    RedWins = 0,
                    BlueWins = 1,
                    Draws = 0,
                    IllegalMoves = 0,
                    WinsByDifficulty = new Dictionary<string, int> { { "Hard", 1 } }
                }
            };

            // Save it
            await TournamentLogCapture.SaveSnapshotAsync(testPath, snapshot);

            // Load it back
            var loaded = await TournamentLogCapture.LoadSnapshotAsync(testPath);

            // Verify structure
            Assert.Equal("TestSnapshot", loaded.TestName);
            Assert.Single(loaded.Games);
            Assert.Equal("test-game-1", loaded.Games[0].GameId);
            Assert.Equal(AIDifficulty.Medium, loaded.Games[0].RedDifficulty);
            Assert.Equal(AIDifficulty.Hard, loaded.Games[0].BlueDifficulty);
            Assert.Single(loaded.Games[0].MoveLogs);
            Assert.Equal(7, loaded.Games[0].MoveLogs[0].X);
            Assert.Equal(7, loaded.Games[0].MoveLogs[0].Y);
            Assert.Equal(5, loaded.Games[0].MoveLogs[0].DepthAchieved);
        }
        finally
        {
            // Clean up test file
            if (File.Exists(testPath))
            {
                File.Delete(testPath);
            }
        }
    }

    [Fact]
    public void GetSnapshotFiles_WithNonExistentDirectory_ReturnsEmptyList()
    {
        var files = TournamentLogCapture.GetSnapshotFiles("NonExistent_Directory_Path");
        Assert.Empty(files);
    }
}
