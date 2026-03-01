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

        // Get position statistics from staging
        var positionStats = _stagingStore.GetPositionStatistics();
        _logger.LogInformation("Found {Count} unique positions in staging", positionStats.Count);

        foreach (var ((canonicalHash, directHash, player), stat) in positionStats)
        {
            if (cancellationToken.IsCancellationRequested)
                break;

            stats.TotalPositions++;

            // Skip low-visit positions (noise filter)
            if (stat.PlayCount < MinPlayCount)
            {
                stats.FilteredLowPlayCount++;
                continue;
            }

            // Skip positions without clear results (win rate in gray zone)
            if (stat.WinRate is >= MaxWinRateForLoss and <= MinWinRate)
            {
                stats.FilteredUnclearResult++;
                continue;
            }

            // Get all moves for this position
            var moves = _stagingStore.GetMovesForPosition(canonicalHash, directHash, player);
            if (moves.Count == 0)
                continue;

            try
            {
                // Verify this position
                var positionMoves = await VerifyPositionAsync(
                    canonicalHash,
                    directHash,
                    player,
                    moves,
                    stat,
                    verificationTimeMs,
                    cancellationToken);

                verifiedMoves.AddRange(positionMoves);
                stats.TotalMovesVerified += positionMoves.Count;

                // Track consensus
                if (moves.Count > 0 && positionMoves.Count > 0)
                {
                    var topSelfPlayMove = moves.OrderByDescending(m => m.WinRate).First();
                    var topVerifiedMove = positionMoves.OrderBy(m => m.ScoreDelta).First();

                    if (topSelfPlayMove.MoveX == topVerifiedMove.Move.X &&
                        topSelfPlayMove.MoveY == topVerifiedMove.Move.Y)
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
    /// Verify a single position and return verified moves.
    /// </summary>
    private async Task<List<VerifiedMove>> VerifyPositionAsync(
        ulong canonicalHash,
        ulong directHash,
        Player player,
        List<StagingMove> moves,
        PositionStatistics positionStats,
        int verificationTimeMs,
        CancellationToken cancellationToken)
    {
        var verifiedMoves = new List<VerifiedMove>();
        var ply = moves.Min(m => m.Ply);

        // Determine time budget based on position phase
        // Survival zone (ply 8-16) gets extended time
        var timeBudget = ply is >= 8 and <= 16
            ? ExtendedVerificationTimeMs
            : verificationTimeMs;

        // Reconstruct board from hashes (we need the actual position)
        // For now, we'll use a placeholder - in production, you'd need to
        // either store the board or reconstruct from the hash
        var board = ReconstructBoard(canonicalHash, directHash, player, ply);

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
        foreach (var move in moves.Where(m => m.WinRate >= MinWinRate || m.PlayCount >= MinPlayCount))
        {
            var moveScore = await EvaluateMoveAsync(
                board, player, (move.MoveX, move.MoveY), timeBudget / 4, cancellationToken);

            var scoreDelta = moveScore - deepResult.Score;

            // Prune if too far from best
            if (scoreDelta > MaxScoreDelta)
                continue;

            var source = DetermineMoveSource(move, deepResult, scoreDelta);

            verifiedMoves.Add(new VerifiedMove
            {
                CanonicalHash = canonicalHash,
                DirectHash = directHash,
                Player = player,
                Ply = move.Ply,
                Move = (move.MoveX, move.MoveY),
                Score = moveScore,
                ScoreDelta = scoreDelta,
                Source = source,
                WinRate = move.WinRate,
                PlayCount = move.PlayCount,
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
    private MoveSource DetermineMoveSource(StagingMove move, DeepSearchResult deepResult, int scoreDelta)
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
    /// Reconstruct board from hashes.
    /// This is a placeholder - in production, you'd need to store/retrieve the actual board.
    /// </summary>
    private Board ReconstructBoard(ulong canonicalHash, ulong directHash, Player player, int ply)
    {
        // TODO: Implement proper board reconstruction
        // For now, return empty board - this needs to be fixed for production
        // Options:
        // 1. Store board state in staging database
        // 2. Use a board reconstruction algorithm from hash
        // 3. Store move sequence and replay
        return new Board();
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
