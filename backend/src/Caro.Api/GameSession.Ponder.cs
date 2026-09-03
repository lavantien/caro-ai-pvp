using Caro.Domain;
using Caro.Engine;

namespace Caro.Api;

public sealed partial class GameSession
{
    // Process-wide kill switch, read once at startup like MATCH_DB_PATH.
    // Mutable so tests can flip it.
    internal static bool PonderEnvDisabled = IsPonderDisabledByEnv();

    private static bool IsPonderDisabledByEnv()
    {
        return Environment.GetEnvironmentVariable("CARO_DISABLE_PONDER")?.ToLowerInvariant() switch
        {
            "1" or "true" => true,
            _ => false,
        };
    }

    // activePonderState records the ponder a player started after its own
    // move: the predicted opponent reply the background search is built on,
    // and the time cap that search was launched with.
    internal sealed class ActivePonderState(Player player, Position predictedReply, long timeCapMs)
    {
        public Player Player { get; } = player;
        public Position PredictedReply { get; } = predictedReply;
        public long TimeCapMs { get; } = timeCapMs;
    }

    // ponderInfo records the ponder that ran while the opponent was
    // thinking: whether the opponent's move matched the prediction, and what
    // the background search reached. It is observability only. The real move
    // is always decided by a fresh budgeted search over the TT the ponder
    // warmed; pondering buys depth through the warm table, never a shortcut
    // move.
    internal sealed class PonderInfo(Player player, bool hit, SearchStats stats)
    {
        public Player Player { get; } = player;
        public bool Hit { get; } = hit;
        public SearchStats Stats { get; } = stats;
    }

    private ActivePonderState? _activePonder;
    private PonderInfo? _pendingPonder;
    internal long _ponderTimeCapMs;

    // Test accessors for ponder state the public API cannot expose.
    internal ActivePonderState? ActivePonderForTest => _activePonder;
    internal PonderInfo? PendingPonderForTest => _pendingPonder;
    internal Caro.Engine.MinimaxAI? RedAIFromTest => _redAI;
    internal Caro.Engine.MinimaxAI? BlueAIFromTest => _blueAI;
    internal Caro.Domain.GameState GameForTest => _game;
    internal void SetClockForTest(Caro.Domain.Player p, long ms)
    {
        lock (_mu)
        {
            if (p == Caro.Domain.Player.Blue)
            {
                _blueTimeMs = ms;
            }
            else
            {
                _redTimeMs = ms;
            }
        }
    }

    internal void SetPonderTimeCapForTest(long capMs) => _ponderTimeCapMs = capMs;

    private bool PonderEnabledForLocked(Player p)
    {
        if (PonderEnvDisabled)
        {
            return false;
        }
        int? diff = DifficultyForLocked(p);
        return diff is >= Constants.Difficulty.MinLevel and <= Constants.Difficulty.MaxLevel
            && Difficulty.GetDifficultyProfile(diff.Value).Ponder;
    }

    private int? DifficultyForLocked(Player p) => p == Player.Blue ? _blueDifficulty : _redDifficulty;

    private MinimaxAI? AiForPlayerLocked(Player p) => p == Player.Red ? _redAI : _blueAI;

    // StopPonderLocked joins the active ponder, if any, and records what it
    // produced for the stats of the next move.
    private void StopPonderLocked(int actualX, int actualY)
    {
        ActivePonderState? active = _activePonder;
        if (active == null)
        {
            return;
        }
        _activePonder = null;

        MinimaxAI? ai = AiForPlayerLocked(active.Player);
        if (ai == null)
        {
            return;
        }
        (PonderOutcome outcome, bool ok) = ai.StopPonder();
        if (!ok)
        {
            return;
        }
        bool hit = outcome.Completed && outcome.PredictedReply == new Position(actualX, actualY);
        _pendingPonder = new PonderInfo(active.Player, hit, outcome.Stats);
    }

    /// <summary>
    /// Returns and clears the recorded ponder info for expected player, if
    /// the opponent's last move ended that player's ponder.
    /// </summary>
    public (SearchStats Stats, bool Hit, bool Had) TakePonderInfo(Player expectedPlayer)
    {
        lock (_mu)
        {
            PonderInfo? info = _pendingPonder;
            _pendingPonder = null;
            if (info == null || info.Player != expectedPlayer)
            {
                return (default(SearchStats), false, false);
            }
            return (info.Stats, info.Hit, true);
        }
    }

    // StartPonderLocked launches mover's ponder on the position after its
    // own move plus the predicted reply. Skipped when pondering is
    // disabled, the AI has no search history, or no prediction is available.
    private void StartPonderLocked(Player mover)
    {
        if (!PonderEnabledForLocked(mover))
        {
            return;
        }
        MinimaxAI? ai = AiForPlayerLocked(mover);
        if (ai == null)
        {
            return;
        }
        (Position predicted, bool ok) = ai.PredictReply(_game.Board);
        if (!ok)
        {
            return;
        }
        Board pondered;
        try
        {
            pondered = _game.Board.PlaceStone(predicted.X, predicted.Y, mover.Opponent());
        }
        catch (CaroException)
        {
            return;
        }

        DifficultyProfile profile = Difficulty.GetDifficultyProfile(DifficultyForLocked(mover)!.Value);
        // ponderTimeCapMs: 0 derives the cap from the opponent's live clock
        // (they must move or flag within it, so it scales with the time
        // control); negative forces a zero budget, the deterministic
        // incompleteness seam for tests.
        long capMs = _ponderTimeCapMs;
        if (capMs == 0)
        {
            capMs = LiveClockMsLocked(mover.Opponent());
        }
        if (capMs < 0)
        {
            capMs = 0;
        }
        if (!ai.StartPonder(pondered, mover, predicted, new PonderConfig
        {
            Threads = profile.Threads,
            MaxDepth = profile.MaxDepth,
            UseVCF = profile.UseVCF,
            VCFDepth = profile.VCFDepth,
            TimeCapMs = capMs,
        }))
        {
            return;
        }
        _activePonder = new ActivePonderState(mover, predicted, capMs);
    }

    // LiveClockMsLocked returns p's remaining time accounting for the clock
    // burning since the last move.
    private long LiveClockMsLocked(Player p)
    {
        long remaining = p == Player.Blue ? _blueTimeMs : _redTimeMs;
        if (_game.IsGameOver)
        {
            return 0;
        }
        long elapsed = ElapsedSinceLastMoveMs();
        return Math.Max(0, remaining - elapsed);
    }

    // ClearPonderStateLocked joins any running ponder without recording an
    // outcome and drops all ponder state. The undo and teardown path.
    private void ClearPonderStateLocked()
    {
        if (_activePonder != null)
        {
            AiForPlayerLocked(_activePonder.Player)?.StopPonder();
            _activePonder = null;
        }
        _pendingPonder = null;
    }
}
