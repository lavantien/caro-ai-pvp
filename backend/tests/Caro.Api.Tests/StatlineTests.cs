using System.Text.Json;
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

/// <summary>
/// Probe fields (ponder/VCF counters) ride alongside the statline without
/// altering its bytes; JSON keys are contract for the tournament tooling.
/// </summary>
public class StatlineProbeFieldsTests
{
    private static MoveDetailResponse Build(
        string moveType = "exact",
        bool? ponderHit = null,
        int? ponderDepth = null,
        long? ponderNodes = null,
        int? vcfDepth = null,
        long? vcfNodes = null,
        int depthAchieved = 9,
        long nodesSearched = 12_345)
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
            DepthAchieved = depthAchieved,
            NodesSearched = nodesSearched,
            NodesPerSecond = 45_000,
            SearchScore = -60,
            TableHitRate = 0.5,
            AllocatedTimeMs = 2_000,
            ThreadCount = 2,
            MoveType = moveType,
            VcfDepth = vcfDepth,
            VcfNodes = vcfNodes,
        };
        return Statline.BuildMoveDetail(resp, "red", 3, 3, stats, 1_500, ponderHit, ponderDepth, ponderNodes);
    }

    [Fact]
    public void PonderFieldsDoNotChangeStatlineBytes()
    {
        MoveDetailResponse withProbe = Build(ponderHit: true, ponderDepth: 12, ponderNodes: 345_678);
        MoveDetailResponse withoutProbe = Build(ponderHit: true);

        Assert.Equal(withoutProbe.Statline, withProbe.Statline);
        Assert.EndsWith(" [PONDER]", withProbe.Statline);
        Assert.True(withProbe.PonderHit);
        Assert.Equal(12, withProbe.EngineStats.PonderDepth);
        Assert.Equal(345_678L, withProbe.EngineStats.PonderNodes);
    }

    [Fact]
    public void NoPonderLeavesProbeFieldsNull()
    {
        MoveDetailResponse detail = Build(ponderHit: null);

        Assert.Null(detail.PonderHit);
        Assert.Null(detail.EngineStats.PonderDepth);
        Assert.Null(detail.EngineStats.PonderNodes);
        Assert.False(detail.Statline.Contains(" [", StringComparison.Ordinal));
    }

    [Fact]
    public void PonderMissKeepsDepthButDropsTag()
    {
        MoveDetailResponse detail = Build(ponderHit: false, ponderDepth: 8, ponderNodes: 12_345);

        Assert.False(detail.PonderHit);
        Assert.Equal(8, detail.EngineStats.PonderDepth);
        Assert.Equal(12_345L, detail.EngineStats.PonderNodes);
        Assert.False(detail.Statline.Contains(" [PONDER]", StringComparison.Ordinal));
    }

    [Fact]
    public void VCFCountersCarryThroughWithZeroDepthStatline()
    {
        MoveDetailResponse detail = Build(moveType: "vcf", vcfDepth: 3, vcfNodes: 9_876, depthAchieved: 0, nodesSearched: 0);

        Assert.Equal(3, detail.EngineStats.VcfDepth);
        Assert.Equal(9_876L, detail.EngineStats.VcfNodes);
        Assert.EndsWith(" [VCF]", detail.Statline);
        Assert.Contains("d=0", detail.Statline, StringComparison.Ordinal);
        Assert.Contains("n=0", detail.Statline, StringComparison.Ordinal);
    }

    [Fact]
    public void JsonContractIncludesProbeKeys()
    {
        MoveDetailResponse detail = Build(ponderHit: true, ponderDepth: 12, ponderNodes: 345_678, vcfDepth: 2, vcfNodes: 500);

        string json = JsonSerializer.Serialize(detail, JsonOptions.Shared);
        foreach (string key in new[] { "ponderDepth", "ponderNodes", "ponderHit", "vcfDepth", "vcfNodes" })
        {
            Assert.Contains($"\"{key}\":", json);
        }
    }
}
