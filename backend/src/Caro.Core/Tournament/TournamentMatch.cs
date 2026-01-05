using Caro.Core.Entities;
using Caro.Core.GameLogic;

namespace Caro.Core.Tournament;

/// <summary>
/// Represents a single tournament match between two AI bots
/// </summary>
public class TournamentMatch
{
    public required string MatchId { get; set; }
    public required AIBot RedBot { get; set; }
    public required AIBot BlueBot { get; set; }
    public bool IsCompleted { get; set; }
    public bool IsInProgress { get; set; }
    public MatchResult? Result { get; set; }

    /// <summary>
    /// Gets the match display name
    /// </summary>
    public string DisplayName => $"{RedBot.Name} vs {BlueBot.Name}";
}

/// <summary>
/// Factory for creating tournament match schedules
/// </summary>
public static class TournamentScheduler
{
    /// <summary>
    /// Generates a full round-robin schedule where each bot plays every other bot twice
    /// (once as Red, once as Blue), with balanced match ordering so every bot plays
    /// approximately the same number of matches before any bot gets a second game.
    ///
    /// The schedule is randomized using Fisher-Yates shuffle with time-based seed,
    /// ensuring different pairing orders across tournaments while maintaining
    /// balanced play constraints.
    /// </summary>
    public static List<TournamentMatch> GenerateRoundRobinSchedule(List<AIBot> bots)
    {
        // First, generate all matches
        var allMatches = new List<TournamentMatch>();

        for (int i = 0; i < bots.Count; i++)
        {
            for (int j = i + 1; j < bots.Count; j++)
            {
                // First match: bot[i] as Red, bot[j] as Blue
                allMatches.Add(new TournamentMatch
                {
                    MatchId = $"{bots[i].Name}-vs-{bots[j].Name}-as-Red",
                    RedBot = bots[i],
                    BlueBot = bots[j],
                    IsCompleted = false,
                    IsInProgress = false
                });

                // Second match: bot[j] as Red, bot[i] as Blue (colors swapped)
                allMatches.Add(new TournamentMatch
                {
                    MatchId = $"{bots[j].Name}-vs-{bots[i].Name}-as-Red",
                    RedBot = bots[j],
                    BlueBot = bots[i],
                    IsCompleted = false,
                    IsInProgress = false
                });
            }
        }

        // SHUFFLE for random starting pairing
        // Use time-based seed for different randomization each tournament
        var random = new Random((int)(DateTime.UtcNow.Ticks & 0xFFFFFFFF));
        ShuffleMatches(allMatches, random);

        // Reorder matches for balanced play
        // Use a "round-based" approach where each bot plays at most once per round
        return ReorderMatchesForBalance(allMatches, bots.Count);
    }

    /// <summary>
    /// Fisher-Yates shuffle for true randomization of match order.
    /// Ensures each tournament starts with different pairings.
    /// </summary>
    private static void ShuffleMatches(List<TournamentMatch> matches, Random random)
    {
        int n = matches.Count;
        while (n > 1)
        {
            n--;
            int k = random.Next(n + 1);
            (matches[k], matches[n]) = (matches[n], matches[k]);
        }
    }

    /// <summary>
    /// Reorders matches so that each bot plays at most once per "round",
    /// ensuring fair distribution throughout the tournament.
    /// Uses a greedy algorithm to assign matches to rounds.
    /// </summary>
    private static List<TournamentMatch> ReorderMatchesForBalance(List<TournamentMatch> matches, int botCount)
    {
        var result = new List<TournamentMatch>();
        var remainingMatches = new List<TournamentMatch>(matches);
        var usedInRound = new HashSet<string>();

        while (remainingMatches.Count > 0)
        {
            usedInRound.Clear();

            // Try to add as many matches as possible to this round
            // where no bot appears more than once
            int addedInRound;
            do
            {
                addedInRound = 0;

                for (int i = 0; i < remainingMatches.Count; i++)
                {
                    var match = remainingMatches[i];
                    string redBotName = match.RedBot.Name;
                    string blueBotName = match.BlueBot.Name;

                    // Check if either bot has already played in this round
                    if (!usedInRound.Contains(redBotName) && !usedInRound.Contains(blueBotName))
                    {
                        // Add this match to the result
                        result.Add(match);
                        usedInRound.Add(redBotName);
                        usedInRound.Add(blueBotName);
                        remainingMatches.RemoveAt(i);
                        addedInRound++;
                        break; // Restart the loop since we modified the list
                    }
                }
            }
            while (addedInRound > 0 && remainingMatches.Count > 0);

            // If we couldn't add any more matches (all remaining matches involve
            // bots that already played in this round), start a new round
            if (remainingMatches.Count > 0 && addedInRound == 0)
            {
                // Force add the first remaining match to break the deadlock
                // (this happens when an odd number of bots causes pairing issues)
                var match = remainingMatches[0];
                result.Add(match);
                usedInRound.Clear(); // Reset for the new round approach
                remainingMatches.RemoveAt(0);
            }
        }

        return result;
    }

    /// <summary>
    /// Calculates total games for a round-robin tournament with n bots
    /// Each pair plays twice, so: n * (n-1)
    /// </summary>
    public static int CalculateTotalGames(int botCount) => botCount * (botCount - 1);

    /// <summary>
    /// Validates bot list for tournament
    /// </summary>
    public static bool IsValidBotList(List<AIBot> bots)
    {
        if (bots.Count < 2)
            return false;

        // Check for duplicate names
        var names = bots.Select(b => b.Name).ToList();
        return names.Distinct().Count() == names.Count;
    }
}
