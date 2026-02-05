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
}

public enum StatsType
{
    MainSearch,
    Pondering,
    VCFSearch
}
