namespace Caro.Core.GameLogic;

/// <summary>
/// AI difficulty levels for Minimax algorithm
/// Optimized for 7+5 time control (7 minutes initial + 5 second increment)
///
/// Complete 11-level gradient from beginner to world-class:
/// D1-D3: Casual/beginner levels with basic lookahead
/// D4-D6: Club player levels with solid tactics
/// D7-D9: Tournament/master levels with deep search
/// D10-D11: Grandmaster levels with maximum strength
/// </summary>
public enum AIDifficulty
{
    /// <summary>
    /// D1 Beginner: Depth 1 with 20% randomness
    /// Suitable for complete beginners learning the rules
    /// </summary>
    Beginner = 1,

    /// <summary>
    /// D2 Easy: Depth 2 with basic lookahead
    /// Understands rules but makes mistakes
    /// </summary>
    Easy = 2,

    /// <summary>
    /// D3 Normal: Depth 3
    /// Casual player level, basic tactical awareness
    /// </summary>
    Normal = 3,

    /// <summary>
    /// D4 Medium: Depth 4
    /// Club player developing skills, decent tactics
    /// </summary>
    Medium = 4,

    /// <summary>
    /// D5 Hard: Depth 5
    /// Strong club player, solid tactical play
    /// </summary>
    Hard = 5,

    /// <summary>
    /// D6 Harder: Depth 6
    /// Experienced player, good pattern recognition
    /// </summary>
    Harder = 6,

    /// <summary>
    /// D7 Very Hard: Depth 7
    /// Tournament level, deep search with all optimizations
    /// Uses: PVS, LMR, Quiescence, TT, History, Aspiration Windows
    /// Parallel search (Lazy SMP) enabled for multi-core speedup
    /// </summary>
    VeryHard = 7,

    /// <summary>
    /// D8 Expert: Depth 8
    /// High-level club player, very deep search
    /// Aggressive pruning with SIMD pattern evaluation
    /// </summary>
    Expert = 8,

    /// <summary>
    /// D9 Master: Depth 9
    /// National master strength
    /// Full optimization pipeline with extended search
    /// </summary>
    Master = 9,

    /// <summary>
    /// D10 Grandmaster: Depth 10
    /// International master level, near-maximum depth
    /// Time-aware iterative deepening with move stability
    /// </summary>
    Grandmaster = 10,

    /// <summary>
    /// D11 Legend: Depth 11+
    /// World-class level, maximum strength
    /// Uses all available techniques: parallel search, deep TT, VCF solver
    /// May extend to depth 12+ in favorable positions with time remaining
    /// </summary>
    Legend = 11
}
