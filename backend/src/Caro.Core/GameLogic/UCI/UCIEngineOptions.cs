namespace Caro.Core.GameLogic.UCI;

/// <summary>
/// UCI engine options that can be configured at runtime.
/// Maps to UCI "setoption name X value Y" commands.
/// </summary>
public class UCIEngineOptions
{
    /// <summary>
    /// Skill level (1-6). Maps to AIDifficulty enum:
    /// 1 = Braindead, 2 = Easy, 3 = Medium, 4 = Hard, 5 = Grandmaster, 6 = Experimental
    /// </summary>
    public int SkillLevel { get; set; } = 3;

    /// <summary>
    /// Whether to use the opening book.
    /// </summary>
    public bool UseOpeningBook { get; set; } = true;

    /// <summary>
    /// Maximum book depth in plies (0-40).
    /// Limits how deep the engine will follow opening book lines.
    /// </summary>
    public int BookDepthLimit { get; set; } = 24;

    /// <summary>
    /// Number of threads to use for parallel search (1-32).
    /// </summary>
    public int Threads { get; set; } = 4;

    /// <summary>
    /// Hash table size in MB (32-4096).
    /// Controls transposition table memory allocation.
    /// </summary>
    public int Hash { get; set; } = 256;

    /// <summary>
    /// Whether to enable pondering (thinking on opponent's time).
    /// </summary>
    public bool Ponder { get; set; } = false;

    /// <summary>
    /// Get the AIDifficulty corresponding to the current Skill Level.
    /// </summary>
    public AIDifficulty GetDifficulty()
    {
        return SkillLevel switch
        {
            1 => AIDifficulty.Braindead,
            2 => AIDifficulty.Easy,
            3 => AIDifficulty.Medium,
            4 => AIDifficulty.Hard,
            5 => AIDifficulty.Grandmaster,
            6 => AIDifficulty.Experimental,
            _ => AIDifficulty.Medium
        };
    }

    /// <summary>
    /// Set difficulty from AIDifficulty enum.
    /// </summary>
    public void SetDifficulty(AIDifficulty difficulty)
    {
        SkillLevel = difficulty switch
        {
            AIDifficulty.Braindead => 1,
            AIDifficulty.Easy => 2,
            AIDifficulty.Medium => 3,
            AIDifficulty.Hard => 4,
            AIDifficulty.Grandmaster => 5,
            AIDifficulty.Experimental => 6,
            _ => 3
        };
    }

    /// <summary>
    /// Parse and apply a UCI setoption command.
    /// Returns true if option was recognized and applied.
    /// </summary>
    public bool SetOption(string name, string? value)
    {
        if (string.IsNullOrWhiteSpace(name))
            return false;

        var normalizedName = name.ToLowerInvariant();

        switch (normalizedName)
        {
            case "skill level":
                if (int.TryParse(value, out int skillLevel))
                {
                    if (skillLevel >= 1 && skillLevel <= 6)
                    {
                        SkillLevel = skillLevel;
                        return true;
                    }
                }
                return false;

            case "use opening book":
                if (bool.TryParse(value, out bool useBook))
                {
                    UseOpeningBook = useBook;
                    return true;
                }
                return false;

            case "book depth limit":
                if (int.TryParse(value, out int bookDepth))
                {
                    if (bookDepth >= 0 && bookDepth <= 40)
                    {
                        BookDepthLimit = bookDepth;
                        return true;
                    }
                }
                return false;

            case "threads":
                if (int.TryParse(value, out int threads))
                {
                    if (threads >= 1 && threads <= 32)
                    {
                        Threads = threads;
                        return true;
                    }
                }
                return false;

            case "hash":
                if (int.TryParse(value, out int hash))
                {
                    if (hash >= 32 && hash <= 4096)
                    {
                        Hash = hash;
                        return true;
                    }
                }
                return false;

            case "ponder":
                if (bool.TryParse(value, out bool ponder))
                {
                    Ponder = ponder;
                    return true;
                }
                return false;

            default:
                return false;
        }
    }

    /// <summary>
    /// Get all available options as UCI option declarations.
    /// </summary>
    public static string[] GetOptionDeclarations()
    {
        return new[]
        {
            "option name Skill Level type spin default 3 min 1 max 6",
            "option name Use Opening Book type check default true",
            "option name Book Depth Limit type spin default 24 min 0 max 40",
            "option name Threads type spin default 4 min 1 max 32",
            "option name Hash type spin default 256 min 32 max 4096",
            "option name Ponder type check default false"
        };
    }
}
