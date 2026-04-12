namespace Caro.Core.GameLogic.UCI;

/// <summary>
/// UCI engine options that can be configured at runtime.
/// Maps to UCI "setoption name X value Y" commands.
/// </summary>
public class UCIEngineOptions
{
    public const string EngineVersion = "1.77.0";

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
    /// Skill level (1-5). 1=Novice, 5=Grandmaster (default).
    /// </summary>
    public int SkillLevel { get; set; } = 5;

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

            case "skill level":
                if (int.TryParse(value, out int skill) && skill >= 1 && skill <= 5)
                {
                    SkillLevel = skill;
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
            "option name Threads type spin default 4 min 1 max 32",
            "option name Hash type spin default 256 min 32 max 4096",
            "option name Ponder type check default false",
            "option name Skill Level type spin default 5 min 1 max 5"
        };
    }
}
