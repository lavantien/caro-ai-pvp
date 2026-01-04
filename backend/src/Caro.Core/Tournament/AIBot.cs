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
/// 22 bots total: 2 per difficulty level (11 levels), all starting at 600 ELO
/// </summary>
public static class AIBotFactory
{
    // D1 Beginner bots
    public static AIBot CreateNovice1()
    {
        return new AIBot
        {
            Name = "Novice Alpha",
            Difficulty = AIDifficulty.Beginner,
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
            Difficulty = AIDifficulty.Beginner,
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

    // D3 Normal bots
    public static AIBot CreateCasual1()
    {
        return new AIBot
        {
            Name = "Casual Alpha",
            Difficulty = AIDifficulty.Normal,
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
            Difficulty = AIDifficulty.Normal,
            ELO = 600,
            Wins = 0,
            Losses = 0,
            Draws = 0
        };
    }

    // D4 Medium bots
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

    // D5 Hard bots
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

    // D6 Harder bots
    public static AIBot CreateAdvanced1()
    {
        return new AIBot
        {
            Name = "Advanced Alpha",
            Difficulty = AIDifficulty.Harder,
            ELO = 600,
            Wins = 0,
            Losses = 0,
            Draws = 0
        };
    }

    public static AIBot CreateAdvanced2()
    {
        return new AIBot
        {
            Name = "Advanced Bravo",
            Difficulty = AIDifficulty.Harder,
            ELO = 600,
            Wins = 0,
            Losses = 0,
            Draws = 0
        };
    }

    // D7 Very Hard bots
    public static AIBot CreateTournament1()
    {
        return new AIBot
        {
            Name = "Tournament Alpha",
            Difficulty = AIDifficulty.VeryHard,
            ELO = 600,
            Wins = 0,
            Losses = 0,
            Draws = 0
        };
    }

    public static AIBot CreateTournament2()
    {
        return new AIBot
        {
            Name = "Tournament Bravo",
            Difficulty = AIDifficulty.VeryHard,
            ELO = 600,
            Wins = 0,
            Losses = 0,
            Draws = 0
        };
    }

    // D8 Expert bots
    public static AIBot CreateExpert1()
    {
        return new AIBot
        {
            Name = "Expert Alpha",
            Difficulty = AIDifficulty.Expert,
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
            Difficulty = AIDifficulty.Expert,
            ELO = 600,
            Wins = 0,
            Losses = 0,
            Draws = 0
        };
    }

    // D9 Master bots
    public static AIBot CreateMaster1()
    {
        return new AIBot
        {
            Name = "Master Alpha",
            Difficulty = AIDifficulty.Master,
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
            Difficulty = AIDifficulty.Master,
            ELO = 600,
            Wins = 0,
            Losses = 0,
            Draws = 0
        };
    }

    // D10 Grandmaster bots
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

    // D11 Legend bots
    public static AIBot CreateLegend1()
    {
        return new AIBot
        {
            Name = "Legend Alpha",
            Difficulty = AIDifficulty.Legend,
            ELO = 600,
            Wins = 0,
            Losses = 0,
            Draws = 0
        };
    }

    public static AIBot CreateLegend2()
    {
        return new AIBot
        {
            Name = "Legend Bravo",
            Difficulty = AIDifficulty.Legend,
            ELO = 600,
            Wins = 0,
            Losses = 0,
            Draws = 0
        };
    }

    /// <summary>
    /// Get all 22 tournament bots (2 per difficulty level)
    /// </summary>
    public static List<AIBot> GetAllTournamentBots()
    {
        return new List<AIBot>
        {
            CreateNovice1(),
            CreateNovice2(),
            CreateRookie1(),
            CreateRookie2(),
            CreateCasual1(),
            CreateCasual2(),
            CreateClub1(),
            CreateClub2(),
            CreateSkilled1(),
            CreateSkilled2(),
            CreateAdvanced1(),
            CreateAdvanced2(),
            CreateTournament1(),
            CreateTournament2(),
            CreateExpert1(),
            CreateExpert2(),
            CreateMaster1(),
            CreateMaster2(),
            CreateGrandmaster1(),
            CreateGrandmaster2(),
            CreateLegend1(),
            CreateLegend2()
        };
    }

    /// <summary>
    /// Create a bot from difficulty level
    /// </summary>
    public static AIBot FromDifficulty(AIDifficulty difficulty) => difficulty switch
    {
        AIDifficulty.Beginner => CreateNovice1(),
        AIDifficulty.Easy => CreateRookie1(),
        AIDifficulty.Normal => CreateCasual1(),
        AIDifficulty.Medium => CreateClub1(),
        AIDifficulty.Hard => CreateSkilled1(),
        AIDifficulty.Harder => CreateAdvanced1(),
        AIDifficulty.VeryHard => CreateTournament1(),
        AIDifficulty.Expert => CreateExpert1(),
        AIDifficulty.Master => CreateMaster1(),
        AIDifficulty.Grandmaster => CreateGrandmaster1(),
        AIDifficulty.Legend => CreateLegend1(),
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
