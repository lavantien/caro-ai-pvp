using System.Text.Json;

namespace Caro.Core.Domain.Configuration;

/// <summary>
/// Tunable evaluation parameters for SPSA optimization.
/// These parameters control how the AI evaluates board positions.
/// </summary>
public sealed class TunableParameters
{
    // Parameter names for logging/serialization
    public static readonly string[] Names =
    {
        nameof(FiveInRowScore),
        nameof(OpenFourScore),
        nameof(ClosedFourScore),
        nameof(OpenThreeScore),
        nameof(ClosedThreeScore),
        nameof(OpenTwoScore),
        nameof(CenterBonus),
        nameof(DefenseMultiplier)
    };

    // Bounds for each parameter (min, max)
    public static readonly (double Min, double Max)[] Bounds =
    {
        (50000.0, 200000.0),   // FiveInRowScore
        (5000.0, 20000.0),     // OpenFourScore
        (500.0, 2000.0),       // ClosedFourScore
        (500.0, 2000.0),       // OpenThreeScore
        (50.0, 200.0),         // ClosedThreeScore
        (50.0, 200.0),         // OpenTwoScore
        (25.0, 100.0),         // CenterBonus
        (1.0, 3.0)             // DefenseMultiplier
    };

    /// <summary>
    /// Score for five stones in a row (winning position)
    /// </summary>
    public double FiveInRowScore { get; set; } = EvaluationConstants.FiveInRowScore;

    /// <summary>
    /// Score for an open four (four in a row with both ends open)
    /// </summary>
    public double OpenFourScore { get; set; } = EvaluationConstants.OpenFourScore;

    /// <summary>
    /// Score for a closed four (four in a row with one end blocked)
    /// </summary>
    public double ClosedFourScore { get; set; } = EvaluationConstants.ClosedFourScore;

    /// <summary>
    /// Score for an open three (three in a row with both ends open)
    /// </summary>
    public double OpenThreeScore { get; set; } = EvaluationConstants.OpenThreeScore;

    /// <summary>
    /// Score for a closed three (three in a row with one end blocked)
    /// </summary>
    public double ClosedThreeScore { get; set; } = EvaluationConstants.ClosedThreeScore;

    /// <summary>
    /// Score for an open two (two in a row with both ends open)
    /// </summary>
    public double OpenTwoScore { get; set; } = EvaluationConstants.OpenTwoScore;

    /// <summary>
    /// Bonus score for center control
    /// </summary>
    public double CenterBonus { get; set; } = EvaluationConstants.CenterBonus;

    /// <summary>
    /// Defense multiplier for asymmetric scoring.
    /// In Caro, blocking opponent threats is MORE important than creating your own.
    /// </summary>
    public double DefenseMultiplier { get; set; } =
        (double)EvaluationConstants.DefenseMultiplierNumerator / EvaluationConstants.DefenseMultiplierDenominator;

    /// <summary>
    /// Create default parameters from EvaluationConstants
    /// </summary>
    public static TunableParameters Default => new();

    /// <summary>
    /// Convert parameters to array for SPSA optimization
    /// </summary>
    public double[] ToArray()
    {
        return new double[]
        {
            FiveInRowScore,
            OpenFourScore,
            ClosedFourScore,
            OpenThreeScore,
            ClosedThreeScore,
            OpenTwoScore,
            CenterBonus,
            DefenseMultiplier
        };
    }

    /// <summary>
    /// Apply parameters from array (from SPSA optimization)
    /// </summary>
    public void ApplyFromArray(double[] values)
    {
        if (values == null || values.Length != Names.Length)
            throw new ArgumentException($"Expected {Names.Length} values, got {values?.Length ?? 0}");

        FiveInRowScore = values[0];
        OpenFourScore = values[1];
        ClosedFourScore = values[2];
        OpenThreeScore = values[3];
        ClosedThreeScore = values[4];
        OpenTwoScore = values[5];
        CenterBonus = values[6];
        DefenseMultiplier = values[7];
    }

    /// <summary>
    /// Clamp all parameters to their valid bounds
    /// </summary>
    public void ClampToBounds()
    {
        var values = ToArray();
        for (int i = 0; i < values.Length; i++)
        {
            values[i] = Math.Max(Bounds[i].Min, Math.Min(Bounds[i].Max, values[i]));
        }
        ApplyFromArray(values);
    }

    /// <summary>
    /// Create a copy of the parameters
    /// </summary>
    public TunableParameters Clone()
    {
        return new TunableParameters
        {
            FiveInRowScore = FiveInRowScore,
            OpenFourScore = OpenFourScore,
            ClosedFourScore = ClosedFourScore,
            OpenThreeScore = OpenThreeScore,
            ClosedThreeScore = ClosedThreeScore,
            OpenTwoScore = OpenTwoScore,
            CenterBonus = CenterBonus,
            DefenseMultiplier = DefenseMultiplier
        };
    }

    /// <summary>
    /// Serialize to JSON file
    /// </summary>
    public void Save(string path)
    {
        var json = JsonSerializer.Serialize(this, new JsonSerializerOptions
        {
            WriteIndented = true
        });
        File.WriteAllText(path, json);
    }

    /// <summary>
    /// Load from JSON file
    /// </summary>
    public static TunableParameters Load(string path)
    {
        var json = File.ReadAllText(path);
        var parameters = JsonSerializer.Deserialize<TunableParameters>(json);
        if (parameters == null)
            throw new InvalidOperationException($"Failed to load parameters from {path}");
        parameters.ClampToBounds();
        return parameters;
    }

    /// <summary>
    /// Get SPSA bounds as arrays
    /// </summary>
    public static (double[] Min, double[] Max) GetBoundsArrays()
    {
        var min = new double[Bounds.Length];
        var max = new double[Bounds.Length];
        for (int i = 0; i < Bounds.Length; i++)
        {
            min[i] = Bounds[i].Min;
            max[i] = Bounds[i].Max;
        }
        return (min, max);
    }

    public override string ToString()
    {
        return $"FiveInRow={FiveInRowScore:F0}, OpenFour={OpenFourScore:F0}, " +
               $"ClosedFour={ClosedFourScore:F0}, OpenThree={OpenThreeScore:F0}, " +
               $"ClosedThree={ClosedThreeScore:F0}, OpenTwo={OpenTwoScore:F0}, " +
               $"Center={CenterBonus:F0}, DefMult={DefenseMultiplier:F2}";
    }
}
