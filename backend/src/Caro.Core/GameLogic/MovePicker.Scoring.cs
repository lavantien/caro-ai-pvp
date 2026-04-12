using System.Runtime.CompilerServices;
using Caro.Core.Domain.Entities;

namespace Caro.Core.GameLogic;

/// <summary>
/// Scoring methods for MovePicker.
/// </summary>
public sealed partial class MovePicker
{
    /// <summary>
    /// Compute scores and categories for all candidate moves.
    /// </summary>
    private void ComputeAllScores()
    {
        var opponent = _player == Player.Red ? Player.Blue : Player.Red;
        var historyTable = _player == Player.Red ? _threadData.HistoryRed : _threadData.HistoryBlue;

        // Get threat moves for blocking
        _mustBlockMoves = GetMustBlockMoves(_board, opponent);

        // Get winning moves for current player
        _winningMoves = GetWinningMoves(_board, _player);

        // Get threat-creating moves
        _threatMoves = GetThreatCreateMoves(_board, _player);

        for (int i = 0; i < _candidates.Count; i++)
        {
            var (x, y) = _candidates[i];
            int score = 0;
            var category = MoveCategory.BadQuiet;

            // 1. TT Move (highest priority if matches)
            if (_ttMove.HasValue && _ttMove.Value == (x, y))
            {
                score = TtMoveScore;
                category = MoveCategory.TtMove;
            }
            // 2. Must Block (opponent's winning threat)
            else if (_mustBlockMoves.Contains((x, y)))
            {
                score = MustBlockScore + GetSecondaryScore(x, y, historyTable);
                category = MoveCategory.MustBlock;
            }
            // 3. Winning Move (creates open four or double threat)
            else if (_winningMoves.Contains((x, y)))
            {
                score = WinningMoveScore + GetSecondaryScore(x, y, historyTable);
                category = MoveCategory.Winning;
            }
            // 4. Threat Create (creates open three)
            else if (_threatMoves.Contains((x, y)))
            {
                score = ThreatCreateScore + GetSecondaryScore(x, y, historyTable);
                category = MoveCategory.ThreatCreate;
            }
            // 5. Killer / Counter Move
            else if (IsKillerOrCounter(x, y, out int killerScore))
            {
                score = killerScore + GetSecondaryScore(x, y, historyTable);
                category = MoveCategory.KillerCounter;
            }
            // 6-7. Quiet moves (good or bad based on history)
            else
            {
                score = GetQuietScore(x, y, historyTable);
                category = score >= GoodQuietThreshold ? MoveCategory.GoodQuiet : MoveCategory.BadQuiet;
            }

            _scores[i] = score;
            _categories[i] = category;
        }
    }

    /// <summary>
    /// Get must-block moves (opponent's winning threats).
    /// </summary>
    private List<(int x, int y)> GetMustBlockMoves(Board board, Player opponent)
    {
        var blocks = new List<(int x, int y)>();
        var threats = _threatDetector.DetectThreats(board, opponent);

        foreach (var threat in threats)
        {
            // FIX: Include StraightThree in must-block moves
            // Open threes (StraightThree) must be blocked early or they become winning threats
            // Previously only StraightFour and BrokenFour were blocked, allowing threes to grow
            if (threat.Type == ThreatType.StraightFour ||
                threat.Type == ThreatType.BrokenFour ||
                threat.Type == ThreatType.StraightThree)
            {
                blocks.AddRange(threat.GainSquares);
            }
        }

        return blocks.Distinct().ToList();
    }

    /// <summary>
    /// Get winning moves (creates open four or double threat).
    /// </summary>
    private List<(int x, int y)> GetWinningMoves(Board board, Player player)
    {
        var winningMoves = new List<(int x, int y)>();

        for (int i = 0; i < _candidates.Count; i++)
        {
            var (x, y) = _candidates[i];

            var testBoard = board.PlaceStone(x, y, player);
            var pattern = Pattern4Evaluator.EvaluatePosition(testBoard, x, y, player);

            if (pattern == Pattern4Evaluator.CaroPattern4.Flex4 ||
                pattern == Pattern4Evaluator.CaroPattern4.DoubleFlex3 ||
                pattern == Pattern4Evaluator.CaroPattern4.Flex4Flex3 ||
                pattern == Pattern4Evaluator.CaroPattern4.Exactly5)
            {
                winningMoves.Add((x, y));
            }
        }

        return winningMoves;
    }

    /// <summary>
    /// Get threat-creating moves (creates open three or better).
    /// </summary>
    private List<(int x, int y)> GetThreatCreateMoves(Board board, Player player)
    {
        var threatMoves = new List<(int x, int y)>();

        for (int i = 0; i < _candidates.Count; i++)
        {
            var (x, y) = _candidates[i];

            var testBoard = board.PlaceStone(x, y, player);
            var pattern = Pattern4Evaluator.EvaluatePosition(testBoard, x, y, player);

            if (pattern >= Pattern4Evaluator.CaroPattern4.Flex3 &&
                pattern != Pattern4Evaluator.CaroPattern4.Flex4 &&
                pattern != Pattern4Evaluator.CaroPattern4.DoubleFlex3 &&
                pattern != Pattern4Evaluator.CaroPattern4.Flex4Flex3 &&
                pattern != Pattern4Evaluator.CaroPattern4.Exactly5)
            {
                threatMoves.Add((x, y));
            }
        }

        return threatMoves;
    }

    /// <summary>
    /// Check if move is a killer or counter-move.
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    private bool IsKillerOrCounter(int x, int y, out int score)
    {
        score = 0;
        bool found = false;

        // Check killer moves
        if (_depth < 20)
        {
            if (_threadData.KillerMoves[_depth, 0] == (x, y))
            {
                score = KillerScore1;
                found = true;
            }
            else if (_threadData.KillerMoves[_depth, 1] == (x, y))
            {
                score = KillerScore2;
                found = true;
            }
        }

        // Check counter-move history
        int currentCell = y * BitBoard.Size + x;
        int counterScore = _counterMoveHistory.GetScore(_player, _threadData.LastOpponentCell, currentCell);
        if (counterScore > 0)
        {
            int adjustedCounterScore = Math.Min(counterScore * 2, CounterMoveScore);
            if (adjustedCounterScore > score)
            {
                score = adjustedCounterScore;
                found = true;
            }
        }

        return found;
    }

    /// <summary>
    /// Get secondary score components (continuation history, proximity).
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    private int GetSecondaryScore(int x, int y, int[,] historyTable)
    {
        int score = 0;
        int currentCell = y * BitBoard.Size + x;

        // Continuation history
        int continuationScore = 0;
        for (int j = 0; j < _threadData.MoveHistoryCount && j < ContinuationHistory.TrackedPlyCount; j++)
        {
            int prevCell = _threadData.MoveHistory[j];
            continuationScore += _continuationHistory.GetScore(_player, prevCell, currentCell);
        }
        score += Math.Min(continuationScore * 3, ContinuationScoreMax);

        // History heuristic
        score += Math.Min(historyTable[x, y] * 2, HistoryScoreMax);

        // Center preference
        int center = _board.BoardSize / 2;
        int centerDist = Math.Abs(x - center) + Math.Abs(y - center);
        score += ((_board.BoardSize * 2 - 4) - centerDist) * 100;

        return score;
    }

    /// <summary>
    /// Get full quiet move score (without threat bonuses).
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    private int GetQuietScore(int x, int y, int[,] historyTable)
    {
        int score = GetSecondaryScore(x, y, historyTable);

        // Counter-move history for quiet moves
        int currentCell = y * BitBoard.Size + x;
        int counterScore = _counterMoveHistory.GetScore(_player, _threadData.LastOpponentCell, currentCell);
        score += Math.Min(counterScore * 2, CounterMoveScore);

        return score;
    }
}
