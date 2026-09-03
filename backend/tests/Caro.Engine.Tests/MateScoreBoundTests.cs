using Caro.Domain;
using Xunit;

namespace Caro.Engine.Tests;

/// <summary>
/// The mate-score ply adjustment must only touch genuine mate scores.
/// Window bounds (+-Score.Infinity) and the VCF display value (WinScore)
/// passing through the transposition table used to be ply-shifted, which
/// produced the s=+100002..100008 corruption recorded in the l1v5-100 draw
/// games (docs/artifacts/tournaments/ANOMALIES.md, finding 2).
/// </summary>
public class MateScoreBoundTests
{
    private const int Inf = Constants.Score.Infinity;
    private const int Win = Constants.Score.WinScore;

    [Fact]
    public void AdjustForStoreLeavesWindowBoundsUnchanged()
    {
        Assert.Equal(Inf, MateScore.AdjustForStore(Inf, 7));
        Assert.Equal(-Inf, MateScore.AdjustForStore(-Inf, 7));
        Assert.Equal(Win + Constants.Search.AbsoluteMaxDepth + 1,
            MateScore.AdjustForStore(Win + Constants.Search.AbsoluteMaxDepth + 1, 7));
        Assert.Equal(Constants.Score.MaxEval, MateScore.AdjustForStore(Constants.Score.MaxEval, 7));
    }

    [Fact]
    public void AdjustForRetrieveLeavesWindowBoundsUnchanged()
    {
        Assert.Equal(Inf, MateScore.AdjustForRetrieve(Inf, 7));
        Assert.Equal(-Inf, MateScore.AdjustForRetrieve(-Inf, 7));
        Assert.Equal(Win + Constants.Search.AbsoluteMaxDepth + 1,
            MateScore.AdjustForRetrieve(Win + Constants.Search.AbsoluteMaxDepth + 1, 7));
        Assert.Equal(Constants.Score.MaxEval, MateScore.AdjustForRetrieve(Constants.Score.MaxEval, 7));
    }

    [Fact]
    public void IsForcedWinScoreRejectsWindowBounds()
    {
        Assert.False(MateScore.IsForcedWinScore(Inf));
        Assert.False(MateScore.IsForcedWinScore(-Inf));
        Assert.True(MateScore.IsForcedWinScore(Win - 5));
        Assert.True(MateScore.IsForcedWinScore(Win + 5));
    }

    [Fact]
    public void AdjustForStoreStillShiftsGenuineMateScores()
    {
        Assert.Equal(Win - 2, MateScore.AdjustForStore(Win - 5, 3));
        Assert.Equal(-(Win - 2), MateScore.AdjustForStore(-(Win - 5), 3));
    }

    [Fact]
    public void AdjustForRetrieveStillShiftsGenuineMateScores()
    {
        Assert.Equal(Win - 8, MateScore.AdjustForRetrieve(Win - 5, 3));
        Assert.Equal(-(Win - 8), MateScore.AdjustForRetrieve(-(Win - 5), 3));
    }
}
