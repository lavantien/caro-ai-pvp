namespace Caro.Domain;

/// <summary>
/// Runtime configuration for the game server. Every property is
/// pre-seeded from Constants, so binding a partial "Caro" section (or none
/// at all) leaves the unlisted values at the compiled defaults; the single
/// source of defaults stays Constants. Validate() runs once at startup and
/// throws with the offending key name.
/// </summary>
public sealed class CaroConfig
{
    public int MaxConcurrentGames { get; init; } = Constants.Limits.MaxConcurrentGames;
    public int AbandonedTimeoutMinutes { get; init; } = Constants.Limits.AbandonedTimeoutMinutes;
    public int DefaultTTSizeMB { get; init; } = Constants.Transposition.DefaultSizeMB;
    public int DefaultSessionTTSizeMB { get; init; } = Constants.Transposition.DefaultSessionSizeMB;
    public int OpeningSpreadRadius { get; init; } = Constants.Opening.SpreadRadius;

    public TimeControlOptions TimeControl { get; init; } = new();
    public UciOptions Uci { get; init; } = new();

    // Keyed by level so partial config binds merge into the seeded entries
    // instead of replacing the whole ladder (a positional list cannot be
    // partially overridden by the configuration binder).
    public Dictionary<int, DifficultyProfileOptions> DifficultyProfiles { get; } =
        Constants.DifficultyProfiles.ToDictionary(p => p.Level, ToOptions);

    public static CaroConfig Default { get; } = new();

    private static DifficultyProfileOptions ToOptions(Constants.DifficultyProfileData p) => new()
    {
        Level = p.Level,
        Name = p.Name,
        TimeFraction = p.TimeFraction,
        MaxDepth = p.MaxDepth,
        ThreadsMode = p.Threads.ToString(),
        UseVCF = p.UseVCF,
        VCFDepth = p.VCFDepth,
        Ponder = p.Ponder,
        TTSizeMB = p.TTSizeMB,
    };

    public void Validate()
    {
        if (MaxConcurrentGames < 1)
        {
            throw new InvalidOperationException("Caro:MaxConcurrentGames must be >= 1");
        }
        if (AbandonedTimeoutMinutes < 1)
        {
            throw new InvalidOperationException("Caro:AbandonedTimeoutMinutes must be >= 1");
        }
        if (DefaultTTSizeMB < 1)
        {
            throw new InvalidOperationException("Caro:DefaultTTSizeMB must be >= 1");
        }
        if (DefaultSessionTTSizeMB < 1)
        {
            throw new InvalidOperationException("Caro:DefaultSessionTTSizeMB must be >= 1");
        }
        if (OpeningSpreadRadius is < 1 or > Constants.Board.Size / 2)
        {
            throw new InvalidOperationException(
                $"Caro:OpeningSpreadRadius must be 1..{Constants.Board.Size / 2}");
        }

        TimeControl.Validate();

        foreach ((int level, DifficultyProfileOptions profile) in DifficultyProfiles)
        {
            if (level < Constants.Difficulty.MinLevel || level > Constants.Difficulty.MaxLevel || profile.Level != level)
            {
                throw new InvalidOperationException(
                    $"Caro:DifficultyProfiles:{level} is outside "
                    + $"{Constants.Difficulty.MinLevel}..{Constants.Difficulty.MaxLevel}");
            }
        }
        foreach (int level in Enumerable.Range(Constants.Difficulty.MinLevel,
                     Constants.Difficulty.MaxLevel - Constants.Difficulty.MinLevel + 1))
        {
            if (!DifficultyProfiles.TryGetValue(level, out DifficultyProfileOptions? profile))
            {
                throw new InvalidOperationException(
                    $"Caro:DifficultyProfiles must contain level {level}");
            }
            profile.Validate();
        }

        Uci.Validate();
        if (DefaultTTSizeMB > Uci.HashMB.Max)
        {
            throw new InvalidOperationException(
                $"Caro:DefaultTTSizeMB must be <= Caro:Uci:HashMB:Max ({Uci.HashMB.Max})");
        }
        if (DefaultSessionTTSizeMB > Uci.HashMB.Max)
        {
            throw new InvalidOperationException(
                $"Caro:DefaultSessionTTSizeMB must be <= Caro:Uci:HashMB:Max ({Uci.HashMB.Max})");
        }
    }
}

/// <summary>
/// Time-control resolution table. Entries map select values and legacy
/// aliases to their canonical form; unknown values resolve to the Default
/// trio, so Default itself does not need to be a key.
/// </summary>
public sealed class TimeControlOptions
{
    public string Default { get; set; } = Constants.TimeControl.Default;
    public long DefaultInitialTimeMs { get; set; } = Constants.TimeControl.DefaultInitialTimeMs;
    public int DefaultIncrementSeconds { get; set; } = Constants.TimeControl.DefaultIncrementSeconds;

    public Dictionary<string, Constants.TimeControlData> Entries { get; } = new(Constants.TimeControls);

    public void Validate()
    {
        if (string.IsNullOrWhiteSpace(Default))
        {
            throw new InvalidOperationException("Caro:TimeControl:Default must not be empty");
        }
        if (DefaultInitialTimeMs < 1)
        {
            throw new InvalidOperationException("Caro:TimeControl:DefaultInitialTimeMs must be >= 1");
        }
        if (DefaultIncrementSeconds < 0)
        {
            throw new InvalidOperationException("Caro:TimeControl:DefaultIncrementSeconds must be >= 0");
        }
        foreach ((string key, Constants.TimeControlData entry) in Entries)
        {
            if (entry.InitialTimeMs < 1)
            {
                throw new InvalidOperationException(
                    $"Caro:TimeControl:Entries:{key}:InitialTimeMs must be >= 1");
            }
            if (entry.IncrementSeconds < 0)
            {
                throw new InvalidOperationException(
                    $"Caro:TimeControl:Entries:{key}:IncrementSeconds must be >= 0");
            }
        }
    }
}

/// <summary>
/// One rung of the difficulty ladder; ThreadsMode is the
/// Constants.ProfileThreads name ("One", "Two", "HalfL5", "L5") so hosts
/// adapt thread counts instead of pinning them.
/// </summary>
public sealed class DifficultyProfileOptions
{
    public int Level { get; set; }
    public string Name { get; set; } = "";
    public double TimeFraction { get; set; }
    public int MaxDepth { get; set; }
    public string ThreadsMode { get; set; } = nameof(Constants.ProfileThreads.One);
    public bool UseVCF { get; set; }
    public int VCFDepth { get; set; }
    public bool Ponder { get; set; }
    public int TTSizeMB { get; set; }

    public void Validate()
    {
        string key = $"Caro:DifficultyProfiles:Level {Level}";
        if (TimeFraction is <= 0 or > 1)
        {
            throw new InvalidOperationException($"{key}:TimeFraction must be in (0, 1]");
        }
        if (MaxDepth is < 1 or > Constants.Search.AbsoluteMaxDepth)
        {
            throw new InvalidOperationException(
                $"{key}:MaxDepth must be 1..{Constants.Search.AbsoluteMaxDepth}");
        }
        if (!Enum.TryParse<Constants.ProfileThreads>(ThreadsMode, out _))
        {
            throw new InvalidOperationException(
                $"{key}:ThreadsMode must be one of {string.Join(", ", Enum.GetNames<Constants.ProfileThreads>())}");
        }
        if (VCFDepth is < 0 or > Constants.Vcf.SearchDepth)
        {
            throw new InvalidOperationException(
                $"{key}:VCFDepth must be 0..{Constants.Vcf.SearchDepth}");
        }
        if (TTSizeMB < 1)
        {
            throw new InvalidOperationException($"{key}:TTSizeMB must be >= 1");
        }
    }
}

/// <summary>
/// UCI option defaults and enforced ranges; the handshake advertisement and
/// the setoption validation read the same values.
/// </summary>
public sealed class UciOptions
{
    public UciBoundOptions Threads { get; } = new()
    {
        Default = Constants.Uci.DefaultThreads,
        Min = Constants.Uci.MinThreads,
        Max = Constants.Uci.MaxThreads,
    };
    public UciBoundOptions HashMB { get; } = new()
    {
        Default = Constants.Uci.DefaultHashMB,
        Min = Constants.Uci.MinHashMB,
        Max = Constants.Uci.MaxHashMB,
    };
    public UciBoundOptions SkillLevel { get; } = new()
    {
        Default = Constants.Uci.DefaultSkillLevel,
        Min = Constants.Uci.MinSkillLevel,
        Max = Constants.Uci.MaxSkillLevel,
    };

    public void Validate()
    {
        Threads.Validate("Caro:Uci:Threads");
        HashMB.Validate("Caro:Uci:HashMB");
        SkillLevel.Validate("Caro:Uci:SkillLevel");
        if (SkillLevel.Min < Constants.Difficulty.MinLevel || SkillLevel.Max > Constants.Difficulty.MaxLevel)
        {
            throw new InvalidOperationException(
                $"Caro:Uci:SkillLevel bounds must stay within "
                + $"{Constants.Difficulty.MinLevel}..{Constants.Difficulty.MaxLevel}");
        }
    }
}

public sealed class UciBoundOptions
{
    public int Default { get; set; }
    public int Min { get; set; }
    public int Max { get; set; }

    public void Validate(string key)
    {
        if (Min < 1 || Min > Default || Default > Max)
        {
            throw new InvalidOperationException(
                $"{key} must satisfy 1 <= Min <= Default <= Max");
        }
    }
}
