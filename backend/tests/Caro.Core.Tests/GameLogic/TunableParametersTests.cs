using Caro.Core.Domain.Configuration;
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
}
