using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;
using Caro.Core.Tournament;

namespace Caro.TournamentRunner;

/// <summary>
/// Shared utility for formatting game move statistics with consistent output across all runners
/// </summary>
public static class GameStatsFormatter
{
    /// <summary>
    /// Format a single move with all statistics in a consistent one-line format
    /// </summary>
    /// <summary>
    /// Format a single move with all statistics in a consistent one-line format
    /// </summary>
    public static string FormatMoveLine(
        int game,
        int moveNumber,
        int x,
        int y,
        Player player,
        AIDifficulty difficulty,
        MoveStats? stats)
    {
        var color = player == Player.Red ? "R" : "B";

        var timeStr = FormatTime(stats?.MoveTimeMs ?? 0);
        var allocStr = FormatTime(stats?.AllocatedTimeMs ?? 0);
        var depthStr = stats != null ? $"D{stats.DepthAchieved}" : "D-";
        var moveTypeStr = FormatMoveType(stats?.MoveType ?? MoveType.Normal, stats?.BookUsed ?? false);

        long mainNodes = stats?.NodesSearched ?? 0;
        double mainNps = stats?.NodesPerSecond ?? 0;
        var nStr = FormatLargeNumber(mainNodes);
        var npsStr = FormatLargeNumber((long)mainNps);

        var ttStr = stats != null ? $"{stats.TableHitRate:F1}%" : "N/A";
        var masterStr = stats != null ? $"{stats.MasterTTPercent:F1}%" : "N/A";
        var helperStr = stats != null ? $"{stats.HelperAvgDepth:F1}" : "N/A";
        var threadsStr = stats?.ThreadCount.ToString() ?? "1";

        long ponderNodes = stats?.PonderNodesSearched ?? 0;
        double ponderNps = stats?.PonderNodesPerSecond ?? 0;
        int ponderDepth = stats?.PonderDepth ?? 0;
        var ponderStr = (stats?.PonderingActive == true && ponderNodes > 0)
            ? $"D{ponderDepth}/{FormatLargeNumber(ponderNodes)}n/{FormatLargeNumber((long)ponderNps)}nps"
            : "-";

        var vcfDepth = stats?.VCFDepthAchieved ?? 0;
        var vcfNodes = stats?.VCFNodesSearched ?? 0;
        var vcfStr = (vcfDepth > 0 || vcfNodes > 0) ? $"{vcfDepth}d/{FormatLargeNumber(vcfNodes)}n" : "-";

        return
            $"    G{game,2} M{moveNumber,3} | {color}({x},{y}) by {difficulty,-12} | " +
            $"T: {timeStr,-7}/{allocStr,-6} | " +
            $"{moveTypeStr,-4} | " +
            $"Th: {threadsStr} | " +
            $"{depthStr,-3} | " +
            $"N: {nStr,-8} | " +
            $"NPS: {npsStr,-8} | " +
            $"TT: {ttStr,-5} | " +
            $"%M: {masterStr,-5} | " +
            $"HD: {helperStr,-4} | " +
            $"P: {ponderStr,-25} | " +
            $"VCF: {vcfStr}";
    }

    /// <summary>
    /// Format move type as a short code for display
    /// </summary>
    private static string FormatMoveType(MoveType moveType, bool bookUsed)
    {
        return moveType switch
        {
            MoveType.Normal => "-",
            MoveType.Book => "Bk",         // Book move (unvalidated)
            MoveType.BookValidated => "Bv", // Book move validated by search
            MoveType.ImmediateWin => "Wn",  // Immediate winning move
            MoveType.ImmediateBlock => "Bl", // Forced block
            MoveType.ErrorRate => "Er",     // Error rate random move
            MoveType.CenterMove => "Ct",    // Center opening move
            MoveType.Emergency => "Em",     // Emergency mode
            _ => "-"
        };
    }

    /// <summary>
    /// Format a time duration in ms to a human-readable string
    /// </summary>
    public static string FormatTime(long ms)
    {
        if (ms < 1000)
            return $"{ms}ms";
        if (ms < 60000)
            return $"{ms / 1000.0:F1}s";
        return $"{ms / 60000.0:F1}m";
    }

    /// <summary>
    /// Format a large number with K/M/B suffixes
    /// </summary>
    public static string FormatLargeNumber(long n)
    {
        if (n < 1_000)
            return n.ToString();
        if (n < 1_000_000)
            return $"{n / 1000.0:F1}K";
        if (n < 1_000_000_000)
            return $"{n / 1_000_000.0:F1}M";
        return $"{n / 1_000_000_000.0:F1}B";
    }

    /// <summary>
    /// Format a game result line
    /// </summary>
    public static string FormatGameResult(int game, AIDifficulty winnerDiff, int moveCount, double durationSec, Player winnerColor = Player.None, bool isDraw = false)
    {
        if (isDraw)
            return $"    → Game {game}: DRAW after {moveCount} moves ({durationSec:F1}s)";

        var colorStr = winnerColor == Player.Red ? "Red" : "Blue";
        return $"    → Game {game}: {winnerDiff} ({colorStr}) wins on move {moveCount} ({durationSec:F1}s)";
    }
}
