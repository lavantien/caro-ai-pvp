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
/// 8 bots total: 2 per difficulty level, all starting at 600 ELO
/// </summary>
public static class AIBotFactory
{
    // Easy bots (depth 1)
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

    // Medium bots (depth 2)
    public static AIBot CreateCasual1()
    {
        return new AIBot
        {
            Name = "Casual Alpha",
            Difficulty = AIDifficulty.Medium,
            ELO = 600,
            Wins = 0,
            Losses = 0,
            Draws = 0
        };
    }

    public static AIBot CreateCasual2()
    {
        return new AIBot
        {
            Name = "Casual Bravo",
            Difficulty = AIDifficulty.Medium,
            ELO = 600,
            Wins = 0,
            Losses = 0,
            Draws = 0
        };
    }

    // Hard bots (depth 3)
    public static AIBot CreateSkilled1()
    {
        return new AIBot
        {
            Name = "Skilled Alpha",
            Difficulty = AIDifficulty.Hard,
            ELO = 600,
            Wins = 0,
            Losses = 0,
            Draws = 0
        };
    }

    public static AIBot CreateSkilled2()
    {
        return new AIBot
        {
            Name = "Skilled Bravo",
            Difficulty = AIDifficulty.Hard,
            ELO = 600,
            Wins = 0,
            Losses = 0,
            Draws = 0
        };
    }

    // Expert bots (depth 5)
    public static AIBot CreateMaster1()
    {
        return new AIBot
        {
            Name = "Master Alpha",
            Difficulty = AIDifficulty.Expert,
            ELO = 600,
            Wins = 0,
            Losses = 0,
            Draws = 0
        };
    }

    public static AIBot CreateMaster2()
    {
        return new AIBot
        {
            Name = "Master Bravo",
            Difficulty = AIDifficulty.Expert,
            ELO = 600,
            Wins = 0,
            Losses = 0,
            Draws = 0
        };
    }

    /// <summary>
    /// Get all 8 tournament bots
    /// </summary>
    public static List<AIBot> GetAllTournamentBots()
    {
        return new List<AIBot>
        {
            CreateRookie1(),
            CreateRookie2(),
            CreateCasual1(),
            CreateCasual2(),
            CreateSkilled1(),
            CreateSkilled2(),
            CreateMaster1(),
            CreateMaster2()
        };
    }

    public static AIBot FromDifficulty(AIDifficulty difficulty)
    {
        return difficulty switch
        {
            AIDifficulty.Easy => CreateRookie1(),
            AIDifficulty.Medium => CreateCasual1(),
            AIDifficulty.Hard => CreateSkilled1(),
            AIDifficulty.Expert => CreateMaster1(),
            _ => throw new ArgumentException($"Unknown difficulty: {difficulty}")
        };
    }
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
        // Using K=32 for standard chess calculations
        const int K = 32;

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
