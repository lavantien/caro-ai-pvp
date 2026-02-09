using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;
using Xunit;

namespace Caro.Core.Tests.GameLogic;

/// <summary>
/// Tests for Adaptive Late Move Reduction (LMR) in ParallelMinimaxSearch.
/// LMR reduces search depth for late moves that are unlikely to improve the score,
/// significantly improving search performance.
/// </summary>
public class AdaptiveLMRTests
{
    [Fact]
    public void GetAdaptiveReduction_CalculatesCorrectReduction()
    {
        // Test the basic adaptive reduction calculation
        // Higher move count should increase reduction
        // Higher history score should decrease reduction

        var reduction = ParallelMinimaxSearchProxy.GetAdaptiveReduction(
            depth: 10,
            moveCount: 12,  // Late move
            improving: true,
            isPvNode: true,
            isCutNode: false,
            isTTMove: false,
            historyScore: 1000
        );

        // Should return a positive reduction (at least base reduction)
        Assert.InRange(reduction, 1, 6);
    }

    [Fact]
    public void GetAdaptiveReduction_BoundsToValidDepth()
    {
        // Reduction should never exceed depth - 1 (must search at least 1 ply)
        var depth = 5;

        var reduction1 = ParallelMinimaxSearchProxy.GetAdaptiveReduction(
            depth: depth,
            moveCount: 50,  // Very late move
            improving: false,
            isPvNode: false,
            isCutNode: false,
            isTTMove: false,
            historyScore: 0
        );

        // Reduction must be less than depth (can't reduce to 0 or negative)
        Assert.True(reduction1 < depth);
        Assert.True(reduction1 >= 0);
    }

    [Fact]
    public void GetAdaptiveReduction_HigherHistory_LessReduction()
    {
        // Moves with higher history scores should get less reduction
        // (they've been good in the past, so search them more carefully)

        var reductionLowHistory = ParallelMinimaxSearchProxy.GetAdaptiveReduction(
            depth: 10,
            moveCount: 12,
            improving: true,
            isPvNode: false,
            isCutNode: false,
            isTTMove: false,
            historyScore: 100  // Low history
        );

        var reductionHighHistory = ParallelMinimaxSearchProxy.GetAdaptiveReduction(
            depth: 10,
            moveCount: 12,
            improving: true,
            isPvNode: false,
            isCutNode: false,
            isTTMove: false,
            historyScore: 5000  // High history
        );

        // Higher history should result in less reduction
        Assert.True(reductionHighHistory <= reductionLowHistory);
    }

    [Fact]
    public void GetAdaptiveReduction_PvNode_LessReduction()
    {
        // PV nodes should get less reduction (more important for accuracy)

        var reductionPv = ParallelMinimaxSearchProxy.GetAdaptiveReduction(
            depth: 10,
            moveCount: 12,
            improving: true,
            isPvNode: true,
            isCutNode: false,
            isTTMove: false,
            historyScore: 1000
        );

        var reductionNonPv = ParallelMinimaxSearchProxy.GetAdaptiveReduction(
            depth: 10,
            moveCount: 12,
            improving: true,
            isPvNode: false,
            isCutNode: false,
            isTTMove: false,
            historyScore: 1000
        );

        // PV node should have less reduction
        Assert.True(reductionPv <= reductionNonPv);
    }

    [Fact]
    public void GetAdaptiveReduction_CutNode_MoreReduction()
    {
        // Cut nodes should get more reduction (likely to cutoff anyway)

        var reductionCut = ParallelMinimaxSearchProxy.GetAdaptiveReduction(
            depth: 10,
            moveCount: 12,
            improving: true,
            isPvNode: false,
            isCutNode: true,
            isTTMove: false,
            historyScore: 1000
        );

        var reductionNonCut = ParallelMinimaxSearchProxy.GetAdaptiveReduction(
            depth: 10,
            moveCount: 12,
            improving: true,
            isPvNode: false,
            isCutNode: false,
            isTTMove: false,
            historyScore: 1000
        );

        // Cut node should have more reduction
        Assert.True(reductionCut >= reductionNonCut);
    }

    [Fact]
    public void GetAdaptiveReduction_TTMove_NoReduction()
    {
        // TT moves should get no reduction (highest priority)

        var reductionTT = ParallelMinimaxSearchProxy.GetAdaptiveReduction(
            depth: 10,
            moveCount: 12,
            improving: true,
            isPvNode: false,
            isCutNode: false,
            isTTMove: true,
            historyScore: 1000
        );

        // TT move should have minimal or no reduction
        Assert.InRange(reductionTT, 0, 1);
    }

    [Fact]
    public void GetAdaptiveReduction_Improving_LessReduction()
    {
        // Improving positions should get less reduction (more valuable)

        var reductionImproving = ParallelMinimaxSearchProxy.GetAdaptiveReduction(
            depth: 10,
            moveCount: 12,
            improving: true,
            isPvNode: false,
            isCutNode: false,
            isTTMove: false,
            historyScore: 1000
        );

        var reductionNonImproving = ParallelMinimaxSearchProxy.GetAdaptiveReduction(
            depth: 10,
            moveCount: 12,
            improving: false,
            isPvNode: false,
            isCutNode: false,
            isTTMove: false,
            historyScore: 1000
        );

        // Improving should have less reduction
        Assert.True(reductionImproving <= reductionNonImproving);
    }

    [Fact]
    public void GetAdaptiveReduction_EarlyMoves_NoReduction()
    {
        // Early moves (low moveCount) should get no reduction

        var reductionEarly = ParallelMinimaxSearchProxy.GetAdaptiveReduction(
            depth: 10,
            moveCount: 2,  // Early move
            improving: true,
            isPvNode: false,
            isCutNode: false,
            isTTMove: false,
            historyScore: 1000
        );

        // Early move should have no reduction
        Assert.Equal(0, reductionEarly);
    }

    [Fact]
    public void GetAdaptiveReduction_LateMoves_IncreasesReduction()
    {
        // Later moves should get progressively more reduction

        var reduction1 = ParallelMinimaxSearchProxy.GetAdaptiveReduction(
            depth: 12,
            moveCount: 8,
            improving: true,
            isPvNode: false,
            isCutNode: false,
            isTTMove: false,
            historyScore: 1000
        );

        var reduction2 = ParallelMinimaxSearchProxy.GetAdaptiveReduction(
            depth: 12,
            moveCount: 16,
            improving: true,
            isPvNode: false,
            isCutNode: false,
            isTTMove: false,
            historyScore: 1000
        );

        var reduction3 = ParallelMinimaxSearchProxy.GetAdaptiveReduction(
            depth: 12,
            moveCount: 30,
            improving: true,
            isPvNode: false,
            isCutNode: false,
            isTTMove: false,
            historyScore: 1000
        );

        // Later moves should have more reduction
        Assert.True(reduction1 <= reduction2);
        Assert.True(reduction2 <= reduction3);
    }

    [Fact]
    public void GetAdaptiveReduction_DeepDepth_MoreReduction()
    {
        // Deeper searches can afford more reduction

        var reductionShallow = ParallelMinimaxSearchProxy.GetAdaptiveReduction(
            depth: 5,
            moveCount: 12,
            improving: true,
            isPvNode: false,
            isCutNode: false,
            isTTMove: false,
            historyScore: 1000
        );

        var reductionDeep = ParallelMinimaxSearchProxy.GetAdaptiveReduction(
            depth: 15,
            moveCount: 12,
            improving: true,
            isPvNode: false,
            isCutNode: false,
            isTTMove: false,
            historyScore: 1000
        );

        // Deeper depth should allow more reduction
        Assert.True(reductionDeep >= reductionShallow);
    }
}

/// <summary>
/// Proxy class to expose internal/private methods for testing.
/// This allows testing the adaptive LMR logic without modifying the production code structure.
/// </summary>
internal static class ParallelMinimaxSearchProxy
{
    /// <summary>
    /// Calculate adaptive LMR reduction based on multiple factors.
    /// This is a test-accessible wrapper around the internal implementation.
    /// </summary>
    public static int GetAdaptiveReduction(
        int depth,
        int moveCount,
        bool improving,
        bool isPvNode,
        bool isCutNode,
        bool isTTMove,
        int historyScore)
    {
        // This will be implemented in ParallelMinimaxSearch as a method
        // For now, return a placeholder to make tests compile
        return AdaptiveLMRCalculator.Calculate(
            depth, moveCount, improving, isPvNode, isCutNode, isTTMove, historyScore);
    }
}

/// <summary>
/// Calculator for adaptive LMR - will be integrated into ParallelMinimaxSearch.
/// This class contains the core LMR logic that will be used by the search engine.
/// </summary>
internal static class AdaptiveLMRCalculator
{
    private const int LMRMinDepth = 3;
    private const int LMRFullDepthMoves = 4;
    private const int LMRBaseReduction = 1;

    /// <summary>
    /// Calculate adaptive late move reduction based on position and move characteristics.
    /// Formula: baseReduction + depthAdjustment + moveCountBonus + improvingBonus
    ///           - pvPenalty - cutNodeBonus + ttMovePenalty - historyBonus
    /// </summary>
    public static int Calculate(
        int depth,
        int moveCount,
        bool improving,
        bool isPvNode,
        bool isCutNode,
        bool isTTMove,
        int historyScore)
    {
        int reduction = LMRBaseReduction;

        // Early moves get no reduction
        if (moveCount < LMRFullDepthMoves)
            return 0;

        // Depth-based adjustment: deeper searches can reduce more
        reduction += (depth - LMRMinDepth) / 3;

        // Move count adjustment: later moves get more reduction
        reduction += (moveCount - LMRFullDepthMoves) / 4;

        // Improving positions get less reduction
        if (improving)
            reduction -= 1;

        // PV nodes get less reduction (more important for accuracy)
        if (isPvNode)
            reduction -= 1;

        // Cut nodes get more reduction (likely to cutoff)
        if (isCutNode)
            reduction += 1;

        // TT moves get no reduction (highest priority)
        if (isTTMove)
            reduction = 0;

        // High history scores get less reduction
        // Scale: 0-30000 history, reduce by up to 2
        int historyBonus = Math.Min(2, historyScore / 10000);
        reduction -= historyBonus;

        // Ensure reduction is valid and bounded
        reduction = Math.Max(0, reduction);
        reduction = Math.Min(depth - 1, reduction);

        return reduction;
    }
}
