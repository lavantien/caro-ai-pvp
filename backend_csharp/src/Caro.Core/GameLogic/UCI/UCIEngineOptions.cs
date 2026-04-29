using Caro.Core.Domain.Configuration;

namespace Caro.Core.GameLogic.UCI;

/// <summary>
/// UCI engine options that can be configured at runtime.
/// Maps to UCI "setoption name X value Y" commands.
/// </summary>
public class UCIEngineOptions
{
    public const string EngineVersion = "1.77.0";

    /// <summary>
    /// Number of threads to use for parallel search (1-MaxEngineThreads).
    /// Defaults to <see cref="ThreadPoolConfig.MaxEngineThreads"/>.
    /// </summary>
    public int Threads { get; set; } = ThreadPoolConfig.MaxEngineThreads;

    /// <summary>
    /// Hash table size in MB (32-4096).
    /// Defaults to <see cref="SearchConstants.DefaultTTSizeMb"/>.
    /// </summary>
    public int Hash { get; set; } = SearchConstants.DefaultTTSizeMb;

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
                    int maxThreads = ThreadPoolConfig.MaxEngineThreads;
                    if (threads >= 1 && threads <= maxThreads)
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
    /// Thread max is set to <see cref="ThreadPoolConfig.MaxEngineThreads"/>.
    /// </summary>
    public static string[] GetOptionDeclarations()
    {
        int maxThreads = ThreadPoolConfig.MaxEngineThreads;
        return new[]
        {
            $"option name Threads type spin default {maxThreads} min 1 max {maxThreads}",
            $"option name Hash type spin default {SearchConstants.DefaultTTSizeMb} min 32 max 4096",
            "option name Ponder type check default false",
            "option name Skill Level type spin default 5 min 1 max 5"
        };
    }
}
