using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;

namespace Caro.Core.Tournament;

/// <summary>
/// Interface for strongly-typed SignalR client methods
/// This provides compile-time safety for client method calls
/// </summary>
public interface ITournamentClient
{
    /// <summary>
    /// Called when a new game starts
    /// </summary>
    Task OnGameStarted(string gameId, string redBot, string blueBot, AIDifficulty redDiff, AIDifficulty blueDiff);

    /// <summary>
    /// Called when the current board state should be updated with a new move
    /// This is the authoritative board update - includes both board state and move info
    /// </summary>
    Task OnBoardUpdate(string gameId, List<BoardCell> board, int moveNumber, string player, int x, int y);

    /// <summary>
    /// Called with engine stats after a move completes
    /// </summary>
    Task OnMovePlayed(MoveEvent moveEvent);

    /// <summary>
    /// Called with real-time thinking stats during AI search (periodic updates)
    /// </summary>
    Task OnThinkingStats(ThinkingStatsEvent statsEvent);

    /// <summary>
    /// Called when a player starts or stops pondering
    /// </summary>
    Task OnPonderingStatus(PonderingStatusEvent statusEvent);

    /// <summary>
    /// Called when a game finishes
    /// </summary>
    Task OnGameFinished(GameFinishedEvent gameFinished);

    /// <summary>
    /// Called with tournament progress updates
    /// </summary>
    Task OnTournamentProgress(int completed, int total, double percent, string currentMatch);

    /// <summary>
    /// Called when the tournament completes
    /// </summary>
    Task OnTournamentCompleted(List<AIBot> finalStandings, int totalGames, long durationMs);

    /// <summary>
    /// Called when tournament status changes
    /// </summary>
    Task OnTournamentStatusChanged(TournamentStatus status, string message);

    /// <summary>
    /// Called when ELO ratings are updated after a game
    /// </summary>
    Task OnELOUpdated(List<AIBot> bots);

    /// <summary>
    /// Called with structured game log entries for debugging
    /// </summary>
    Task OnGameLog(GameLogEvent logEntry);
}

/// <summary>
/// Tournament status enumeration
/// </summary>
public enum TournamentStatus
{
    Idle,
    Running,
    Paused,
    Completed
}

/// <summary>
/// Current state of the tournament
/// </summary>
public class TournamentState
{
    public TournamentStatus Status { get; set; } = TournamentStatus.Idle;
    public int CompletedGames { get; set; }
    public int TotalGames { get; set; }
    public double ProgressPercent => TotalGames > 0 ? (double)CompletedGames / TotalGames * 100 : 0;
    public List<AIBot> Bots { get; set; } = new();
    public List<MatchResult> MatchHistory { get; set; } = new();
    public CurrentMatchInfo? CurrentMatch { get; set; }
    public DateTime StartTimeUtc { get; set; }
    public DateTime? EndTimeUtc { get; set; }
    public TimeSpan Elapsed => Status == TournamentStatus.Completed && EndTimeUtc.HasValue
        ? EndTimeUtc.Value - StartTimeUtc
        : DateTime.UtcNow - StartTimeUtc;
}

/// <summary>
/// Information about the current match being played
/// </summary>
public class CurrentMatchInfo
{
    public string GameId { get; set; } = string.Empty;
    public string RedBotName { get; set; } = string.Empty;
    public string BlueBotName { get; set; } = string.Empty;
    public AIDifficulty RedDifficulty { get; set; }
    public AIDifficulty BlueDifficulty { get; set; }
    public int MoveNumber { get; set; }
    public List<BoardCell> Board { get; set; } = new();
    public long RedTimeRemainingMs { get; set; }
    public long BlueTimeRemainingMs { get; set; }
    public int InitialTimeSeconds { get; set; } = 420;
    public int IncrementSeconds { get; set; } = 5;
}

/// <summary>
/// Board cell for serialization
/// </summary>
public class BoardCell
{
    public int X { get; set; }
    public int Y { get; set; }
    public string Player { get; set; } = "none";
}

/// <summary>
/// Move event for real-time updates
/// </summary>
public class MoveEvent
{
    public string GameId { get; set; } = string.Empty;
    public int X { get; set; }
    public int Y { get; set; }
    public string Player { get; set; } = string.Empty;
    public int MoveNumber { get; set; }
    public long RedTimeRemainingMs { get; set; }
    public long BlueTimeRemainingMs { get; set; }

    // Engine statistics
    public int DepthAchieved { get; set; }
    public long NodesSearched { get; set; }
    public double NodesPerSecond { get; set; }
    public double TableHitRate { get; set; }
    public bool PonderingActive { get; set; }
    public int VCFDepthAchieved { get; set; }
    public long VCFNodesSearched { get; set; }
}

/// <summary>
/// Engine statistics for a single move
/// </summary>
public record MoveStats(
    int DepthAchieved,
    long NodesSearched,
    double NodesPerSecond,
    double TableHitRate,
    bool PonderingActive,
    int VCFDepthAchieved,
    long VCFNodesSearched,
    int ThreadCount,
    string? ParallelDiagnostics = null,
    long MoveTimeMs = 0,
    double MasterTTPercent = 0,     // % of TT reads from master thread
    double HelperAvgDepth = 0,      // Average depth achieved by helper threads
    long AllocatedTimeMs = 0,       // Time allocated for this move
    long PonderNodesSearched = 0,   // Nodes searched during pondering (opponent's turn)
    double PonderNodesPerSecond = 0, // NPS during pondering
    int PonderDepth = 0             // Depth achieved during pondering
);

/// <summary>
/// Game finished event
/// </summary>
public class GameFinishedEvent
{
    public string GameId { get; set; } = string.Empty;
    public string Winner { get; set; } = string.Empty;
    public string Loser { get; set; } = string.Empty;
    public bool IsDraw { get; set; }
    public bool EndedByTimeout { get; set; }
    public int TotalMoves { get; set; }
    public long DurationMs { get; set; }
    public List<AIBot> UpdatedBots { get; set; } = new();
}

/// <summary>
/// Real-time thinking stats event (sent periodically during AI search)
/// </summary>
public class ThinkingStatsEvent
{
    public string GameId { get; set; } = string.Empty;
    public string Player { get; set; } = string.Empty;  // "red" or "blue"
    public int CurrentDepth { get; set; }
    public long NodesSearched { get; set; }
    public double NodesPerSecond { get; set; }
    public double TableHitRate { get; set; }
    public int VCFDepthAchieved { get; set; }
    public long VCFNodesSearched { get; set; }
    public long ElapsedMs { get; set; }
}

/// <summary>
/// Pondering status event (sent when a player starts/stops pondering)
/// </summary>
public class PonderingStatusEvent
{
    public string GameId { get; set; } = string.Empty;
    public string Player { get; set; } = string.Empty;  // "red" or "blue" - who is pondering
    public bool IsPondering { get; set; }
    public string? PredictedMove { get; set; }  // "x,y" format if pondering
}

/// <summary>
/// Structured game log event for debugging and monitoring
/// </summary>
public class GameLogEvent
{
    public string Timestamp { get; set; } = string.Empty;  // HH:mm:ss.fff format
    public string Level { get; set; } = "info";  // "info", "warning", "error"
    public string Source { get; set; } = "system";  // "red", "blue", "system"
    public string Message { get; set; } = string.Empty;
}
