using Caro.Core.Domain.Configuration;
using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;
using Xunit;

namespace Caro.Core.Tests.GameLogic;

/// <summary>
/// Unit tests for TunableParameters
/// </summary>
public sealed class TunableParametersTests
{
    [Fact]
    public void Default_HasCorrectValues()
    {
        // Arrange & Act
        var parameters = TunableParameters.Default;

        // Assert - verify defaults match EvaluationConstants
        Assert.Equal(EvaluationConstants.FiveInRowScore, parameters.FiveInRowScore);
        Assert.Equal(EvaluationConstants.OpenFourScore, parameters.OpenFourScore);
        Assert.Equal(EvaluationConstants.ClosedFourScore, parameters.ClosedFourScore);
        Assert.Equal(EvaluationConstants.OpenThreeScore, parameters.OpenThreeScore);
        Assert.Equal(EvaluationConstants.ClosedThreeScore, parameters.ClosedThreeScore);
        Assert.Equal(EvaluationConstants.OpenTwoScore, parameters.OpenTwoScore);
        Assert.Equal(EvaluationConstants.CenterBonus, parameters.CenterBonus);

        var expectedDefMult = (double)EvaluationConstants.DefenseMultiplierNumerator / EvaluationConstants.DefenseMultiplierDenominator;
        Assert.Equal(expectedDefMult, parameters.DefenseMultiplier);
    }

    [Fact]
    public void ToArrayAndBack_RoundTrips()
    {
        // Arrange
        var original = new TunableParameters
        {
            FiveInRowScore = 123456,
            OpenFourScore = 12345,
            ClosedFourScore = 1234,
            OpenThreeScore = 1234,
            ClosedThreeScore = 123,
            OpenTwoScore = 123,
            CenterBonus = 67,
            DefenseMultiplier = 2.5
        };

        // Act
        var array = original.ToArray();
        var restored = new TunableParameters();
        restored.ApplyFromArray(array);

        // Assert
        Assert.Equal(original.FiveInRowScore, restored.FiveInRowScore);
        Assert.Equal(original.OpenFourScore, restored.OpenFourScore);
        Assert.Equal(original.ClosedFourScore, restored.ClosedFourScore);
        Assert.Equal(original.OpenThreeScore, restored.OpenThreeScore);
        Assert.Equal(original.ClosedThreeScore, restored.ClosedThreeScore);
        Assert.Equal(original.OpenTwoScore, restored.OpenTwoScore);
        Assert.Equal(original.CenterBonus, restored.CenterBonus);
        Assert.Equal(original.DefenseMultiplier, restored.DefenseMultiplier);
    }

    [Fact]
    public void ClampToBounds_WorksWithAllParameters()
    {
        // Arrange
        var parameters = new TunableParameters
        {
            FiveInRowScore = double.MaxValue,
            OpenFourScore = double.MinValue,
            DefenseMultiplier = 100.0
        };

        // Act
        parameters.ClampToBounds();

        // Assert
        Assert.Equal(TunableParameters.Bounds[0].Max, parameters.FiveInRowScore);
        Assert.Equal(TunableParameters.Bounds[1].Min, parameters.OpenFourScore);
        Assert.Equal(TunableParameters.Bounds[7].Max, parameters.DefenseMultiplier);
    }

    [Fact]
    public void GetBoundsArrays_ReturnsCorrectLength()
    {
        // Arrange & Act
        var (min, max) = TunableParameters.GetBoundsArrays();

        // Assert
        Assert.Equal(TunableParameters.Names.Length, min.Length);
        Assert.Equal(TunableParameters.Names.Length, max.Length);
    }

    [Fact]
    public void Clone_CreatesIndependentCopy()
    {
        // Arrange
        var original = new TunableParameters
        {
            FiveInRowScore = 150000,
            DefenseMultiplier = 2.0
        };

        // Act
        var clone = original.Clone();
        original.FiveInRowScore = 999999;

        // Assert
        Assert.NotEqual(original.FiveInRowScore, clone.FiveInRowScore);
        Assert.Equal(150000, clone.FiveInRowScore);
    }

    [Fact]
    public void ApplyFromArray_WithInvalidLength_Throws()
    {
        // Arrange
        var parameters = new TunableParameters();
        var wrongLengthArray = new double[] { 1, 2, 3 }; // Should be 8 elements

        // Act & Assert
        Assert.Throws<ArgumentException>(() => parameters.ApplyFromArray(wrongLengthArray));
    }

    [Fact]
    public void SPSAParameters_Presets_AreValid()
    {
        // Assert - verify presets are created correctly
        var defaultParams = SPSAParameters.Default;
        Assert.Equal(0.602, defaultParams.Alpha);
        Assert.Equal(0.101, defaultParams.Gamma);
        Assert.True(defaultParams.A > 0);
        Assert.True(defaultParams.C > 0);

        var aggressive = SPSAParameters.Aggressive;
        Assert.True(aggressive.A > defaultParams.A);  // More aggressive

        var conservative = SPSAParameters.Conservative;
        Assert.True(conservative.A < defaultParams.A);  // Less aggressive
    }

    [Fact]
    public void SPSAParameters_InvalidValues_Throw()
    {
        // Alpha must be in (0, 1)
        Assert.Throws<ArgumentOutOfRangeException>(() => new SPSAParameters(alpha: 0));
        Assert.Throws<ArgumentOutOfRangeException>(() => new SPSAParameters(alpha: 1));
        Assert.Throws<ArgumentOutOfRangeException>(() => new SPSAParameters(alpha: -0.5));

        // Gamma must be in (0, 1)
        Assert.Throws<ArgumentOutOfRangeException>(() => new SPSAParameters(gamma: 0));

        // A must be positive
        Assert.Throws<ArgumentOutOfRangeException>(() => new SPSAParameters(a: 0));
        Assert.Throws<ArgumentOutOfRangeException>(() => new SPSAParameters(a: -1));

        // C must be positive
        Assert.Throws<ArgumentOutOfRangeException>(() => new SPSAParameters(c: 0));
    }

    [Fact]
    public void Bounds_AllPowersOfTwoOrReasonable()
    {
        // Check that bounds are reasonable for game evaluation
        for (int i = 0; i < TunableParameters.Bounds.Length; i++)
        {
            var (min, max) = TunableParameters.Bounds[i];
            Assert.True(min < max, $"Bounds[{i}]: min should be less than max");
            Assert.True(min > 0, $"Bounds[{i}]: min should be positive (except DefenseMultiplier)");
            Assert.True(max > 0, $"Bounds[{i}]: max should be positive");
        }
    }

    [Fact]
    public void BitBoardEvaluator_WithParameters_ProducesConsistentResults()
    {
        // Arrange
        var board = new Board();
        board = board.PlaceStone(9, 9, Player.Red);
        board = board.PlaceStone(10, 10, Player.Blue);
        board = board.PlaceStone(8, 8, Player.Red);

        var defaultParams = TunableParameters.Default;
        var customParams = new TunableParameters
        {
            FiveInRowScore = 150000,
            OpenFourScore = 15000,
            ClosedFourScore = 1500,
            OpenThreeScore = 1500,
            ClosedThreeScore = 150,
            OpenTwoScore = 150,
            CenterBonus = 75,
            DefenseMultiplier = 1.5
        };

        // Act
        var defaultScore = BitBoardEvaluator.EvaluateWithParameters(board, Player.Red, defaultParams);
        var customScore = BitBoardEvaluator.EvaluateWithParameters(board, Player.Red, customParams);

        // Assert - both should produce valid scores
        Assert.True(defaultScore != 0, "Default evaluation should produce non-zero score");
        Assert.True(customScore != 0, "Custom evaluation should produce non-zero score");

        // Different parameters should produce different results in most cases
        // (not guaranteed, but likely for this position)
    }

    [Fact]
    public void BitBoardEvaluator_CustomParametersMatchDefault()
    {
        // Arrange
        var board = new Board();
        board = board.PlaceStone(9, 9, Player.Red);
        board = board.PlaceStone(10, 10, Player.Blue);

        var defaultParams = TunableParameters.Default;

        // Act - evaluate with default static method and parameterized method
        var staticScore = BitBoardEvaluator.Evaluate(board, Player.Red);
        var paramScore = BitBoardEvaluator.EvaluateWithParameters(board, Player.Red, defaultParams);

        // Assert - should produce identical results
        Assert.Equal(staticScore, paramScore);
    }

    [Fact]
    public void DefaultEvaluationParameterProvider_RoundTrip()
    {
        // Arrange
        var original = new TunableParameters
        {
            FiveInRowScore = 123456,
            DefenseMultiplier = 2.0
        };

        var provider = new DefaultEvaluationParameterProvider();

        // Act
        provider.SetParameters(original);
        var retrieved = provider.GetParameters();

        // Assert
        Assert.Equal(original.FiveInRowScore, retrieved.FiveInRowScore);
        Assert.Equal(original.DefenseMultiplier, retrieved.DefenseMultiplier);

        // Modify original should not affect retrieved (clone)
        original.FiveInRowScore = 999999;
        Assert.NotEqual(original.FiveInRowScore, retrieved.FiveInRowScore);
    }
}
