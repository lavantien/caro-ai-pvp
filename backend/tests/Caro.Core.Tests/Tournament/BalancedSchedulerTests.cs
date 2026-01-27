using Caro.Core.GameLogic;
using Caro.Core.Tournament;
using Xunit;

namespace Caro.Core.Tests.Tournament;

/// <summary>
/// Tests for balanced round-robin tournament scheduling
/// Ensures fair match distribution where each bot plays at most once per round
/// </summary>
public class BalancedSchedulerTests
{
    [Fact]
    public void GenerateRoundRobinSchedule_With4Bots_CreatesCorrectTotalMatches()
    {
        // Arrange: 4 bots should generate 4 * (4-1) = 12 matches
        var bots = new List<AIBot>
        {
            new() { Name = "A", Difficulty = AIDifficulty.Braindead },
            new() { Name = "B", Difficulty = AIDifficulty.Easy },
            new() { Name = "C", Difficulty = AIDifficulty.Easy },
            new() { Name = "D", Difficulty = AIDifficulty.Medium }
        };

        // Act
        var matches = TournamentScheduler.GenerateRoundRobinSchedule(bots);

        // Assert: 4 bots * 3 opponents each = 12 matches (each pair plays twice)
        Assert.Equal(12, matches.Count);
    }

    [Fact]
    public void GenerateRoundRobinSchedule_EachBotPlaysEachOpponentTwice()
    {
        // Arrange
        var bots = new List<AIBot>
        {
            new() { Name = "A", Difficulty = AIDifficulty.Braindead },
            new() { Name = "B", Difficulty = AIDifficulty.Easy },
            new() { Name = "C", Difficulty = AIDifficulty.Easy }
        };

        // Act
        var matches = TournamentScheduler.GenerateRoundRobinSchedule(bots);

        // Assert: Each pair should play twice (once as Red, once as Blue)
        // A vs B (A red), B vs A (B red)
        // A vs C (A red), C vs A (C red)
        // B vs C (B red), C vs B (C red)
        Assert.Equal(6, matches.Count);

        // Verify A plays B twice
        var aVsBMatches = matches.Where(m =>
            (m.RedBot.Name == "A" && m.BlueBot.Name == "B") ||
            (m.RedBot.Name == "B" && m.BlueBot.Name == "A")).ToList();
        Assert.Equal(2, aVsBMatches.Count);
    }

    [Fact]
    public void GenerateRoundRobinSchedule_BalancedDistribution_EachBotPlaysAtMostOncePerRound()
    {
        // Arrange
        var bots = new List<AIBot>
        {
            new() { Name = "A", Difficulty = AIDifficulty.Braindead },
            new() { Name = "B", Difficulty = AIDifficulty.Easy },
            new() { Name = "C", Difficulty = AIDifficulty.Easy },
            new() { Name = "D", Difficulty = AIDifficulty.Medium }
        };

        // Act
        var matches = TournamentScheduler.GenerateRoundRobinSchedule(bots);

        // The key assertion: In the first (botCount/2) matches, each bot appears at most once
        // For 4 bots, we can have 2 matches in parallel (4 bots / 2 per match)
        // So the first 2 matches should have 4 unique bots
        var uniqueBotsInFirstTwoMatches = new HashSet<string>();
        for (int i = 0; i < 2 && i < matches.Count; i++)
        {
            uniqueBotsInFirstTwoMatches.Add(matches[i].RedBot.Name);
            uniqueBotsInFirstTwoMatches.Add(matches[i].BlueBot.Name);
        }

        // For 4 bots, first 2 matches should have all 4 bots (no repeats)
        Assert.Equal(4, uniqueBotsInFirstTwoMatches.Count);

        // Additionally verify: no bot appears in both match 0 AND match 1
        var match0Bots = new HashSet<string> { matches[0].RedBot.Name, matches[0].BlueBot.Name };
        var match1Bots = new HashSet<string> { matches[1].RedBot.Name, matches[1].BlueBot.Name };
        Assert.Empty(match0Bots.Intersect(match1Bots));
    }

    [Fact]
    public void GenerateRoundRobinSchedule_With22Bots_First11Matches_HaveAllUniqueBots()
    {
        // Arrange: Use actual tournament bots
        var bots = AIBotFactory.GetAllTournamentBots();

        // Act
        var matches = TournamentScheduler.GenerateRoundRobinSchedule(bots);

        // Assert: First 11 matches should involve all 22 bots exactly once
        // (22 bots / 2 per match = 11 matches can run in parallel)
        var botAppearances = new Dictionary<string, int>();
        foreach (var bot in bots)
        {
            botAppearances[bot.Name] = 0;
        }

        // Check first 11 matches
        for (int i = 0; i < 11 && i < matches.Count; i++)
        {
            botAppearances[matches[i].RedBot.Name]++;
            botAppearances[matches[i].BlueBot.Name]++;
        }

        // Each bot should appear exactly once in the first 11 matches
        var failures = new List<string>();
        foreach (var bot in bots)
        {
            if (botAppearances[bot.Name] != 1)
            {
                failures.Add($"Bot {bot.Name} should appear exactly once in first 11 matches, but appeared {botAppearances[bot.Name]} times");
            }
        }
        if (failures.Count > 0)
        {
            Assert.Fail(string.Join("\n", failures));
        }

        // Also verify total unique bots in first 11 matches is 22
        var uniqueBots = new HashSet<string>();
        for (int i = 0; i < 11 && i < matches.Count; i++)
        {
            uniqueBots.Add(matches[i].RedBot.Name);
            uniqueBots.Add(matches[i].BlueBot.Name);
        }
        Assert.Equal(22, uniqueBots.Count);
    }

    [Fact]
    public void GenerateRoundRobinSchedule_FirstMatchNotAlwaysSameBot()
    {
        // Arrange
        var bots = AIBotFactory.GetAllTournamentBots();

        // Act
        var matches = TournamentScheduler.GenerateRoundRobinSchedule(bots);

        // Assert: First match should NOT be Novice Alpha vs someone
        // With balanced scheduling, it should be distributed
        // Check that Novice Alpha doesn't appear in the first few matches too often
        var noviceAlpha = bots.First(b => b.Name.Contains("Novice") && b.Name.EndsWith("Alpha"));

        int noviceAlphaInFirst5 = 0;
        for (int i = 0; i < 5 && i < matches.Count; i++)
        {
            if (matches[i].RedBot.Name == noviceAlpha.Name || matches[i].BlueBot.Name == noviceAlpha.Name)
            {
                noviceAlphaInFirst5++;
            }
        }

        // Novice Alpha should appear at most once in first 5 matches
        Assert.True(noviceAlphaInFirst5 <= 1,
            $"Novice Alpha appeared {noviceAlphaInFirst5} times in first 5 matches (should be at most 1)");
    }

    [Fact]
    public void GenerateRoundRobinSchedule_LogsFullSchedule()
    {
        // Arrange
        var bots = AIBotFactory.GetAllTournamentBots();

        // Act
        var matches = TournamentScheduler.GenerateRoundRobinSchedule(bots);

        // Assert - log for visual verification
        Console.WriteLine($"Total matches: {matches.Count}");
        Console.WriteLine("First 22 matches (should have all bots appear once):");
        for (int i = 0; i < Math.Min(22, matches.Count); i++)
        {
            Console.WriteLine($"  {i + 1}. {matches[i].RedBot.Name} ({matches[i].RedBot.Difficulty}) vs {matches[i].BlueBot.Name} ({matches[i].BlueBot.Difficulty})");
        }

        // Verify distribution: count appearances in first 11 matches (first round)
        var appearances = new Dictionary<string, int>();
        foreach (var bot in bots)
        {
            appearances[bot.Name] = 0;
        }

        for (int i = 0; i < Math.Min(11, matches.Count); i++)
        {
            appearances[matches[i].RedBot.Name]++;
            appearances[matches[i].BlueBot.Name]++;
        }

        Console.WriteLine("\nBot appearances in first 11 matches (each should be exactly 1):");
        foreach (var kvp in appearances.OrderBy(a => a.Key))
        {
            Console.WriteLine($"  {kvp.Key}: {kvp.Value}");
        }
    }
}
