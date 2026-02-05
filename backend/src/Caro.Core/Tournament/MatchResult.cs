using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;

namespace Caro.Core.Tournament;

/// <summary>
/// Result of a single AI vs AI match
/// </summary>
public class MatchResult
{
    public required Player Winner { get; init; }
    public required Player Loser { get; init; }
    public required int TotalMoves { get; init; }
    public required long DurationMs { get; init; }
    public required List<long> MoveTimesMs { get; init; } // Time for each move
    public required AIDifficulty WinnerDifficulty { get; init; }
    public required AIDifficulty LoserDifficulty { get; init; }
    public required Board FinalBoard { get; init; }
    public required string WinnerBotName { get; init; }
    public required string LoserBotName { get; init; }

    /// <summary>
    /// Average move time in milliseconds
    /// </summary>
    public double AverageMoveTimeMs => MoveTimesMs.Count > 0
        ? MoveTimesMs.Average()
        : 0;

    /// <summary>
    /// Was this a draw (both players ran out of time)?
    /// </summary>
    public bool IsDraw { get; init; }

    /// <summary>
    /// Did the game end by timeout?
    /// </summary>
    public bool EndedByTimeout { get; init; }
}
