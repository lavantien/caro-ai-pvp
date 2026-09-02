using System.Globalization;
using Caro.Domain;
using Caro.Engine;

namespace Caro.Api;

/// <summary>
/// Formats the per-move statline. Byte-parity with the Go engine's output
/// is a contract: tournament artifacts and stats tooling parse these lines.
/// </summary>
internal static class Statline
{
    internal static string FormatStatlineNodes(long n) => n switch
    {
        >= 1_000_000 => FormatF1(n / 1_000_000.0) + "M",
        >= 1_000 => FormatF1(n / 1_000.0) + "K",
        _ => n.ToString(CultureInfo.InvariantCulture),
    };

    internal static string FormatStatlineNps(double nps) => nps switch
    {
        >= 1_000_000 => FormatF0(nps / 1_000_000.0) + "M",
        >= 1_000 => FormatF0(nps / 1_000.0) + "K",
        _ => FormatF0(nps),
    };

    // Go's fmt %.Nf rounds half to even; replicate so midpoints match.
    private static string FormatF1(double v) =>
        Math.Round(v, 1, MidpointRounding.ToEven).ToString("F1", CultureInfo.InvariantCulture);

    private static string FormatF0(double v) =>
        Math.Round(v, 0, MidpointRounding.ToEven).ToString("F0", CultureInfo.InvariantCulture);

    private static string FormatSigned(int v) =>
        v.ToString("+#;-#;+0", CultureInfo.InvariantCulture);

    internal static MoveDetailResponse BuildMoveDetail(
        GameResponse resp, string player, int x, int y, SearchStats stats, long thinkTimeMs, bool ponderHit)
    {
        int moveNum = resp.MoveNumber - 1;
        string pos = $"{(char)('a' + x)}{y + 1}";
        long remainingMs = (long)(resp.RedTimeRemaining * 1000);
        if (player == Player.Blue.ToName())
        {
            remainingMs = (long)(resp.BlueTimeRemaining * 1000);
        }

        string mt = MoveTypes.Exact;
        if (stats.MoveType.Length != 0)
        {
            mt = stats.MoveType;
        }

        string vcfTag = "";
        if (mt == MoveTypes.Vcf)
        {
            vcfTag = " [VCF]";
        }
        if (ponderHit)
        {
            vcfTag += " [PONDER]";
        }

        string statline = string.Create(CultureInfo.InvariantCulture,
            $"M{moveNum,2} {player,-4} {pos}  d={stats.DepthAchieved,-2} n={FormatStatlineNodes(stats.NodesSearched),-7} nps={FormatStatlineNps(stats.NodesPerSecond),-5} tt={(int)(stats.TableHitRate * 100),3}% s={FormatSigned(stats.SearchScore)} thr={stats.ThreadCount} t={thinkTimeMs / 1000.0:F1}s alloc={stats.AllocatedTimeMs / 1000.0:F1}s{vcfTag}");

        return new MoveDetailResponse
        {
            MoveNumber = moveNum,
            Player = player,
            Pos = pos,
            Statline = statline,
            ThinkTimeMs = thinkTimeMs,
            RemainingTimeMs = remainingMs,
            EngineStats = new EngineStatsResponse
            {
                Depth = stats.DepthAchieved,
                Nodes = stats.NodesSearched,
                NPS = stats.NodesPerSecond,
                TTHitRate = stats.TableHitRate,
                Score = stats.SearchScore,
                Threads = stats.ThreadCount,
                AllocatedTimeMs = stats.AllocatedTimeMs,
                MoveType = mt,
            },
        };
    }

    internal static string OpponentOf(string currentPlayer) =>
        currentPlayer == Player.Red.ToName() ? Player.Blue.ToName() : Player.Red.ToName();
}
