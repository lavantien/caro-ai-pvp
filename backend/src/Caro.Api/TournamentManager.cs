using Caro.Api.Logging;
using Caro.Core.Concurrency;
using Caro.Core.Entities;
using Caro.Core.GameLogic;
using Caro.Core.Tournament;
using TournamentELO = Caro.Core.Tournament.ELOCalculator;
using Microsoft.Extensions.Hosting;
using Microsoft.AspNetCore.SignalR;

namespace Caro.Api;

// Keep ITournamentClient and related types in Caro.Core.Tournament namespace
// This file is in Caro.Api but references types from Caro.Core.Tournament

/// <summary>
/// Manages long-running AI tournaments with pause/resume capability
/// Runs as a background service and broadcasts updates via SignalR
/// </summary>
public sealed class TournamentManager : BackgroundService
{
    private readonly IHubContext<TournamentHub, ITournamentClient> _hub;
    private readonly ILogger<TournamentManager> _logger;
    private readonly GameLogService _logService;

    // ReaderWriterLockSlim allows multiple concurrent readers but exclusive writers
    // GetState() is read-heavy (called frequently by polling clients)
    private readonly ReaderWriterLockSlim _stateLock = new();

    // Channels for fire-and-forget broadcasts (replaces Task.Run pattern)
    private readonly AsyncQueue<MoveBroadcast> _moveQueue;
    private readonly AsyncQueue<BoardUpdateBroadcast> _boardQueue;
    private readonly AsyncQueue<GameLogBroadcast> _logQueue;

    // Atomic status using Interlocked.CompareExchange for thread-safe transitions
    private volatile TournamentStatus _status = TournamentStatus.Idle;

    private List<AIBot> _bots = new();
    private List<TournamentMatch> _scheduledMatches = new();
    private List<TournamentMatch> _completedMatches = new();
    private TournamentState _state = new();
    private CancellationTokenSource? _tournamentCts;
    private Task? _tournamentTask;

    public TournamentManager(
        IHubContext<TournamentHub, ITournamentClient> hub,
        ILogger<TournamentManager> logger,
        GameLogService logService)
    {
        _hub = hub;
        _logger = logger;
        _logService = logService;

        // Create Channel-based queues for broadcasts
        // DropOldest mode prevents unbounded growth under high load
        _moveQueue = new AsyncQueue<MoveBroadcast>(
            ProcessMoveBroadcastAsync,
            capacity: 100,
            queueName: "MoveBroadcast",
            dropOldest: true);

        _boardQueue = new AsyncQueue<BoardUpdateBroadcast>(
            ProcessBoardBroadcastAsync,
            capacity: 50,
            queueName: "BoardUpdate",
            dropOldest: true);

        _logQueue = new AsyncQueue<GameLogBroadcast>(
            ProcessLogBroadcastAsync,
            capacity: 200,
            queueName: "GameLog",
            dropOldest: true);
    }

    /// <summary>
    /// Gets the current tournament state (thread-safe with ReaderWriterLockSlim)
    /// Multiple readers can access simultaneously without blocking each other
    /// </summary>
    public TournamentState GetState()
    {
        _stateLock.EnterReadLock();
        try
        {
            // Return a deep copy to avoid external modifications
            return new TournamentState
            {
                Status = _state.Status,
                CompletedGames = _state.CompletedGames,
                TotalGames = _state.TotalGames,
                Bots = _state.Bots.Select(b => new AIBot
                {
                    Name = b.Name,
                    Difficulty = b.Difficulty,
                    ELO = b.ELO,
                    Wins = b.Wins,
                    Losses = b.Losses,
                    Draws = b.Draws
                }).ToList(),
                MatchHistory = _state.MatchHistory.Select(m => new MatchResult
                {
                    Winner = m.Winner,
                    Loser = m.Loser,
                    TotalMoves = m.TotalMoves,
                    DurationMs = m.DurationMs,
                    MoveTimesMs = new List<long>(m.MoveTimesMs),
                    WinnerDifficulty = m.WinnerDifficulty,
                    LoserDifficulty = m.LoserDifficulty,
                    FinalBoard = m.FinalBoard,
                    IsDraw = m.IsDraw,
                    EndedByTimeout = m.EndedByTimeout,
                    WinnerBotName = m.WinnerBotName,
                    LoserBotName = m.LoserBotName,
                }).ToList(),
                CurrentMatch = _state.CurrentMatch == null ? null : new CurrentMatchInfo
                {
                    GameId = _state.CurrentMatch.GameId,
                    RedBotName = _state.CurrentMatch.RedBotName,
                    BlueBotName = _state.CurrentMatch.BlueBotName,
                    RedDifficulty = _state.CurrentMatch.RedDifficulty,
                    BlueDifficulty = _state.CurrentMatch.BlueDifficulty,
                    MoveNumber = _state.CurrentMatch.MoveNumber,
                    Board = new List<BoardCell>(_state.CurrentMatch.Board),
                    RedTimeRemainingMs = _state.CurrentMatch.RedTimeRemainingMs,
                    BlueTimeRemainingMs = _state.CurrentMatch.BlueTimeRemainingMs,
                    InitialTimeSeconds = _state.CurrentMatch.InitialTimeSeconds,
                    IncrementSeconds = _state.CurrentMatch.IncrementSeconds
                },
                StartTimeUtc = _state.StartTimeUtc,
                EndTimeUtc = _state.EndTimeUtc
            };
        }
        finally
        {
            _stateLock.ExitReadLock();
        }
    }

    /// <summary>
    /// Starts the tournament with all 22 bots
    /// Uses Interlocked.CompareExchange for atomic status transition
    /// </summary>
    public async Task<bool> StartTournamentAsync()
    {
        // Atomic check-and-set: only proceed if currently Idle
        if (Interlocked.CompareExchange(ref _status, TournamentStatus.Running, TournamentStatus.Idle)
            != TournamentStatus.Idle)
        {
            _logger.LogWarning("Tournament is already running or paused");
            return false;
        }

        _stateLock.EnterWriteLock();
        try
        {
            // Initialize bots
            _bots = AIBotFactory.GetAllTournamentBots();
            _scheduledMatches = TournamentScheduler.GenerateRoundRobinSchedule(_bots);
            _completedMatches = new List<TournamentMatch>();

            _state = new TournamentState
            {
                Status = TournamentStatus.Running,
                TotalGames = _scheduledMatches.Count,
                CompletedGames = 0,
                Bots = new List<AIBot>(_bots),
                MatchHistory = new List<MatchResult>(),
                CurrentMatch = null,
                StartTimeUtc = DateTime.UtcNow,
                EndTimeUtc = null
            };

            _tournamentCts = new CancellationTokenSource();
        }
        finally
        {
            _stateLock.ExitWriteLock();
        }

        _logger.LogInformation("Tournament started: {TotalGames} games scheduled", _scheduledMatches.Count);
        await _hub.Clients.All.OnTournamentStatusChanged(TournamentStatus.Running,
            $"Tournament started with {_scheduledMatches.Count} games");

        // Start the tournament loop
        _tournamentTask = RunTournamentLoopAsync(_tournamentCts.Token);

        return true;
    }

    /// <summary>
    /// Pauses the tournament after the current game completes
    /// Uses Interlocked.CompareExchange for atomic status transition
    /// </summary>
    public async Task<bool> PauseTournamentAsync()
    {
        // Atomic check-and-set: only proceed if currently Running
        if (Interlocked.CompareExchange(ref _status, TournamentStatus.Paused, TournamentStatus.Running)
            != TournamentStatus.Running)
        {
            return false;
        }

        _stateLock.EnterWriteLock();
        try
        {
            _state.Status = TournamentStatus.Paused;
        }
        finally
        {
            _stateLock.ExitWriteLock();
        }

        _logger.LogInformation("Tournament pause requested");
        await _hub.Clients.All.OnTournamentStatusChanged(TournamentStatus.Paused,
            "Tournament pausing after current game...");

        return true;
    }

    /// <summary>
    /// Resumes a paused tournament
    /// Uses Interlocked.CompareExchange for atomic status transition
    /// </summary>
    public async Task<bool> ResumeTournamentAsync()
    {
        // Atomic check-and-set: only proceed if currently Paused
        if (Interlocked.CompareExchange(ref _status, TournamentStatus.Running, TournamentStatus.Paused)
            != TournamentStatus.Paused)
        {
            return false;
        }

        _stateLock.EnterWriteLock();
        try
        {
            _state.Status = TournamentStatus.Running;
        }
        finally
        {
            _stateLock.ExitWriteLock();
        }

        _logger.LogInformation("Tournament resumed");
        await _hub.Clients.All.OnTournamentStatusChanged(TournamentStatus.Running,
            "Tournament resumed");

        return true;
    }

    /// <summary>
    /// Background service entry point - keeps the service alive
    /// </summary>
    protected override async Task ExecuteAsync(CancellationToken stoppingToken)
    {
        _logger.LogInformation("TournamentManager background service started");

        // Just keep the service alive, actual work happens in RunTournamentLoopAsync
        while (!stoppingToken.IsCancellationRequested)
        {
            await Task.Delay(1000, stoppingToken);
        }

        _logger.LogInformation("TournamentManager background service stopping");
    }

    /// <summary>
    /// Main tournament execution loop
    /// </summary>
    private async Task RunTournamentLoopAsync(CancellationToken ct)
    {
        var engine = new TournamentEngine();

        try
        {
            while (!ct.IsCancellationRequested && _scheduledMatches.Count > 0)
            {
                // Check if paused (read volatile _status directly, no lock needed)
                if (_status == TournamentStatus.Paused)
                {
                    await Task.Delay(500, ct);
                    continue;
                }

                // Get next match
                var match = _scheduledMatches[0];
                _scheduledMatches.RemoveAt(0);

                // Update current match info
                _stateLock.EnterWriteLock();
                try
                {
                    match.IsInProgress = true;
                    _state.CurrentMatch = new CurrentMatchInfo
                    {
                        GameId = Guid.NewGuid().ToString(),
                        RedBotName = match.RedBot.Name,
                        BlueBotName = match.BlueBot.Name,
                        RedDifficulty = match.RedBot.Difficulty,
                        BlueDifficulty = match.BlueBot.Difficulty,
                        MoveNumber = 0,
                        Board = new List<BoardCell>(),
                        RedTimeRemainingMs = 420 * 1000,
                        BlueTimeRemainingMs = 420 * 1000
                    };
                }
                finally
                {
                    _stateLock.ExitWriteLock();
                }

                // Notify clients game started
                await _hub.Clients.All.OnGameStarted(
                    _state.CurrentMatch.GameId,
                    match.RedBot.Name,
                    match.BlueBot.Name,
                    match.RedBot.Difficulty,
                    match.BlueBot.Difficulty
                );

                // Capture current game ID for callbacks (avoid lock in callbacks)
                var currentGameId = _state.CurrentMatch?.GameId ?? string.Empty;

                // Run the match with callbacks for live updates
                var result = await Task.Run(() =>
                {
                    return engine.RunGame(
                        match.RedBot.Difficulty,
                        match.BlueBot.Difficulty,
                        maxMoves: 225,
                        initialTimeSeconds: 420,
                        incrementSeconds: 5,
                        ponderingEnabled: true,
                        onMove: (x, y, player, moveNumber, redTimeMs, blueTimeMs, stats) =>
                        {
                            // Channel-based broadcast: enqueue without blocking
                            _moveQueue.TryEnqueue(new MoveBroadcast
                            {
                                GameId = currentGameId,
                                X = x,
                                Y = y,
                                Player = player,
                                MoveNumber = moveNumber,
                                RedTimeRemainingMs = redTimeMs,
                                BlueTimeRemainingMs = blueTimeMs,
                                Stats = stats
                            });
                        },
                        onBoardUpdate: (board, moveNumber, redTimeMs, blueTimeMs, lastMoveX, lastMoveY, lastMovePlayer) =>
                        {
                            // Channel-based broadcast: enqueue without blocking
                            // Capture board state here (avoid accessing board after callback returns)
                            var boardCells = ExtractBoardCells(board);

                            _boardQueue.TryEnqueue(new BoardUpdateBroadcast
                            {
                                GameId = currentGameId,
                                BoardCells = boardCells,
                                MoveNumber = moveNumber,
                                RedTimeRemainingMs = redTimeMs,
                                BlueTimeRemainingMs = blueTimeMs,
                                LastMoveX = lastMoveX,
                                LastMoveY = lastMoveY,
                                LastMovePlayer = lastMovePlayer
                            });
                        },
                        onLog: (level, source, message) =>
                        {
                            // Channel-based broadcast: enqueue without blocking
                            _logQueue.TryEnqueue(new GameLogBroadcast
                            {
                                Entry = new GameLogEntry
                                {
                                    Timestamp = DateTime.UtcNow.ToString("HH:mm:ss.fff"),
                                    Level = level,
                                    Source = source,
                                    Message = message,
                                    GameId = currentGameId
                                }
                            });
                        },
                        redBotName: match.RedBot.Name,
                        blueBotName: match.BlueBot.Name
                    );
                }, ct);

                // Process result
                await ProcessMatchResultAsync(match, result);

                // Pause between games for viewer to appreciate result
                await Task.Delay(2500, ct);
            }

            // Tournament complete
            await CompleteTournamentAsync();
        }
        catch (OperationCanceledException)
        {
            _logger.LogInformation("Tournament loop cancelled");
        }
        catch (Exception ex)
        {
            _logger.LogError(ex, "Error in tournament loop");
        }
    }

    /// <summary>
    /// Processes a completed match result and updates ELO ratings
    /// </summary>
    private async Task ProcessMatchResultAsync(TournamentMatch match, MatchResult result)
    {
        // Find the actual bot instances
        var redBot = _bots.First(b => b.Name == match.RedBot.Name);
        var blueBot = _bots.First(b => b.Name == match.BlueBot.Name);

        // Update ELO ratings - determine winner and loser first
        if (!result.IsDraw)
        {
            var winnerBot = result.Winner == Player.Red ? redBot : blueBot;
            var loserBot = result.Winner == Player.Red ? blueBot : redBot;
            TournamentELO.UpdateELOs(winnerBot, loserBot, isDraw: false);
        }
        else
        {
            TournamentELO.UpdateELOs(redBot, blueBot, isDraw: true);
        }

        // Update match
        match.IsCompleted = true;
        match.IsInProgress = false;
        match.Result = result;

        _stateLock.EnterWriteLock();
        try
        {
            _completedMatches.Add(match);
            _state.CompletedGames++;
            _state.MatchHistory.Add(result);
            _state.CurrentMatch = null;

            // Update bot list in state
            _state.Bots = _bots.Select(b => new AIBot
            {
                Name = b.Name,
                Difficulty = b.Difficulty,
                ELO = b.ELO,
                Wins = b.Wins,
                Losses = b.Losses,
                Draws = b.Draws
            }).ToList();
        }
        finally
        {
            _stateLock.ExitWriteLock();
        }

        // Send updates to clients
        await _hub.Clients.All.OnGameFinished(new GameFinishedEvent
        {
            GameId = _state.CurrentMatch?.GameId ?? Guid.NewGuid().ToString(),
            Winner = result.Winner.ToString().ToLower(),
            Loser = result.Loser.ToString().ToLower(),
            IsDraw = result.IsDraw,
            EndedByTimeout = result.EndedByTimeout,
            TotalMoves = result.TotalMoves,
            DurationMs = result.DurationMs,
            UpdatedBots = _state.Bots.ToList()
        });

        await _hub.Clients.All.OnTournamentProgress(
            _state.CompletedGames,
            _state.TotalGames,
            _state.ProgressPercent,
            _scheduledMatches.Count > 0
                ? $"{_scheduledMatches[0].RedBot.Name} vs {_scheduledMatches[0].BlueBot.Name}"
                : "Finalizing..."
        );

        await _hub.Clients.All.OnELOUpdated(_state.Bots.ToList());
    }

    /// <summary>
    /// Marks tournament as completed and sends final results
    /// </summary>
    private async Task CompleteTournamentAsync()
    {
        Interlocked.Exchange(ref _status, TournamentStatus.Completed);

        _stateLock.EnterWriteLock();
        try
        {
            _state.Status = TournamentStatus.Completed;
            _state.EndTimeUtc = DateTime.UtcNow;
        }
        finally
        {
            _stateLock.ExitWriteLock();
        }

        var finalStandings = _bots.OrderByDescending(b => b.ELO).ToList();

        await _hub.Clients.All.OnTournamentCompleted(
            finalStandings,
            _state.CompletedGames,
            (long)_state.Elapsed.TotalMilliseconds
        );

        await _hub.Clients.All.OnTournamentStatusChanged(TournamentStatus.Completed,
            $"Tournament completed! {_state.CompletedGames} games in {_state.Elapsed:hh\\:mm\\:ss}");

        _logger.LogInformation("Tournament completed: {Games} games in {Duration}",
            _state.CompletedGames, _state.Elapsed);
    }

    /// <summary>
    /// Extracts occupied board cells from a board
    /// Called outside of lock context to avoid holding lock during iteration
    /// </summary>
    private static List<BoardCell> ExtractBoardCells(Board board)
    {
        var cells = new List<BoardCell>();
        for (int bx = 0; bx < 15; bx++)
        {
            for (int by = 0; by < 15; by++)
            {
                var cell = board.GetCell(bx, by);
                if (cell.Player != Player.None)
                {
                    cells.Add(new BoardCell
                    {
                        X = bx,
                        Y = by,
                        Player = cell.Player.ToString().ToLower()
                    });
                }
            }
        }
        return cells;
    }

    /// <summary>
    /// Processes move broadcasts from the channel (background processing)
    /// </summary>
    private async ValueTask ProcessMoveBroadcastAsync(MoveBroadcast broadcast)
    {
        try
        {
            await _hub.Clients.All.OnMovePlayed(new MoveEvent
            {
                GameId = broadcast.GameId,
                X = broadcast.X,
                Y = broadcast.Y,
                Player = broadcast.Player.ToString().ToLower(),
                MoveNumber = broadcast.MoveNumber,
                RedTimeRemainingMs = broadcast.RedTimeRemainingMs,
                BlueTimeRemainingMs = broadcast.BlueTimeRemainingMs,
                DepthAchieved = broadcast.Stats?.DepthAchieved ?? 0,
                NodesSearched = broadcast.Stats?.NodesSearched ?? 0,
                NodesPerSecond = broadcast.Stats?.NodesPerSecond ?? 0,
                TableHitRate = broadcast.Stats?.TableHitRate ?? 0,
                PonderingActive = broadcast.Stats?.PonderingActive ?? false,
                VCFDepthAchieved = broadcast.Stats?.VCFDepthAchieved ?? 0,
                VCFNodesSearched = broadcast.Stats?.VCFNodesSearched ?? 0
            });
        }
        catch (Exception ex)
        {
            _logger.LogError(ex, "Error broadcasting move");
        }
    }

    /// <summary>
    /// Processes board update broadcasts from the channel (background processing)
    /// </summary>
    private async ValueTask ProcessBoardBroadcastAsync(BoardUpdateBroadcast broadcast)
    {
        try
        {
            // Broadcast board state to clients
            await _hub.Clients.All.OnBoardUpdate(
                broadcast.GameId,
                broadcast.BoardCells,
                broadcast.MoveNumber,
                broadcast.LastMovePlayer.ToString().ToLower(),
                broadcast.LastMoveX,
                broadcast.LastMoveY
            );

            // Update current match state (minimal lock time)
            _stateLock.EnterWriteLock();
            try
            {
                if (_state.CurrentMatch != null)
                {
                    _state.CurrentMatch.Board = broadcast.BoardCells;
                    _state.CurrentMatch.MoveNumber = broadcast.MoveNumber;
                    _state.CurrentMatch.RedTimeRemainingMs = broadcast.RedTimeRemainingMs;
                    _state.CurrentMatch.BlueTimeRemainingMs = broadcast.BlueTimeRemainingMs;
                }
            }
            finally
            {
                _stateLock.ExitWriteLock();
            }
        }
        catch (Exception ex)
        {
            _logger.LogError(ex, "Error broadcasting board update");
        }
    }

    /// <summary>
    /// Processes log broadcasts from the channel (background processing)
    /// </summary>
    private async ValueTask ProcessLogBroadcastAsync(GameLogBroadcast broadcast)
    {
        try
        {
            // Persist to SQLite database
            await _logService.LogAsync(broadcast.Entry);

            // Broadcast to clients
            await _hub.Clients.All.OnGameLog(new GameLogEvent
            {
                Timestamp = broadcast.Entry.Timestamp,
                Level = broadcast.Entry.Level,
                Source = broadcast.Entry.Source,
                Message = broadcast.Entry.Message
            });
        }
        catch (Exception ex)
        {
            _logger.LogError(ex, "Error processing game log");
        }
    }

    /// <summary>
    /// Stops the tournament service gracefully
    /// </summary>
    public override async Task StopAsync(CancellationToken cancellationToken)
    {
        _tournamentCts?.Cancel();

        if (_tournamentTask != null)
        {
            await Task.WhenAny(_tournamentTask, Task.Delay(5000, cancellationToken));
        }

        // Dispose broadcast queues
        _moveQueue.Dispose();
        _boardQueue.Dispose();
        _logQueue.Dispose();

        await base.StopAsync(cancellationToken);
    }

    /// <summary>
    /// Broadcast record for move events
    /// </summary>
    private record MoveBroadcast
    {
        public required string GameId { get; init; }
        public int X { get; init; }
        public int Y { get; init; }
        public Player Player { get; init; }
        public int MoveNumber { get; init; }
        public long RedTimeRemainingMs { get; init; }
        public long BlueTimeRemainingMs { get; init; }
        public MoveStats? Stats { get; init; }
    }

    /// <summary>
    /// Broadcast record for board update events
    /// </summary>
    private record BoardUpdateBroadcast
    {
        public required string GameId { get; init; }
        public required List<BoardCell> BoardCells { get; init; }
        public int MoveNumber { get; init; }
        public long RedTimeRemainingMs { get; init; }
        public long BlueTimeRemainingMs { get; init; }
        public int LastMoveX { get; init; }
        public int LastMoveY { get; init; }
        public Player LastMovePlayer { get; init; }
    }

    /// <summary>
    /// Broadcast record for log events
    /// </summary>
    private record GameLogBroadcast
    {
        public required GameLogEntry Entry { get; init; }
    }
}
