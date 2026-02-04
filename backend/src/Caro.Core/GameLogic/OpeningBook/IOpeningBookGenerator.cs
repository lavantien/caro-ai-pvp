using Caro.Core.Entities;

namespace Caro.Core.GameLogic;

/// <summary>
/// Interface for offline opening book generation.
/// Uses deep search to evaluate positions and store recommended moves.
/// </summary>
public interface IOpeningBookGenerator
{
    /// <summary>
    /// Generate opening book up to specified depth.
    /// Runs recursively from the empty board position.
    /// </summary>
    /// <param name="maxDepth">Maximum ply depth to generate (e.g., 24 for 12 moves per side)</param>
    /// <param name="targetDepth">Search depth for each position evaluation (e.g., 22-26)</param>
    /// <param name="cancellationToken">Cancellation token for async operation</param>
    /// <returns>Generation result with statistics</returns>
    Task<BookGenerationResult> GenerateAsync(
        int maxDepth,
        int targetDepth,
        CancellationToken cancellationToken = default);

    /// <summary>
    /// Verify all book entries for blunders using deep search.
    /// Ensures no stored moves lead to significant tactical disadvantages.
    /// </summary>
    /// <param name="cancellationToken">Cancellation token for async operation</param>
    /// <returns>Verification result with blunder details</returns>
    Task<VerificationResult> VerifyAsync(
        CancellationToken cancellationToken = default);

    /// <summary>
    /// Generate moves for a specific position.
    /// Used for incremental book updates.
    /// </summary>
    /// <param name="board">Board position to evaluate</param>
    /// <param name="player">Player whose turn it is</param>
    /// <param name="searchDepth">Depth to search for evaluation</param>
    /// <param name="maxMoves">Maximum number of moves to store for this position</param>
    /// <param name="cancellationToken">Cancellation token</param>
    /// <returns>Array of evaluated book moves</returns>
    Task<BookMove[]> GenerateMovesForPositionAsync(
        Board board,
        Player player,
        int searchDepth,
        int maxMoves,
        CancellationToken cancellationToken = default);

    /// <summary>
    /// Get the current progress of book generation.
    /// </summary>
    GenerationProgress GetProgress();

    /// <summary>
    /// Cancel the ongoing generation process.
    /// </summary>
    void Cancel();
}

/// <summary>
/// Progress information for book generation.
/// </summary>
public sealed record GenerationProgress(
    int PositionsEvaluated,
    int PositionsStored,
    int TotalPositions,
    double PercentComplete,
    string CurrentPhase,
    TimeSpan ElapsedTime,
    TimeSpan? EstimatedTimeRemaining,

    // Depth-weighted progress fields
    int CurrentDepth = 0,
    int PositionsCompletedAtCurrentDepth = 0,
    int TotalPositionsAtCurrentDepth = 0
);
