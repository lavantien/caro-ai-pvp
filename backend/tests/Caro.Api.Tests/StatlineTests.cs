using Caro.Api;
using Caro.Engine;
using Xunit;

namespace Caro.Api.Tests;

/// <summary>
/// Statline move-type and ponder tags; these strings are parsed by stats
/// tooling, so the exact suffixes are contract.
/// </summary>
public class StatlineTagsTests
{
    private static MoveDetailResponse Build(string moveType, bool ponderHit)
    {
        GameResponse resp = new()
        {
            CurrentPlayer = "red",
            MoveNumber = 5,
            RedTimeRemaining = 100.0,
            BlueTimeRemaining = 200.0,
        };
        SearchStats stats = new()
        {
            DepthAchieved = 9,
            NodesSearched = 12_345,
            NodesPerSecond = 45_000,
            SearchScore = -60,
            TableHitRate = 0.5,
            AllocatedTimeMs = 2_000,
            ThreadCount = 2,
            MoveType = moveType,
        };
        return Statline.BuildMoveDetail(resp, "red", 3, 3, stats, 1_500, ponderHit);
    }

    [Fact]
    public void VCFTag()
    {
        MoveDetailResponse detail = Build("vcf", ponderHit: false);
        Assert.EndsWith(" [VCF]", detail.Statline);
        Assert.Equal("vcf", detail.EngineStats.MoveType);
    }

    [Fact]
    public void VCFBlockTag()
    {
        MoveDetailResponse detail = Build("vcf-block", ponderHit: false);
        Assert.EndsWith(" [VCF-BLOCK]", detail.Statline);
        Assert.Equal("vcf-block", detail.EngineStats.MoveType);
    }

    [Fact]
    public void PonderTag()
    {
        MoveDetailResponse detail = Build("exact", ponderHit: true);
        Assert.EndsWith(" [PONDER]", detail.Statline);
    }

    [Fact]
    public void EmptyMoveTypeFallsBackToExact()
    {
        MoveDetailResponse detail = Build("", ponderHit: false);
        Assert.Equal("exact", detail.EngineStats.MoveType);
        Assert.False(detail.Statline.Contains(" [", StringComparison.Ordinal));
    }
}
