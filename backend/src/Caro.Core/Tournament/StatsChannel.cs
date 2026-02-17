using System.Threading.Channels;
using Caro.Core.Domain.Entities;

namespace Caro.Core.Tournament;

public interface IStatsPublisher
{
    Channel<MoveStatsEvent> StatsChannel { get; }
    string PublisherId { get; }
}

public class MoveStatsEvent
{
    public required string PublisherId { get; init; }
    public required Player Player { get; init; }
    public required StatsType Type { get; init; }
    public required int DepthAchieved { get; init; }
    public required long NodesSearched { get; init; }
    public required double NodesPerSecond { get; init; }
    public required double TableHitRate { get; init; }
    public required bool PonderingActive { get; init; }
    public required int VCFDepthAchieved { get; init; }
    public required long VCFNodesSearched { get; init; }
    public required int ThreadCount { get; init; }
    public required long MoveTimeMs { get; init; }
    public required long TimestampMs { get; init; }

    public double MasterTTPercent { get; init; }
    public double HelperAvgDepth { get; init; }
    public long AllocatedTimeMs { get; init; }
    public MoveType MoveType { get; init; } = MoveType.Normal;
}

public enum StatsType
{
    MainSearch,
    Pondering,
    VCFSearch
}

/// <summary>
/// Type of move determination - indicates how the move was selected
/// </summary>
public enum MoveType
{
    Normal,           // Full search performed
    Book,             // Opening book move (unvalidated)
    BookValidated,    // Book move validated by search
    ImmediateWin,     // Immediate winning move found
    ImmediateBlock,   // Forced block of opponent's winning move
    ErrorRate,        // Random move due to error rate (Braindead)
    CenterMove,       // First move at center (opening)
    Emergency         // Emergency mode (low time)
}
