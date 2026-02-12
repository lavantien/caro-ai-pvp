using System.Runtime.CompilerServices;
using Caro.Core.Domain.Configuration;
using Caro.Core.Domain.Entities;

namespace Caro.Core.GameLogic;

/// <summary>
/// Staged move picker for Caro Gomoku with threat-based ordering.
/// Adapts Stockfish's staged approach to Caro's threat hierarchy.
/// 
/// Stage order (highest to lowest priority):
/// 1. TT_MOVE - Transposition table move (already proven good)
/// 2. MUST_BLOCK - Mandatory blocks (opponent's open four/five threat)
/// 3. WINNING_MOVE - Creates winning position (open four, double threat)
/// 4. THREAT_CREATE - Creates threats (open three, broken four)
/// 5. KILLER_COUNTER - Killer moves and counter-move history
/// 6. GOOD_QUIET - High history scores
/// 7. BAD_QUIET - Remaining moves
/// </summary>
public sealed class MovePicker
{
    /// <summary>
    /// Move picker stages ordered by priority.
    /// Each stage generates moves of a specific category.
    /// </summary>
    public enum Stage : byte
    {
        /// <summary>Not started</summary>
        None = 0,
        /// <summary>Transposition table move</summary>
        TT_MOVE = 1,
        /// <summary>Must block opponent's winning threat</summary>
        MUST_BLOCK = 2,
        /// <summary>Creates winning position (open four, double threat)</summary>
        WINNING_MOVE = 3,
        /// <summary>Creates threat (open three)</summary>
        THREAT_CREATE = 4,
        /// <summary>Killer moves and counter-move responses</summary>
        KILLER_COUNTER = 5,
        /// <summary>High history quiet moves</summary>
        GOOD_QUIET = 6,
        /// <summary>Remaining quiet moves</summary>
        BAD_QUIET = 7,
        /// <summary>All moves exhausted</summary>
        Done = 8
    }

    // Import scoring constants from centralized configuration
    private const int GoodQuietThreshold = MoveOrderingConstants.GoodQuietThreshold;
    private const int MustBlockScore = MoveOrderingConstants.MustBlockScore;
    private const int WinningMoveScore = MoveOrderingConstants.WinningMoveScore;
    private const int ThreatCreateScore = MoveOrderingConstants.ThreatCreateScore;
    private const int TtMoveScore = MoveOrderingConstants.TtMoveScore;
    private const int KillerScore1 = MoveOrderingConstants.KillerScore1;
    private const int KillerScore2 = MoveOrderingConstants.KillerScore2;
    private const int CounterMoveScore = MoveOrderingConstants.CounterMoveScore;
    private const int ContinuationScoreMax = MoveOrderingConstants.ContinuationScoreMax;
    private const int HistoryScoreMax = MoveOrderingConstants.HistoryScoreMax;

    // Picker state
    private readonly List<(int x, int y)> _candidates;
    private readonly Board _board;
    private readonly Player _player;
    private readonly int _depth;
    private readonly (int x, int y)? _ttMove;
    private readonly ThreadData _threadData;
    private readonly ContinuationHistory _continuationHistory;
    private readonly CounterMoveHistory _counterMoveHistory;
    private readonly ThreatDetector _threatDetector;

    // Pre-computed scores for all candidates
    private readonly int[] _scores;
    private readonly MoveCategory[] _categories;

    // Current stage and index within stage
    private Stage _currentStage;
    private int _currentStageIndex;
    private int _stageStartIndex;
    private int _stageEndIndex;

    // Cached threat moves
    private List<(int x, int y)>? _mustBlockMoves;
    private List<(int x, int y)>? _winningMoves;
    private List<(int x, int y)>? _threatMoves;

    /// <summary>
    /// Thread data for move picker (killer moves, history tables).
    /// </summary>
    public sealed class ThreadData
    {
        public int ThreadIndex;
        public (int x, int y)[,] KillerMoves = new (int x, int y)[20, 2];
        public int[,] HistoryRed = new int[BitBoard.Size, BitBoard.Size];
        public int[,] HistoryBlue = new int[BitBoard.Size, BitBoard.Size];
        public int[] MoveHistory = new int[ContinuationHistory.TrackedPlyCount];
        public int MoveHistoryCount;
        public int LastOpponentCell = -1;
    }

    /// <summary>
    /// Category classification for each move.
    /// </summary>
    private enum MoveCategory : byte
    {
        None = 0,
        TtMove = 1,
        MustBlock = 2,
        Winning = 3,
        ThreatCreate = 4,
        KillerCounter = 5,
        GoodQuiet = 6,
        BadQuiet = 7
    }

    /// <summary>
    /// Create a new move picker.
    /// </summary>
    public MovePicker(
        List<(int x, int y)> candidates,
        Board board,
        Player player,
        int depth,
        (int x, int y)? ttMove,
        ThreadData threadData,
        ContinuationHistory continuationHistory,
        CounterMoveHistory counterMoveHistory)
    {
        _candidates = candidates;
        _board = board;
        _player = player;
        _depth = depth;
        _ttMove = ttMove;
        _threadData = threadData;
        _continuationHistory = continuationHistory;
        _counterMoveHistory = counterMoveHistory;
        _threatDetector = new ThreatDetector();

        _scores = new int[candidates.Count];
        _categories = new MoveCategory[candidates.Count];

        _currentStage = Stage.None;
        _currentStageIndex = 0;
        _stageStartIndex = 0;
        _stageEndIndex = 0;

        // Pre-compute all scores and categories
        ComputeAllScores();
        SortByScore();
    }

    /// <summary>
    /// Get the next move to search. Returns null when all moves exhausted.
    /// Automatically advances through stages.
    /// </summary>
    public (int x, int y)? NextMove()
    {
        while (_currentStage != Stage.Done)
        {
            // Advance to next stage if current stage exhausted
            if (_currentStage == Stage.None || _currentStageIndex >= _stageEndIndex)
            {
                if (!AdvanceStage())
                    return null;
            }

            // Return next move from current stage
            if (_currentStageIndex < _stageEndIndex)
            {
                var move = _candidates[_currentStageIndex];
                _currentStageIndex++;
                return move;
            }
        }

        return null;
    }

    /// <summary>
    /// Get all remaining moves (for bulk operations).
    /// </summary>
    public List<(int x, int y)> GetRemainingMoves()
    {
        var result = new List<(int x, int y)>();
        (int x, int y)? move;
        while ((move = NextMove()) != null)
        {
            result.Add(move.Value);
        }
        return result;
    }

    /// <summary>
    /// Get current stage (for diagnostics).
    /// </summary>
    public Stage CurrentStage => _currentStage;

    /// <summary>
    /// Advance to the next non-empty stage.
    /// Returns false if all stages exhausted.
    /// </summary>
    private bool AdvanceStage()
    {
        while (true)
        {
            _currentStage = _currentStage switch
            {
                Stage.None => Stage.TT_MOVE,
                Stage.TT_MOVE => Stage.MUST_BLOCK,
                Stage.MUST_BLOCK => Stage.WINNING_MOVE,
                Stage.WINNING_MOVE => Stage.THREAT_CREATE,
                Stage.THREAT_CREATE => Stage.KILLER_COUNTER,
                Stage.KILLER_COUNTER => Stage.GOOD_QUIET,
                Stage.GOOD_QUIET => Stage.BAD_QUIET,
                Stage.BAD_QUIET => Stage.Done,
                _ => Stage.Done
            };

            if (_currentStage == Stage.Done)
                return false;

            // Find the range of moves in this stage
            _stageStartIndex = _currentStageIndex;
            _stageEndIndex = FindStageEnd(_currentStage);

            if (_stageEndIndex > _stageStartIndex)
                return true;

            // Empty stage, try next
            _currentStageIndex = _stageEndIndex;
        }
    }

    /// <summary>
    /// Find the end index of moves in the given stage.
    /// Assumes array is sorted by category then score.
    /// </summary>
    private int FindStageEnd(Stage stage)
    {
        MoveCategory targetCategory = stage switch
        {
            Stage.TT_MOVE => MoveCategory.TtMove,
            Stage.MUST_BLOCK => MoveCategory.MustBlock,
            Stage.WINNING_MOVE => MoveCategory.Winning,
            Stage.THREAT_CREATE => MoveCategory.ThreatCreate,
            Stage.KILLER_COUNTER => MoveCategory.KillerCounter,
            Stage.GOOD_QUIET => MoveCategory.GoodQuiet,
            Stage.BAD_QUIET => MoveCategory.BadQuiet,
            _ => MoveCategory.None
        };

        // Binary search for end of this category
        for (int i = _stageStartIndex; i < _candidates.Count; i++)
        {
            if (_categories[i] != targetCategory)
                return i;
        }

        return _candidates.Count;
    }

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
            // Straight four and broken four require immediate response
            if (threat.Type == ThreatType.StraightFour || threat.Type == ThreatType.BrokenFour)
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

        for (int x = 0; x < BitBoard.Size; x++)
        {
            for (int y = 0; y < BitBoard.Size; y++)
            {
                if (!board.GetCell(x, y).IsEmpty)
                    continue;

                // Simulate placing a stone
                var testBoard = board.PlaceStone(x, y, player);
                var pattern = Pattern4Evaluator.EvaluatePosition(testBoard, x, y, player);

                // Open four, double flex3, or flex4+flex3 are winning
                if (pattern == Pattern4Evaluator.CaroPattern4.Flex4 ||
                    pattern == Pattern4Evaluator.CaroPattern4.DoubleFlex3 ||
                    pattern == Pattern4Evaluator.CaroPattern4.Flex4Flex3 ||
                    pattern == Pattern4Evaluator.CaroPattern4.Exactly5)
                {
                    winningMoves.Add((x, y));
                }
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

        for (int x = 0; x < BitBoard.Size; x++)
        {
            for (int y = 0; y < BitBoard.Size; y++)
            {
                if (!board.GetCell(x, y).IsEmpty)
                    continue;

                // Simulate placing a stone
                var testBoard = board.PlaceStone(x, y, player);
                var pattern = Pattern4Evaluator.EvaluatePosition(testBoard, x, y, player);

                // Open three or better (but not already in winning moves)
                if (pattern >= Pattern4Evaluator.CaroPattern4.Flex3 &&
                    pattern != Pattern4Evaluator.CaroPattern4.Flex4 &&
                    pattern != Pattern4Evaluator.CaroPattern4.DoubleFlex3 &&
                    pattern != Pattern4Evaluator.CaroPattern4.Flex4Flex3 &&
                    pattern != Pattern4Evaluator.CaroPattern4.Exactly5)
                {
                    threatMoves.Add((x, y));
                }
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

    /// <summary>
    /// Sort candidates by score (descending), maintaining category order.
    /// </summary>
    private void SortByScore()
    {
        // Create index array for sorting
        var indices = new int[_candidates.Count];
        for (int i = 0; i < indices.Length; i++)
            indices[i] = i;

        // Sort by category first, then by score within category
        Array.Sort(indices, (a, b) =>
        {
            int catCompare = _categories[a].CompareTo(_categories[b]);
            if (catCompare != 0) return catCompare;

            return _scores[b].CompareTo(_scores[a]); // Descending by score
        });

        // Reorder arrays based on sorted indices
        var newCandidates = new List<(int x, int y)>(_candidates.Count);
        var newScores = new int[_candidates.Count];
        var newCategories = new MoveCategory[_candidates.Count];

        for (int i = 0; i < indices.Length; i++)
        {
            newCandidates.Add(_candidates[indices[i]]);
            newScores[i] = _scores[indices[i]];
            newCategories[i] = _categories[indices[i]];
        }

        // Update arrays
        for (int i = 0; i < _candidates.Count; i++)
        {
            _candidates[i] = newCandidates[i];
            _scores[i] = newScores[i];
            _categories[i] = newCategories[i];
        }
    }

    /// <summary>
    /// Get move score for diagnostics.
    /// </summary>
    public int GetMoveScore(int index)
    {
        if (index < 0 || index >= _scores.Length)
            return 0;
        return _scores[index];
    }

    /// <summary>
    /// Get move category for diagnostics.
    /// </summary>
    public Stage GetMoveStage(int index)
    {
        if (index < 0 || index >= _categories.Length)
            return Stage.None;
        return _categories[index] switch
        {
            MoveCategory.TtMove => Stage.TT_MOVE,
            MoveCategory.MustBlock => Stage.MUST_BLOCK,
            MoveCategory.Winning => Stage.WINNING_MOVE,
            MoveCategory.ThreatCreate => Stage.THREAT_CREATE,
            MoveCategory.KillerCounter => Stage.KILLER_COUNTER,
            MoveCategory.GoodQuiet => Stage.GOOD_QUIET,
            MoveCategory.BadQuiet => Stage.BAD_QUIET,
            _ => Stage.None
        };
    }
}
