using Caro.Domain;
using Caro.Engine;
using Caro.Persistence;
using Microsoft.Extensions.Logging;

namespace Caro.Api;

public sealed partial class GameHandlers
{
    private void LogHumanMove(string gameId, int x, int y, string mover, GameResponse resp)
    {
        if (matches == null)
        {
            return;
        }
        int moveNum = resp.MoveNumber;
        if (moveNum > 0)
        {
            moveNum--;
        }
        try
        {
            matches.RecordMove(new MoveRecord
            {
                GameID = gameId,
                MoveNumber = moveNum,
                Player = mover,
                PosX = x,
                PosY = y,
                IsBot = false,
            });
        }
        catch (Exception e)
        {
            logger?.StoreFailure(e, "record move", gameId);
        }
        if (resp.IsGameOver)
        {
            CompleteRecordedGame(gameId, resp.Winner, resp.MoveNumber);
        }
    }

    private void LogAIMove(string gameId, int x, int y, string mover, GameResponse resp, int? difficulty,
        SearchStats stats, long thinkTimeMs, int? ponderDepth, long? ponderNodes)
    {
        if (matches == null)
        {
            return;
        }
        int moveNum = resp.MoveNumber;
        if (moveNum > 0)
        {
            moveNum--;
        }
        long remainingMs = (long)(resp.RedTimeRemaining * Constants.Time.MsPerSecond);
        if (mover == Player.Blue.ToName())
        {
            remainingMs = (long)(resp.BlueTimeRemaining * Constants.Time.MsPerSecond);
        }
        string mt = stats.MoveType.Length == 0 ? MoveTypes.Exact : stats.MoveType;

        try
        {
            matches.RecordMove(new MoveRecord
            {
                GameID = gameId,
                MoveNumber = moveNum,
                Player = mover,
                PosX = x,
                PosY = y,
                IsBot = true,
                Difficulty = difficulty,
                ThinkTimeMs = thinkTimeMs,
                RemainingTimeMs = remainingMs,
                SearchDepth = stats.DepthAchieved,
                NodesSearched = stats.NodesSearched,
                NPS = stats.NodesPerSecond,
                TTHitRate = stats.TableHitRate,
                SearchScore = stats.SearchScore,
                ThreadsUsed = stats.ThreadCount,
                AllocatedTimeMs = stats.AllocatedTimeMs,
                MoveType = mt,
                PonderDepth = ponderDepth,
                PonderNodes = ponderNodes,
                VcfDepth = stats.VcfDepth,
                VcfNodes = stats.VcfNodes,
            });
        }
        catch (Exception e)
        {
            logger?.StoreFailure(e, "record move", gameId);
        }
        if (resp.IsGameOver)
        {
            CompleteRecordedGame(gameId, resp.Winner, resp.MoveNumber);
        }
    }

    private void CompleteRecordedGame(string gameId, string winner, int moveNumber)
    {
        try
        {
            matches!.CompleteGame(gameId, winner, moveNumber);
        }
        catch (Exception e)
        {
            logger?.StoreFailure(e, "complete game", gameId);
        }
    }
}
