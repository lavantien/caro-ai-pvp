using System.Diagnostics;
using Caro.Domain;
using Caro.Engine;

namespace Caro.Api;

/// <summary>
/// One live game: immutable domain state plus mutable Fischer clocks, owned
/// AI instances, and ponder orchestration, all under a single lock.
/// </summary>
public sealed partial class GameSession
{
    private readonly object _mu = new();
    private GameState _game;
    private long _redTimeMs;
    private long _blueTimeMs;
    private DateTime _lastMoveAt;
    private readonly int? _redDifficulty;
    private readonly int? _blueDifficulty;
    private readonly Func<int> _activeGameCount;
    private readonly CaroConfig _config;
    private MinimaxAI? _redAI;
    private MinimaxAI? _blueAI;

    public GameSession(
        string timeControl,
        long initialTimeMs,
        int incrementSeconds,
        GameMode mode,
        int? redDiff,
        int? blueDiff,
        Func<int> activeGameCount,
        CaroConfig? config = null)
    {
        _game = GameState.NewGameState(mode, timeControl, initialTimeMs, incrementSeconds);
        _redTimeMs = initialTimeMs;
        _blueTimeMs = initialTimeMs;
        _lastMoveAt = DateTime.UtcNow;
        _redDifficulty = redDiff;
        _blueDifficulty = blueDiff;
        _activeGameCount = activeGameCount;
        _config = config ?? CaroConfig.Default;
    }

    /// <summary>
    /// Plays a seeded two-stone opening (red from the center region, blue
    /// replying locally) so engine-vs-engine samples are not all the same
    /// game. Deterministic per seed.
    /// </summary>
    internal void ApplyRandomOpening(long seed)
    {
        OpeningRng rng = new(seed);
        int low = Constants.Board.Size / 2 - _config.OpeningSpreadRadius;
        int high = Constants.Board.Size / 2 + _config.OpeningSpreadRadius - 1;
        int rx = low + rng.Next(high - low + 1);
        int ry = low + rng.Next(high - low + 1);
        _game = _game.WithMove(rx, ry);

        int bx = rx - _config.OpeningSpreadRadius + rng.Next(2 * _config.OpeningSpreadRadius + 1);
        int by = ry - _config.OpeningSpreadRadius + rng.Next(2 * _config.OpeningSpreadRadius + 1);
        bx = Math.Clamp(bx, 0, Constants.Board.Size - 1);
        by = Math.Clamp(by, 0, Constants.Board.Size - 1);
        if (bx == rx && by == ry)
        {
            bx = (bx + 1) % Constants.Board.Size;
        }
        _game = _game.WithMove(bx, by);
    }

    // openingRNG is a splitmix64 generator: small, deterministic, seedable.
    private sealed class OpeningRng(long seed)
    {
        private ulong _state = (ulong)seed;

        public int Next(int n)
        {
            _state += SplitMix64.GoldenGamma;
            return (int)(SplitMix64.Mix(_state) % (ulong)n);
        }
    }

    private long ElapsedSinceLastMoveMs() => (long)(DateTime.UtcNow - _lastMoveAt).TotalMilliseconds;

    // Test seams for scenarios the public API cannot produce directly:
    // rewound clocks and synthetic board states.
    internal void BackdateLastMoveForTest(TimeSpan age)
    {
        lock (_mu)
        {
            _lastMoveAt = DateTime.UtcNow - age;
        }
    }

    internal void InstallBoardForTest(Board board, int moveNumber, Player currentPlayer)
    {
        lock (_mu)
        {
            _game = new GameState(board, currentPlayer, moveNumber, isGameOver: false, Player.None,
                winningLine: null, endReason: string.Empty,
                boardHistory: [], moveHistory: [],
                _game.TimeControl, _game.InitialTimeMs, _game.IncrementSeconds, _game.GameMode);
        }
    }

    public GameResponse GetResponse()
    {
        lock (_mu)
        {
            CheckTimeoutLocked();
            return BuildResponse();
        }
    }

    public bool IsGameOver()
    {
        lock (_mu)
        {
            CheckTimeoutLocked();
            return _game.IsGameOver;
        }
    }

    // CheckTimeoutLocked adjudicates a flag fall: if the player on the clock
    // has let it run out since the last move, they lose on time.
    private void CheckTimeoutLocked()
    {
        if (_game.IsGameOver)
        {
            return;
        }
        long elapsed = ElapsedSinceLastMoveMs();
        if (_game.CurrentPlayer == Player.Blue ? elapsed >= _blueTimeMs : elapsed >= _redTimeMs)
        {
            if (_game.CurrentPlayer == Player.Blue)
            {
                _blueTimeMs = 0;
            }
            else
            {
                _redTimeMs = 0;
            }
            Player winner = _game.CurrentPlayer == Player.Blue ? Player.Red : Player.Blue;
            _game = _game.WithTimeout(winner);
            DisposeAI();
        }
    }

    public DateTime LastActivityAt()
    {
        lock (_mu)
        {
            return _lastMoveAt;
        }
    }

    public (Board Board, Player Player, bool IsGameOver, long TimeRemainingMs, int IncrementSeconds, int MoveNumber, int? Difficulty) ExtractForAI()
    {
        lock (_mu)
        {
            CheckTimeoutLocked();

            long timeRemaining = _redTimeMs;
            int? diff = _redDifficulty;
            if (_game.CurrentPlayer == Player.Blue)
            {
                timeRemaining = _blueTimeMs;
                diff = _blueDifficulty;
            }

            return (_game.Board, _game.CurrentPlayer, _game.IsGameOver,
                timeRemaining, _game.IncrementSeconds, _game.MoveNumber, diff);
        }
    }

    public MinimaxAI GetOrCreateAI(Player player)
    {
        // Compute the thread budget before taking the session lock: the
        // callback locks the store, and the store's ActiveGameCount locks
        // sessions, so locking in the other order would deadlock.
        int threads = Difficulty.GetEngineThreadsForLoad(_activeGameCount());
        int? diff = player == Player.Blue ? _blueDifficulty : _redDifficulty;
        int ttSizeMB = _config.DefaultSessionTTSizeMB;
        if (diff is >= Constants.Difficulty.MinLevel and <= Constants.Difficulty.MaxLevel)
        {
            ttSizeMB = Difficulty.GetDifficultyProfile(diff.Value, _config).TTSizeMB;
        }

        lock (_mu)
        {
            if (player == Player.Red)
            {
                return _redAI ??= new MinimaxAI(threads, ttSizeMB, _config.TimeManagement);
            }
            return _blueAI ??= new MinimaxAI(threads, ttSizeMB, _config.TimeManagement);
        }
    }

    /// <summary>
    /// Applies a move the engine computed for expectedPlayer. The search
    /// runs unlocked for seconds, so the turn is re-validated here: if
    /// another move landed first, the stale result is rejected instead of
    /// being played for the wrong color.
    /// </summary>
    public GameResponse ApplyAIMove(int x, int y, Player expectedPlayer)
    {
        lock (_mu)
        {
            CheckTimeoutLocked();

            if (_game.IsGameOver)
            {
                throw new GameOverException();
            }
            if (_game.CurrentPlayer != expectedPlayer)
            {
                throw new NotPlayerTurnException();
            }
            return ApplyMoveLocked(x, y);
        }
    }

    /// <summary>
    /// Validates that a human may move right now: spectators cannot inject
    /// moves into AI-vs-AI games, and in player-vs-AI the human cannot move
    /// on the engine's turn.
    /// </summary>
    public GameResponse ApplyHumanMove(int x, int y)
    {
        lock (_mu)
        {
            CheckTimeoutLocked();

            if (_game.IsGameOver)
            {
                throw new GameOverException();
            }
            switch (_game.GameMode)
            {
                case GameMode.AivAI:
                    throw new NotPlayerTurnException();
                case GameMode.PvAI:
                    bool aiIsRed = _redDifficulty != null;
                    if ((aiIsRed && _game.CurrentPlayer == Player.Red)
                        || (!aiIsRed && _game.CurrentPlayer == Player.Blue))
                    {
                        throw new NotPlayerTurnException();
                    }
                    break;
            }
            return ApplyMoveLocked(x, y);
        }
    }

    public GameResponse ApplyMove(int x, int y)
    {
        lock (_mu)
        {
            CheckTimeoutLocked();

            if (_game.IsGameOver)
            {
                throw new GameOverException();
            }
            return ApplyMoveLocked(x, y);
        }
    }

    // ApplyMoveLocked applies a move for the current player.
    private GameResponse ApplyMoveLocked(int x, int y)
    {
        Player mover = _game.CurrentPlayer;
        GameState newGame = _game.WithMove(x, y);
        StopPonderLocked(x, y);

        WinResult result = WinDetector.CheckWinFromMove(newGame.Board, x, y);
        if (result.HasWinner)
        {
            newGame = newGame.WithGameOver(result.Winner, result.WinningLine);
        }
        else if (newGame.MoveNumber >= Constants.Board.MaxMoves)
        {
            newGame = newGame.WithDraw();
        }

        DateTime now = DateTime.UtcNow;
        long elapsed = (long)(now - _lastMoveAt).TotalMilliseconds;
        long inc = newGame.IncrementSeconds * 1000L;
        if (_game.CurrentPlayer == Player.Red)
        {
            _redTimeMs = Math.Max(0, _redTimeMs - elapsed + inc);
        }
        else
        {
            _blueTimeMs = Math.Max(0, _blueTimeMs - elapsed + inc);
        }
        _lastMoveAt = now;

        _game = newGame;

        if (newGame.IsGameOver)
        {
            DisposeAI();
        }
        else
        {
            StartPonderLocked(mover);
        }

        return BuildResponse();
    }

    public GameResponse UndoLastMove()
    {
        lock (_mu)
        {
            // Any ponder or staged hit refers to a position that is about to
            // disappear; drop it all before taking moves back.
            ClearPonderStateLocked();

            GameState newGame = _game.UndoMove();
            _game = newGame;

            // In player-vs-AI a single ply of undo would hand the turn
            // straight to the engine (its reply comes free). Take back a
            // full turn so the human is on the move again.
            if (_game.GameMode == GameMode.PvAI && !_game.IsGameOver && AiOwnsTurnLocked() && _game.BoardHistory.Length > 0)
            {
                _game = _game.UndoMove();
            }
            return BuildResponse();
        }
    }

    // AiOwnsTurnLocked reports whether the engine side is to move.
    private bool AiOwnsTurnLocked() => _redDifficulty != null
        ? _game.CurrentPlayer == Player.Red
        : _game.CurrentPlayer == Player.Blue;

    public void DisposeAI()
    {
        // Reentrant on session paths (the lock is a monitor); store-driven
        // teardown reaches here without the lock and still synchronizes.
        lock (_mu)
        {
            ClearPonderStateLocked();
            _redAI?.Dispose();
            _redAI = null;
            _blueAI?.Dispose();
            _blueAI = null;
        }
    }

    private GameResponse BuildResponse()
    {
        List<CellResponse> cells = new(Constants.Board.Size * Constants.Board.Size);
        for (int y = 0; y < Constants.Board.Size; y++)
        {
            for (int x = 0; x < Constants.Board.Size; x++)
            {
                Player player = _game.Board.GetPlayerAt(x, y);
                cells.Add(new CellResponse(x, y, player.ToName()));
            }
        }

        List<PositionResponse>? winningLine = null;
        if (_game.WinningLine is not null)
        {
            winningLine = new List<PositionResponse>(_game.WinningLine.Length);
            foreach (Position p in _game.WinningLine)
            {
                winningLine.Add(new PositionResponse(p.X, p.Y));
            }
        }

        // Clocks display live: the player on the move has been burning time
        // since the last move landed.
        long redTime = _redTimeMs;
        long blueTime = _blueTimeMs;
        if (!_game.IsGameOver)
        {
            long elapsed = ElapsedSinceLastMoveMs();
            if (_game.CurrentPlayer == Player.Red)
            {
                redTime = Math.Max(0, redTime - elapsed);
            }
            else if (_game.CurrentPlayer == Player.Blue)
            {
                blueTime = Math.Max(0, blueTime - elapsed);
            }
        }

        return new GameResponse
        {
            Board = cells,
            CurrentPlayer = _game.CurrentPlayer.ToName(),
            MoveNumber = _game.MoveNumber,
            IsGameOver = _game.IsGameOver,
            Winner = _game.Winner.ToName(),
            EndReason = _game.EndReason,
            WinningLine = winningLine,
            RedTimeRemaining = redTime / 1000.0,
            BlueTimeRemaining = blueTime / 1000.0,
            TimeControl = _game.TimeControl,
            InitialTime = (int)(_game.InitialTimeMs / 1000),
            Increment = _game.IncrementSeconds,
            GameMode = _game.GameMode.ToName(),
            RedDifficulty = _redDifficulty,
            BlueDifficulty = _blueDifficulty,
        };
    }
}
