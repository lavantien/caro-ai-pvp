using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;
using Xunit;

namespace Caro.Core.Tests.GameLogic;

/// <summary>
/// Tests for ContestManager (Contempt Factor).
/// Contempt factor adjusts evaluation based on position type and estimated strength difference.
/// Positive contempt = prefer avoiding draws (play riskier)
/// Negative contempt = prefer safer play (accept draws)
/// Range: -200 to +200 centipawns
/// Expected ELO gain: +5-20
/// </summary>
public class ContestManagerTests
{
    [Fact]
    public void EqualPosition_PositiveContempt()
    {
        // Arrange: Create a contest manager
        var manager = new ContestManager(baseContempt: 50);

        // Act: Calculate contempt for equal position (eval = 0)
        int contempt = manager.CalculateContempt(eval: 0, estimatedDifficulty: 0.5);

        // Assert: In equal positions, positive contempt should be applied
        // to avoid draws and play for a win
        Assert.True(contempt > 0, "Contempt should be positive in equal positions");
        Assert.InRange(contempt, -200, 200);
    }

    [Fact]
    public void WinningPosition_NegativeContempt()
    {
        // Arrange: Create a contest manager
        var manager = new ContestManager(baseContempt: 50);

        // Act: Calculate contempt for winning position (positive eval)
        int contempt = manager.CalculateContempt(eval: 300, estimatedDifficulty: 0.5);

        // Assert: In winning positions, contempt should be reduced or negative
        // to play safer and consolidate the advantage
        Assert.True(contempt < 50, "Contempt should be reduced in winning positions");
        Assert.InRange(contempt, -200, 200);
    }

    [Fact]
    public void LosingPosition_IncreasedContempt()
    {
        // Arrange: Create a contest manager
        var manager = new ContestManager(baseContempt: 50);

        // Act: Calculate contempt for losing position (negative eval)
        int contempt = manager.CalculateContempt(eval: -300, estimatedDifficulty: 0.5);

        // Assert: In losing positions, contempt should be increased
        // to take risks and try to complicate the game
        Assert.True(contempt >= 50, "Contempt should be increased in losing positions");
        Assert.InRange(contempt, -200, 200);
    }

    [Fact]
    public void BoundsClamping_ClampsToValidRange()
    {
        // Arrange: Create contest manager with extreme base contempt
        var manager = new ContestManager(baseContempt: 500);

        // Act: Calculate contempt (should be clamped)
        int contempt = manager.CalculateContempt(eval: 10000, estimatedDifficulty: 0.5);

        // Assert: Contempt should be clamped to [-200, 200] range
        Assert.InRange(contempt, -200, 200);
    }

    [Fact]
    public void BoundsClamping_NegativeBaseContempt()
    {
        // Arrange: Create contest manager with negative base contempt
        var manager = new ContestManager(baseContempt: -500);

        // Act: Calculate contempt (should be clamped)
        int contempt = manager.CalculateContempt(eval: -10000, estimatedDifficulty: 0.5);

        // Assert: Contempt should be clamped to [-200, 200] range
        Assert.InRange(contempt, -200, 200);
    }

    [Fact]
    public void ZeroBaseContempt_StillCalculates()
    {
        // Arrange: Create contest manager with zero contempt
        var manager = new ContestManager(baseContempt: 0);

        // Act: Calculate contempt for losing position
        int contempt = manager.CalculateContempt(eval: -200, estimatedDifficulty: 0.5);

        // Assert: Even with zero base, losing position generates contempt
        Assert.True(contempt > 0, "Losing position should generate contempt even with zero base");
    }

    [Fact]
    public void ApplyContemptToEvaluation_AdjustsScoreCorrectly()
    {
        // Arrange: Create contest manager
        var manager = new ContestManager(baseContempt: 50);

        int originalEval = 100; // Positive evaluation
        int contempt = manager.CalculateContempt(eval: 0, estimatedDifficulty: 0.5);

        // Act: Apply contempt manually
        int adjustedEval = originalEval + contempt;

        // Assert: Score should be adjusted by contempt factor
        // Positive contempt increases the evaluation (avoid draws)
        Assert.True(adjustedEval > originalEval);
    }

    [Fact]
    public void ApplyContemptToEvaluation_WinningPositionDecreasesScore()
    {
        // Arrange: Create contest manager
        var manager = new ContestManager(baseContempt: 50);

        int originalEval = 500; // High positive evaluation
        int contempt = manager.CalculateContempt(eval: 500, estimatedDifficulty: 0.5);

        // Act: Apply contempt
        int adjustedEval = originalEval + contempt;

        // Assert: Score should be adjusted downward (play safer)
        Assert.True(adjustedEval < originalEval);
    }

    [Fact]
    public void DifficultyAdjustment_IncreasesWithDifficulty()
    {
        // Arrange: Create contest manager
        var manager = new ContestManager(baseContempt: 50);

        // Act: Calculate contempt with different difficulties
        int contemptLow = manager.CalculateContempt(eval: 0, estimatedDifficulty: 0.2);
        int contemptHigh = manager.CalculateContempt(eval: 0, estimatedDifficulty: 0.8);

        // Assert: Higher difficulty should increase contempt
        Assert.True(contemptHigh > contemptLow, "Higher difficulty should increase contempt");
    }

    [Fact]
    public void SmoothTransition_EvalChange()
    {
        // Arrange: Create contest manager
        var manager = new ContestManager(baseContempt: 20);

        // Act: Gradual eval change
        int contempt1 = manager.CalculateContempt(eval: -100, estimatedDifficulty: 0.5);
        int contempt2 = manager.CalculateContempt(eval: 0, estimatedDifficulty: 0.5);
        int contempt3 = manager.CalculateContempt(eval: 100, estimatedDifficulty: 0.5);

        // Assert: Contempt should decrease as position improves
        Assert.True(contempt1 >= contempt2 && contempt2 >= contempt3,
            "Contempt should decrease as position improves");
    }

    [Fact]
    public void GetSet_BaseContempt()
    {
        // Arrange
        var manager = new ContestManager(baseContempt: 30);

        // Act & Assert
        Assert.Equal(30, manager.BaseContempt);

        // Modify
        manager.BaseContempt = 50;
        Assert.Equal(50, manager.BaseContempt);
    }

    [Fact]
    public void Reset_RestoresInitialState()
    {
        // Arrange
        var manager = new ContestManager(baseContempt: 20);
        manager.BaseContempt = 50;

        // Act
        manager.Reset();

        // Assert - After reset, base contempt should return to initial
        Assert.Equal(20, manager.BaseContempt);
    }
}
