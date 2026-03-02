using Caro.Core.Domain.Entities;
using Microsoft.Extensions.Logging;

namespace Caro.Core.GameLogic;

/// <summary>
/// Generates opening book positions through self-play (engine vs engine).
///
/// Separated Pipeline Architecture (Actor Phase):
/// - Records ALL moves to staging store (not main book)
/// - Staging is verified later by MoveVerifier (Critic phase)
/// - Only verified moves enter the main book
///
/// Design principles:
/// - Parallel game execution (1 thread per game for max throughput)
/// - Temperature-based sampling with score delta threshold
/// - Dirichlet noise for opening diversity
/// - Store games in SGF format (one row per game)
/// - Time control: 1+0 (1 minute + 0 second increment) - fast games for volume
/// </summary>
public sealed class SelfPlayGenerator
{
    // Time control defaults - fast games for high volume
    private const int DefaultBaseTimeMs = 60000;     // 1 minute = 60,000ms
    private const int DefaultIncrementMs = 0;        // No increment

    // Sampling thresholds
    private const int ScoreDeltaThreshold = 400;      // Prune moves >400cp worse
    private const double DirichletEpsilon = 0.25;     // Noise weight
    private const double DirichletAlpha = 0.3;        // Noise concentration

    // Parallel execution - 1 thread per game maximizes CPU throughput
    private const int DefaultWorkerCount = 16;  // Will be overridden by Environment.ProcessorCount
    private const int ThreadsPerWorker = 1;     // Single thread per game avoids SMP overhead

    private readonly IStagingBookStore _stagingStore;
    private readonly IOpeningBookStore? _bookStore;
    private readonly IPositionCanonicalizer _canonicalizer;
    private readonly ILogger<SelfPlayGenerator> _logger;
    private readonly ILoggerFactory _loggerFactory;
    private readonly Random _random = new();

    public SelfPlayGenerator(
        IStagingBookStore stagingStore,
        IPositionCanonicalizer? canonicalizer = null,
        ILoggerFactory? loggerFactory = null,
        IOpeningBookStore? bookStore = null)
    {
        _stagingStore = stagingStore ?? throw new ArgumentNullException(nameof(stagingStore));
        _canonicalizer = canonicalizer ?? new PositionCanonicalizer();
        _loggerFactory = loggerFactory ?? Microsoft.Extensions.Logging.Abstractions.NullLoggerFactory.Instance;
        _logger = _loggerFactory.CreateLogger<SelfPlayGenerator>();
        _bookStore = bookStore;
    }

    /// <summary>
    /// Legacy constructor - throws exception directing to new API.
    /// </summary>
    [Obsolete("Use constructor with IStagingBookStore for separated pipeline")]
    public SelfPlayGenerator(
        IOpeningBookStore store,
        IPositionCanonicalizer? canonicalizer = null,
        ILoggerFactory? loggerFactory = null)
    {
        throw new InvalidOperationException(
            "SelfPlayGenerator now requires IStagingBookStore. " +
            "Use the constructor with IStagingBookStore for the separated pipeline.");
    }

    /// <summary>
    /// Generate self-play games with parallel execution.
    /// Uses temperature-based sampling and stores games in SGF format.
    /// </summary>
    public async Task<SelfPlaySummary> GenerateGamesAsync(
        int gameCount,
        int baseTimeMs = DefaultBaseTimeMs,
        int incrementMs = DefaultIncrementMs,
        int maxMoves = 200,
        int maxPly = 16,
        int workerCount = DefaultWorkerCount,
        CancellationToken cancellationToken = default)
    {
        return await GenerateGamesAsync(
            new Board(),
            Player.Red,
            gameCount,
            baseTimeMs,
            incrementMs,
            maxMoves,
            maxPly,
            workerCount,
            cancellationToken);
    }

    /// <summary>
    /// Generate self-play games starting from a specific position.
    /// </summary>
    public async Task<SelfPlaySummary> GenerateGamesAsync(
        Board startingPosition,
        Player startingPlayer,
        int gameCount,
        int baseTimeMs = DefaultBaseTimeMs,
        int incrementMs = DefaultIncrementMs,
        int maxMoves = 200,
        int maxPly = 16,
        int workerCount = DefaultWorkerCount,
        CancellationToken cancellationToken = default)
    {
        var summary = new SelfPlaySummary
        {
            MaxPlyRecorded = maxPly,
            TimeControl = $"{baseTimeMs / 60000}+{incrementMs / 1000}"
        };

        _logger.LogInformation(
            "Starting parallel self-play: {Games} games, {Workers} workers, " +
            "time control: {TimeControl}, max ply: {MaxPly}",
            gameCount, workerCount, summary.TimeControl, maxPly);

        // Initialize staging store
        _stagingStore.Initialize();

        // Create worker tasks for parallel execution
        var gamesPerWorker = gameCount / workerCount;
        var extraGames = gameCount % workerCount;

        var tasks = new List<Task<WorkerSummary>>();
        var completedGames = 0;
        var lockObj = new object();

        for (int w = 0; w < workerCount; w++)
        {
            var workerGameCount = gamesPerWorker + (w < extraGames ? 1 : 0);
            if (workerGameCount == 0) continue;

            var workerId = w;
            // Note: Do NOT pass cancellationToken to Task.Run - let tasks handle cancellation
            // internally via IsCancellationRequested checks. Passing the token causes
            // Task.WhenAll to throw TaskCanceledException before we can return partial results.
            var task = Task.Run(async () =>
            {
                var workerSummary = new WorkerSummary { WorkerId = workerId };

                for (int g = 0; g < workerGameCount && !cancellationToken.IsCancellationRequested; g++)
                {
                    try
                    {
                        var game = await PlayGameAsync(
                            startingPosition,
                            startingPlayer,
                            baseTimeMs,
                            incrementMs,
                            maxMoves,
                            cancellationToken);

                        // Record game to staging (SGF format)
                        RecordGameToStaging(game, summary.TimeControl, maxPly);

                        lock (lockObj)
                        {
                            if (game.Winner == Player.Red)
                                workerSummary.RedWins++;
                            else if (game.Winner == Player.Blue)
                                workerSummary.BlueWins++;
                            else
                                workerSummary.Draws++;

                            workerSummary.TotalMoves += game.TotalMoves;
                            workerSummary.GamesPlayed++;
                            completedGames++;

                            // Progress reporting
                            if (completedGames % 10 == 0)
                            {
                                _logger.LogInformation(
                                    "Self-play progress: {Completed}/{Total} games",
                                    completedGames, gameCount);
                            }
                        }
                    }
                    catch (OperationCanceledException)
                    {
                        break;
                    }
                    catch (Exception ex)
                    {
                        _logger.LogError(ex, "Error in worker {WorkerId} game {GameId}", workerId, g);
                    }
                }

                return workerSummary;
            });

            tasks.Add(task);
        }

        // Wait for all workers to complete
        WorkerSummary[] workerSummaries;
        try
        {
            workerSummaries = await Task.WhenAll(tasks);
        }
        catch (OperationCanceledException)
        {
            // Cancellation requested - return what we have so far
            _logger.LogInformation("Self-play cancelled after {Games} games", summary.TotalGames);
            return summary;
        }

        // Aggregate results
        foreach (var ws in workerSummaries)
        {
            summary.RedWins += ws.RedWins;
            summary.BlueWins += ws.BlueWins;
            summary.Draws += ws.Draws;
            summary.TotalMoves += ws.TotalMoves;
            summary.StagingMovesRecorded += ws.GamesPlayed * maxPly; // Approximate
        }

        // Flush staging store
        _stagingStore.Flush();

        _logger.LogInformation(
            "Self-play complete: {Total} games, R:{Red} B:{Blue} D:{Draw}, avg moves: {Avg:F1}",
            summary.TotalGames, summary.RedWins, summary.BlueWins, summary.Draws, summary.AverageMoves);

        return summary;
    }

    /// <summary>
    /// Play a single game with temperature-based sampling.
    /// </summary>
    private async Task<PlayedGame> PlayGameAsync(
        Board startPosition,
        Player startPlayer,
        int baseTimeMs,
        int incrementMs,
        int maxMoves,
        CancellationToken cancellationToken)
    {
        var board = startPosition;
        var currentPlayer = startPlayer;
        var moveList = new List<(int X, int Y)>();

        // Create AI with SelfPlay difficulty (Grandmaster-level but no book)
        var ai = new MinimaxAI(ttSizeMb: 64, logger: _loggerFactory.CreateLogger<MinimaxAI>());

        // Time tracking per player
        var redTime = baseTimeMs;
        var blueTime = baseTimeMs;

        int moveCount = 0;
        Player winner = Player.None;

        while (moveCount < maxMoves && !cancellationToken.IsCancellationRequested)
        {
            var moveStartTime = DateTime.UtcNow;

            // Get move with temperature-based sampling
            var (x, y) = SelectMoveWithSampling(
                board,
                currentPlayer,
                ai,
                moveCount);  // ply = moveCount

            if (x < 0 || y < 0)
            {
                // No valid move available
                break;
            }

            // Record move
            moveList.Add((x, y));

            // Make move
            board = board.PlaceStone(x, y, currentPlayer);
            moveCount++;

            // Update time
            var elapsed = (int)(DateTime.UtcNow - moveStartTime).TotalMilliseconds;
            var increment = currentPlayer == Player.Red ? incrementMs : incrementMs;

            if (currentPlayer == Player.Red)
            {
                redTime -= elapsed;
                redTime += increment;
                if (redTime <= 0)
                {
                    winner = Player.Blue;  // Red loses on time
                    break;
                }
            }
            else
            {
                blueTime -= elapsed;
                blueTime += increment;
                if (blueTime <= 0)
                {
                    winner = Player.Red;  // Blue loses on time
                    break;
                }
            }

            // Check for win
            var winResult = new WinDetector().CheckWin(board);
            if (winResult.Winner != Player.None)
            {
                winner = winResult.Winner;
                break;
            }

            // Switch player
            currentPlayer = currentPlayer == Player.Red ? Player.Blue : Player.Red;

            await Task.Yield();
        }

        // Clear AI state between games to prevent position leakage
        ai.ClearTranspositionTable();

        return new PlayedGame(winner, moveCount, moveList);
    }

    /// <summary>
    /// Select move using temperature-based sampling with safety filters.
    /// </summary>
    private (int X, int Y) SelectMoveWithSampling(
        Board board,
        Player player,
        MinimaxAI ai,
        int ply)
    {
        // Get all candidate moves with scores using quick evaluation
        var candidates = ai.GetCandidateMovesWithScores(
            board, player, AIDifficulty.Grandmaster, timeMs: 500);

        if (candidates.Count == 0)
        {
            return (-1, -1);
        }

        // Step 1: Score Delta Threshold (safety filter)
        var bestScore = candidates.Max(c => c.Score);
        var safeCandidates = candidates
            .Where(c => bestScore - c.Score < ScoreDeltaThreshold)
            .ToList();

        // Fallback: If all moves filtered, keep the best one
        if (safeCandidates.Count == 0)
        {
            safeCandidates = candidates.OrderByDescending(c => c.Score).Take(1).ToList();
        }

        // Step 2: Apply Dirichlet Noise for early opening (plies 0-6)
        if (ply < 6)
        {
            ApplyDirichletNoise(safeCandidates);
        }

        // Step 3: Dynamic Temperature
        double temperature = GetTemperature(ply);

        // Step 4: Softmax Sampling
        return SampleMove(safeCandidates, temperature);
    }

    /// <summary>
    /// Get dynamic temperature based on game phase.
    /// </summary>
    private static double GetTemperature(int ply)
    {
        // Plies 0-8 (Opening): High temp 1.5-2.0 for diversity
        // Plies 8-16 (Transition): Medium temp 0.8-1.2
        // Plies 16+ (Midgame): No randomness - optimal play
        return ply switch
        {
            < 8 => 1.8,
            < 16 => 1.0,
            _ => 0.0
        };
    }

    /// <summary>
    /// Apply Dirichlet noise to move priors for exploration.
    /// </summary>
    private void ApplyDirichletNoise(List<MoveCandidate> candidates)
    {
        if (candidates.Count == 0) return;

        // Generate Dirichlet noise
        var noise = GenerateDirichletNoise(candidates.Count, DirichletAlpha);

        for (int i = 0; i < candidates.Count; i++)
        {
            // Blend original score with noise
            var originalWeight = 1.0 - DirichletEpsilon;
            var noisyScore = originalWeight * candidates[i].Score + DirichletEpsilon * noise[i] * 1000;
            candidates[i] = candidates[i] with { Score = (int)noisyScore };
        }
    }

    /// <summary>
    /// Generate Dirichlet-distributed noise.
    /// </summary>
    private double[] GenerateDirichletNoise(int count, double alpha)
    {
        var noise = new double[count];
        double sum = 0;

        // Sample from Gamma distribution (approximation of Dirichlet)
        for (int i = 0; i < count; i++)
        {
            // Box-Muller transform for Gamma approximation
            var u1 = _random.NextDouble();
            var u2 = _random.NextDouble();

            // Simple exponential distribution as Gamma(alpha, 1) approximation
            var sample = -Math.Log(u1) / alpha;
            noise[i] = Math.Max(0.001, sample);
            sum += noise[i];
        }

        // Normalize
        if (sum > 0)
        {
            for (int i = 0; i < count; i++)
            {
                noise[i] /= sum;
            }
        }

        return noise;
    }

    /// <summary>
    /// Sample a move using softmax over scores.
    /// </summary>
    private (int X, int Y) SampleMove(List<MoveCandidate> candidates, double temperature)
    {
        if (candidates.Count == 0)
        {
            return (-1, -1);
        }

        if (candidates.Count == 1)
        {
            return (candidates[0].X, candidates[0].Y);
        }

        // Compute softmax probabilities
        var logits = candidates.Select(c => c.Score / (temperature * 100.0)).ToArray();
        var maxLogit = logits.Max();
        var expLogits = logits.Select(l => Math.Exp(l - maxLogit)).ToArray();
        var sumExp = expLogits.Sum();
        var probs = expLogits.Select(e => e / sumExp).ToArray();

        // Sample
        var r = _random.NextDouble();
        double cumProb = 0;
        for (int i = 0; i < candidates.Count; i++)
        {
            cumProb += probs[i];
            if (r <= cumProb)
            {
                return (candidates[i].X, candidates[i].Y);
            }
        }

        // Fallback to last
        var last = candidates[^1];
        return (last.X, last.Y);
    }

    /// <summary>
    /// Record a completed game to staging store.
    /// </summary>
    private void RecordGameToStaging(PlayedGame game, string timeControl, int maxPly)
    {
        var sgfMoves = SelfPlayGameRecord.ToSgf(game.MoveList);

        var gameRecord = new SelfPlayGameRecord
        {
            SgfMoves = sgfMoves,
            Winner = game.Winner,
            TotalMoves = game.TotalMoves,
            TimeControl = timeControl,
            Temperature = 1.8,  // Default opening temperature
            Difficulty = AIDifficulty.Grandmaster,
            MoveList = game.MoveList
        };

        _stagingStore.RecordGame(gameRecord);

        // Position-level data is reconstructed during Phase 2 verification
        // by replaying SGF move sequences - no need to store twice
    }

    /// <summary>
    /// Internal record for a played game.
    /// </summary>
    private sealed record PlayedGame(
        Player Winner,
        int TotalMoves,
        List<(int X, int Y)> MoveList)
    {
        public long GameId { get; set; }
    }

    /// <summary>
    /// Internal worker summary.
    /// </summary>
    private sealed class WorkerSummary
    {
        public int WorkerId { get; set; }
        public int GamesPlayed { get; set; }
        public int RedWins { get; set; }
        public int BlueWins { get; set; }
        public int Draws { get; set; }
        public int TotalMoves { get; set; }
    }
}

/// <summary>
/// Candidate move with score.
/// </summary>
public sealed record MoveCandidate
{
    public required int X { get; init; }
    public required int Y { get; init; }
    public required int Score { get; set; }
}

/// <summary>
/// Summary of self-play generation session.
/// </summary>
public sealed class SelfPlaySummary
{
    public int RedWins { get; set; }
    public int BlueWins { get; set; }
    public int Draws { get; set; }
    public int TotalMoves { get; set; }
    public int StagingMovesRecorded { get; set; }
    public int MaxPlyRecorded { get; set; }
    public string? TimeControl { get; set; }

    public int TotalGames => RedWins + BlueWins + Draws;
    public double AverageMoves => TotalGames > 0 ? (double)TotalMoves / TotalGames : 0;

    [Obsolete("Use StagingMovesRecorded for separated pipeline")]
    public int MovesStoredToBook { get; set; }
}
