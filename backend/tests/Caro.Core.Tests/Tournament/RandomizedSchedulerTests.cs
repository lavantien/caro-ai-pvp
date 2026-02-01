using Xunit;
using Caro.Core.Tournament;
using Caro.Core.GameLogic;
using FluentAssertions;

namespace Caro.Core.Tests.Tournament;

/// <summary>
/// Tests for randomized tournament matchmaking
/// Verifies that:
/// 1. Schedule is randomized (different order each time)
/// 2. First round has all unique bots (balanced play constraint)
/// 3. Total games count is correct for round-robin
/// 4. No duplicate match IDs exist
/// </summary>
public class RandomizedSchedulerTests
{
    [Fact]
    public void GenerateRoundRobinSchedule_RandomStartingPairing()
    {
        var bots = AIBotFactory.GetAllTournamentBots();

        // Generate two schedules with a small delay to ensure different random seeds
        var schedule1 = TournamentScheduler.GenerateRoundRobinSchedule(bots);

        // Small delay to ensure different random seed
        Thread.Sleep(10);

        var schedule2 = TournamentScheduler.GenerateRoundRobinSchedule(bots);

        // First matches should be different (randomized)
        // Note: There's a very small chance they could be the same by random chance
        // but for 22 bots (462 matches), probability is extremely low
        schedule1[0].MatchId.Should().NotBe(schedule2[0].MatchId,
            "first matches should differ due to randomization");
    }

    [Fact]
    public void GenerateRoundRobinSchedule_FirstRound_AllUniqueBots()
    {
        var bots = AIBotFactory.GetAllTournamentBots();
        var matches = TournamentScheduler.GenerateRoundRobinSchedule(bots);

        // First half of matches should have all unique bots (no bot plays twice in first round)
        var firstRoundBots = new HashSet<string>();
        int firstRoundCount = bots.Count / 2;
        for (int i = 0; i < firstRoundCount; i++)
        {
            firstRoundBots.Contains(matches[i].RedBot.Name).Should().BeFalse(
                $"Red bot {matches[i].RedBot.Name} already played in first round at index {i}");
            firstRoundBots.Contains(matches[i].BlueBot.Name).Should().BeFalse(
                $"Blue bot {matches[i].BlueBot.Name} already played in first round at index {i}");

            firstRoundBots.Add(matches[i].RedBot.Name);
            firstRoundBots.Add(matches[i].BlueBot.Name);
        }

        firstRoundBots.Count.Should().Be(bots.Count);
    }

    [Fact]
    public void GenerateRoundRobinSchedule_TotalGames_Correct()
    {
        var bots = AIBotFactory.GetAllTournamentBots();
        var matches = TournamentScheduler.GenerateRoundRobinSchedule(bots);

        // Each bot plays each opponent twice (once as Red, once as Blue)
        // Total games = n * (n-1) where n is number of bots
        int expectedGames = TournamentScheduler.CalculateTotalGames(bots.Count);
        matches.Count.Should().Be(expectedGames);
    }

    [Fact]
    public void GenerateRoundRobinSchedule_SmallerSet_TotalGamesCorrect()
    {
        // Test with smaller bot set for faster verification
        var bots = new List<AIBot>
        {
            new AIBot { Name = "Bot1", Difficulty = AIDifficulty.Easy, ELO = 600 },
            new AIBot { Name = "Bot2", Difficulty = AIDifficulty.Easy, ELO = 600 },
            new AIBot { Name = "Bot3", Difficulty = AIDifficulty.Medium, ELO = 600 },
            new AIBot { Name = "Bot4", Difficulty = AIDifficulty.Medium, ELO = 600 }
        };

        var matches = TournamentScheduler.GenerateRoundRobinSchedule(bots);

        // 4 bots, each plays 3 opponents twice = 4 * 3 = 12 games
        matches.Count.Should().Be(12);
    }

    [Fact]
    public void GenerateRoundRobinSchedule_SmallerSet_FirstRoundUniqueBots()
    {
        var bots = new List<AIBot>
        {
            new AIBot { Name = "Bot1", Difficulty = AIDifficulty.Easy, ELO = 600 },
            new AIBot { Name = "Bot2", Difficulty = AIDifficulty.Easy, ELO = 600 },
            new AIBot { Name = "Bot3", Difficulty = AIDifficulty.Medium, ELO = 600 },
            new AIBot { Name = "Bot4", Difficulty = AIDifficulty.Medium, ELO = 600 }
        };

        var matches = TournamentScheduler.GenerateRoundRobinSchedule(bots);

        // First 2 matches should have all 4 unique bots
        var firstRoundBots = new HashSet<string>();
        int roundSize = bots.Count / 2; // 2 matches for 4 bots

        for (int i = 0; i < roundSize; i++)
        {
            firstRoundBots.Contains(matches[i].RedBot.Name).Should().BeFalse();
            firstRoundBots.Contains(matches[i].BlueBot.Name).Should().BeFalse();

            firstRoundBots.Add(matches[i].RedBot.Name);
            firstRoundBots.Add(matches[i].BlueBot.Name);
        }

        firstRoundBots.Count.Should().Be(4);
    }

    [Fact]
    public void GenerateRoundRobinSchedule_EachBotHasCorrectNumberOfGames()
    {
        var bots = new List<AIBot>
        {
            new AIBot { Name = "Bot1", Difficulty = AIDifficulty.Easy, ELO = 600 },
            new AIBot { Name = "Bot2", Difficulty = AIDifficulty.Easy, ELO = 600 },
            new AIBot { Name = "Bot3", Difficulty = AIDifficulty.Medium, ELO = 600 },
            new AIBot { Name = "Bot4", Difficulty = AIDifficulty.Medium, ELO = 600 }
        };

        var matches = TournamentScheduler.GenerateRoundRobinSchedule(bots);

        // Count games for each bot
        var botGameCounts = new Dictionary<string, int>();
        foreach (var bot in bots)
        {
            botGameCounts[bot.Name] = 0;
        }

        foreach (var match in matches)
        {
            botGameCounts[match.RedBot.Name]++;
            botGameCounts[match.BlueBot.Name]++;
        }

        // Each bot should play 2 * (n-1) = 6 games (each opponent twice)
        foreach (var kvp in botGameCounts)
        {
            kvp.Value.Should().Be(6,
                $"Bot {kvp.Key} should play 6 games, played {kvp.Value}");
        }
    }

    [Fact]
    public void GenerateRoundRobinSchedule_NoDuplicateMatchIds()
    {
        var bots = AIBotFactory.GetAllTournamentBots();
        var matches = TournamentScheduler.GenerateRoundRobinSchedule(bots);

        var matchIds = new HashSet<string>();
        var duplicates = new List<string>();

        foreach (var match in matches)
        {
            if (matchIds.Contains(match.MatchId))
            {
                duplicates.Add(match.MatchId);
            }
            matchIds.Add(match.MatchId);
        }

        duplicates.Should().BeEmpty($"Found duplicate match IDs: {string.Join(", ", duplicates)}");
    }

    [Fact]
    public void GenerateRoundRobinSchedule_RandomizationDistribution_Verified()
    {
        // Run multiple times and verify first matches are distributed
        var bots = new List<AIBot>
        {
            new AIBot { Name = "Bot1", Difficulty = AIDifficulty.Easy, ELO = 600 },
            new AIBot { Name = "Bot2", Difficulty = AIDifficulty.Easy, ELO = 600 },
            new AIBot { Name = "Bot3", Difficulty = AIDifficulty.Medium, ELO = 600 },
            new AIBot { Name = "Bot4", Difficulty = AIDifficulty.Medium, ELO = 600 }
        };

        var firstMatches = new HashSet<string>();

        // Generate 50 schedules and collect first matches
        for (int i = 0; i < 50; i++)
        {
            var schedule = TournamentScheduler.GenerateRoundRobinSchedule(bots);
            firstMatches.Add(schedule[0].MatchId);
            Thread.Sleep(1); // Small delay for different random seed
        }

        // With 12 possible matches and 50 trials, we should see significant variety
        // Not all 12, but certainly more than 1-2
        firstMatches.Count.Should().BeGreaterThan(3,
            $"Randomization appears broken - only saw {firstMatches.Count} unique first matches out of 50 trials");
    }

    [Fact]
    public void CalculateTotalGames_ReturnsCorrectValue()
    {
        // n * (n-1) for round-robin where each pair plays twice
        TournamentScheduler.CalculateTotalGames(4).Should().Be(12);  // 4*3 = 12
        TournamentScheduler.CalculateTotalGames(5).Should().Be(20);  // 5*4 = 20
        TournamentScheduler.CalculateTotalGames(22).Should().Be(462); // 22*21 = 462
    }
}
