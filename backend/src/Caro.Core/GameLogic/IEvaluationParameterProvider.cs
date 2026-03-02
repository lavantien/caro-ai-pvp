using Caro.Core.Domain.Configuration;

namespace Caro.Core.GameLogic;

/// <summary>
/// Interface for providing evaluation parameters to the AI.
/// Allows runtime parameter injection for SPSA tuning.
/// </summary>
public interface IEvaluationParameterProvider
{
    /// <summary>
    /// Get the current tunable parameters
    /// </summary>
    TunableParameters GetParameters();

    /// <summary>
    /// Update the parameters (for SPSA tuning)
    /// </summary>
    void SetParameters(TunableParameters parameters);
}

/// <summary>
/// Default implementation that uses static EvaluationConstants
/// </summary>
public sealed class DefaultEvaluationParameterProvider : IEvaluationParameterProvider
{
    private TunableParameters _parameters = new();

    public TunableParameters GetParameters() => _parameters.Clone();

    public void SetParameters(TunableParameters parameters)
    {
        _parameters = parameters.Clone();
    }
}
