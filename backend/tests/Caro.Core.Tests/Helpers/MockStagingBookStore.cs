using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;

namespace Caro.Core.Tests.Helpers;

/// <summary>
/// Mock implementation of IStagingBookStore for testing.
/// </summary>
public sealed class MockStagingBookStore : IStagingBookStore
{
    private readonly List<StagingPosition> _positions = new();
    private readonly Dictionary<(ulong CanonicalHash, ulong DirectHash, Player Player), PositionStatistics> _stats = new();
    private readonly Dictionary<(ulong CanonicalHash, ulong DirectHash, Player Player, int MoveX, int MoveY), int> _moveResults = new();

    public void RecordMove(
        ulong canonicalHash,
        ulong directHash,
        Player player,
        int ply,
        int moveX,
        int moveY,
        int gameResult,
        long gameId,
        int timeBudgetMs)
    {
        _positions.Add(new StagingPosition
        {
            CanonicalHash = canonicalHash,
            DirectHash = directHash,
            Player = player,
            Ply = ply,
            MoveX = moveX,
            MoveY = moveY,
            TimeBudgetMs = timeBudgetMs
        });

        var posKey = (canonicalHash, directHash, player, moveX, moveY);
        _moveResults[posKey] = gameResult;

        var key = (canonicalHash, directHash, player);
        if (!_stats.ContainsKey(key))
        {
            _stats[key] = new PositionStatistics
            {
                PlayCount = 0,
                WinCount = 0,
                WinRate = 0.0,
                AvgTimeBudgetMs = 0,
                DrawCount = 0,
                LossCount = 0
            };
        }

        var stat = _stats[key];
        var newPlayCount = stat.PlayCount + 1;
        var newWinCount = stat.WinCount + (gameResult == 1 ? 1 : 0);
        var newDrawCount = stat.DrawCount + (gameResult == 0 ? 1 : 0);
        var newLossCount = stat.LossCount + (gameResult == -1 ? 1 : 0);

        _stats[key] = new PositionStatistics
        {
            PlayCount = newPlayCount,
            WinCount = newWinCount,
            WinRate = (double)newWinCount / newPlayCount,
            AvgTimeBudgetMs = timeBudgetMs,
            DrawCount = newDrawCount,
            LossCount = newLossCount
        };
    }

    public IEnumerable<StagingPosition> GetPositionsForVerification(int maxPly)
    {
        return _positions.Where(p => p.Ply <= maxPly);
    }

    public Dictionary<(ulong CanonicalHash, ulong DirectHash, Player Player), PositionStatistics> GetPositionStatistics()
        => _stats;

    public List<StagingMove> GetMovesForPosition(ulong canonicalHash, ulong directHash, Player player)
    {
        var key = (canonicalHash, directHash, player);
        if (!_stats.TryGetValue(key, out var stat))
        {
            return new List<StagingMove>();
        }

        var matchingPositions = _positions
            .Where(p => p.CanonicalHash == canonicalHash && p.DirectHash == directHash && p.Player == player)
            .ToList();

        var moves = new List<StagingMove>();
        var seenMoves = new HashSet<(int, int)>();

        foreach (var pos in matchingPositions)
        {
            var moveKey = (pos.MoveX, pos.MoveY);
            if (seenMoves.Contains(moveKey))
            {
                continue;
            }
            seenMoves.Add(moveKey);

            var moveResultKey = (canonicalHash, directHash, player, pos.MoveX, pos.MoveY);
            var gameResult = _moveResults.TryGetValue(moveResultKey, out var result) ? result : 0;

            moves.Add(new StagingMove
            {
                MoveX = pos.MoveX,
                MoveY = pos.MoveY,
                Ply = pos.Ply,
                GameResult = gameResult,
                PlayCount = matchingPositions.Count,
                WinRate = stat.WinRate
            });
        }

        return moves;
    }

    public void Flush() { }
    public void Clear()
    {
        _positions.Clear();
        _stats.Clear();
        _moveResults.Clear();
    }

    public void Initialize() { }
    public long GetPositionCount() => _positions.Count;
    public long GetGameCount() => _positions.Select(p => p.Ply).Distinct().Count();

    public void Dispose() { }
}
