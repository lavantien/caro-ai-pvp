namespace Caro.Core.GameLogic;

/// <summary>
/// AI difficulty levels optimized for human play (bot vs bot is for testing only)
/// 5 semantic levels with progressive feature disclosure
/// </summary>
public enum AIDifficulty
{
    /// <summary>
    /// D1 Braindead: Depth 1-2 with 50% error rate
    /// Suitable for complete beginners learning the rules
    /// Makes random-ish mistakes, very predictable
    /// </summary>
    Braindead = 1,

    /// <summary>
    /// D2 Easy: Depth 3-4 with basic lookahead
    /// Understands rules but makes mistakes
    /// No advanced optimizations, basic move ordering only
    /// </summary>
    Easy = 2,

    /// <summary>
    /// D3 Medium: Depth 5-6
    /// Club player level with solid tactics
    /// Pondering enabled for better time utilization
    /// </summary>
    Medium = 3,

    /// <summary>
    /// D4 Hard: Depth 7-8
    /// Strong player with deep search
    /// Pondering + Lazy SMP (parallel search with (N/2)-1 threads)
    /// </summary>
    Hard = 4,

    /// <summary>
    /// D5 Grandmaster: Adaptive depth (9-11 based on position)
    /// Maximum strength with position-aware depth selection
    /// Lazy SMP + SIMD + VCF solver (optimized)
    /// Adapts search depth based on game phase and tactical complexity
    /// </summary>
    Grandmaster = 5
}
