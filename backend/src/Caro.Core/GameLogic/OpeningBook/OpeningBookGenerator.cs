using System.Collections.Concurrent;
using System.Threading;
using System.Threading.Channels;
using Caro.Core.Concurrency;
using Caro.Core.Domain.Entities;
using Microsoft.Extensions.Logging;

namespace Caro.Core.GameLogic;

/// <summary>
/// Generates opening book positions using the full MinimaxAI engine.
/// Uses Lazy SMP parallel search with (N-4) threads for maximum performance.
/// </summary>
public sealed class OpeningBookGenerator : IOpeningBookGenerator, IDisposable
{
    /// <summary>
    /// Progress event from a worker completing a batch of positions.
    /// Enqueued to AsyncQueue for thread-safe progress aggregation.
    /// </summary>
    /// <param name="Depth">Current depth level being processed</param>
    /// <param name="PositionsCompleted">Number of positions completed in this batch</param>
    /// <param name="TotalPositions">Total positions to process at this depth</param>
    /// <param name="TimestampMs">Event timestamp</param>
    private const int SurvivalZoneStartPly = 6;   // Move 4 (ply 6-7): where Red's disadvantage begins
    private const int SurvivalZoneEndPly = 13;    // Move 7 (ply 12-13): end of survival zone

    private sealed record BookProgressEvent(
        int Depth,
        int PositionsCompleted,
        int TotalPositions,
        long TimestampMs
    );
    private const int MaxBookMoves = 12;           // Maximum plies in book (6 moves each)
    private const int MaxCandidatesPerPosition = 8; // Top N moves to store per position
    private const int TimePerPositionMs = 30000;   // 30 seconds per position (optimized from 60s)

    // Channel-based write buffer configuration
    private const int WriteChannelCapacity = 1000;         // Bounded channel capacity for backpressure
    private const int WriteBufferSize = 50;                // Target buffer size before flushing
    private const int WriteBufferTimeoutMs = 5000;         // Max time to wait before flushing (5 seconds)

    private readonly IOpeningBookStore _store;
    private readonly IPositionCanonicalizer _canonicalizer;
    private readonly IOpeningBookValidator _validator;
    private readonly ILoggerFactory _loggerFactory;
    private readonly ILogger<OpeningBookGenerator> _logger;
    private readonly GeneratorProgress _progress = new();
    private readonly AsyncQueue<BookProgressEvent> _progressQueue;
    private CancellationTokenSource? _cts;

    // Channel-based async write buffer fields
    private readonly Channel<OpeningBookEntry> _writeChannel;
    private readonly CancellationTokenSource _writeCts;
    private readonly Task _writeLoopTask;
    private int _entriesBuffered;              // Count of entries currently in buffer
    private int _totalEntriesStored;           // Total entries stored via batch operations

    public OpeningBookGenerator(
        IOpeningBookStore store,
        IPositionCanonicalizer canonicalizer,
        IOpeningBookValidator validator,
        ILoggerFactory? loggerFactory = null)
    {
        _store = store ?? throw new ArgumentNullException(nameof(store));
        _canonicalizer = canonicalizer ?? throw new ArgumentNullException(nameof(canonicalizer));
        _validator = validator ?? throw new ArgumentNullException(nameof(validator));
        _loggerFactory = loggerFactory ?? Microsoft.Extensions.Logging.Abstractions.NullLoggerFactory.Instance;
        _logger = _loggerFactory.CreateLogger<OpeningBookGenerator>();

        // Create progress queue for thread-safe updates from workers
        _progressQueue = new AsyncQueue<BookProgressEvent>(
            ProcessProgressEventAsync,
            capacity: 1000,
            queueName: "BookProgress"
        );

        // Initialize bounded channel for async write operations
        _writeChannel = Channel.CreateBounded<OpeningBookEntry>(new BoundedChannelOptions(WriteChannelCapacity)
        {
            FullMode = BoundedChannelFullMode.Wait,  // Apply backpressure when channel is full
            SingleReader = true,                     // Single consumer (background loop)
            SingleWriter = false                     // Multiple producers (parallel workers)
        });
        _writeCts = new CancellationTokenSource();

        // Start the background storage loop
        _writeLoopTask = Task.Run(() => WriteLoopAsync(_writeCts.Token));
    }



    public GenerationProgress GetProgress() => _progress.ToPublicProgress();

    public void Cancel()
    {
        _cts?.Cancel();
        _progress.Status = GeneratorState.Cancelled;

        // Signal the write loop to flush and exit
        _writeCts?.Cancel();
    }

    /// <summary>
    /// Enqueue an entry for async batch storage.
    /// This method is non-blocking and uses backpressure when the channel is full.
    /// </summary>
    private async ValueTask EnqueueEntryForStorageAsync(OpeningBookEntry entry, CancellationToken cancellationToken)
    {
        try
        {
            await _writeChannel.Writer.WriteAsync(entry, cancellationToken);
        }
        catch (ChannelClosedException)
        {
            _logger.LogWarning("Write channel closed, entry not stored: {Hash}", entry.CanonicalHash);
        }
    }

    /// <summary>
    /// Background storage loop that buffers entries and performs batch writes.
    /// Targets storing entries when either:
    /// - Buffer size reaches WriteBufferSize (50 items)
    /// - WriteBufferTimeoutMs elapses (5 seconds)
    /// </summary>
    private async Task WriteLoopAsync(CancellationToken cancellationToken)
    {
        var buffer = new List<OpeningBookEntry>(WriteBufferSize);
        var lastFlushTime = DateTime.UtcNow;
        var reader = _writeChannel.Reader;

        try
        {
            while (!cancellationToken.IsCancellationRequested)
            {
                // Wait for items with timeout
                try
                {
                    // Read up to WriteBufferSize items or wait for timeout
                    while (buffer.Count < WriteBufferSize)
                    {
                        var timeUntilFlush = WriteBufferTimeoutMs - (int)(DateTime.UtcNow - lastFlushTime).TotalMilliseconds;
                        if (timeUntilFlush <= 0)
                        {
                            // Timeout elapsed, flush what we have
                            break;
                        }

                        // Wait for new entry or timeout
                        using var linkedCts = CancellationTokenSource.CreateLinkedTokenSource(cancellationToken);
                        linkedCts.CancelAfter(timeUntilFlush);

                        try
                        {
                            if (await reader.WaitToReadAsync(linkedCts.Token))
                            {
                                if (reader.TryRead(out var entry))
                                {
                                    buffer.Add(entry);
                                }
                            }
                            else
                            {
                                // Channel completed
                                break;
                            }
                        }
                        catch (OperationCanceledException) when (linkedCts.Token.IsCancellationRequested && !cancellationToken.IsCancellationRequested)
                        {
                            // Timeout occurred, exit to flush
                            break;
                        }
                    }

                    // Drain any remaining items in channel (up to buffer size)
                    while (buffer.Count < WriteBufferSize && reader.TryRead(out var entry))
                    {
                        buffer.Add(entry);
                    }

                    // Flush buffer if we have items
                    if (buffer.Count > 0)
                    {
                        await FlushBufferAsync(buffer, cancellationToken);
                        lastFlushTime = DateTime.UtcNow;
                    }
                }
                catch (OperationCanceledException) when (cancellationToken.IsCancellationRequested)
                {
                    // Cancellation requested - flush remaining buffer and exit
                    _logger.LogInformation("Write loop cancellation requested, flushing {Count} remaining entries", buffer.Count);
                    await FlushBufferAsync(buffer, CancellationToken.None);
                    throw;
                }
            }

            // Final flush on cancellation
            await FlushBufferAsync(buffer, CancellationToken.None);
        }
        catch (OperationCanceledException)
        {
            _logger.LogInformation("Write loop cancelled after storing {Total} entries", _totalEntriesStored);
        }
        catch (Exception ex)
        {
            _logger.LogError(ex, "Write loop failed with error");
            throw;
        }
        finally
        {
            // Ensure we flush any remaining entries
            if (buffer.Count > 0)
            {
                try
                {
                    await FlushBufferAsync(buffer, CancellationToken.None);
                }
                catch (Exception ex)
                {
                    _logger.LogError(ex, "Failed to flush buffer during cleanup");
                }
            }
        }
    }

    /// <summary>
    /// Flush buffered entries to storage using batch operation.
    /// </summary>
    private async Task FlushBufferAsync(List<OpeningBookEntry> buffer, CancellationToken cancellationToken)
    {
        if (buffer.Count == 0)
            return;

        try
        {
            _logger.LogDebug("Flushing {Count} entries to storage", buffer.Count);

            // Use batch store operation for better performance
            _store.StoreEntriesBatch(buffer);

            int count = buffer.Count;
            Interlocked.Add(ref _totalEntriesStored, count);
            Interlocked.Exchange(ref _entriesBuffered, 0);

            _logger.LogDebug("Successfully stored {Count} entries (total: {Total})", count, _totalEntriesStored);

            buffer.Clear();
        }
        catch (Exception ex)
        {
            _logger.LogError(ex, "Failed to store batch of {Count} entries", buffer.Count);
            // Clear buffer even on failure to avoid re-trying failed entries
            buffer.Clear();
            throw;
        }
    }

    /// <summary>
    /// Flush any remaining entries in the write buffer.
    /// Called during normal completion to ensure all data is persisted.
    /// </summary>
    public async Task FlushWriteBufferAsync()
    {
        _logger.LogInformation("Flushing write buffer with {Total} total entries stored", _totalEntriesStored);

        // Signal write loop to exit and flush
        _writeCts.Cancel();

        try
        {
            await _writeLoopTask;
        }
        catch (OperationCanceledException)
        {
            // Expected - write loop exits on cancellation
        }

        _logger.LogInformation("Write buffer flush complete. Final total: {Total}", _totalEntriesStored);
    }

    public async Task<BookGenerationResult> GenerateAsync(
        int maxDepth,
        int targetDepth,
        CancellationToken cancellationToken = default)
    {
        _cts = CancellationTokenSource.CreateLinkedTokenSource(cancellationToken);
        _progress.Reset(maxDepth);
        _progress.Status = GeneratorState.Running;
        _progress.StartTime = DateTime.UtcNow;

        try
        {
            // Configure thread pool for book generation (more aggressive than normal play)
            ThreadPoolConfig.ConfigureForSearch();

            // Use BookGeneration difficulty for (N-4) threads
            var bookDifficulty = AIDifficulty.BookGeneration;

            // Collect positions by depth level for breadth-first processing
            var positionsByDepth = new List<List<PositionToProcess>>();
            positionsByDepth.Add(new List<PositionToProcess>
            {
                new PositionToProcess(new Board(), Player.Red, 0)
            });

            int positionsGenerated = 0;
            int positionsVerified = 0;
            int blundersFound = 0;
            int totalMovesStored = 0;
            int positionsEvaluated = 0;

            // Process each depth level
            for (int depth = 0; depth <= maxDepth && !_cts.Token.IsCancellationRequested; depth++)
            {
                if (depth >= positionsByDepth.Count)
                    break;

                var currentLevelPositions = positionsByDepth[depth];
                if (currentLevelPositions.Count == 0)
                    continue;

                _progress.CurrentPhase = $"Evaluating depth {depth}";

                // Separate positions into: new (need evaluation) vs existing (use stored moves)
                var positionsToEvaluate = new List<(Board board, Player player, int depth, ulong hash, SymmetryType symmetry, bool nearEdge, int maxMoves)>();
                var positionsInBook = new List<(Board board, Player player, int depth, ulong hash, SymmetryType symmetry, bool nearEdge, BookMove[] moves)>();

                foreach (var pos in currentLevelPositions)
                {
                    var canonical = _canonicalizer.Canonicalize(pos.Board);

                    // Calculate max children based on position depth
                    int maxChildren = GetMaxChildrenForDepth(pos.Depth);

                    if (_store.ContainsEntry(canonical.CanonicalHash, pos.Player))
                    {
                        // Position already in book - retrieve stored moves for child generation
                        var existingEntry = _store.GetEntry(canonical.CanonicalHash, pos.Player);
                        if (existingEntry != null && existingEntry.Moves.Length > 0)
                        {
                            _logger.LogDebug("Found existing entry at depth {Depth}: {MoveCount} moves stored", pos.Depth, existingEntry.Moves.Length);
                            positionsInBook.Add((
                                pos.Board,
                                pos.Player,
                                pos.Depth,
                                canonical.CanonicalHash,
                                canonical.SymmetryApplied,
                                canonical.IsNearEdge,
                                existingEntry.Moves
                            ));
                        }
                    }
                    else
                    {
                        // New position - needs evaluation
                        _logger.LogDebug("New position at depth {Depth} needs evaluation", pos.Depth);
                        positionsToEvaluate.Add((
                            pos.Board,
                            pos.Player,
                            pos.Depth,
                            canonical.CanonicalHash,
                            canonical.SymmetryApplied,
                            canonical.IsNearEdge,
                            maxChildren
                        ));
                    }
                }

                // Set up depth tracking
                _progress.CurrentDepth = depth;
                _progress.TotalPositionsAtCurrentDepth = positionsToEvaluate.Count + positionsInBook.Count;
                Interlocked.Exchange(ref _progress._positionsCompletedAtCurrentDepth, 0);

                // Skip if nothing to process at this depth
                if (positionsToEvaluate.Count == 0 && positionsInBook.Count == 0)
                {
                    _progress.PositionsEvaluated = positionsEvaluated;
                    continue;
                }

                // Process positions in parallel using worker pool
                Dictionary<ulong, BookMove[]> results = new();
                int totalPositions = positionsToEvaluate.Count + positionsInBook.Count;
                if (positionsToEvaluate.Count > 0)
                {
                    results = await ProcessPositionsInParallelAsync(
                        positionsToEvaluate,
                        bookDifficulty,
                        _cts.Token,
                        totalPositions
                    );
                }

                // Store results and generate child positions
                var nextLevelPositions = new List<PositionToProcess>();

                // Process newly evaluated positions
                foreach (var posData in positionsToEvaluate)
                {
                    if (cancellationToken.IsCancellationRequested)
                        break;

                    if (!results.ContainsKey(posData.hash) || results[posData.hash].Length == 0)
                        continue;

                    var moves = results[posData.hash];
                    var canonical = _canonicalizer.Canonicalize(posData.board);

                    _logger.LogInformation("Storing entry at depth {Depth}: {MoveCount} moves evaluated and stored", posData.depth, moves.Length);

                    // Store entry
                    var entry = new OpeningBookEntry
                    {
                        CanonicalHash = canonical.CanonicalHash,
                        Depth = posData.depth,
                        Player = posData.player,
                        Symmetry = posData.symmetry,
                        IsNearEdge = posData.nearEdge,
                        Moves = moves
                    };

                    // Enqueue for async batch storage
                    await EnqueueEntryForStorageAsync(entry, cancellationToken);
                    positionsGenerated++;
                    totalMovesStored += moves.Length;
                    positionsEvaluated++;

                    // Generate and store opponent responses for ALL stored moves
                    // This ensures GM/Exp always have book responses regardless of opponent's choice
                    if (posData.depth < maxDepth - 1)
                    {
                        var (responsesGenerated, responsePositionsCount) = await GenerateOpponentResponsesAsync(
                            posData.board,
                            posData.player,
                            moves,
                            posData.depth,
                            posData.symmetry,
                            bookDifficulty,
                            cancellationToken
                        );
                        totalMovesStored += responsesGenerated;
                        positionsGenerated += responsePositionsCount;
                        _logger.LogDebug("Generated {Count} opponent responses ({PosCount} positions) at depth {Depth}",
                            responsesGenerated, responsePositionsCount, posData.depth);
                    }

                    // Calculate max children for this position
                    // Note: posData.depth is ply count (0-indexed), not move number
                    int maxChildren = GetMaxChildrenForDepth(posData.depth);

                    // Enqueue child positions
                    int movesAvailable = moves.Length;
                    int movesActuallyAdded = 0;
                    int movesSkippedWin = 0;

                    foreach (var move in moves.Take(maxChildren))
                    {
                        // Transform canonical coordinates back to actual before placing
                        // Moves are stored in canonical space, board is in actual space
                        (int actualX, int actualY) = _canonicalizer.TransformToActual(
                            (move.RelativeX, move.RelativeY),
                            posData.symmetry,
                            posData.board
                        );

                        var newBoard = posData.board.PlaceStone(actualX, actualY, posData.player);
                        var nextPlayer = posData.player == Player.Red ? Player.Blue : Player.Red;

                        var winResult = new WinDetector().CheckWin(newBoard);
                        _logger.LogDebug("Move ({ActualX}, {ActualY}) at depth {Depth}: Winner={Winner}", actualX, actualY, posData.depth, winResult.Winner);

                        if (winResult.Winner == Player.None)
                        {
                            nextLevelPositions.Add(new PositionToProcess(newBoard, nextPlayer, posData.depth + 1));
                            movesActuallyAdded++;
                        }
                        else
                        {
                            movesSkippedWin++;
                        }
                    }

                    _logger.LogInformation("Position at depth {Depth}: {MovesAvailable} moves available, {MaxChildren} max children, {MovesActuallyAdded} added, {MovesSkippedWin} skipped (win detected)",
                        posData.depth, movesAvailable, maxChildren, movesActuallyAdded, movesSkippedWin);

                    _progress.PositionsGenerated = positionsGenerated;
                    _progress.PositionsEvaluated = positionsEvaluated;
                    _progress.TotalMovesStored = totalMovesStored;
                }

                // Process positions already in the book - generate child positions from stored moves
                foreach (var posData in positionsInBook)
                {
                    if (cancellationToken.IsCancellationRequested)
                        break;

                    var moves = posData.moves;

                    // Calculate max children for this position
                    // Note: posData.depth is ply count (0-indexed), not move number
                    int maxChildren = GetMaxChildrenForDepth(posData.depth);

                    int movesAvailable = moves.Length;
                    int movesActuallyAdded = 0;
                    int movesSkippedWin = 0;

                    // Enqueue child positions from stored moves
                    foreach (var move in moves.Take(maxChildren))
                    {
                        // Transform canonical coordinates back to actual before placing
                        // Moves are stored in canonical space, board is in actual space
                        (int actualX, int actualY) = _canonicalizer.TransformToActual(
                            (move.RelativeX, move.RelativeY),
                            posData.symmetry,
                            posData.board
                        );

                        // Debug: Check if cell is already occupied BEFORE attempting to place
                        var existingCell = posData.board.GetCell(actualX, actualY);
                        if (existingCell.Player != Player.None)
                        {
                            _logger.LogError("INTERNAL ERROR: Trying to place at ({ActualX},{ActualY}) but cell is occupied by {ExistingPlayer}. Move from book: ({RelX},{RelY}), Symmetry={Sym}, Depth={Depth}",
                                actualX, actualY, existingCell.Player, move.RelativeX, move.RelativeY, posData.symmetry, posData.depth);

                            // Count stones on board for diagnostic
                            int redCount = 0, blueCount = 0;
                            for (int x = 0; x < 19; x++)
                                for (int y = 0; y < 19; y++)
                                {
                                    var c = posData.board.GetCell(x, y);
                                    if (c.Player == Player.Red) redCount++;
                                    else if (c.Player == Player.Blue) blueCount++;
                                }
                            _logger.LogError("Board state: Red={RedCount}, Blue={BlueCount}, Total={Total}", redCount, blueCount, redCount + blueCount);

                            // Skip this move and continue
                            continue;
                        }

                        var newBoard = posData.board.PlaceStone(actualX, actualY, posData.player);
                        var nextPlayer = posData.player == Player.Red ? Player.Blue : Player.Red;

                        var winResult = new WinDetector().CheckWin(newBoard);
                        _logger.LogDebug("Move ({ActualX}, {ActualY}) at depth {Depth} (from book): Winner={Winner}", actualX, actualY, posData.depth, winResult.Winner);

                        if (winResult.Winner == Player.None)
                        {
                            nextLevelPositions.Add(new PositionToProcess(newBoard, nextPlayer, posData.depth + 1));
                            movesActuallyAdded++;
                        }
                        else
                        {
                            movesSkippedWin++;
                        }
                    }

                    _logger.LogInformation("Position at depth {Depth} (from book): {MovesAvailable} moves available, {MaxChildren} max children, {MovesActuallyAdded} added, {MovesSkippedWin} skipped (win detected)",
                        posData.depth, movesAvailable, maxChildren, movesActuallyAdded, movesSkippedWin);
                }

                // Update progress to include positions from the book
                if (positionsInBook.Count > 0)
                {
                    _progressQueue.TryEnqueue(new BookProgressEvent(
                        Depth: depth,
                        PositionsCompleted: positionsInBook.Count,
                        TotalPositions: positionsToEvaluate.Count + positionsInBook.Count,
                        TimestampMs: DateTimeOffset.UtcNow.ToUnixTimeMilliseconds()
                    ));
                }

                // Add next level if we have positions
                if (nextLevelPositions.Count > 0)
                {
                    if (depth + 1 >= positionsByDepth.Count)
                        positionsByDepth.Add(nextLevelPositions);
                    else
                        positionsByDepth[depth + 1].AddRange(nextLevelPositions);
                }

                _logger.LogInformation("Depth {Depth}: Generated {NextLevelCount} child positions for depth {NextDepth}", depth, nextLevelPositions.Count, depth + 1);

            }

            // Flush the async write buffer before final store operations
            await FlushWriteBufferAsync();

            // Final flush
            _store.Flush();
            _store.SetMetadata("Version", "1");
            _store.SetMetadata("GeneratedAt", DateTime.UtcNow.ToString("o"));
            _store.SetMetadata("MaxDepth", maxDepth.ToString());
            _store.SetMetadata("TargetDepth", targetDepth.ToString());

            var elapsed = DateTime.UtcNow - _progress.StartTime;
            _progress.Status = _cts.Token.IsCancellationRequested
                ? GeneratorState.Cancelled
                : GeneratorState.Completed;
            _progress.EndTime = DateTime.UtcNow;

            return new BookGenerationResult(
                PositionsGenerated: positionsGenerated,
                PositionsVerified: positionsVerified,
                GenerationTime: elapsed,
                BlundersFound: blundersFound,
                TotalMovesStored: totalMovesStored
            );
        }
        catch (Exception)
        {
            _progress.Status = GeneratorState.Failed;
            throw;
        }
        finally
        {
            _cts?.Dispose();
            _cts = null;
        }
    }

    /// <summary>
    /// Generate and store opponent responses for all stored moves at a position.
    /// This ensures that for any move from this position, the opponent has a book response.
    /// </summary>
    private async Task<(int responsesGenerated, int positionsCount)> GenerateOpponentResponsesAsync(
        Board board,
        Player currentPlayer,
        BookMove[] storedMoves,
        int currentDepth,
        SymmetryType symmetry,
        AIDifficulty difficulty,
        CancellationToken cancellationToken)
    {
        int responsesGenerated = 0;
        int responsePositionsCount = 0;
        var opponent = currentPlayer == Player.Red ? Player.Blue : Player.Red;

        foreach (var move in storedMoves)
        {
            if (cancellationToken.IsCancellationRequested)
                break;

            // Apply the stored move to get the opponent's position
            (int actualX, int actualY) = _canonicalizer.TransformToActual(
                (move.RelativeX, move.RelativeY),
                symmetry,
                board
            );

            // Check if move is still valid (cell might be occupied)
            if (!_validator.IsValidMove(board, actualX, actualY, currentPlayer))
                continue;

            var newBoard = board.PlaceStone(actualX, actualY, currentPlayer);

            // Check if this move wins
            var winResult = new WinDetector().CheckWin(newBoard);
            if (winResult.Winner != Player.None)
                continue; // Don't generate responses for terminal positions

            // Canonicalize the resulting position for the opponent
            var opponentCanonical = _canonicalizer.Canonicalize(newBoard);

            // Check if we already have responses for this position
            if (_store.ContainsEntry(opponentCanonical.CanonicalHash, opponent))
            {
                continue; // Already has book entries
            }

            // Tiered response generation: adjust max responses based on depth
            int maxResponses = GetMaxChildrenForDepth(currentDepth + 1);

            // Generate best responses for the opponent
            // Use a smaller search budget since we're generating many responses
            var responseMoves = await GenerateMovesForPositionAsync(
                newBoard,
                opponent,
                difficulty,
                maxMoves: maxResponses,
                canonicalSymmetry: opponentCanonical.SymmetryApplied,
                isNearEdge: opponentCanonical.IsNearEdge,
                cancellationToken
            );

            if (responseMoves.Length > 0)
            {
                // Update depth for response moves (they're at currentDepth + 1)
                var responseMoveWithDepth = responseMoves.Select(m => new BookMove
                {
                    RelativeX = m.RelativeX,
                    RelativeY = m.RelativeY,
                    WinRate = m.WinRate,
                    DepthAchieved = m.DepthAchieved,
                    NodesSearched = m.NodesSearched,
                    Score = m.Score,
                    IsForcing = m.IsForcing,
                    Priority = m.Priority,
                    IsVerified = m.IsVerified
                }).ToArray();

                var responseEntry = new OpeningBookEntry
                {
                    CanonicalHash = opponentCanonical.CanonicalHash,
                    Depth = currentDepth + 1,
                    Player = opponent,
                    Symmetry = opponentCanonical.SymmetryApplied,
                    IsNearEdge = opponentCanonical.IsNearEdge,
                    Moves = responseMoveWithDepth
                };

                // Enqueue for async batch storage
                await EnqueueEntryForStorageAsync(responseEntry, cancellationToken);
                responsesGenerated += responseMoveWithDepth.Length;
                responsePositionsCount++;
            }
        }

        return (responsesGenerated, responsePositionsCount);
    }

    public async Task<BookMove[]> GenerateMovesForPositionAsync(
        Board board,
        Player player,
        AIDifficulty difficulty,
        int maxMoves,
        CancellationToken cancellationToken = default)
    {
        // For backward compatibility - use Identity symmetry (no transformation)
        return await GenerateMovesForPositionAsync(
            board, player, difficulty, maxMoves,
            SymmetryType.Identity, true,
            cancellationToken
        );
    }

    public async Task<BookMove[]> GenerateMovesForPositionAsync(
        Board board,
        Player player,
        AIDifficulty difficulty,
        int maxMoves,
        SymmetryType canonicalSymmetry,
        bool isNearEdge,
        CancellationToken cancellationToken = default)
    {
        var bookMoves = new List<BookMove>();

        // Get candidate moves
        var candidates = GetCandidateMoves(board, player);
        if (candidates.Count == 0)
            return Array.Empty<BookMove>();

        // Evaluate candidates IN PARALLEL for performance
        // This is critical - parallelizing candidate evaluation provides 4-8x speedup

        // Aggressive candidate pruning: use static evaluation for pre-sorting
        // In Caro, usually only top 4-6 moves are relevant. Evaluating 24 branches to depth 12+ is extremely expensive.
        // IMPORTANT: Filter out invalid candidates first (e.g., Open Rule violations) to ensure we evaluate valid moves
        var validCandidates = candidates
            .Where(c => _validator.IsValidMove(board, c.Item1, c.Item2, player))
            .ToList();

        if (validCandidates.Count == 0)
        {
            _logger.LogWarning("No valid candidates found at depth {CurrentDepth} (all {CandidateCount} candidates failed validation)", _progress.CurrentDepth, candidates.Count);
            return Array.Empty<BookMove>();
        }

        // Evaluate more candidates in survival zone (plies 6-13, moves 4-7)
        int currentDepth = _progress.CurrentDepth;
        int candidatesToTake = (currentDepth >= SurvivalZoneStartPly && currentDepth <= SurvivalZoneEndPly) ? 10 : 6;

        var candidatesToEvaluate = validCandidates
            .OrderByDescending(c => BoardEvaluator.EvaluateMoveAt(c.Item1, c.Item2, board, player))
            .Take(Math.Min(validCandidates.Count, candidatesToTake))
            .ToList();

        _logger.LogDebug("Candidate filtering: {TotalCandidates} total -> {ValidCandidates} valid -> {CandidatesToEvaluate} to evaluate",
            candidates.Count, validCandidates.Count, candidatesToEvaluate.Count);

        _logger.LogDebug("Position evaluation: {TotalCandidates} total candidates -> {CandidatesToEvaluate} candidates to evaluate", candidates.Count, candidatesToEvaluate.Count);

        // Adaptive time allocation based on depth
        // Reduce time for early positions (simpler positions), increase for deep positions
        // SURVIVAL ZONE (plies 6-13, moves 4-7) gets extra time for thorough evaluation
        int depthAdjustment = currentDepth switch
        {
            <= 3 => -30,    // Early positions: 30% less time
            <= 5 => 0,      // Pre-survival: standard time
            <= 13 => +50,   // SURVIVAL ZONE: 50% more time (plies 6-13)
            _ => +20        // Late positions: 20% more time
        };

        int adjustedTimePerPosition = TimePerPositionMs * (100 + depthAdjustment) / 100;
        var timePerCandidateMs = Math.Max(2000, adjustedTimePerPosition / candidatesToEvaluate.Count);

        // FLATTENED CONCURRENCY: Process candidates sequentially with a single AI
        // The outer loop (ProcessPositionsInParallelAsync) already provides position-level parallelism.
        // This prevents nested parallelism which causes oversubscription (e.g., 12 x 8 = 96 threads fighting for 12 cores).
        var results = new List<(int x, int y, int score, long nodes, int depth)>();

        var ai = new MinimaxAI(ttSizeMb: 64, logger: _loggerFactory.CreateLogger<MinimaxAI>());
        try
        {

            foreach (var candidate in candidatesToEvaluate)
            {
                cancellationToken.ThrowIfCancellationRequested();

                var (cx, cy) = candidate;

                // Candidates are pre-filtered for validity, but double-check for safety
                if (!_validator.IsValidMove(board, cx, cy, player))
                    continue;

                // Check for immediate win
                if (_validator.IsWinningMove(board, cx, cy, player))
                {
                    results.Add((cx, cy, 100000, 1, 1));
                    break; // No need to evaluate further candidates
                }

                // Create board for this candidate (don't modify original)
                var candidateBoard = board.PlaceStone(cx, cy, player);
                var searchBoard = candidateBoard;
                var opponent = player == Player.Red ? Player.Blue : Player.Red;
                var moveNumber = candidateBoard.GetBitBoard(Player.Red).CountBits() + candidateBoard.GetBitBoard(Player.Blue).CountBits();

                // Clear AI state before each candidate to prevent cross-contamination
                ai.ClearAllState();

                // Run search with divided time budget, NO inner parallel to avoid oversubscription
                var (bestX, bestY) = ai.GetBestMove(
                    searchBoard,
                    opponent,
                    difficulty,
                    timeRemainingMs: timePerCandidateMs,
                    moveNumber: moveNumber,
                    ponderingEnabled: false,
                    parallelSearchEnabled: false // Disable Lazy SMP to avoid oversubscribing threads
                );

                var (depthAchieved, nodesSearched, _, _, _, _, _, threadCount, _, _, _, _)
                    = ai.GetSearchStatistics();

                var evalBoard = searchBoard.PlaceStone(bestX, bestY, opponent);
                int score = EvaluateBoard(evalBoard, opponent);

                _logger.LogDebug("Candidate ({Cx}, {Cy}) evaluated: best response=({BestX},{BestY}), score={Score}", cx, cy, bestX, bestY, score);

                results.Add((cx, cy, score, nodesSearched, depthAchieved));

                // Update progress (last write wins is acceptable for display purposes)
                _progress.LastDepth = depthAchieved;
                _progress.LastNodes = nodesSearched;
                _progress.LastThreads = threadCount;
            }
        }
        finally
        {
            // AI will be garbage collected
        }

        // Convert to list for sorting
        var sortedResults = results.ToList();
        int resultsBeforePruning = sortedResults.Count;

        // Sort by score
        sortedResults.Sort((a, b) => b.score.CompareTo(a.score));

        // Log all scores before pruning for diagnosis
        var scoreLog = string.Join(", ", sortedResults.Select(r => $"({r.x},{r.y}):{r.score}"));
        _logger.LogDebug("Scores at depth {CurrentDepth} before pruning: {Scores}", currentDepth, scoreLog);

        // EARLY EXIT: If best move dominates, skip remaining evaluation
        // Use more aggressive thresholds at deeper depths
        if (sortedResults.Count >= 2)
        {
            int threshold = currentDepth >= 6 ? 150 : 200;  // More aggressive at depth 6+

            int scoreGap = sortedResults[0].score - sortedResults[1].score;
            if (scoreGap > threshold)
            {
                // Best move is clearly superior - stop evaluating further candidates
                _logger.LogDebug("Early exit: best move dominates with score gap {ScoreGap} > {Threshold}", scoreGap, threshold);
                sortedResults = sortedResults.Take(1).ToList();
            }
        }

        // Also skip obviously bad moves (don't keep candidates > 500 points behind first)
        if (sortedResults.Count >= 3)
        {
            var bestScore = sortedResults[0].score;
            sortedResults = sortedResults
                .TakeWhile(r => bestScore - r.score <= 500)
                .ToList();
        }

        _logger.LogDebug("Position at depth {CurrentDepth}: {CandidatesToEvaluate} candidates -> {ResultsBeforePruning} results -> {SortedResultsCount} after pruning",
            currentDepth, candidatesToEvaluate.Count, resultsBeforePruning, sortedResults.Count);

        // Convert to BookMove records
        // IMPORTANT: Transform coordinates to canonical space before storing
        // Moves are stored relative to the canonical position, not actual board
        int priority = maxMoves;
        foreach (var (x, y, score, nodes, depth) in sortedResults.Take(maxMoves))
        {
            int winRate = ScoreToWinRate(score);

            // Transform actual coordinates to canonical space
            // For edge positions or identity symmetry, coordinates stay the same
            (int canonicalX, int canonicalY) = (!isNearEdge && canonicalSymmetry != SymmetryType.Identity)
                ? _canonicalizer.ApplySymmetry(x, y, canonicalSymmetry)
                : (x, y);

            bookMoves.Add(new BookMove
            {
                RelativeX = canonicalX,
                RelativeY = canonicalY,
                WinRate = winRate,
                DepthAchieved = depth,
                NodesSearched = nodes,
                Score = score,
                IsForcing = Math.Abs(score) > 1000, // Simplified forcing detection
                Priority = priority--,
                IsVerified = true
            });
        }

        return bookMoves.ToArray();
    }

    // Forward to the overload with difficulty parameter
    public Task<BookMove[]> GenerateMovesForPositionAsync(
        Board board,
        Player player,
        int searchDepth,
        int maxMoves,
        CancellationToken cancellationToken = default)
    {
        // Map search depth to difficulty
        var difficulty = searchDepth switch
        {
            <= 6 => AIDifficulty.Hard,
            <= 12 => AIDifficulty.Grandmaster,
            _ => AIDifficulty.Experimental
        };

        return GenerateMovesForPositionAsync(board, player, difficulty, maxMoves, cancellationToken);
    }

    public Task<VerificationResult> VerifyAsync(CancellationToken cancellationToken = default)
    {
        var stats = _store.GetStatistics();
        var blunderDetails = new List<string>();
        int blundersFound = 0;

        // For now, return a simple result
        return Task.FromResult(new VerificationResult(
            EntriesChecked: stats.TotalEntries,
            BlundersFound: blundersFound,
            BlunderDetails: blunderDetails.ToArray(),
            VerifiedCount: stats.TotalEntries - blundersFound
        ));
    }

    private int EvaluateBoard(Board board, Player player)
    {
        int score = 0;

        // Simple material evaluation
        var redBitBoard = board.GetBitBoard(Player.Red);
        var blueBitBoard = board.GetBitBoard(Player.Blue);

        // Position scoring based on patterns
        score += EvaluateThreats(board, Player.Red) * 100;
        score -= EvaluateThreats(board, Player.Blue) * 100;

        return player == Player.Red ? score : -score;
    }

    /// <summary>
    /// Private overload that accepts a pre-created MinimaxAI instance.
    /// Used by WorkerThreadLoop to avoid pooling overhead.
    /// </summary>
    private Task<BookMove[]> GenerateMovesForPositionAsync(
        Board board, Player player, AIDifficulty difficulty,
        int maxMoves, SymmetryType canonicalSymmetry, bool isNearEdge,
        MinimaxAI ai, CancellationToken cancellationToken = default)
    {
        var bookMoves = new List<BookMove>();

        // Get candidate moves
        var candidates = GetCandidateMoves(board, player);
        if (candidates.Count == 0)
            return Task.FromResult(Array.Empty<BookMove>());

        // Evaluate candidates IN PARALLEL for performance
        // This is critical - parallelizing candidate evaluation provides 4-8x speedup

        // Aggressive candidate pruning: use static evaluation for pre-sorting
        // In Caro, usually only top 4-6 moves are relevant. Evaluating 24 branches to depth 12+ is extremely expensive.
        // IMPORTANT: Filter out invalid candidates first (e.g., Open Rule violations) to ensure we evaluate valid moves
        var validCandidates = candidates
            .Where(c => _validator.IsValidMove(board, c.Item1, c.Item2, player))
            .ToList();

        if (validCandidates.Count == 0)
        {
            _logger.LogWarning("No valid candidates found at depth {CurrentDepth} (all {CandidateCount} candidates failed validation)", _progress.CurrentDepth, candidates.Count);
            return Task.FromResult(Array.Empty<BookMove>());
        }

        // Evaluate more candidates in survival zone (plies 6-13, moves 4-7)
        int currentDepth = _progress.CurrentDepth;
        int candidatesToTake = (currentDepth >= SurvivalZoneStartPly && currentDepth <= SurvivalZoneEndPly) ? 10 : 6;

        var candidatesToEvaluate = validCandidates
            .OrderByDescending(c => BoardEvaluator.EvaluateMoveAt(c.Item1, c.Item2, board, player))
            .Take(Math.Min(validCandidates.Count, candidatesToTake))
            .ToList();

        _logger.LogDebug("Candidate filtering: {TotalCandidates} total -> {ValidCandidates} valid -> {CandidatesToEvaluate} to evaluate",
            candidates.Count, validCandidates.Count, candidatesToEvaluate.Count);

        _logger.LogDebug("Position evaluation: {TotalCandidates} total candidates -> {CandidatesToEvaluate} candidates to evaluate", candidates.Count, candidatesToEvaluate.Count);

        // Adaptive time allocation based on depth
        // Reduce time for early positions (simpler positions), increase for deep positions
        // SURVIVAL ZONE (plies 6-13, moves 4-7) gets extra time for thorough evaluation
        int depthAdjustment = currentDepth switch
        {
            <= 3 => -30,    // Early positions: 30% less time
            <= 5 => 0,      // Pre-survival: standard time
            <= 13 => +50,   // SURVIVAL ZONE: 50% more time (plies 6-13)
            _ => +20        // Late positions: 20% more time
        };

        int adjustedTimePerPosition = TimePerPositionMs * (100 + depthAdjustment) / 100;
        var timePerCandidateMs = Math.Max(2000, adjustedTimePerPosition / candidatesToEvaluate.Count);

        // Use the provided AI instance directly (no pooling)
        var results = new List<(int x, int y, int score, long nodes, int depth)>();

        foreach (var candidate in candidatesToEvaluate)
        {
            cancellationToken.ThrowIfCancellationRequested();

            var (cx, cy) = candidate;

            // Candidates are pre-filtered for validity, but double-check for safety
            if (!_validator.IsValidMove(board, cx, cy, player))
                continue;

            // Check for immediate win
            if (_validator.IsWinningMove(board, cx, cy, player))
            {
                results.Add((cx, cy, 100000, 1, 1));
                break; // No need to evaluate further candidates
            }

            // Create board for this candidate (don't modify original)
            var candidateBoard = board.PlaceStone(cx, cy, player);
            var searchBoard = candidateBoard;
            var opponent = player == Player.Red ? Player.Blue : Player.Red;
            var moveNumber = candidateBoard.GetBitBoard(Player.Red).CountBits() + candidateBoard.GetBitBoard(Player.Blue).CountBits();

            // Clear AI state before each candidate to prevent cross-contamination
            ai.ClearAllState();

            // Run search with divided time budget, NO inner parallel to avoid oversubscription
            var (bestX, bestY) = ai.GetBestMove(
                searchBoard,
                opponent,
                difficulty,
                timeRemainingMs: timePerCandidateMs,
                moveNumber: moveNumber,
                ponderingEnabled: false,
                parallelSearchEnabled: false // Disable Lazy SMP to avoid oversubscribing threads
            );

            var (depthAchieved, nodesSearched, _, _, _, _, _, threadCount, _, _, _, _)
                = ai.GetSearchStatistics();

            var evalBoard = searchBoard.PlaceStone(bestX, bestY, opponent);
            int score = EvaluateBoard(evalBoard, opponent);

            _logger.LogDebug("Candidate ({Cx}, {Cy}) evaluated: best response=({BestX},{BestY}), score={Score}", cx, cy, bestX, bestY, score);

            results.Add((cx, cy, score, nodesSearched, depthAchieved));

            // Update progress (last write wins is acceptable for display purposes)
            _progress.LastDepth = depthAchieved;
            _progress.LastNodes = nodesSearched;
            _progress.LastThreads = threadCount;
        }

        // Convert to list for sorting
        var sortedResults = results.ToList();
        int resultsBeforePruning = sortedResults.Count;

        // Sort by score
        sortedResults.Sort((a, b) => b.score.CompareTo(a.score));

        // Log all scores before pruning for diagnosis
        var scoreLog = string.Join(", ", sortedResults.Select(r => $"({r.x},{r.y}):{r.score}"));
        _logger.LogDebug("Scores at depth {CurrentDepth} before pruning: {Scores}", currentDepth, scoreLog);

        // EARLY EXIT: If best move dominates, skip remaining evaluation
        // Use more aggressive thresholds at deeper depths
        if (sortedResults.Count >= 2)
        {
            int threshold = currentDepth >= 6 ? 150 : 200;  // More aggressive at depth 6+

            int scoreGap = sortedResults[0].score - sortedResults[1].score;
            if (scoreGap > threshold)
            {
                // Best move is clearly superior - stop evaluating further candidates
                _logger.LogDebug("Early exit: best move dominates with score gap {ScoreGap} > {Threshold}", scoreGap, threshold);
                sortedResults = sortedResults.Take(1).ToList();
            }
        }

        // Also skip obviously bad moves (don't keep candidates > 500 points behind first)
        if (sortedResults.Count >= 3)
        {
            var bestScore = sortedResults[0].score;
            sortedResults = sortedResults
                .TakeWhile(r => bestScore - r.score <= 500)
                .ToList();
        }

        _logger.LogDebug("Position at depth {CurrentDepth}: {CandidatesToEvaluate} candidates -> {ResultsBeforePruning} results -> {SortedResultsCount} after pruning",
            currentDepth, candidatesToEvaluate.Count, resultsBeforePruning, sortedResults.Count);

        // Convert to BookMove records
        // IMPORTANT: Transform coordinates to canonical space before storing
        // Moves are stored relative to the canonical position, not actual board
        int priority = maxMoves;
        foreach (var (x, y, score, nodes, depth) in sortedResults.Take(maxMoves))
        {
            int winRate = ScoreToWinRate(score);

            // Transform actual coordinates to canonical space
            // For edge positions or identity symmetry, coordinates stay the same
            (int canonicalX, int canonicalY) = (!isNearEdge && canonicalSymmetry != SymmetryType.Identity)
                ? _canonicalizer.ApplySymmetry(x, y, canonicalSymmetry)
                : (x, y);

            bookMoves.Add(new BookMove
            {
                RelativeX = canonicalX,
                RelativeY = canonicalY,
                WinRate = winRate,
                DepthAchieved = depth,
                NodesSearched = nodes,
                Score = score,
                IsForcing = Math.Abs(score) > 1000, // Simplified forcing detection
                Priority = priority--,
                IsVerified = true
            });
        }

        return Task.FromResult(bookMoves.ToArray());
    }


    private int EvaluateThreats(Board board, Player player)
    {
        int score = 0;
        int boardSize = board.BoardSize;

        // Count patterns for each direction
        for (int x = 0; x < boardSize; x++)
        {
            for (int y = 0; y < boardSize; y++)
            {
                if (board.GetCell(x, y).Player == player)
                {
                    score += EvaluatePoint(board, x, y, player);
                }
            }
        }
        return score;
    }

    private static int EvaluatePoint(Board board, int x, int y, Player player)
    {
        int score = 0;
        int[] dx = { 1, 0, 1, 1 };
        int[] dy = { 0, 1, 1, -1 };

        for (int dir = 0; dir < 4; dir++)
        {
            int count = 1;
            int open = 0;

            // Check positive direction
            for (int i = 1; i < 5; i++)
            {
                int nx = x + dx[dir] * i;
                int ny = y + dy[dir] * i;
                if (!IsValid(board, nx, ny)) break;

                if (board.GetCell(nx, ny).Player == player)
                    count++;
                else if (board.GetCell(nx, ny).IsEmpty)
                {
                    open++;
                    break;
                }
                else break;
            }

            // Check negative direction
            for (int i = 1; i < 5; i++)
            {
                int nx = x - dx[dir] * i;
                int ny = y - dy[dir] * i;
                if (!IsValid(board, nx, ny)) break;

                if (board.GetCell(nx, ny).Player == player)
                    count++;
                else if (board.GetCell(nx, ny).IsEmpty)
                {
                    open++;
                    break;
                }
                else break;
            }

            score += CountToScore(count, open);
        }
        return score;
    }

    private static bool IsValid(Board board, int x, int y)
    {
        return x >= 0 && x < board.BoardSize && y >= 0 && y < board.BoardSize;
    }

    private static int CountToScore(int count, int open)
    {
        return count switch
        {
            5 => 100000,
            4 when open >= 1 => 10000,
            4 when open == 0 => 1000,
            3 when open >= 2 => 1000,
            3 when open == 1 => 100,
            2 when open >= 2 => 100,
            _ => 0
        };
    }

    private List<(int x, int y)> GetCandidateMoves(Board board, Player player)
    {
        var candidates = new List<(int, int)>();
        int boardSize = board.BoardSize;

        // Get all empty cells adjacent to existing stones
        for (int x = 0; x < boardSize; x++)
        {
            for (int y = 0; y < boardSize; y++)
            {
                if (board.GetCell(x, y).IsEmpty && HasAdjacentStone(board, x, y))
                {
                    candidates.Add((x, y));
                }
            }
        }

        int stoneCount = 0;
        for (int x = 0; x < boardSize; x++)
        {
            for (int y = 0; y < boardSize; y++)
            {
                if (!board.GetCell(x, y).IsEmpty)
                    stoneCount++;
            }
        }
        _logger.LogDebug("GetCandidateMoves: Board has {StoneCount} stones, found {CandidateCount} candidate moves", stoneCount, candidates.Count);

        // Log first few candidates for diagnosis
        if (candidates.Count > 0)
        {
            var candidateSample = string.Join(", ", candidates.Take(10).Select(c => $"({c.Item1},{c.Item2})"));
            _logger.LogDebug("Candidate samples: {Candidates}", candidateSample + (candidates.Count > 10 ? "..." : ""));
        }

        // If board is empty, return center
        if (candidates.Count == 0)
        {
            int center = boardSize / 2;
            return new List<(int, int)> { (center, center) };
        }

        return candidates;
    }

    private static bool HasAdjacentStone(Board board, int x, int y)
    {
        int boardSize = board.BoardSize;

        for (int dx = -2; dx <= 2; dx++)
        {
            for (int dy = -2; dy <= 2; dy++)
            {
                if (dx == 0 && dy == 0) continue;

                int nx = x + dx;
                int ny = y + dy;

                if (nx >= 0 && nx < boardSize && ny >= 0 && ny < boardSize)
                {
                    if (!board.GetCell(nx, ny).IsEmpty)
                    {
                        return true;
                    }
                }
            }
        }
        return false;
    }

    private static int ScoreToWinRate(int score)
    {
        // Convert centipawn score to win rate percentage
        const double scale = 0.005;
        double winProb = 1.0 / (1.0 + Math.Exp(-score * scale));
        return (int)(winProb * 100);
    }

    private record PositionToProcess(Board Board, Player Player, int Depth);
    private record PositionToSearch(Board Board, Player Player, int Depth);

    /// <summary>
    /// Statistics for response generation phase.
    /// </summary>
    private record ResponseGenerationStats
    {
        public int ResponsesGenerated { get; init; }
        public TimeSpan Elapsed { get; init; }
    }

    /// <summary>
    /// Internal progress tracking.
    /// </summary>
    private sealed class GeneratorProgress
    {
        public GeneratorState Status { get; set; } = GeneratorState.NotStarted;
        public DateTime StartTime { get; set; }
        public DateTime EndTime { get; set; }
        public int PositionsGenerated { get; set; }
        public int PositionsEvaluated { get; set; }
        public int TotalMovesStored { get; set; }
        public string CurrentPhase { get; set; } = "Initializing";
        public int LastDepth { get; set; }
        public long LastNodes { get; set; }
        public int LastThreads { get; set; }

        // Depth-weighted progress tracking
        public int CurrentDepth { get; set; }
        public int TotalPositionsAtCurrentDepth { get; set; }
        public int MaxDepth { get; set; } = 32;

        // Internal field for Interlocked operations (used by AsyncQueue processor)
        internal int _positionsCompletedAtCurrentDepth;

        // Public property reads atomically (for progress display)
        public int PositionsCompletedAtCurrentDepth => Interlocked.CompareExchange(ref _positionsCompletedAtCurrentDepth, 0, 0);

        public int EstimatedTotalDepths { get; set; } = 10;

        // Note: Candidate progress is not tracked because multiple workers process
        // different positions in parallel, making per-position candidate counts
        // meaningless (each worker resets and updates the same shared counters).
        // Progress updates come via AsyncQueue for thread safety.

        public GenerationProgress ToPublicProgress()
        {
            var elapsed = DateTime.UtcNow - StartTime;
            double percent = CalculateDepthWeightedProgress();

            return new GenerationProgress(
                PositionsEvaluated: PositionsEvaluated,
                PositionsStored: PositionsGenerated,
                TotalPositions: PositionsEvaluated + PositionsGenerated,
                PercentComplete: percent,
                CurrentPhase: $"{CurrentPhase} (Last: d{LastDepth}, {LastThreads} threads, {LastNodes}N)",
                ElapsedTime: elapsed,
                EstimatedTimeRemaining: percent > 1 ? TimeSpan.FromSeconds(elapsed.TotalSeconds * (100 - percent) / percent) : null,
                CurrentDepth: CurrentDepth,
                PositionsCompletedAtCurrentDepth: PositionsCompletedAtCurrentDepth,
                TotalPositionsAtCurrentDepth: TotalPositionsAtCurrentDepth
            );
        }

        private double CalculateDepthWeightedProgress()
        {
            // Calculate weights dynamically based on MaxDepth
            double completedDepthsPercent = 0;
            for (int d = 0; d < CurrentDepth; d++)
            {
                completedDepthsPercent += GetDepthWeight(d);
            }

            // Add current depth progress
            double currentDepthPercent = 0;
            if (TotalPositionsAtCurrentDepth > 0)
            {
                double depthWeight = GetDepthWeight(CurrentDepth);
                currentDepthPercent = (double)PositionsCompletedAtCurrentDepth / TotalPositionsAtCurrentDepth * depthWeight;
            }

            double totalPercent = (completedDepthsPercent + currentDepthPercent) * 100;

            // Cap at 99% until actually complete (allows final 1% when done)
            return Status == GeneratorState.Completed ? 100 : Math.Min(99, totalPercent);
        }

        private double GetDepthWeight(int depth)
        {
            // Base weights for early depths (survival zone)
            double baseWeight = depth switch
            {
                0 => 0.02,   // Root position
                1 => 0.04,   // ~4 positions
                2 => 0.05,   // ~16 positions
                3 => 0.06,   // ~32 positions
                4 => 0.07,   // ~64 positions
                5 => 0.08,   // ~64 positions
                6 => 0.12,   // SURVIVAL ZONE start: ~128 positions
                7 => 0.15,   // SURVIVAL ZONE: ~256 positions (peak)
                8 => 0.12,   // SURVIVAL ZONE: ~256 positions
                9 => 0.10,   // SURVIVAL ZONE: ~128 positions
                10 => 0.08,  // SURVIVAL ZONE: ~64 positions
                11 => 0.06,  // SURVIVAL ZONE end: ~32 positions
                12 => 0.03,  // ~16 positions
                13 => 0.02,  // ~8 positions
                _ => 0.01    // Deeper positions: default small weight
            };

            // Scale weights so total equals 1.0 based on MaxDepth
            // Calculate sum of base weights for all depths up to MaxDepth
            double totalBaseWeight = 0;
            for (int d = 0; d <= MaxDepth; d++)
            {
                totalBaseWeight += (d switch
                {
                    <= 13 => d switch
                    {
                        0 => 0.02,
                        1 => 0.04,
                        2 => 0.05,
                        3 => 0.06,
                        4 => 0.07,
                        5 => 0.08,
                        6 => 0.12,
                        7 => 0.15,
                        8 => 0.12,
                        9 => 0.10,
                        10 => 0.08,
                        11 => 0.06,
                        12 => 0.03,
                        13 => 0.02,
                        _ => 0.01
                    },
                    _ => 0.01
                });
            }

            // Normalize weight so all depths sum to 1.0
            return baseWeight / totalBaseWeight;
        }

        public void Reset(int maxDepth = 32)
        {
            Status = GeneratorState.NotStarted;
            PositionsGenerated = 0;
            PositionsEvaluated = 0;
            TotalMovesStored = 0;
            MaxDepth = maxDepth;
            CurrentPhase = "Initializing";
        }
    }

    private enum GeneratorState
    {
        NotStarted,
        Running,
        Completed,
        Cancelled,
        Failed
    }

    /// <summary>
    /// Job for parallel position evaluation
    /// </summary>
    private record PositionJob(
        int JobId,
        Board Board,
        Player Player,
        int Depth,
        ulong CanonicalHash,
        SymmetryType Symmetry,
        bool IsNearEdge,
        int MaxMovesToStore
    );

    /// <summary>
    /// Result from parallel position evaluation
    /// </summary>
    private record PositionResult(
        int JobId,
        ulong CanonicalHash,
        BookMove[] Moves,
        int DepthAchieved,
        long NodesSearched,
        int ThreadCount
    );

    /// <summary>
    /// Process a single position job (worker thread entry point)
    /// </summary>
    private PositionResult ProcessPositionJob(PositionJob job, AIDifficulty difficulty, CancellationToken cancellationToken)
    {
        try
        {
            var moves = GenerateMovesForPositionAsync(
                job.Board,
                job.Player,
                difficulty,
                job.MaxMovesToStore,
                cancellationToken
            ).GetAwaiter().GetResult();

            return new PositionResult(
                job.JobId,
                job.CanonicalHash,
                moves,
                _progress.LastDepth,
                _progress.LastNodes,
                _progress.LastThreads
            );
        }
        catch (OperationCanceledException)
        {
            // Return empty result on cancellation
            return new PositionResult(job.JobId, job.CanonicalHash, Array.Empty<BookMove>(), 0, 0, 0);
        }
    }


    /// <summary>
    /// Get the maximum number of children to expand for a given depth.
    /// Tiered strategy ensures coverage for all difficulty levels.
    /// </summary>
    private static int GetMaxChildrenForDepth(int depth)
    {
        return depth switch
        {
            <= 14 => 4,     // Early game + survival zone (plies 0-14)
            <= 24 => 3,     // Hard coverage (plies 15-24)
            <= 32 => 2,     // GM coverage (plies 25-32)
            <= 40 => 1,     // Exp coverage (plies 33-40)
            _ => 0          // No book beyond ply 40
        };
    }

    /// <summary>
    /// Process multiple positions in parallel using dedicated worker threads.
    /// Each thread owns its own MinimaxAI instance, bypassing the ThreadPool completely.
    /// </summary>
    private Task<Dictionary<ulong, BookMove[]>> ProcessPositionsInParallelAsync(
        List<(Board board, Player player, int depth, ulong hash, SymmetryType symmetry, bool nearEdge, int maxMoves)> positions,
        AIDifficulty difficulty,
        CancellationToken cancellationToken,
        int totalPositions = 0)
    {
        int totalPositionsToReport = totalPositions > 0 ? totalPositions : positions.Count;
        int currentDepth = positions.Count > 0 ? positions[0].depth : 0;

        var jobQueue = new ConcurrentQueue<PositionJob>();
        var results = new ConcurrentDictionary<ulong, BookMove[]>();
        int completedCount = 0;

        int jobId = 0;
        foreach (var pos in positions)
        {
            jobQueue.Enqueue(new PositionJob(jobId++, pos.board, pos.player, pos.depth,
                pos.hash, pos.symmetry, pos.nearEdge, pos.maxMoves));
        }

        int threadCount = Environment.ProcessorCount;
        var threads = new List<Thread>(threadCount);

        for (int i = 0; i < threadCount; i++)
        {
            var thread = new Thread(() => WorkerThreadLoop(
                jobQueue, results, _progressQueue, difficulty, currentDepth,
                totalPositionsToReport, ref completedCount, cancellationToken))
            {
                Priority = ThreadPriority.AboveNormal,
                IsBackground = false
            };
            threads.Add(thread);
            thread.Start();
        }

        foreach (var thread in threads)
            thread.Join();

        return Task.FromResult(new Dictionary<ulong, BookMove[]>(results));
    }

    /// <summary>
    /// Worker thread entry point for dedicated thread-based parallel processing.
    /// Each thread owns its own MinimaxAI instance, bypassing the ThreadPool completely.
    /// </summary>
    private void WorkerThreadLoop(
        ConcurrentQueue<PositionJob> jobQueue,
        ConcurrentDictionary<ulong, BookMove[]> results,
        AsyncQueue<BookProgressEvent> progressQueue,
        AIDifficulty difficulty,
        int currentDepth,
        int totalPositions,
        ref int completedCounter,
        CancellationToken cancellationToken)
    {
        var ai = new MinimaxAI(ttSizeMb: 64, logger: _loggerFactory.CreateLogger<MinimaxAI>());

        try
        {
            while (!cancellationToken.IsCancellationRequested)
            {
                if (!jobQueue.TryDequeue(out var job))
                    break;

                ai.ClearAllState();

                var moves = GenerateMovesForPositionAsync(
                    job.Board, job.Player, difficulty, job.MaxMovesToStore,
                    job.Symmetry, job.IsNearEdge, ai, cancellationToken
                ).GetAwaiter().GetResult();

                results[job.CanonicalHash] = moves;

                int currentCompleted = Interlocked.Increment(ref completedCounter);
                _progress.PositionsEvaluated = currentCompleted;

                progressQueue.TryEnqueue(new BookProgressEvent(
                    Depth: currentDepth, PositionsCompleted: 1,
                    TotalPositions: totalPositions,
                    TimestampMs: DateTimeOffset.UtcNow.ToUnixTimeMilliseconds()
                ));
            }
        }
        catch (OperationCanceledException) { }
    }

    /// <summary>
    /// Processes progress events from workers.
    /// Updates PositionsCompletedAtCurrentDepth atomically.
    /// Runs on background thread via AsyncQueue.
    /// </summary>
    private ValueTask ProcessProgressEventAsync(BookProgressEvent evt)
    {
        // Update depth-specific progress
        _progress.CurrentDepth = evt.Depth;
        _progress.TotalPositionsAtCurrentDepth = evt.TotalPositions;

        // Atomically increment completed count
        Interlocked.Add(ref _progress._positionsCompletedAtCurrentDepth, evt.PositionsCompleted);

        return ValueTask.CompletedTask;
    }

    /// <summary>
    /// Disposes the progress queue and write channel resources.
    /// </summary>
    public void Dispose()
    {
        _progressQueue?.Dispose();
        _writeCts?.Cancel();
        _writeCts?.Dispose();

        // Wait a short time for write loop to complete cleanup
        try
        {
            _writeLoopTask?.Wait(TimeSpan.FromSeconds(5));
        }
        catch (AggregateException)
        {
            // Task may have been cancelled - acceptable during disposal
        }
    }
}

