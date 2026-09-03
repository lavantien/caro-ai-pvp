using System.Diagnostics;
using System.Security.Cryptography;
using System.Text.Json;
using Caro.Domain;
using Caro.Engine;
using Caro.Persistence;
using Microsoft.AspNetCore.Http;
using Microsoft.Extensions.Logging;

namespace Caro.Api;

public sealed partial class GameHandlers(GameStore store, MatchStore? matches = null, ILogger? logger = null, CaroConfig? config = null)
{
    private const int GameIdByteLength = 8;

    private readonly CaroConfig _config = config ?? CaroConfig.Default;

    private static string NewGameId() =>
        Convert.ToHexString(RandomNumberGenerator.GetBytes(GameIdByteLength)).ToLowerInvariant();

    /// <summary>
    /// Search options for engines without a difficulty level set: the top
    /// ladder profile's strength knobs at full parallelism.
    /// </summary>
    private static SearchOptions DefaultSearchOptions(long timeRemainingMs, int incrementSeconds, int moveNumber)
    {
        Constants.DifficultyProfileData top = Constants.DifficultyProfiles[^1];
        return new SearchOptions
        {
            TimeRemainingMs = timeRemainingMs,
            IncrementMs = (long)(incrementSeconds * Constants.Time.MsPerSecond),
            MoveNumber = moveNumber,
            ParallelEnabled = true,
            TimeFraction = top.TimeFraction,
            UseVCF = top.UseVCF,
        };
    }

    public async Task CreateGameAsync(HttpContext http)
    {
        // Evict finished and idle sessions first so stale games never block
        // new ones between the periodic sweeps.
        store.CleanupCompleted();
        if (store.Count() >= _config.MaxConcurrentGames)
        {
            throw new TooManyGamesException();
        }

        CreateGameRequest req;
        try
        {
            req = await http.Request.ReadFromJsonAsync<CreateGameRequest>(JsonOptions.Shared)
                ?? new CreateGameRequest();
        }
        catch (JsonException e)
        {
            await ResponseJson.Write(http, 400, new ErrorResponse { Error = "bad_request", Message = e.Message });
            return;
        }

        (string timeControl, long initialTimeMs, int incrementSeconds) = TimeControls.Resolve(req.TimeControl, _config.TimeControl);

        GameMode gameMode = GameModes.Parse(req.GameMode);
        int? redDiff = req.RedDifficulty;
        int? blueDiff = req.BlueDifficulty;
        if (req.Difficulty != null)
        {
            redDiff ??= req.Difficulty;
            blueDiff ??= req.Difficulty;
        }

        if (redDiff is < Constants.Difficulty.MinLevel or > Constants.Difficulty.MaxLevel
            || blueDiff is < Constants.Difficulty.MinLevel or > Constants.Difficulty.MaxLevel)
        {
            throw new InvalidLevelException();
        }

        string gameId = NewGameId();
        GameSession session = new(timeControl, initialTimeMs, incrementSeconds, gameMode,
            redDiff, blueDiff, store.ActiveGameCount, _config);
        if (req.RandomOpening && gameMode == GameMode.AivAI)
        {
            session.ApplyRandomOpening(req.Seed);
        }
        store.Set(gameId, session);

        if (matches != null)
        {
            string redType = "human";
            string blueType = "human";
            if (gameMode == GameMode.AivAI)
            {
                redType = "bot";
                blueType = "bot";
            }
            else if (gameMode == GameMode.PvAI)
            {
                if (redDiff != null)
                {
                    redType = "bot";
                }
                else
                {
                    blueType = "bot";
                }
            }
            try
            {
                matches.CreateGame(new GameRecord
                {
                    ID = gameId,
                    GameMode = gameMode.ToName(),
                    TimeControl = timeControl,
                    RedType = redType,
                    BlueType = blueType,
                    RedDifficulty = redDiff,
                    BlueDifficulty = blueDiff,
                });
            }
            catch (Exception e)
            {
                logger?.StoreFailure(e, "create game", gameId);
            }
        }

        await ResponseJson.Write(http, 200, new NewGameResponse(gameId, session.GetResponse()));
    }

    public async Task GetGameAsync(HttpContext http)
    {
        string id = (string)http.Request.RouteValues["id"]!;
        if (!store.TryGet(id, out GameSession session))
        {
            throw new GameNotFoundException();
        }
        await ResponseJson.Write(http, 200, new StateResponse(session.GetResponse()));
    }

    public async Task MakeMoveAsync(HttpContext http)
    {
        string id = (string)http.Request.RouteValues["id"]!;
        if (!store.TryGet(id, out GameSession session))
        {
            throw new GameNotFoundException();
        }

        MoveRequest req;
        try
        {
            req = await http.Request.ReadFromJsonAsync<MoveRequest>(JsonOptions.Shared)
                ?? new MoveRequest();
        }
        catch (JsonException e)
        {
            await ResponseJson.Write(http, 400, new ErrorResponse { Error = "bad_request", Message = e.Message });
            return;
        }

        GameResponse resp = session.ApplyHumanMove(req.X, req.Y);

        LogHumanMove(id, req.X, req.Y, resp);

        await ResponseJson.Write(http, 200, new StateResponse(resp));
    }

    public async Task MakeAIMoveAsync(HttpContext http)
    {
        string id = (string)http.Request.RouteValues["id"]!;
        if (!store.TryGet(id, out GameSession session))
        {
            throw new GameNotFoundException();
        }

        (Board board, Player player, bool isGameOver, long timeRemainingMs, int incrementSeconds, int moveNumber, int? difficulty) =
            session.ExtractForAI();
        if (isGameOver)
        {
            throw new GameOverException();
        }

        Stopwatch sw = Stopwatch.StartNew();

        // Whatever the ponder did, the real search decides the move; the
        // info only annotates the statline and the persisted row.
        (SearchStats ponderStats, bool ponderHit, bool hadPonder) = session.TakePonderInfo(player);

        MinimaxAI ai = session.GetOrCreateAI(player);

        SearchOptions opts;
        if (difficulty is >= Constants.Difficulty.MinLevel and <= Constants.Difficulty.MaxLevel)
        {
            DifficultyProfile profile = Difficulty.GetDifficultyProfile(difficulty.Value, _config);
            opts = new SearchOptions
            {
                TimeRemainingMs = timeRemainingMs,
                IncrementMs = (long)(incrementSeconds * Constants.Time.MsPerSecond),
                MoveNumber = moveNumber,
                ThreadCount = profile.Threads,
                ParallelEnabled = profile.Threads > 1,
                TimeFraction = profile.TimeFraction,
                UseVCF = profile.UseVCF,
                VCFMaxDepth = profile.VCFDepth,
                MaxDepth = profile.MaxDepth,
            };
        }
        else
        {
            opts = DefaultSearchOptions(timeRemainingMs, incrementSeconds, moveNumber);
        }

        CancellationToken ctx = http.RequestAborted;
        (int x, int y, SearchStats stats) = await Task.Run(() => ai.GetBestMove(board, player, opts, ctx), ctx);
        long thinkTime = (long)sw.Elapsed.TotalMilliseconds;

        GameResponse resp = session.ApplyAIMove(x, y, player);

        int? ponderDepth = null;
        long? ponderNodes = null;
        if (hadPonder)
        {
            ponderDepth = ponderStats.DepthAchieved;
            ponderNodes = ponderStats.NodesSearched;
        }
        LogAIMove(id, x, y, resp, difficulty, stats, thinkTime, ponderDepth, ponderNodes);

        MoveDetailResponse moveDetail = Statline.BuildMoveDetail(resp, player.ToName(), x, y, stats, thinkTime,
            hadPonder ? ponderHit : null, ponderDepth, ponderNodes);
        logger?.MoveStatline(id, moveDetail.Statline);
        await ResponseJson.Write(http, 200, new MoveResponse(resp, moveDetail));
    }

    public async Task UndoMoveAsync(HttpContext http)
    {
        string id = (string)http.Request.RouteValues["id"]!;
        if (!store.TryGet(id, out GameSession session))
        {
            throw new GameNotFoundException();
        }

        GameResponse resp = session.UndoLastMove();
        await ResponseJson.Write(http, 200, new StateResponse(resp));
    }

    public async Task DeleteGameAsync(HttpContext http)
    {
        string id = (string)http.Request.RouteValues["id"]!;
        if (!store.TryGet(id, out GameSession session))
        {
            throw new GameNotFoundException();
        }
        if (matches != null)
        {
            GameResponse resp = session.GetResponse();
            string winner = resp.Winner;
            if (winner.Length == 0 || winner == Player.None.ToName())
            {
                winner = EndReasons.Abandoned;
            }
            try
            {
                matches.CompleteGame(id, winner, resp.MoveNumber);
            }
            catch (Exception e)
            {
                logger?.StoreFailure(e, "complete game", id);
            }
        }
        store.Delete(id);
        await ResponseJson.Write(http, 200, new DeletedResponse(true));
    }
}
