using Caro.Api.Logging;
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
public class TournamentManager : BackgroundService
{
    private readonly IHubContext<TournamentHub, ITournamentClient> _hub;
    private readonly ILogger<TournamentManager> _logger;
    private readonly GameLogService _logService;
    private readonly object _stateLock = new();

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
    }

    /// <summary>
    /// Gets the current tournament state (thread-safe)
    /// </summary>
    public TournamentState GetState()
    {
        lock (_stateLock)
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
    }

    /// <summary>
    /// Starts the tournament with all 22 bots
    /// </summary>
    public async Task<bool> StartTournamentAsync()
    {
        lock (_stateLock)
        {
            if (_state.Status == TournamentStatus.Running)
            {
                _logger.LogWarning("Tournament is already running");
                return false;
            }

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

        _logger.LogInformation("Tournament started: {TotalGames} games scheduled", _scheduledMatches.Count);
        await _hub.Clients.All.OnTournamentStatusChanged(TournamentStatus.Running,
            $"Tournament started with {_scheduledMatches.Count} games");

        // Start the tournament loop
        _tournamentTask = RunTournamentLoopAsync(_tournamentCts.Token);

        return true;
    }

    /// <summary>
    /// Pauses the tournament after the current game completes
    /// </summary>
    public async Task<bool> PauseTournamentAsync()
    {
        lock (_stateLock)
        {
            if (_state.Status != TournamentStatus.Running)
                return false;

            _state.Status = TournamentStatus.Paused;
        }

        _logger.LogInformation("Tournament pause requested");
        await _hub.Clients.All.OnTournamentStatusChanged(TournamentStatus.Paused,
            "Tournament pausing after current game...");

        return true;
    }

    /// <summary>
    /// Resumes a paused tournament
    /// </summary>
    public async Task<bool> ResumeTournamentAsync()
    {
        lock (_stateLock)
        {
            if (_state.Status != TournamentStatus.Paused)
                return false;

            _state.Status = TournamentStatus.Running;
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
                // Check if paused
                TournamentStatus currentStatus;
                lock (_stateLock)
                {
                    currentStatus = _state.Status;
                }

                if (currentStatus == TournamentStatus.Paused)
                {
                    await Task.Delay(500, ct);
                    continue;
                }

                // Get next match
                var match = _scheduledMatches[0];
                _scheduledMatches.RemoveAt(0);

                // Update current match info
                lock (_stateLock)
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

                // Notify clients game started
                await _hub.Clients.All.OnGameStarted(
                    _state.CurrentMatch.GameId,
                    match.RedBot.Name,
                    match.BlueBot.Name,
                    match.RedBot.Difficulty,
                    match.BlueBot.Difficulty
                );

                // Run the match with callbacks for live updates
                var result = await Task.Run(() =>
                {
                    string currentGameId = _state.CurrentMatch?.GameId ?? string.Empty;
                    Player? lastMovePlayer = null;

                    return engine.RunGame(
                        match.RedBot.Difficulty,
                        match.BlueBot.Difficulty,
                        maxMoves: 225,
                        initialTimeSeconds: 420,
                        incrementSeconds: 5,
                        ponderingEnabled: true,
                        onMove: (x, y, player, moveNumber, redTimeMs, blueTimeMs, stats) =>
                        {
                            // Track last move player for pondering detection
                            lastMovePlayer = player;

                            // Fire-and-forget broadcast move stats to all clients
                            _ = Task.Run(async () =>
                            {
                                try
                                {
                                    await _hub.Clients.All.OnMovePlayed(new MoveEvent
                                    {
                                        GameId = currentGameId,
                                        X = x,
                                        Y = y,
                                        Player = player.ToString().ToLower(),
                                        MoveNumber = moveNumber,
                                        RedTimeRemainingMs = redTimeMs,
                                        BlueTimeRemainingMs = blueTimeMs,
                                        DepthAchieved = stats?.DepthAchieved ?? 0,
                                        NodesSearched = stats?.NodesSearched ?? 0,
                                        NodesPerSecond = stats?.NodesPerSecond ?? 0,
                                        TableHitRate = stats?.TableHitRate ?? 0,
                                        PonderingActive = stats?.PonderingActive ?? false,
                                        VCFDepthAchieved = stats?.VCFDepthAchieved ?? 0,
                                        VCFNodesSearched = stats?.VCFNodesSearched ?? 0
                                    });
                                }
                                catch (Exception ex)
                                {
                                    _logger.LogError(ex, "Error broadcasting move");
                                }
                            });
                        },
                        onBoardUpdate: (board, moveNumber, redTimeMs, blueTimeMs, lastMoveX, lastMoveY, lastMovePlayer) =>
                        {
                            // Fire-and-forget broadcast board state with move info
                            _ = Task.Run(async () =>
                            {
                                try
                                {
                                    var boardCells = new List<BoardCell>();

                                    for (int bx = 0; bx < 15; bx++)
                                    {
                                        for (int by = 0; by < 15; by++)
                                        {
                                            var cell = board.GetCell(bx, by);
                                            if (cell.Player != Player.None)
                                            {
                                                boardCells.Add(new BoardCell { X = bx, Y = by, Player = cell.Player.ToString().ToLower() });
                                            }
                                        }
                                    }

                                    // Atomic update: board + move info together (using actual move coordinates)
                                    await _hub.Clients.All.OnBoardUpdate(
                                        currentGameId,
                                        boardCells,
                                        moveNumber,
                                        lastMovePlayer.ToString().ToLower(),
                                        lastMoveX,
                                        lastMoveY
                                    );

                                    // Update current match state with time
                                    lock (_stateLock)
                                    {
                                        if (_state.CurrentMatch != null)
                                        {
                                            _state.CurrentMatch.Board = boardCells;
                                            _state.CurrentMatch.MoveNumber = moveNumber;
                                            _state.CurrentMatch.RedTimeRemainingMs = redTimeMs;
                                            _state.CurrentMatch.BlueTimeRemainingMs = blueTimeMs;
                                        }
                                    }
                                }
                                catch (Exception ex)
                                {
                                    _logger.LogError(ex, "Error broadcasting board update");
                                }
                            });
                        },
                        onLog: (level, source, message) =>
                        {
                            // Fire-and-forget: persist to database and broadcast to clients
                            _ = Task.Run(async () =>
                            {
                                try
                                {
                                    // Persist to SQLite database
                                    await _logService.LogAsync(new GameLogEntry
                                    {
                                        Timestamp = DateTime.UtcNow.ToString("HH:mm:ss.fff"),
                                        Level = level,
                                        Source = source,
                                        Message = message,
                                        GameId = currentGameId
                                    });

                                    // Broadcast to clients
                                    await _hub.Clients.All.OnGameLog(new GameLogEvent
                                    {
                                        Timestamp = DateTime.UtcNow.ToString("HH:mm:ss.fff"),
                                        Level = level,
                                        Source = source,
                                        Message = message
                                    });
                                }
                                catch (Exception ex)
                                {
                                    _logger.LogError(ex, "Error processing game log");
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

        lock (_stateLock)
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
        lock (_stateLock)
        {
            _state.Status = TournamentStatus.Completed;
            _state.EndTimeUtc = DateTime.UtcNow;
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
    /// Stops the tournament service gracefully
    /// </summary>
    public override async Task StopAsync(CancellationToken cancellationToken)
    {
        _tournamentCts?.Cancel();

        if (_tournamentTask != null)
        {
            await Task.WhenAny(_tournamentTask, Task.Delay(5000, cancellationToken));
        }

        await base.StopAsync(cancellationToken);
    }
}
