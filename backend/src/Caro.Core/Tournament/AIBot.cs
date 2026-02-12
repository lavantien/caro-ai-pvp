using Caro.Core.Domain.Configuration;
using Caro.Core.GameLogic;

namespace Caro.Core.Tournament;

/// <summary>
/// AI Bot with name, difficulty, and ELO rating
/// </summary>
public class AIBot
{
    public required string Name { get; init; }
    public required AIDifficulty Difficulty { get; init; }
    public int ELO { get; set; }
    public int Wins { get; set; }
    public int Losses { get; set; }
    public int Draws { get; set; }
    public int GamesPlayed => Wins + Losses + Draws;

    /// <summary>
    /// Win rate as percentage (0-100)
    /// </summary>
    public double WinRate => GamesPlayed > 0 ? (double)Wins / GamesPlayed * 100 : 0;

    public override string ToString()
    {
        return $"{Name} ({Difficulty}) - ELO: {ELO} ({Wins}-{Losses}-{Draws})";
    }
}

/// <summary>
/// Factory for creating AI bots with names and initial ELO ratings
/// 10 bots total: 2 per difficulty level (5 levels), all starting at 600 ELO
/// </summary>
public static class AIBotFactory
{
    // D1 Braindead bots
    public static AIBot CreateNovice1()
    {
        return new AIBot
        {
            Name = "Novice Alpha",
            Difficulty = AIDifficulty.Braindead,
            ELO = 600,
            Wins = 0,
            Losses = 0,
            Draws = 0
        };
    }

    public static AIBot CreateNovice2()
    {
        return new AIBot
        {
            Name = "Novice Bravo",
            Difficulty = AIDifficulty.Braindead,
            ELO = 600,
            Wins = 0,
            Losses = 0,
            Draws = 0
        };
    }

    // D2 Easy bots
    public static AIBot CreateRookie1()
    {
        return new AIBot
        {
            Name = "Rookie Alpha",
            Difficulty = AIDifficulty.Easy,
            ELO = 600,
            Wins = 0,
            Losses = 0,
            Draws = 0
        };
    }

    public static AIBot CreateRookie2()
    {
        return new AIBot
        {
            Name = "Rookie Bravo",
            Difficulty = AIDifficulty.Easy,
            ELO = 600,
            Wins = 0,
            Losses = 0,
            Draws = 0
        };
    }

    // D3 Medium bots
    public static AIBot CreateClub1()
    {
        return new AIBot
        {
            Name = "Club Alpha",
            Difficulty = AIDifficulty.Medium,
            ELO = 600,
            Wins = 0,
            Losses = 0,
            Draws = 0
        };
    }

    public static AIBot CreateClub2()
    {
        return new AIBot
        {
            Name = "Club Bravo",
            Difficulty = AIDifficulty.Medium,
            ELO = 600,
            Wins = 0,
            Losses = 0,
            Draws = 0
        };
    }

    // D4 Hard bots
    public static AIBot CreateExpert1()
    {
        return new AIBot
        {
            Name = "Expert Alpha",
            Difficulty = AIDifficulty.Hard,
            ELO = 600,
            Wins = 0,
            Losses = 0,
            Draws = 0
        };
    }

    public static AIBot CreateExpert2()
    {
        return new AIBot
        {
            Name = "Expert Bravo",
            Difficulty = AIDifficulty.Hard,
            ELO = 600,
            Wins = 0,
            Losses = 0,
            Draws = 0
        };
    }

    // D5 Grandmaster bots
    public static AIBot CreateGrandmaster1()
    {
        return new AIBot
        {
            Name = "Grandmaster Alpha",
            Difficulty = AIDifficulty.Grandmaster,
            ELO = 600,
            Wins = 0,
            Losses = 0,
            Draws = 0
        };
    }

    public static AIBot CreateGrandmaster2()
    {
        return new AIBot
        {
            Name = "Grandmaster Bravo",
            Difficulty = AIDifficulty.Grandmaster,
            ELO = 600,
            Wins = 0,
            Losses = 0,
            Draws = 0
        };
    }

    /// <summary>
    /// Get all 10 tournament bots (2 per difficulty level)
    /// </summary>
    public static List<AIBot> GetAllTournamentBots()
    {
        return new List<AIBot>
        {
            CreateNovice1(),
            CreateNovice2(),
            CreateRookie1(),
            CreateRookie2(),
            CreateClub1(),
            CreateClub2(),
            CreateExpert1(),
            CreateExpert2(),
            CreateGrandmaster1(),
            CreateGrandmaster2()
        };
    }

    /// <summary>
    /// Create a bot from difficulty level
    /// </summary>
    public static AIBot FromDifficulty(AIDifficulty difficulty) => difficulty switch
    {
        AIDifficulty.Braindead => CreateNovice1(),
        AIDifficulty.Easy => CreateRookie1(),
        AIDifficulty.Medium => CreateClub1(),
        AIDifficulty.Hard => CreateExpert1(),
        AIDifficulty.Grandmaster => CreateGrandmaster1(),
        _ => throw new ArgumentException($"Unknown difficulty: {difficulty}")
    };
}

/// <summary>
/// ELO Rating Calculator
/// </summary>
public static class ELOCalculator
{
    /// <summary>
    /// Calculate ELO change after a game
    /// </summary>
    /// <param name="winnerELO">Current ELO of winner</param>
    /// <param name="loserELO">Current ELO of loser</param>
    /// <param name="isDraw">Was the game a draw?</param>
    /// <returns>ELO change (positive for winner, negative for loser)</returns>
    public static (int winnerChange, int loserChange) CalculateELOChange(int winnerELO, int loserELO, bool isDraw = false)
    {
        // K-factor: How much ELO can change per game
        // Using centralized constant for consistency
        const int K = GameConstants.EloKFactor;

        // Calculate expected scores
        double expectedWinner = 1.0 / (1.0 + Math.Pow(10, (loserELO - winnerELO) / 400.0));
        double expectedLoser = 1.0 - expectedWinner;

        // Actual scores (1 for win, 0.5 for draw, 0 for loss)
        double actualWinner = isDraw ? 0.5 : 1.0;
        double actualLoser = isDraw ? 0.5 : 0.0;

        // Calculate ELO changes
        int winnerChange = (int)Math.Round(K * (actualWinner - expectedWinner));
        int loserChange = (int)Math.Round(K * (actualLoser - expectedLoser));

        return (winnerChange, loserChange);
    }

    /// <summary>
    /// Update ELO ratings for both bots after a game
    /// </summary>
    public static void UpdateELOs(AIBot winner, AIBot loser, bool isDraw = false)
    {
        var (winnerChange, loserChange) = CalculateELOChange(winner.ELO, loser.ELO, isDraw);

        winner.ELO += winnerChange;
        loser.ELO += loserChange;

        if (isDraw)
        {
            winner.Draws++;
            loser.Draws++;
        }
        else
        {
            winner.Wins++;
            loser.Losses++;
        }
    }
}
