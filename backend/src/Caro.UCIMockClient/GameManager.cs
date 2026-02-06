using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;
using Caro.Core.GameLogic.UCI;

namespace Caro.UCIMockClient;

/// <summary>
/// Result of a single game.
/// </summary>
public record GameResult
{
    public int GameNumber { get; init; }
    public string RedBotName { get; init; } = string.Empty;
    public string BlueBotName { get; init; } = string.Empty;
    public Player Winner { get; init; }
    public int TotalMoves { get; init; }
    public long DurationMs { get; init; }
    public bool EndedByTimeout { get; init; }
    public List<MoveRecord> Moves { get; init; } = new();
    public string ResultString => Winner == Player.None ? "Draw" : $"{Winner} wins";
}

/// <summary>
/// Record of a single move with timing info.
/// </summary>
public record MoveRecord
{
    public int MoveNumber { get; init; }
    public Player Player { get; init; }
    public string UCIMove { get; init; } = string.Empty;
    public int X { get; init; }
    public int Y { get; init; }
    public long MoveTimeMs { get; init; }
    public long RemainingTimeRedMs { get; init; }
    public long RemainingTimeBlueMs { get; init; }
    public string Info { get; init; } = string.Empty;
}

/// <summary>
/// Manages matches between two UCI engines with proper time controls.
/// </summary>
public class GameManager
{
    private readonly UCIMockClient _redEngine;
    private readonly UCIMockClient _blueEngine;
    private readonly int _initialTimeSeconds;
    private readonly int _incrementSeconds;
    private readonly int _maxMoves;

    /// <summary>
    /// Create a new game manager.
    /// </summary>
    /// <param name="redEngine">Engine playing Red (White in UCI)</param>
    /// <param name="blueEngine">Engine playing Blue (Black in UCI)</param>
    /// <param name="initialTimeSeconds">Initial time per player in seconds</param>
    /// <param name="incrementSeconds">Increment per move in seconds</param>
    /// <param name="maxMoves">Maximum moves before draw</param>
    public GameManager(
        UCIMockClient redEngine,
        UCIMockClient blueEngine,
        int initialTimeSeconds = 180,
        int incrementSeconds = 2,
        int maxMoves = 361)
    {
        _redEngine = redEngine ?? throw new ArgumentNullException(nameof(redEngine));
        _blueEngine = blueEngine ?? throw new ArgumentNullException(nameof(blueEngine));
        _initialTimeSeconds = initialTimeSeconds;
        _incrementSeconds = incrementSeconds;
        _maxMoves = maxMoves;
    }

    /// <summary>
    /// Run a single game between the two engines.
    /// </summary>
    /// <param name="gameNumber">Game number for display</param>
    /// <param name="redBotName">Name of the Red bot</param>
    /// <param name="blueBotName">Name of the Blue bot</param>
    /// <param name="progress">Optional progress callback</param>
    /// <param name="logInfo">Optional info callback</param>
    /// <returns>Game result with all move information</returns>
    public async Task<GameResult> RunGameAsync(
        int gameNumber,
        string redBotName,
        string blueBotName,
        Action<MoveRecord>? progress = null,
        Action<string>? logInfo = null)
    {
        logInfo?.Invoke($"Game {gameNumber}: {redBotName} (Red) vs {blueBotName} (Blue)");

        // Initialize new game for both engines
        _redEngine.NewGame();
        _blueEngine.NewGame();

        var board = new Board();
        var moves = new List<MoveRecord>();
        var totalMoves = 0;
        var stopwatch = System.Diagnostics.Stopwatch.StartNew();

        // Time banks in milliseconds
        long redTimeRemainingMs = (long)_initialTimeSeconds * 1000;
        long blueTimeRemainingMs = (long)_initialTimeSeconds * 1000;
        long incrementMs = (long)_incrementSeconds * 1000;

        bool endedByTimeout = false;
        Player timeoutPlayer = Player.None;

        while (totalMoves < _maxMoves)
        {
            var currentPlayer = (totalMoves % 2) == 0 ? Player.Red : Player.Blue;
            var isRed = currentPlayer == Player.Red;
            var currentEngine = isRed ? _redEngine : _blueEngine;
            var opponentEngine = isRed ? _blueEngine : _redEngine;

            // Get move timing
            var moveStopwatch = System.Diagnostics.Stopwatch.StartNew();
            string uciMove;
            string lastInfo = "";

            try
            {
                // Subscribe to info messages during this move
                string? capturedInfo = null;
                currentEngine.OnInfo += info => capturedInfo = info;

                // Get the move (don't add to history yet - we'll sync both engines)
                uciMove = await currentEngine.GetMoveAsync(
                    redTimeRemainingMs,
                    blueTimeRemainingMs,
                    incrementMs,
                    incrementMs,
                    addToHistory: false  // We'll sync both engines manually
                );

                lastInfo = capturedInfo ?? "";
                currentEngine.OnInfo -= info => { };

                moveStopwatch.Stop();
            }
            catch (TimeoutException)
            {
                moveStopwatch.Stop();
                // Engine ran out of time - opponent wins
                endedByTimeout = true;
                timeoutPlayer = currentPlayer;
                break;
            }
            catch (Exception ex)
            {
                moveStopwatch.Stop();
                logInfo?.Invoke($"Error getting move from {currentPlayer}: {ex.Message}");
                // Treat as forfeit
                endedByTimeout = true;
                timeoutPlayer = currentPlayer;
                break;
            }

            var moveTimeMs = moveStopwatch.ElapsedMilliseconds;

            // Parse UCI move to coordinates
            Caro.Core.Domain.Entities.Position position;
            try
            {
                position = UCIMoveNotation.FromUCI(uciMove);
            }
            catch (ArgumentException ex)
            {
                logInfo?.Invoke($"Invalid UCI move '{uciMove}' from {currentPlayer}: {ex.Message}");
                // Invalid move = forfeit
                endedByTimeout = true;
                timeoutPlayer = currentPlayer;
                break;
            }

            // Validate move is on empty cell
            if (!board.IsEmpty(position.X, position.Y))
            {
                logInfo?.Invoke($"Illegal move: {currentPlayer} attempted to play on occupied cell ({position.X}, {position.Y})");
                // Illegal move = forfeit
                endedByTimeout = true;
                timeoutPlayer = currentPlayer;
                break;
            }

            // Update time bank
            if (isRed)
            {
                redTimeRemainingMs -= moveTimeMs;
                redTimeRemainingMs += incrementMs;
                if (redTimeRemainingMs < 0)
                {
                    endedByTimeout = true;
                    timeoutPlayer = Player.Red;
                    break;
                }
            }
            else
            {
                blueTimeRemainingMs -= moveTimeMs;
                blueTimeRemainingMs += incrementMs;
                if (blueTimeRemainingMs < 0)
                {
                    endedByTimeout = true;
                    timeoutPlayer = Player.Blue;
                    break;
                }
            }

            // Make the move
            board = board.PlaceStone(position.X, position.Y, currentPlayer);

            var moveRecord = new MoveRecord
            {
                MoveNumber = totalMoves + 1,
                Player = currentPlayer,
                UCIMove = uciMove,
                X = position.X,
                Y = position.Y,
                MoveTimeMs = moveTimeMs,
                RemainingTimeRedMs = redTimeRemainingMs,
                RemainingTimeBlueMs = blueTimeRemainingMs,
                Info = lastInfo
            };
            moves.Add(moveRecord);

            // Sync the move to both engines so they have the same game state
            _redEngine.AddMove(uciMove);
            _blueEngine.AddMove(uciMove);

            progress?.Invoke(moveRecord);

            // Check for win
            var detector = new WinDetector();
            var result = detector.CheckWin(board);
            if (result.HasWinner)
            {
                stopwatch.Stop();
                logInfo?.Invoke($"Game {gameNumber}: {result.Winner} wins at move {totalMoves + 1}");
                return new GameResult
                {
                    GameNumber = gameNumber,
                    RedBotName = redBotName,
                    BlueBotName = blueBotName,
                    Winner = result.Winner,
                    TotalMoves = totalMoves + 1,
                    DurationMs = stopwatch.ElapsedMilliseconds,
                    EndedByTimeout = false,
                    Moves = moves
                };
            }

            totalMoves++;
        }

        stopwatch.Stop();

        // Determine final result
        Player winner;
        if (endedByTimeout)
        {
            winner = timeoutPlayer == Player.Red ? Player.Blue : Player.Red;
            logInfo?.Invoke($"Game {gameNumber}: {winner} wins by timeout ( opponent ran out of time)");
        }
        else
        {
            winner = Player.None; // Draw
            logInfo?.Invoke($"Game {gameNumber}: Draw after {totalMoves} moves");
        }

        return new GameResult
        {
            GameNumber = gameNumber,
            RedBotName = redBotName,
            BlueBotName = blueBotName,
            Winner = winner,
            TotalMoves = totalMoves,
            DurationMs = stopwatch.ElapsedMilliseconds,
            EndedByTimeout = endedByTimeout,
            Moves = moves
        };
    }

    /// <summary>
    /// Run a series of games with alternating colors.
    /// </summary>
    /// <param name="botA">First bot engine</param>
    /// <param name="botB">Second bot engine</param>
    /// <param name="botAName">Name of bot A</param>
    /// <param name="botBName">Name of bot B</param>
    /// <param name="totalGames">Total number of games to play</param>
    /// <param name="progress">Progress callback</param>
    /// <param name="logInfo">Info callback</param>
    /// <returns>List of game results</returns>
    public static async Task<List<GameResult>> RunMatchAsync(
        UCIMockClient botA,
        UCIMockClient botB,
        string botAName,
        string botBName,
        int totalGames = 4,
        Action<GameResult>? progress = null,
        Action<string>? logInfo = null)
    {
        var results = new List<GameResult>();

        for (int i = 0; i < totalGames; i++)
        {
            // Alternate colors: Games 0,1 have A as Red; Games 2,3 have B as Red
            var botAIsRed = i < 2;
            
            UCIMockClient redEngine = botAIsRed ? botA : botB;
            UCIMockClient blueEngine = botAIsRed ? botB : botA;
            string redBotName = botAIsRed ? botAName : botBName;
            string blueBotName = botAIsRed ? botBName : botAName;

            var manager = new GameManager(redEngine, blueEngine);
            var result = await manager.RunGameAsync(
                i + 1,
                redBotName,
                blueBotName,
                logInfo: logInfo
            );

            results.Add(result);
            progress?.Invoke(result);
        }

        return results;
    }
}
