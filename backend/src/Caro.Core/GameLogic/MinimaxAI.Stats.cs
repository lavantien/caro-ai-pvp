using System.Threading.Channels;
using Caro.Core.Domain.Configuration;
using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic.Pondering;
using Caro.Core.GameLogic.Search;
using Caro.Core.GameLogic.TimeManagement;

namespace Caro.Core.GameLogic;

/// <summary>
/// MinimaxAI partial class - Pondering support and stats publishing.
/// </summary>
public partial class MinimaxAI
{
    #region Pondering Support

    /// <summary>
    /// Get the ponderer instance for external access
    /// </summary>
    public Ponderer GetPonderer() => _ponderer;

    /// <summary>
    /// Get the last calculated Principal Variation
    /// </summary>
    public PV GetLastPV() => _lastPV;

    /// <summary>
    /// Stop any active pondering and publish ponder stats with explicit player color
    /// </summary>
    public void StopPondering(Player forPlayer)
    {
        _ponderer.StopPondering();
        PublishPonderStats(forPlayer);
    }

    /// <summary>
    /// Start pondering immediately (at start of opponent's turn, without waiting for prediction)
    /// </summary>
    public void StartPonderingNow(Board board, Player currentPlayerToMove, Player thisAIColor)
    {
        var ponderTimeMs = TimeBudgetCalculator.CalculatePonderTime(null);
        if (ponderTimeMs > 0)
        {
            _ponderer.StartPondering(board, currentPlayerToMove, null, thisAIColor, ponderTimeMs);
        }
    }

    /// <summary>
    /// Start pondering after making a move (for opponent's response)
    /// This is a stateless version - all parameters passed explicitly
    /// </summary>
    public void StartPonderingAfterMove(Board board, Player opponentToMove, Player thisAIColor, PV? lastPV = null)
    {
        var predictedOpponentMove = lastPV?.GetPredictedOpponentMove() ?? _lastPV.GetPredictedOpponentMove();

        var ponderTimeMs = TimeBudgetCalculator.CalculatePonderTime(null);
        if (ponderTimeMs > 0)
        {
            _ponderer.StartPondering(board, opponentToMove, predictedOpponentMove, thisAIColor, ponderTimeMs);
        }
    }

    /// <summary>
    /// Reset pondering state (call when starting a new game)
    /// </summary>
    public void ResetPondering()
    {
        _ponderer.Reset();
        _lastPV = PV.Empty;
        _lastBoard = null;
    }

    /// <summary>
    /// Get pondering statistics
    /// </summary>
    public string GetPonderingStatistics() => _ponderer.GetStatistics();

    /// <summary>
    /// Get last ponder result statistics (nodes searched during opponent's turn)
    /// Returns (depth, nodesSearched, nodesPerSecond, timeSpentMs)
    /// </summary>
    public (int Depth, long NodesSearched, double NodesPerSecond, long TimeSpentMs) GetLastPonderStats(Player forPlayer)
    {
        var ponderResult = _ponderer.GetCurrentResult();
        var depth = ponderResult.Depth;
        var nodesSearched = ponderResult.NodesSearched;
        var timeSpentMs = ponderResult.TimeSpentMs;
        var nps = timeSpentMs > 0 ? (double)nodesSearched * 1000 / timeSpentMs : 0;

        return (depth, nodesSearched, nps, timeSpentMs);
    }

    /// <summary>
    /// Get search statistics for the last move
    /// </summary>
    public (int DepthAchieved, long NodesSearched, double NodesPerSecond, double TableHitRate, bool PonderingActive, int VCFDepthAchieved, long VCFNodesSearched, int ThreadCount, string? ParallelDiagnostics, double MasterTTPercent, double HelperAvgDepth, long AllocatedTimeMs, MoveType MoveType, int SearchScore, double FmcPercent, double Ebf) GetSearchStatistics()
    {
        double hitRate = _tableLookups > 0 ? (double)_tableHits / _tableLookups * 100 : 0;
        var elapsedMs = _searchStopwatch.ElapsedMilliseconds;
        double nps = elapsedMs > 0 ? (double)_nodesSearched * 1000 / elapsedMs : 0;

        // Parse % from master and helper avg depth from diagnostics string
        double masterTTPercent = 0;
        double helperAvgDepth = 0;

        if (!string.IsNullOrEmpty(_lastParallelDiagnostics))
        {
            // Parse "% from master" from TT part
            var ttMatch = System.Text.RegularExpressions.Regex.Match(_lastParallelDiagnostics, @"(\d+\.?\d*)% from master");
            if (ttMatch.Success && double.TryParse(ttMatch.Groups[1].Value, out var ttPercent))
            {
                masterTTPercent = ttPercent;
            }

            // Parse "avg=X.X" from Depths part
            var avgMatch = System.Text.RegularExpressions.Regex.Match(_lastParallelDiagnostics, @"avg=([\d\.]+)");
            if (avgMatch.Success && double.TryParse(avgMatch.Groups[1].Value, out var avgDepth))
            {
                helperAvgDepth = avgDepth;
            }
        }

        int displayScore = _lastSearchScore;
        if (displayScore <= int.MinValue + 2000) displayScore = 0;
        else if (displayScore >= int.MaxValue - 2000) displayScore = 100_000;

        return (_depthAchieved, _nodesSearched, nps, hitRate, _lastPonderingEnabled, _vcfDepthAchieved, _vcfNodesSearched, _lastThreadCount, _lastParallelDiagnostics, masterTTPercent, helperAvgDepth, _lastAllocatedTimeMs, _moveType, displayScore, _lastFmcPercent, _lastEbf);
    }

    /// <summary>
    /// Publish search statistics to the stats channel
    /// Called automatically after each search
    /// </summary>
    public void PublishSearchStats(Player player, StatsType statsType, long moveTimeMs)
    {
        var (depthAchieved, nodesSearched, nps, hitRate, ponderingActive, vcfDepthAchieved, vcfNodesSearched, threadCount, _, masterTTPercent, helperAvgDepth, allocatedTimeMs, moveType, _, _, _) = GetSearchStatistics();

        var statsEvent = new MoveStatsEvent
        {
            PublisherId = _publisherId,
            Player = player,
            Type = statsType,
            DepthAchieved = depthAchieved,
            NodesSearched = nodesSearched,
            NodesPerSecond = nps,
            TableHitRate = hitRate,
            PonderingActive = ponderingActive,
            VCFDepthAchieved = vcfDepthAchieved,
            VCFNodesSearched = vcfNodesSearched,
            ThreadCount = threadCount,
            MoveTimeMs = moveTimeMs,
            TimestampMs = DateTimeOffset.UtcNow.ToUnixTimeMilliseconds(),
            MasterTTPercent = masterTTPercent,
            HelperAvgDepth = helperAvgDepth,
            AllocatedTimeMs = allocatedTimeMs,
            MoveType = moveType
        };

        _statsChannel.Writer.TryWrite(statsEvent);
    }

    /// <summary>
    /// Publish pondering statistics to the stats channel
    /// </summary>
    public void PublishPonderStats(Player player)
    {
        var (depth, nodesSearched, nps, timeSpentMs) = GetLastPonderStats(player);

        if (nodesSearched == 0 && timeSpentMs == 0)
            return;

        var statsEvent = new MoveStatsEvent
        {
            PublisherId = _publisherId,
            Player = player,
            Type = StatsType.Pondering,
            DepthAchieved = depth,
            NodesSearched = nodesSearched,
            NodesPerSecond = nps,
            TableHitRate = 0,
            PonderingActive = true,
            VCFDepthAchieved = 0,
            VCFNodesSearched = 0,
            ThreadCount = 0,
            MoveTimeMs = timeSpentMs,
            TimestampMs = DateTimeOffset.UtcNow.ToUnixTimeMilliseconds()
        };

        _statsChannel.Writer.TryWrite(statsEvent);
    }

    #endregion
}
