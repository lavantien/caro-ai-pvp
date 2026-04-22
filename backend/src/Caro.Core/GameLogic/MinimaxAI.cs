using System.Runtime.CompilerServices;
using System.Diagnostics;
using System.Threading.Channels;
using Caro.Core.Domain.Configuration;
using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic.TimeManagement;
using Caro.Core.GameLogic.Pondering;
using Caro.Core.GameLogic.Search;
using Microsoft.Extensions.Logging;
using SHC = Caro.Core.Domain.Configuration.SearchHeuristicConstants;

namespace Caro.Core.GameLogic;

/// <summary>
/// AI opponent using Minimax algorithm with alpha-beta pruning and advanced optimizations
/// Optimizations: Transposition Table, Killer Heuristic, History Heuristic, Improved Move Ordering, Iterative Deepening, VCF Solver
/// Parallel Search: Lazy SMP with conservative thread count (processorCount/2)-1
/// Time management: Intelligent time allocation optimized for 7+5 time control
/// Pondering: Constant pondering during opponent's turn
/// Stats: Publisher-subscriber pattern for real-time stats reporting
/// </summary>
public partial class MinimaxAI : IStatsPublisher, IDisposable
{
    private readonly BoardEvaluator _evaluator = new();
    private TranspositionTable _transpositionTable;
    private readonly WinDetector _winDetector = new();
    private readonly ThreatDetector _threatDetector = new();
    private readonly ThreatSpaceSearch _vcfSolver = new();
    private readonly VCFSolver _inTreeVCFSolver;  // In-tree VCF solver for Lazy SMP

    // Time management for 7+5 time control
    private readonly TimeManager _timeManager = new();

    // Adaptive time management using PID-like controller
    private readonly AdaptiveTimeManager _adaptiveTimeManager = new();

    // Track initial time for adaptive depth thresholds
    // -1 means "unknown, will infer from first move"
    private long _inferredInitialTimeMs = -1;

    // Track thread count used for last search (for diagnostics)
    private int _lastThreadCount = 1;

    // Track parallel diagnostics from last search
    private string? _lastParallelDiagnostics = null;

    // Parallel search (Lazy SMP provides 4-8x speedup on multi-core systems)
    private ParallelMinimaxSearch _parallelSearch;

    // Search radius around existing stones from centralized config
    private const int MaxSearchRadius = SearchConstants.MaxSearchRadius;

    // Board size constant for array sizing and bounds checking
    private const int BoardSize = GameConstants.BoardSize;

    // Search heuristics: killer moves, history tables, butterfly tables
    private readonly SearchHeuristics _heuristics = new();
    private readonly MoveOrderer _moveOrderer;

    // Track transposition table hits for debugging
    private int _tableHits;
    private int _tableLookups;

    // Track search statistics for last move
    private long _nodesSearched;
    private int _depthAchieved;
    private int _vcfNodesSearched;
    private int _vcfDepthAchieved;
    private readonly Stopwatch _searchStopwatch = new();
    private long _lastAllocatedTimeMs;  // Track time allocated for last move
    private bool _lastPonderingEnabled;  // Track if pondering was enabled for last move
    private MoveType _moveType;  // How the last move was determined
    private int _lastSearchScore;  // Score from last search
    private double _lastFmcPercent;  // First Move Cutoff % from last search
    private double _lastEbf;  // Effective Branching Factor from last search

    // Time control for search timeout
    private long _searchHardBoundMs;
    // Check time at regular intervals (power of 2 for efficient masking)
    // 16 = check every ~16 nodes. At 1M nodes/sec, this checks every ~16us
    // Frequent checking allows fine-grained timeout control
    private const int TimeCheckInterval = SearchConstants.TimeCheckInterval;
    private bool _searchStopped;

    // Pondering (thinking on opponent's time)
    private readonly Ponderer _ponderer = new();
    private PV _lastPV = PV.Empty;
    private Board? _lastBoard;
    private Player _lastPlayer;

    // Stats publisher-subscriber pattern
    private static int _instanceCounter = 0;
    private readonly string _publisherId;
    private readonly Channel<MoveStatsEvent> _statsChannel;
    public Channel<MoveStatsEvent> StatsChannel => _statsChannel;
    public string PublisherId => _publisherId;

    // Optional logger for diagnostics
    private readonly ILogger<MinimaxAI> _logger;

    // Random source for tie-breaking and error rate simulation (injectable for deterministic tests)
    private readonly Random? _random;

    // Optional parameter provider for SPSA tuning (null = use default constants)
    private readonly IEvaluationParameterProvider? _parameterProvider;

    // Mutable SearchBoard for high-performance search (make/unmake pattern)
    // Reused across searches to avoid allocations
    private readonly SearchBoard _searchBoard = new();

    public MinimaxAI(int ttSizeMb = TimeConstants.DefaultHashSizeMb, ILogger<MinimaxAI>? logger = null, Random? random = null, IEvaluationParameterProvider? parameterProvider = null)
    {
        _logger = logger ?? Microsoft.Extensions.Logging.Abstractions.NullLogger<MinimaxAI>.Instance;
        _random = random;  // null means use Random.Shared (default behavior)
        _parameterProvider = parameterProvider;  // null = use default evaluation constants
        _publisherId = Interlocked.Increment(ref _instanceCounter).ToString();
        _statsChannel = Channel.CreateUnbounded<MoveStatsEvent>();

        // Initialize with passed size parameter
        _transpositionTable = new TranspositionTable(ttSizeMb);
        _parallelSearch = new ParallelMinimaxSearch(ttSizeMb);

        _inTreeVCFSolver = new VCFSolver(_vcfSolver);
        _moveOrderer = new MoveOrderer(_heuristics);
    }

    /// <summary>
    /// Evaluate board position using custom parameters if provider is set.
    /// Falls back to default evaluation if no parameter provider is configured.
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    private int EvaluateBoard(SearchBoard board, Player player)
    {
        if (_parameterProvider != null)
        {
            return _evaluator.Evaluate(board, player, _parameterProvider.GetParameters());
        }
        return _evaluator.Evaluate(board, player);
    }

    /// <summary>
    /// Evaluate immutable Board position using custom parameters if provider is set.
    /// Falls back to default evaluation if no parameter provider is configured.
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    private int EvaluateBoard(Board board, Player player)
    {
        if (_parameterProvider != null)
        {
            return _evaluator.Evaluate(board, player, _parameterProvider.GetParameters());
        }
        return _evaluator.Evaluate(board, player);
    }

    // Helper methods for random operations (uses injected Random or Random.Shared)
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    private double NextRandomDouble() => _random?.NextDouble() ?? Random.Shared.NextDouble();

    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    private int NextRandomInt(int maxValue) => _random?.Next(maxValue) ?? Random.Shared.Next(maxValue);


}
