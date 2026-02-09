using Caro.Core.GameLogic;
using Xunit;

namespace Caro.Core.Tests.GameLogic;

/// <summary>
/// Tests for SPSA (Simultaneous Perturbation Stochastic Approximation) optimizer.
/// SPSA is a gradient-free optimization algorithm for tuning engine parameters.
///
/// Expected ELO gain: +20-40 through optimized evaluation weights.
/// </summary>
public class SPSATests
{
    [Fact]
    public void PerturbationVector_CorrectRange()
    {
        // Arrange
        var parameters = new SPSAParameters(
            alpha: 0.602,
            gamma: 0.101,
            a: 1.0,
            c: 0.1,
            a_decay: 100,
            c_decay: 10);

        var optimizer = new SPSAOptimizer(parameters);

        // Act - Generate perturbation vector for 10 parameters
        double[] theta = new double[10] { 1, 2, 3, 4, 5, 6, 7, 8, 9, 10 };
        double[] delta = optimizer.GeneratePerturbation(theta.Length);

        // Assert - Delta should be +/-1 values (Bernoulli distribution)
        Assert.Equal(theta.Length, delta.Length);
        foreach (double d in delta)
        {
            Assert.True(d == 1.0 || d == -1.0, $"Delta values should be +/- 1, got: {d}");
        }
    }

    [Fact]
    public void ParameterUpdate_FollowsGradient()
    {
        // Arrange
        var parameters = new SPSAParameters(
            alpha: 0.602,
            gamma: 0.101,
            a: 1.0,
            c: 0.1,
            a_decay: 100,
            c_decay: 10);

        var optimizer = new SPSAOptimizer(parameters);
        double[] theta = new double[2] { 5.0, 10.0 };

        // Act - Simulate two evaluations
        // For minimization: y_plus should be higher (worse), y_minus lower (better)
        double[] delta = optimizer.GeneratePerturbation(2);
        double y_plus = 100.0;  // Higher (worse for minimization)
        double y_minus = 50.0;  // Lower (better for minimization)

        // Update parameters
        double[] newTheta = optimizer.UpdateParameters(theta, delta, y_plus, y_minus);

        // Assert - Parameters should be updated (not equal to original)
        Assert.NotNull(newTheta);
        Assert.Equal(2, newTheta.Length);
        // New theta should differ from original since gradient is non-zero
        Assert.True(newTheta[0] != theta[0] || newTheta[1] != theta[1]);
    }

    [Fact]
    public void Convergence_WithSimpleObjective()
    {
        // Arrange
        var parameters = new SPSAParameters(
            alpha: 0.602,
            gamma: 0.101,
            a: 1.0,      // Smaller A for more stable updates
            c: 0.5,      // Smaller C for smaller perturbations
            a_decay: 50,
            c_decay: 10,
            minValues: new double[] { -20, -20 },
            maxValues: new double[] { 20, 20 });

        // Use a fixed seed for reproducibility
        var optimizer = new SPSAOptimizer(parameters, seed: 42);

        // Objective: minimize f(x,y) = (x-5)^2 + (y-10)^2
        // Optimal at x=5, y=10 with score 0
        // At (0,0): score = 25 + 100 = 125
        double[] theta = new double[2] { 0.0, 0.0 }; // Start far from optimum

        // Act - Run several iterations
        for (int i = 0; i < 100; i++)
        {
            double[] delta = optimizer.GeneratePerturbation(2);
            // Use C at iteration (k+1) since UpdateParameters will increment
            double ck = optimizer.GetGainC(optimizer.CurrentIteration + 1);

            double[] theta_plus = new double[]
            {
                theta[0] + ck * delta[0],
                theta[1] + ck * delta[1]
            };
            double y_plus = SimpleObjectiveMinimize(theta_plus);

            double[] theta_minus = new double[]
            {
                theta[0] - ck * delta[0],
                theta[1] - ck * delta[1]
            };
            double y_minus = SimpleObjectiveMinimize(theta_minus);

            theta = optimizer.UpdateParameters(theta, delta, y_plus, y_minus);
        }

        // Assert - Should converge toward optimum (5, 10)
        // Due to stochastic nature and limited iterations, we just check
        // that we made some progress (not necessarily full convergence)
        double finalScore = SimpleObjectiveMinimize(theta);
        double startScore = SimpleObjectiveMinimize(new double[] { 0, 0 }); // = 125

        Assert.True(finalScore < startScore,
            $"Should improve objective. Start: {startScore}, Final: {finalScore}");
    }

    [Fact]
    public void StepSize_DecaysOverTime()
    {
        // Arrange
        var parameters = new SPSAParameters(
            alpha: 0.602,
            gamma: 0.101,
            a: 100.0,
            c: 1.0,
            a_decay: 50,
            c_decay: 10);

        var optimizer = new SPSAOptimizer(parameters);

        // Act - Get step sizes at different iterations
        double ak1 = optimizer.GetGainA(1);
        double ak50 = optimizer.GetGainA(50);
        double ak100 = optimizer.GetGainA(100);

        // Assert - Step size should decrease over time
        Assert.True(ak1 > ak50, $"Gain A should decrease: a1={ak1}, a50={ak50}");
        Assert.True(ak50 > ak100, $"Gain A should decrease: a50={ak50}, a100={ak100}");
    }

    [Fact]
    public void PerturbationSize_DecaysOverTime()
    {
        // Arrange
        var parameters = new SPSAParameters(
            alpha: 0.602,
            gamma: 0.101,
            a: 100.0,
            c: 1.0,
            a_decay: 50,
            c_decay: 10);

        var optimizer = new SPSAOptimizer(parameters);

        // Act - Get perturbation sizes at different iterations
        double ck1 = optimizer.GetGainC(1);
        double ck10 = optimizer.GetGainC(10);
        double ck50 = optimizer.GetGainC(50);

        // Assert - Perturbation size should decrease over time
        Assert.True(ck1 >= ck10, $"Gain C should decrease: c1={ck1}, c10={ck10}");
        Assert.True(ck10 >= ck50, $"Gain C should decrease: c10={ck10}, c50={ck50}");
    }

    [Fact]
    public void Reset_RestartsIteration()
    {
        // Arrange
        var parameters = new SPSAParameters(
            alpha: 0.602,
            gamma: 0.101,
            a: 100.0,
            c: 1.0,
            a_decay: 50,
            c_decay: 10);

        var optimizer = new SPSAOptimizer(parameters);

        // Act - Run some iterations
        double[] theta = new double[] { 1, 2, 3, 4, 5 };
        double[] delta = optimizer.GeneratePerturbation(5);
        optimizer.UpdateParameters(theta, delta, 100, 50);
        delta = optimizer.GeneratePerturbation(5);
        optimizer.UpdateParameters(theta, delta, 100, 50);
        int iterationBeforeReset = optimizer.CurrentIteration;

        // Reset
        optimizer.Reset();

        // Assert - Iteration should restart
        Assert.Equal(0, optimizer.CurrentIteration);
        Assert.True(iterationBeforeReset > 0);
    }

    [Fact]
    public void Parameters_ClampToBounds()
    {
        // Arrange
        var parameters = new SPSAParameters(
            alpha: 0.602,
            gamma: 0.101,
            a: 1.0,
            c: 0.1,
            a_decay: 100,
            c_decay: 10,
            minValues: new double[] { -10, -10 },
            maxValues: new double[] { 10, 10 });

        var optimizer = new SPSAOptimizer(parameters);
        double[] theta = new double[] { 0.0, 0.0 };

        // Act - Update with extreme gradient
        double[] delta = new double[] { 1, 1 };
        double y_plus = 1000.0;  // Strong positive signal
        double y_minus = -1000.0; // Strong negative signal

        double[] newTheta = optimizer.UpdateParameters(theta, delta, y_plus, y_minus);

        // Assert - Values should be clamped to [-10, 10]
        Assert.InRange(newTheta[0], -10, 10);
        Assert.InRange(newTheta[1], -10, 10);
    }

    [Fact]
    public void CurrentIteration_Increases()
    {
        // Arrange
        var parameters = new SPSAParameters(
            alpha: 0.602,
            gamma: 0.101,
            a: 100.0,
            c: 1.0,
            a_decay: 50,
            c_decay: 10);

        var optimizer = new SPSAOptimizer(parameters);

        // Act & Assert
        Assert.Equal(0, optimizer.CurrentIteration);

        optimizer.GeneratePerturbation(5);
        Assert.Equal(0, optimizer.CurrentIteration); // No increment yet

        double[] theta = new double[] { 1, 2, 3, 4, 5 };
        double[] delta = optimizer.GeneratePerturbation(5);
        optimizer.UpdateParameters(theta, delta, 100, 50);
        Assert.Equal(1, optimizer.CurrentIteration);

        delta = optimizer.GeneratePerturbation(5);
        optimizer.UpdateParameters(theta, delta, 100, 50);
        Assert.Equal(2, optimizer.CurrentIteration);
    }

    // Helper methods

    /// <summary>
    /// Simple quadratic objective function for testing (minimization).
    /// Minimizes (x-5)^2 + (y-10)^2
    /// Optimal at x=5, y=10 with value 0.
    /// </summary>
    private static double SimpleObjectiveMinimize(double[] theta)
    {
        double x = theta[0];
        double y = theta[1];
        // Distance from (5, 10) - we want to minimize this
        return (x - 5) * (x - 5) + (y - 10) * (y - 10);
    }

    /// <summary>
    /// Simple quadratic objective function for testing (maximization).
    /// Maximizes -(x-5)^2 - (y-10)^2
    /// Optimal at x=5, y=10 with value 0.
    /// </summary>
    private static double SimpleObjective(double[] theta)
    {
        double x = theta[0];
        double y = theta[1];
        // Negative distance from (5, 10)
        return -((x - 5) * (x - 5) + (y - 10) * (y - 10));
    }
}
