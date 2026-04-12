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
public sealed partial class MovePicker
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
    /// Sort candidates by score (descending), maintaining category order.
    /// </summary>
    private void SortByScore()
    {
        int count = _candidates.Count;

        // Insertion sort in-place by (category ascending, score descending)
        for (int i = 1; i < count; i++)
        {
            int j = i;
            while (j > 0)
            {
                int catCompare = _categories[j].CompareTo(_categories[j - 1]);
                bool shouldSwap = catCompare < 0 ||
                    (catCompare == 0 && _scores[j] > _scores[j - 1]);

                if (!shouldSwap) break;

                (_candidates[j], _candidates[j - 1]) = (_candidates[j - 1], _candidates[j]);
                (_scores[j], _scores[j - 1]) = (_scores[j - 1], _scores[j]);
                (_categories[j], _categories[j - 1]) = (_categories[j - 1], _categories[j]);
                j--;
            }
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
