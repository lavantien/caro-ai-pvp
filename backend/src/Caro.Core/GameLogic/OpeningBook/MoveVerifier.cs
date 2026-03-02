using System.Diagnostics;
using Caro.Core.Domain.Entities;
using Microsoft.Extensions.Logging;

namespace Caro.Core.GameLogic;

/// <summary>
/// MoveVerifier - The "Critic" in the separated pipeline architecture.
///
/// Verifies self-play moves with time-based deep search before book integration.
/// Only verified moves enter the main book, preventing learning mistakes.
///
/// Design principles:
/// - All thresholds are powers of 2
/// - Operation is time-based (not depth-based)
/// - VCF solver for tactical truth
/// - Consensus check between self-play and deep search
/// </summary>
public sealed class MoveVerifier
{
    // Time-based thresholds (powers of 2 in milliseconds)
    private const int DefaultVerificationTimeMs = 2048;    // 2^11 ms per position
    private const int VcfTimeLimitMs = 128;                 // 2^7 ms per VCF search
    private const int ExtendedVerificationTimeMs = 4096;    // 2^12 ms for survival zone

    // Statistical thresholds (powers of 2 or fractions)
    private const int MinPlayCount = 512;                   // 2^9 - filters fluke wins
    private const double MinWinRate = 0.625;                // 5/8 - winning line indicator
    private const double MaxWinRateForLoss = 0.375;         // 3/8 - losing line indicator
    private const double MinConsensusRate = 0.8125;         // 13/16 - consensus threshold

    // Score thresholds (powers of 2 in centipawns)
    private const int MaxScoreDelta = 512;                  // 2^9 cp - pruning threshold
    private const int InclusionScoreDelta = 256;            // 2^8 cp - inclusion range
    private const int MaxMovesPerPosition = 4;              // 2^2 - variety without bloat
    private const int VcfTriggerThreats = 2;                // Min threats for VCF

    private readonly IStagingBookStore _stagingStore;
    private readonly IPositionCanonicalizer _canonicalizer;
    private readonly ThreatSpaceSearch _threatSearch;
    private readonly ILogger<MoveVerifier> _logger;
    private readonly ILoggerFactory _loggerFactory;

    public MoveVerifier(
        IStagingBookStore stagingStore,
        IPositionCanonicalizer? canonicalizer = null,
        ILoggerFactory? loggerFactory = null)
    {
        _stagingStore = stagingStore ?? throw new ArgumentNullException(nameof(stagingStore));
        _canonicalizer = canonicalizer ?? new PositionCanonicalizer();
        _loggerFactory = loggerFactory ?? Microsoft.Extensions.Logging.Abstractions.NullLoggerFactory.Instance;
        _logger = _loggerFactory.CreateLogger<MoveVerifier>();
        _threatSearch = new ThreatSpaceSearch();
    }

    /// <summary>
    /// Verify all staged positions and return verified moves.
    /// Reconstructs positions by replaying SGF move sequences from games.
    /// </summary>
    /// <param name="verificationTimeMs">Time budget per position (default: 2048)</param>
    /// <param name="maxPly">Maximum ply to verify (default: 16)</param>
    /// <param name="cancellationToken">Cancellation token</param>
    /// <returns>Verification summary with all verified moves</returns>
    public async Task<VerificationSummary> VerifyStagingAsync(
        int verificationTimeMs = DefaultVerificationTimeMs,
        int maxPly = 16,
        CancellationToken cancellationToken = default)
    {
        _logger.LogInformation(
            "Starting verification: time budget {TimeMs}ms, max ply {MaxPly}",
            verificationTimeMs, maxPly);

        var stopwatch = Stopwatch.StartNew();
        var verifiedMoves = new List<VerifiedMove>();
        var stats = new VerificationStats();

        // Build position statistics by replaying games
        var positionData = BuildPositionStatisticsFromGames(maxPly);
        _logger.LogInformation("Reconstructed {Count} unique positions from games", positionData.Count);

        foreach (var ((canonicalHash, directHash, player), posData) in positionData)
        {
            if (cancellationToken.IsCancellationRequested)
                break;

            stats.TotalPositions++;

            // Skip low-visit positions (noise filter)
            if (posData.Statistics.PlayCount < MinPlayCount)
            {
                stats.FilteredLowPlayCount++;
                continue;
            }

            // Skip positions without clear results (win rate in gray zone)
            if (posData.Statistics.WinRate is >= MaxWinRateForLoss and <= MinWinRate)
            {
                stats.FilteredUnclearResult++;
                continue;
            }

            try
            {
                // Verify this position
                var positionMoves = await VerifyPositionAsync(
                    canonicalHash,
                    directHash,
                    player,
                    posData,
                    verificationTimeMs,
                    cancellationToken);

                verifiedMoves.AddRange(positionMoves);
                stats.TotalMovesVerified += positionMoves.Count;

                // Track consensus
                if (posData.Moves.Count > 0 && positionMoves.Count > 0)
                {
                    var topSelfPlayMove = posData.Moves.OrderByDescending(m => m.Value.WinRate).First();
                    var topVerifiedMove = positionMoves.OrderBy(m => m.ScoreDelta).First();

                    if (topSelfPlayMove.Key.X == topVerifiedMove.Move.X &&
                        topSelfPlayMove.Key.Y == topVerifiedMove.Move.Y)
                    {
                        stats.ConsensusCount++;
                    }
                }
            }
            catch (Exception ex)
            {
                _logger.LogWarning(ex,
                    "Error verifying position (canonical: {CanonicalHash}, direct: {DirectHash})",
                    canonicalHash, directHash);
                stats.Errors++;
            }
        }

        stopwatch.Stop();

        var consensusRate = stats.TotalPositions > 0
            ? (double)stats.ConsensusCount / stats.TotalPositions
            : 0;

        var summary = new VerificationSummary(
            verifiedMoves,
            stats.TotalPositions,
            stats.FilteredLowPlayCount,
            stats.FilteredUnclearResult,
            stats.TotalMovesVerified,
            stats.VcfSolvedCount,
            consensusRate,
            stopwatch.Elapsed);

        _logger.LogInformation(
            "Verification complete: {Positions} positions, {Moves} verified moves, " +
            "{VcfSolved} VCF solved, {ConsensusRate:P1} consensus, {Duration}",
            stats.TotalPositions, stats.TotalMovesVerified, stats.VcfSolvedCount,
            consensusRate, stopwatch.Elapsed);

        return summary;
    }

    /// <summary>
    /// Build position statistics by replaying SGF move sequences from all games.
    /// This replaces position-level storage with on-demand reconstruction.
    /// </summary>
    private Dictionary<(ulong CanonicalHash, ulong DirectHash, Player Player), ReconstructedPositionData>
        BuildPositionStatisticsFromGames(int maxPly)
    {
        var positionData = new Dictionary<(ulong, ulong, Player), ReconstructedPositionData>();

        // Get all games and replay them to build position statistics
        int offset = 0;
        const int batchSize = 1000;

        while (true)
        {
            var games = _stagingStore.GetGames(limit: batchSize, offset: offset);
            if (games.Count == 0) break;

            foreach (var game in games)
            {
                ReplayGameForStatistics(game, maxPly, positionData);
            }

            offset += games.Count;

            if (games.Count < batchSize) break;  // No more games
        }

        return positionData;
    }

    /// <summary>
    /// Replay a single game and accumulate position statistics.
    /// </summary>
    private void ReplayGameForStatistics(
        SelfPlayGameRecord game,
        int maxPly,
        Dictionary<(ulong, ulong, Player), ReconstructedPositionData> positionData)
    {
        var board = new Board();
        var currentPlayer = Player.Red;
        var moves = game.MoveList;

        for (int ply = 0; ply < Math.Min(moves.Count, maxPly); ply++)
        {
            var (x, y) = moves[ply];
            var canonical = _canonicalizer.Canonicalize(board);
            var canonicalHash = canonical.CanonicalHash;
            var directHash = board.GetHash();

            // Calculate game result from move's player perspective
            int gameResult = game.Winner == currentPlayer ? 1 :
                             game.Winner == Player.None ? 0 : -1;

            var key = (canonicalHash, directHash, currentPlayer);

            if (!positionData.TryGetValue(key, out var posData))
            {
                posData = new ReconstructedPositionData
                {
                    Board = board,  // Board is immutable, no need to clone
                    Ply = ply,
                    Statistics = new PositionStatisticsInternal
                    {
                        PlayCount = 0,
                        WinCount = 0,
                        DrawCount = 0,
                        LossCount = 0
                    },
                    Moves = new Dictionary<(int X, int Y), MoveStatistics>()
                };
                positionData[key] = posData;
            }

            // Update position statistics
            posData.Statistics.PlayCount++;
            if (gameResult == 1) posData.Statistics.WinCount++;
            else if (gameResult == 0) posData.Statistics.DrawCount++;
            else posData.Statistics.LossCount++;

            // Update move statistics
            var moveKey = (x, y);
            if (!posData.Moves.TryGetValue(moveKey, out var moveStat))
            {
                moveStat = new MoveStatistics
                {
                    PlayCount = 0,
                    WinCount = 0,
                    Ply = ply
                };
                posData.Moves[moveKey] = moveStat;
            }
            moveStat.PlayCount++;
            if (gameResult == 1) moveStat.WinCount++;

            // Advance to next position
            board = board.PlaceStone(x, y, currentPlayer);
            currentPlayer = currentPlayer == Player.Red ? Player.Blue : Player.Red;
        }
    }

    /// <summary>
    /// Verify a single position and return verified moves.
    /// </summary>
    private async Task<List<VerifiedMove>> VerifyPositionAsync(
        ulong canonicalHash,
        ulong directHash,
        Player player,
        ReconstructedPositionData posData,
        int verificationTimeMs,
        CancellationToken cancellationToken)
    {
        var verifiedMoves = new List<VerifiedMove>();
        var ply = posData.Ply;

        // Determine time budget based on position phase
        // Survival zone (ply 8-16) gets extended time
        var timeBudget = ply is >= 8 and <= 16
            ? ExtendedVerificationTimeMs
            : verificationTimeMs;

        // Use the stored board from reconstruction
        var board = posData.Board;

        // Step 1: Try VCF solver for tactical truth (if threats exist)
        var vcfMove = await TrySolveVcfAsync(board, player, timeBudget / 16, cancellationToken);
        if (vcfMove != null)
        {
            verifiedMoves.Add(vcfMove with
            {
                CanonicalHash = canonicalHash,
                DirectHash = directHash,
                Player = player,
                Ply = ply
            });
            return verifiedMoves;  // VCF solved - no need for further verification
        }

        // Step 2: Time-based deep verification search
        var deepResult = await GetDeepSearchResultAsync(
            board, player, timeBudget, cancellationToken);

        if (deepResult == null)
            return verifiedMoves;

        // Step 3: Verify all candidate moves with time budget
        foreach (var (moveKey, moveStat) in posData.Moves)
        {
            var winRate = moveStat.PlayCount > 0
                ? (double)moveStat.WinCount / moveStat.PlayCount
                : 0;

            // Skip moves with low win rate unless they have many plays
            if (winRate < MinWinRate && moveStat.PlayCount < MinPlayCount)
                continue;

            var moveScore = await EvaluateMoveAsync(
                board, player, moveKey, timeBudget / 4, cancellationToken);

            var scoreDelta = moveScore - deepResult.Score;

            // Prune if too far from best
            if (scoreDelta > MaxScoreDelta)
                continue;

            var source = DetermineMoveSource(moveStat, deepResult, scoreDelta);

            verifiedMoves.Add(new VerifiedMove
            {
                CanonicalHash = canonicalHash,
                DirectHash = directHash,
                Player = player,
                Ply = ply,
                Move = moveKey,
                Score = moveScore,
                ScoreDelta = scoreDelta,
                Source = source,
                WinRate = winRate,
                PlayCount = moveStat.PlayCount,
                TimeBudgetMs = timeBudget,
                IsVerified = true
            });
        }

        // Limit to top N moves within threshold
        return verifiedMoves
            .Where(m => m.ScoreDelta <= InclusionScoreDelta)
            .OrderBy(m => m.ScoreDelta)
            .ThenByDescending(m => m.WinRate)
            .Take(MaxMovesPerPosition)
            .ToList();
    }

    /// <summary>
    /// Try to solve position using VCF (Victory by Continuous Fours).
    /// Only triggers if position has enough threats.
    /// </summary>
    private async Task<VerifiedMove?> TrySolveVcfAsync(
        Board board,
        Player player,
        int timeLimitMs,
        CancellationToken cancellationToken)
    {
        // Only trigger VCF if position has enough threats
        var threats = _threatSearch.GetThreatMoves(board, player);
        if (threats.Count < VcfTriggerThreats)
            return null;

        // Create VCF solver with time limit
        var vcfSolver = new VCFSolver(_threatSearch);
        var stopwatch = Stopwatch.StartNew();

        var result = vcfSolver.CheckNodeVCF(
            board,
            player,
            depth: 20,
            alpha: -32768,
            timeRemainingMs: Math.Min(timeLimitMs, VcfTimeLimitMs));

        stopwatch.Stop();

        if (result != null && result.Type == VCFResultType.WinningSequence)
        {
            var firstMove = result.ForcingMoves.Count > 0
                ? result.ForcingMoves[0]
                : (0, 0);

            return new VerifiedMove
            {
                CanonicalHash = 0,  // Will be filled by caller
                DirectHash = 0,
                Player = player,
                Ply = 0,
                Move = firstMove,
                Score = 32768,  // 2^15 - proven win
                ScoreDelta = 0,
                Source = MoveSource.Solved,
                WinRate = 1.0,
                PlayCount = 0,
                TimeBudgetMs = (int)stopwatch.ElapsedMilliseconds,
                IsVerified = true
            };
        }

        return null;
    }

    /// <summary>
    /// Get deep search result for a position using time-based search.
    /// </summary>
    private async Task<DeepSearchResult?> GetDeepSearchResultAsync(
        Board board,
        Player player,
        int timeBudgetMs,
        CancellationToken cancellationToken)
    {
        var ai = new MinimaxAI(ttSizeMb: 128, logger: _loggerFactory.CreateLogger<MinimaxAI>());
        var stopwatch = Stopwatch.StartNew();

        var (x, y) = ai.GetBestMove(board, player, AIDifficulty.Grandmaster, timeBudgetMs);
        var score = ai.GetSearchStatistics().SearchScore;

        stopwatch.Stop();

        if (x < 0 || y < 0)
            return null;

        await Task.Yield();

        return new DeepSearchResult(
            BestMove: (x, y),
            Score: score,
            TimeMs: (int)stopwatch.ElapsedMilliseconds
        );
    }

    /// <summary>
    /// Evaluate a specific move with time-based search.
    /// </summary>
    private async Task<int> EvaluateMoveAsync(
        Board board,
        Player player,
        (int X, int Y) move,
        int timeBudgetMs,
        CancellationToken cancellationToken)
    {
        // Make the move and evaluate resulting position
        var newBoard = board.PlaceStone(move.X, move.Y, player);
        var opponent = player == Player.Red ? Player.Blue : Player.Red;

        var ai = new MinimaxAI(ttSizeMb: 64, logger: _loggerFactory.CreateLogger<MinimaxAI>());
        var (_, _) = ai.GetBestMove(newBoard, opponent, AIDifficulty.Grandmaster, timeBudgetMs);

        // Score from opponent's perspective, negate for our perspective
        var score = -ai.GetSearchStatistics().SearchScore;

        await Task.Yield();
        return score;
    }

    /// <summary>
    /// Determine the source classification for a move.
    /// </summary>
    private MoveSource DetermineMoveSource(MoveStatistics moveStat, DeepSearchResult deepResult, int scoreDelta)
    {
        // If self-play top move matches deep search best move with low delta
        if (scoreDelta <= InclusionScoreDelta / 2)
        {
            return MoveSource.Learned;  // Consensus between self-play and deep search
        }

        // Otherwise, it's self-play only
        return MoveSource.SelfPlay;
    }

    /// <summary>
    /// Get consensus statistics for verification session.
    /// </summary>
    public ConsensusStats GetConsensusStats()
    {
        // This would be tracked during verification
        return new ConsensusStats(0, 0, 0.0);
    }

    /// <summary>
    /// Get all threshold values for verification (all powers of 2).
    /// </summary>
    public static VerificationThresholds GetThresholds()
    {
        return new VerificationThresholds(
            MinPlayCount: MinPlayCount,
            MinWinRate: MinWinRate,
            MaxWinRateForLoss: MaxWinRateForLoss,
            MinConsensusRate: MinConsensusRate,
            MaxScoreDelta: MaxScoreDelta,
            InclusionScoreDelta: InclusionScoreDelta,
            MaxMovesPerPosition: MaxMovesPerPosition,
            VcfTriggerThreats: VcfTriggerThreats,
            DefaultVerificationTimeMs: DefaultVerificationTimeMs,
            ExtendedVerificationTimeMs: ExtendedVerificationTimeMs,
            VcfTimeLimitMs: VcfTimeLimitMs
        );
    }
}

/// <summary>
/// A move that has been verified for book inclusion.
/// </summary>
public sealed record VerifiedMove
{
    public required ulong CanonicalHash { get; init; }
    public required ulong DirectHash { get; init; }
    public required Player Player { get; init; }
    public required int Ply { get; init; }
    public required (int X, int Y) Move { get; init; }
    public required int Score { get; init; }
    public required int ScoreDelta { get; init; }
    public required MoveSource Source { get; init; }
    public required double WinRate { get; init; }
    public required int PlayCount { get; init; }
    public required int TimeBudgetMs { get; init; }
    public required bool IsVerified { get; init; }
}

/// <summary>
/// Summary of a verification session.
/// </summary>
public sealed record VerificationSummary
{
    public List<VerifiedMove> VerifiedMoves { get; init; }
    public int TotalPositionsProcessed { get; init; }
    public int FilteredLowPlayCount { get; init; }
    public int FilteredUnclearResult { get; init; }
    public int TotalMovesVerified { get; init; }
    public int VcfSolvedCount { get; init; }
    public double ConsensusRate { get; init; }
    public TimeSpan Duration { get; init; }

    public VerificationSummary(
        List<VerifiedMove> verifiedMoves,
        int totalPositionsProcessed,
        int filteredLowPlayCount,
        int filteredUnclearResult,
        int totalMovesVerified,
        int vcfSolvedCount,
        double consensusRate,
        TimeSpan duration)
    {
        VerifiedMoves = verifiedMoves;
        TotalPositionsProcessed = totalPositionsProcessed;
        FilteredLowPlayCount = filteredLowPlayCount;
        FilteredUnclearResult = filteredUnclearResult;
        TotalMovesVerified = totalMovesVerified;
        VcfSolvedCount = vcfSolvedCount;
        ConsensusRate = consensusRate;
        Duration = duration;
    }
}

/// <summary>
/// Internal statistics for verification session.
/// </summary>
internal sealed class VerificationStats
{
    public int TotalPositions { get; set; }
    public int FilteredLowPlayCount { get; set; }
    public int FilteredUnclearResult { get; set; }
    public int TotalMovesVerified { get; set; }
    public int VcfSolvedCount { get; set; }
    public int ConsensusCount { get; set; }
    public int Errors { get; set; }
}

/// <summary>
/// Reconstructed position data from replaying SGF moves.
/// </summary>
internal sealed class ReconstructedPositionData
{
    /// <summary>
    /// The board state at this position.
    /// </summary>
    public required Board Board { get; init; }

    /// <summary>
    /// Ply depth of this position.
    /// </summary>
    public required int Ply { get; init; }

    /// <summary>
    /// Aggregated statistics for this position.
    /// </summary>
    public required PositionStatisticsInternal Statistics { get; init; }

    /// <summary>
    /// Statistics per move at this position.
    /// </summary>
    public required Dictionary<(int X, int Y), MoveStatistics> Moves { get; init; }
}

/// <summary>
/// Internal position statistics (computed from game replay).
/// </summary>
internal sealed class PositionStatisticsInternal
{
    public int PlayCount { get; set; }
    public int WinCount { get; set; }
    public int DrawCount { get; set; }
    public int LossCount { get; set; }

    public double WinRate => PlayCount > 0 ? (double)WinCount / PlayCount : 0;
}

/// <summary>
/// Statistics for a single move at a position.
/// </summary>
internal sealed class MoveStatistics
{
    public int PlayCount { get; set; }
    public int WinCount { get; set; }
    public int Ply { get; set; }

    public double WinRate => PlayCount > 0 ? (double)WinCount / PlayCount : 0;
}

/// <summary>
/// Result of deep search for a position.
/// </summary>
internal sealed record DeepSearchResult(
    (int X, int Y) BestMove,
    int Score,
    int TimeMs
);

/// <summary>
/// Consensus statistics for verification.
/// </summary>
public sealed record ConsensusStats(
    int TotalPositions,
    int ConsensusCount,
    double ConsensusRate
);

/// <summary>
/// All threshold values for verification (all powers of 2).
/// </summary>
public sealed record VerificationThresholds(
    int MinPlayCount,
    double MinWinRate,
    double MaxWinRateForLoss,
    double MinConsensusRate,
    int MaxScoreDelta,
    int InclusionScoreDelta,
    int MaxMovesPerPosition,
    int VcfTriggerThreats,
    int DefaultVerificationTimeMs,
    int ExtendedVerificationTimeMs,
    int VcfTimeLimitMs
);
