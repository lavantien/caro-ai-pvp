using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;
using Caro.Core.Tournament;

namespace Caro.TournamentRunner;

/// <summary>
/// Aggregates move statistics for baseline benchmarking.
/// Collects MoveStats from TournamentEngine callbacks and computes
/// mode, median, and mean for performance metrics.
/// </summary>
public class BaselineStatisticsAggregator
{
    private readonly List<MoveStatsWithDifficulty> _allMoves = new();
    private readonly List<int> _moveCountsPerGame = new();
    private readonly List<VCFTriggerRecord> _vcfTriggers = new();
    private readonly Dictionary<MoveType, int> _moveTypeCounts = new();

    // Per-difficulty tracking
    private readonly Dictionary<AIDifficulty, List<MoveStatsWithDifficulty>> _movesByDifficulty = new();
    private readonly Dictionary<AIDifficulty, List<int>> _moveCountsByDifficulty = new();
    private readonly Dictionary<AIDifficulty, Dictionary<MoveType, int>> _moveTypeCountsByDifficulty = new();
    private readonly Dictionary<AIDifficulty, List<VCFTriggerRecord>> _vcfTriggersByDifficulty = new();

    public int HigherDifficultyWins { get; private set; }
    public int LowerDifficultyWins { get; private set; }
    public int Draws { get; private set; }
    public AIDifficulty HigherDifficulty { get; set; } = AIDifficulty.Braindead;
    public AIDifficulty LowerDifficulty { get; set; } = AIDifficulty.Braindead;

    /// <summary>
    /// Record a move with its statistics
    /// </summary>
    public void RecordMove(MoveStats stats)
    {
        RecordMove(stats, AIDifficulty.Braindead);
    }

    /// <summary>
    /// Record a move with its statistics and the difficulty that made it
    /// </summary>
    public void RecordMove(MoveStats stats, AIDifficulty difficulty)
    {
        var moveWithDiff = new MoveStatsWithDifficulty(stats, difficulty);
        _allMoves.Add(moveWithDiff);

        // Track move type
        var moveType = stats.MoveType;
        if (!_moveTypeCounts.ContainsKey(moveType))
            _moveTypeCounts[moveType] = 0;
        _moveTypeCounts[moveType]++;

        // Track VCF triggers
        if (stats.VCFDepthAchieved > 0 || stats.VCFNodesSearched > 0)
        {
            _vcfTriggers.Add(new VCFTriggerRecord(
                _moveCountsPerGame.Count + 1, // Current game number
                _allMoves.Count, // Current move number
                stats.VCFDepthAchieved,
                stats.VCFNodesSearched,
                difficulty
            ));
        }

        // Track per-difficulty
        if (!_movesByDifficulty.ContainsKey(difficulty))
        {
            _movesByDifficulty[difficulty] = new List<MoveStatsWithDifficulty>();
            _moveCountsByDifficulty[difficulty] = new List<int>();
            _moveTypeCountsByDifficulty[difficulty] = new Dictionary<MoveType, int>();
            _vcfTriggersByDifficulty[difficulty] = new List<VCFTriggerRecord>();
        }

        _movesByDifficulty[difficulty].Add(moveWithDiff);

        if (!_moveTypeCountsByDifficulty[difficulty].ContainsKey(moveType))
            _moveTypeCountsByDifficulty[difficulty][moveType] = 0;
        _moveTypeCountsByDifficulty[difficulty][moveType]++;

        if (stats.VCFDepthAchieved > 0 || stats.VCFNodesSearched > 0)
        {
            _vcfTriggersByDifficulty[difficulty].Add(new VCFTriggerRecord(
                _moveCountsPerGame.Count + 1,
                _movesByDifficulty[difficulty].Count,
                stats.VCFDepthAchieved,
                stats.VCFNodesSearched,
                difficulty
            ));
        }
    }

    /// <summary>
    /// Record a game result
    /// </summary>
    public void RecordGameResult(AIDifficulty winner, int moveCount, bool isDraw)
    {
        _moveCountsPerGame.Add(moveCount);

        if (isDraw)
        {
            Draws++;
        }
        else if (winner == HigherDifficulty)
        {
            HigherDifficultyWins++;
        }
        else
        {
            LowerDifficultyWins++;
        }
    }

    /// <summary>
    /// Get all aggregated statistics
    /// </summary>
    public BaselineStatistics GetStatistics()
    {
        return new BaselineStatistics
        {
            TotalGames = _moveCountsPerGame.Count,
            HigherDifficultyWins = HigherDifficultyWins,
            LowerDifficultyWins = LowerDifficultyWins,
            Draws = Draws,
            HigherDifficulty = HigherDifficulty,
            LowerDifficulty = LowerDifficulty,

            // Discrete metrics (mode meaningful)
            MoveCount = ComputeDiscreteStats(_moveCountsPerGame.Select(x => (double)x).ToList()),
            MasterDepth = ComputeDiscreteStats(_allMoves.Select(m => (double)m.DepthAchieved).ToList()),
            FirstMoveCutoffPercent = ComputeDiscreteStats(_allMoves.Select(m => m.FirstMoveCutoffPercent).ToList()),

            // Continuous metrics (median/mean only)
            NPS = ComputeContinuousStats(_allMoves.Select(m => m.NodesPerSecond).ToList()),
            HelperAvgDepth = ComputeContinuousStats(_allMoves.Select(m => m.HelperAvgDepth).ToList()),
            TimeUsedMs = ComputeContinuousStats(_allMoves.Select(m => (double)m.MoveTimeMs).ToList()),
            TimeAllocatedMs = ComputeContinuousStats(_allMoves.Select(m => (double)m.AllocatedTimeMs).ToList()),
            TTHitRate = ComputeContinuousStats(_allMoves.Select(m => m.TableHitRate).ToList()),
            EffectiveBranchingFactor = ComputeContinuousStats(_allMoves.Select(m => m.EffectiveBranchingFactor).ToList()),

            // VCF triggers
            VCFTriggers = _vcfTriggers.ToList(),

            // Move type distribution
            MoveTypeDistribution = ComputeMoveTypeDistribution(),

            // Per-difficulty statistics
            PerDifficultyStats = GetAllPerDifficultyStatistics()
        };
    }

    private DiscreteMetricStats ComputeDiscreteStats(List<double> values)
    {
        if (values.Count == 0)
            return new DiscreteMetricStats { Mode = 0, Median = 0, Mean = 0 };

        return new DiscreteMetricStats
        {
            Mode = ComputeMode(values),
            Median = ComputeMedian(values),
            Mean = values.Average()
        };
    }

    private ContinuousMetricStats ComputeContinuousStats(List<double> values)
    {
        if (values.Count == 0)
            return new ContinuousMetricStats { Median = 0, Mean = 0 };

        return new ContinuousMetricStats
        {
            Median = ComputeMedian(values),
            Mean = values.Average()
        };
    }

    private double ComputeMode(List<double> values)
    {
        if (values.Count == 0) return 0;

        // Round to nearest integer for mode calculation
        var roundedValues = values.Select(v => Math.Round(v)).GroupBy(v => v)
            .OrderByDescending(g => g.Count())
            .ThenBy(g => g.Key)
            .ToList();

        return roundedValues.Count > 0 ? roundedValues[0].Key : 0;
    }

    private double ComputeMedian(List<double> values)
    {
        if (values.Count == 0) return 0;

        var sorted = values.OrderBy(v => v).ToList();
        int mid = sorted.Count / 2;

        if (sorted.Count % 2 == 0)
            return (sorted[mid - 1] + sorted[mid]) / 2.0;
        return sorted[mid];
    }

    private Dictionary<MoveType, (int Count, double Percentage)> ComputeMoveTypeDistribution()
    {
        var total = _moveTypeCounts.Values.Sum();
        if (total == 0) return new Dictionary<MoveType, (int, double)>();

        return _moveTypeCounts.ToDictionary(
            kvp => kvp.Key,
            kvp => (kvp.Value, (double)kvp.Value / total * 100)
        );
    }

    /// <summary>
    /// Reset all collected statistics
    /// </summary>
    public void Reset()
    {
        _allMoves.Clear();
        _moveCountsPerGame.Clear();
        _vcfTriggers.Clear();
        _moveTypeCounts.Clear();
        HigherDifficultyWins = 0;
        LowerDifficultyWins = 0;
        Draws = 0;
        _movesByDifficulty.Clear();
        _moveCountsByDifficulty.Clear();
        _moveTypeCountsByDifficulty.Clear();
        _vcfTriggersByDifficulty.Clear();
    }

    /// <summary>
    /// Get per-difficulty statistics for a specific difficulty
    /// </summary>
    public PerDifficultyStatistics GetPerDifficultyStatistics(AIDifficulty difficulty)
    {
        if (!_movesByDifficulty.ContainsKey(difficulty) || _movesByDifficulty[difficulty].Count == 0)
        {
            return new PerDifficultyStatistics { Difficulty = difficulty };
        }

        var moves = _movesByDifficulty[difficulty];
        var moveTypeCounts = _moveTypeCountsByDifficulty.GetValueOrDefault(difficulty, new Dictionary<MoveType, int>());
        var vcfTriggers = _vcfTriggersByDifficulty.GetValueOrDefault(difficulty, new List<VCFTriggerRecord>());

        return new PerDifficultyStatistics
        {
            Difficulty = difficulty,
            TotalMoves = moves.Count,

            // Discrete metrics
            MasterDepth = ComputeDiscreteStats(moves.Select(m => (double)m.DepthAchieved).ToList()),
            FirstMoveCutoffPercent = ComputeDiscreteStats(moves.Select(m => m.FirstMoveCutoffPercent).ToList()),

            // Continuous metrics
            NPS = ComputeContinuousStats(moves.Select(m => m.NodesPerSecond).ToList()),
            HelperAvgDepth = ComputeContinuousStats(moves.Select(m => m.HelperAvgDepth).ToList()),
            TimeUsedMs = ComputeContinuousStats(moves.Select(m => (double)m.MoveTimeMs).ToList()),
            TimeAllocatedMs = ComputeContinuousStats(moves.Select(m => (double)m.AllocatedTimeMs).ToList()),
            TTHitRate = ComputeContinuousStats(moves.Select(m => m.TableHitRate).ToList()),
            EffectiveBranchingFactor = ComputeContinuousStats(moves.Select(m => m.EffectiveBranchingFactor).ToList()),

            // VCF triggers
            VCFTriggers = vcfTriggers.ToList(),

            // Move type distribution
            MoveTypeDistribution = ComputeMoveTypeDistributionForDifficulty(moveTypeCounts)
        };
    }

    /// <summary>
    /// Get per-difficulty statistics for all difficulties that have moves
    /// </summary>
    public Dictionary<AIDifficulty, PerDifficultyStatistics> GetAllPerDifficultyStatistics()
    {
        var result = new Dictionary<AIDifficulty, PerDifficultyStatistics>();
        foreach (var difficulty in _movesByDifficulty.Keys)
        {
            result[difficulty] = GetPerDifficultyStatistics(difficulty);
        }
        return result;
    }

    private Dictionary<MoveType, (int Count, double Percentage)> ComputeMoveTypeDistributionForDifficulty(
        Dictionary<MoveType, int> moveTypeCounts)
    {
        var total = moveTypeCounts.Values.Sum();
        if (total == 0) return new Dictionary<MoveType, (int, double)>();

        return moveTypeCounts.ToDictionary(
            kvp => kvp.Key,
            kvp => (kvp.Value, (double)kvp.Value / total * 100)
        );
    }
}

/// <summary>
/// Record of a VCF trigger event
/// </summary>
public record VCFTriggerRecord(int Game, int Move, int Depth, long Nodes, AIDifficulty Difficulty = default);

/// <summary>
/// Wrapper for MoveStats that includes the difficulty that made the move
/// </summary>
public record MoveStatsWithDifficulty(MoveStats Stats, AIDifficulty Difficulty)
{
    public int DepthAchieved => Stats.DepthAchieved;
    public long NodesSearched => Stats.NodesSearched;
    public double NodesPerSecond => Stats.NodesPerSecond;
    public double TableHitRate => Stats.TableHitRate;
    public bool PonderingActive => Stats.PonderingActive;
    public int VCFDepthAchieved => Stats.VCFDepthAchieved;
    public long VCFNodesSearched => Stats.VCFNodesSearched;
    public int ThreadCount => Stats.ThreadCount;
    public long MoveTimeMs => Stats.MoveTimeMs;
    public double MasterTTPercent => Stats.MasterTTPercent;
    public double HelperAvgDepth => Stats.HelperAvgDepth;
    public long AllocatedTimeMs => Stats.AllocatedTimeMs;
    public MoveType MoveType => Stats.MoveType;
    public double EffectiveBranchingFactor => Stats.EffectiveBranchingFactor;
    public double FirstMoveCutoffPercent => Stats.FirstMoveCutoffPercent;
}

/// <summary>
/// Statistics for a discrete metric (mode is meaningful)
/// </summary>
public record DiscreteMetricStats
{
    public double Mode { get; init; }
    public double Median { get; init; }
    public double Mean { get; init; }
}

/// <summary>
/// Statistics for a continuous metric (mode not meaningful)
/// </summary>
public record ContinuousMetricStats
{
    public double Median { get; init; }
    public double Mean { get; init; }
}

/// <summary>
/// Complete baseline statistics for a matchup
/// </summary>
public class BaselineStatistics
{
    // Game results
    public int TotalGames { get; init; }
    public int HigherDifficultyWins { get; init; }
    public int LowerDifficultyWins { get; init; }
    public int Draws { get; init; }
    public AIDifficulty HigherDifficulty { get; init; }
    public AIDifficulty LowerDifficulty { get; init; }

    // Discrete metrics (mode meaningful)
    public DiscreteMetricStats MoveCount { get; init; } = new();
    public DiscreteMetricStats MasterDepth { get; init; } = new();
    public DiscreteMetricStats FirstMoveCutoffPercent { get; init; } = new();

    // Continuous metrics (median/mean only)
    public ContinuousMetricStats NPS { get; init; } = new();
    public ContinuousMetricStats HelperAvgDepth { get; init; } = new();
    public ContinuousMetricStats TimeUsedMs { get; init; } = new();
    public ContinuousMetricStats TimeAllocatedMs { get; init; } = new();
    public ContinuousMetricStats TTHitRate { get; init; } = new();
    public ContinuousMetricStats EffectiveBranchingFactor { get; init; } = new();

    // VCF triggers
    public List<VCFTriggerRecord> VCFTriggers { get; init; } = new();

    // Move type distribution
    public Dictionary<MoveType, (int Count, double Percentage)> MoveTypeDistribution { get; init; } = new();

    // Per-difficulty statistics
    public Dictionary<AIDifficulty, PerDifficultyStatistics> PerDifficultyStats { get; init; } = new();
}

/// <summary>
/// Statistics for a single difficulty level aggregated across all matchups where it played
/// </summary>
public class PerDifficultyStatistics
{
    public AIDifficulty Difficulty { get; init; }
    public int TotalMoves { get; init; }

    // Discrete metrics (mode meaningful)
    public DiscreteMetricStats MasterDepth { get; init; } = new();
    public DiscreteMetricStats FirstMoveCutoffPercent { get; init; } = new();

    // Continuous metrics (median/mean only)
    public ContinuousMetricStats NPS { get; init; } = new();
    public ContinuousMetricStats HelperAvgDepth { get; init; } = new();
    public ContinuousMetricStats TimeUsedMs { get; init; } = new();
    public ContinuousMetricStats TimeAllocatedMs { get; init; } = new();
    public ContinuousMetricStats TTHitRate { get; init; } = new();
    public ContinuousMetricStats EffectiveBranchingFactor { get; init; } = new();

    // VCF triggers for this difficulty
    public List<VCFTriggerRecord> VCFTriggers { get; init; } = new();

    // Move type distribution for this difficulty
    public Dictionary<MoveType, (int Count, double Percentage)> MoveTypeDistribution { get; init; } = new();
}
