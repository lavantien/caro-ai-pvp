using Caro.Domain;
using Caro.Engine;
using Xunit;

namespace Caro.Engine.Tests;

/// <summary>
/// Regression ports of the Go search_fixes, lmr_guard, scorehierarchy and
/// heuristics_retention suites.
/// </summary>
public class SearchFixesTests
{
    /// <summary>
    /// Gives red an open four on row 5 (columns 3-6): any completion at
    /// (2,5) or (7,5) is an immediate win.
    /// </summary>
    internal static Board RedHasOpenFour()
    {
        Board b = Board.NewBoard();
        for (int x = 3; x <= 6; x++)
        {
            b = b.PlaceStone(x, 5, Player.Red);
        }
        return b.PlaceStone(10, 10, Player.Blue);
    }

    [Fact]
    public void ForcedWinStopsDeepening()
    {
        Board b = RedHasOpenFour();
        using TranspositionTable tt = new(1);
        SearchHeuristics heuristics = new();
        SearchConfig opts = new() { MaxDepth = Constants.Search.AbsoluteMaxDepth, TimeLimitMs = 10_000, Threads = 1 };

        (_, _, SearchStats stats) = SearchEngine.SearchPosition(b, Player.Red, opts, tt, heuristics, CancellationToken.None);

        Assert.Equal(1, stats.DepthAchieved);
        Assert.True(stats.SearchScore >= Constants.Score.WinScore - Constants.Search.AbsoluteMaxDepth);
    }

    [Fact]
    public void ZeroTimeFallbackIsOrdered()
    {
        Board b = RedHasOpenFour();
        using TranspositionTable tt = new(1);
        SearchHeuristics heuristics = new();
        SearchConfig opts = new() { MaxDepth = Constants.Search.AbsoluteMaxDepth, TimeLimitMs = 0, Threads = 1 };

        (int x, int y, SearchStats stats) = SearchEngine.SearchPosition(b, Player.Red, opts, tt, heuristics, CancellationToken.None);

        bool completes = (x == 2 || x == 7) && y == 5;
        Assert.True(completes,
            $"with no time for any depth the fallback must be an ordered move (the winning completion), got ({x},{y})");
        Assert.Equal("timeout-fallback", stats.MoveType);
    }

    [Fact]
    public void SoftLimitStopsBeforeHardBound()
    {
        Board b = Board.NewBoard()
            .PlaceStone(8, 8, Player.Red)
            .PlaceStone(9, 9, Player.Blue)
            .PlaceStone(7, 7, Player.Red)
            .PlaceStone(10, 10, Player.Blue)
            .PlaceStone(6, 8, Player.Red)
            .PlaceStone(9, 6, Player.Blue);
        using TranspositionTable tt = new(1);
        SearchHeuristics heuristics = new();
        SearchConfig opts = new() { MaxDepth = Constants.Search.AbsoluteMaxDepth, TimeLimitMs = 5000, SoftLimitMs = 500, Threads = 1 };

        System.Diagnostics.Stopwatch sw = System.Diagnostics.Stopwatch.StartNew();
        (_, _, SearchStats stats) = SearchEngine.SearchPosition(b, Player.Red, opts, tt, heuristics, CancellationToken.None);
        sw.Stop();

        Assert.True(stats.DepthAchieved >= 1);
        Assert.True(sw.ElapsedMilliseconds < 4000,
            $"search must stop near the soft limit instead of burning to the hard bound (elapsed {sw.ElapsedMilliseconds}ms)");
    }

    [Fact]
    public void TTStoreStampsCurrentAge()
    {
        using TranspositionTable tt = new(1);
        // Two distinct hashes sharing one slot (offset by the table stride).
        ulong h1 = 0xABCD;
        ulong h2 = h1 + tt.ShardStrideForTest();
        tt.Store(new TTEntry { Hash = h1, Score = 100, Depth = 10 });

        for (int i = 0; i < 3; i++)
        {
            tt.IncrementAge();
        }
        tt.Store(new TTEntry { Hash = h2, Score = 5, Depth = 2 });

        Assert.False(tt.Lookup(h1, out _), "aged entry must lose the slot to the fresh write");
        Assert.True(tt.Lookup(h2, out TTEntry entry));
        Assert.Equal(5, entry.Score);
        Assert.Equal(3, entry.Age);
    }

    [Fact]
    public void RootSearchStoresBoundFlagOnFailLow()
    {
        Board b = Board.NewBoard()
            .PlaceStone(7, 7, Player.Red)
            .PlaceStone(8, 8, Player.Blue);
        SearchBoard sb = new(b);
        List<Position> candidates = Candidates.GetCandidates(sb, Constants.Board.MaxSearchRadius);
        using TranspositionTable tt = new(1);
        SearchHeuristics heuristics = new();
        using TimeMonitor monitor = new(5000, CancellationToken.None);

        // Quiet position: true score is far below alpha, so the root search must fail low.
        (_, _, int score) = SearchEngine.SearchRoot(sb, Player.Red, 2, 24_000, 25_000, tt, heuristics, candidates, monitor, null);
        Assert.True(score <= 24_000, "precondition: search must fail low against a high alpha");

        Assert.True(tt.Lookup(sb.Hash(), out TTEntry entry), "root search must store its result");
        Assert.Equal(TTEntryType.UpperBound, entry.Type);
    }

    [Fact]
    public void QuiescenceIsFailSoft()
    {
        Board b = RedHasOpenFour();
        SearchBoard sb = new(b);
        SearchHeuristics heuristics = new();
        using TimeMonitor monitor = new(5000, CancellationToken.None);

        int standPat = Evaluation.Evaluate(sb, Player.Red);
        Assert.True(standPat > 200, "precondition: stand-pat must exceed beta");

        int score = SearchEngine.Quiesce(sb, Player.Red, 100, 200, Constants.Search.MaxQuiescenceDepth, heuristics, monitor, 0);
        Assert.Equal(standPat, score);
    }

    [Fact]
    public void TTSizeScalesWithLevel()
    {
        DifficultyProfile low = Difficulty.GetDifficultyProfile(1);
        DifficultyProfile high = Difficulty.GetDifficultyProfile(5);
        Assert.True(high.TTSizeMB > low.TTSizeMB);
        Assert.True(low.TTSizeMB <= 64);
    }
}

public class LmrGuardTests
{
    [Fact]
    public void LMRReductionSkipsTacticalMoves()
    {
        Assert.Equal(0, SearchEngine.LmrReduction(8, 12, true, -100));
        Assert.Equal(3, SearchEngine.LmrReduction(8, 12, false, -100));
        Assert.Equal(2, SearchEngine.LmrReduction(8, 12, false, 500));
        Assert.Equal(1, SearchEngine.LmrReduction(8, 5, false, 500));
        Assert.Equal(0, SearchEngine.LmrReduction(2, 12, false, 500));
        Assert.Equal(2, SearchEngine.LmrReduction(3, 12, false, -100));
    }

    [Fact]
    public void PickerFlagsWinningMovesAsTactical()
    {
        Board b = SearchFixesTests.RedHasOpenFour();
        SearchBoard sb = new(b);
        List<Position> candidates = Candidates.GetCandidates(sb, Constants.Board.MaxSearchRadius);

        MovePicker picker = new(candidates, sb, Player.Red, 6, null, new SearchHeuristics(), new Position(-1, -1));
        bool sawTactical = false;
        bool sawQuiet = false;
        while (picker.Next(out Position m))
        {
            if (picker.LastMoveTactical())
            {
                sawTactical = true;
                Assert.True(MoveOrdering.WouldWin(sb, m.X, m.Y, Player.Red),
                    "only winning completions may be flagged tactical from the winning stage");
            }
            else
            {
                sawQuiet = true;
            }
        }
        Assert.True(sawTactical, "winning completions must be flagged");
        Assert.True(sawQuiet, "quiet moves must not be flagged");
    }

    [Fact]
    public void PickerFlagsMustBlockMovesAsTactical()
    {
        Board b = Board.NewBoard();
        for (int x = 3; x <= 6; x++)
        {
            b = b.PlaceStone(x, 5, Player.Blue);
        }
        b = b.PlaceStone(10, 10, Player.Red);
        SearchBoard sb = new(b);
        List<Position> candidates = Candidates.GetCandidates(sb, Constants.Board.MaxSearchRadius);

        MovePicker picker = new(candidates, sb, Player.Red, 6, null, new SearchHeuristics(), new Position(-1, -1));
        bool sawBlockTactical = false;
        while (picker.Next(out Position m))
        {
            if ((m.X == 2 || m.X == 7) && m.Y == 5 && picker.LastMoveTactical())
            {
                sawBlockTactical = true;
            }
        }
        Assert.True(sawBlockTactical,
            "the only moves that stop an opponent open four must be flagged tactical");
    }
}

public class ScoreHierarchyTests
{
    [Fact]
    public void ScoreHierarchyOrdering()
    {
        Assert.True(Constants.Score.Infinity > Constants.Score.WinScore);
        Assert.True(Constants.Score.WinScore > Constants.Score.MaxEval);
        Assert.True(Constants.Score.MaxEval > Evaluation.Flex4WinBonus);
    }

    [Fact]
    public void AspirationWindowGomokuScale()
    {
        Assert.True(Constants.Search.AspirationWindowSize >= Evaluation.Flex3Score);
    }

    [Fact]
    public void FiveScoreEqualsWinScore()
    {
        Assert.Equal(Constants.Score.WinScore, Evaluation.FiveScore);
    }

    [Fact]
    public void MaxCorrectedEvalEqualsMaxEval()
    {
        Assert.Equal(Constants.Score.MaxEval, Evaluation.MaxCorrectedEval);
    }

    [Fact]
    public void FiveScoreBoundedByWinScore()
    {
        Assert.True(Evaluation.FiveScore <= Constants.Score.WinScore);
    }

    [Fact]
    public void EvalClampedBelowWinScore()
    {
        Board b = Board.NewBoard();
        for (int x = 3; x < 7; x++)
        {
            b = b.PlaceStone(x, 5, Player.Red);
        }
        SearchBoard sb = new(b);
        int score = Evaluation.Evaluate(sb, Player.Red);
        Assert.True(score < Constants.Score.WinScore);
    }

    [Fact]
    public void SearchNoGhostScores()
    {
        Board b = Board.NewBoard();
        for (int x = 3; x < 7; x++)
        {
            b = b.PlaceStone(x, 5, Player.Red);
        }
        b = b.PlaceStone(10, 10, Player.Blue);

        using TranspositionTable tt = new(1);
        SearchHeuristics heuristics = new();
        SearchConfig opts = new() { MaxDepth = 20, TimeLimitMs = 1, Threads = 1 };

        (_, _, SearchStats stats) = SearchEngine.SearchPosition(b, Player.Red, opts, tt, heuristics, CancellationToken.None);
        Assert.NotEqual(-60_000, stats.SearchScore);
        Assert.NotEqual(60_000, stats.SearchScore);
    }

    [Fact]
    public void MateInOneBeatsMateInThree()
    {
        Board b = Board.NewBoard();
        for (int x = 3; x < 7; x++)
        {
            b = b.PlaceStone(x, 5, Player.Red);
        }
        b = b.PlaceStone(10, 10, Player.Blue);

        using TranspositionTable tt = new(1);
        SearchHeuristics heuristics = new();
        SearchConfig opts = new() { MaxDepth = 4, TimeLimitMs = 5000, Threads = 1 };

        (_, _, SearchStats stats) = SearchEngine.SearchPosition(b, Player.Red, opts, tt, heuristics, CancellationToken.None);
        Assert.True(stats.SearchScore >= Constants.Score.WinScore - Constants.Search.AbsoluteMaxDepth);
        Assert.True(stats.SearchScore < Constants.Score.WinScore);
    }

    [Fact]
    public void TTMateScoreRoundTrip()
    {
        int plyStored = 3;
        int mateScore = Constants.Score.WinScore - plyStored;

        int stored = MateScore.AdjustForStore(mateScore, plyStored);
        Assert.True(stored >= Constants.Score.WinScore);

        int plyRetrieve = 5;
        int retrieved = MateScore.AdjustForRetrieve(stored, plyRetrieve);
        Assert.Equal(Constants.Score.WinScore - plyRetrieve, retrieved);

        int retrievedEarly = MateScore.AdjustForRetrieve(stored, 1);
        Assert.True(retrievedEarly > retrieved);
        Assert.Equal(Constants.Score.WinScore - 1, retrievedEarly);
    }

    [Fact]
    public void TTNonMateScoreUnchanged()
    {
        const int normalScore = 5000;
        Assert.Equal(normalScore, MateScore.AdjustForStore(normalScore, 3));
        Assert.Equal(normalScore, MateScore.AdjustForRetrieve(normalScore, 3));
    }

    [Fact]
    public void AbortPreservesPreviousDepth()
    {
        Board b = Board.NewBoard();
        for (int x = 3; x < 7; x++)
        {
            b = b.PlaceStone(x, 5, Player.Red);
        }
        b = b.PlaceStone(10, 10, Player.Blue);

        using TranspositionTable tt = new(1);
        SearchHeuristics heuristics = new();
        SearchConfig opts = new() { MaxDepth = 20, TimeLimitMs = 200, Threads = 1 };

        (_, _, SearchStats stats) = SearchEngine.SearchPosition(b, Player.Red, opts, tt, heuristics, CancellationToken.None);
        Assert.True(stats.DepthAchieved > 0);
        Assert.True(stats.SearchScore < Constants.Score.WinScore);
    }

    [Fact]
    public void AbortDoesNotPoisonTT()
    {
        Board b = Board.NewBoard()
            .PlaceStone(8, 8, Player.Red)
            .PlaceStone(9, 9, Player.Blue);

        using TranspositionTable tt = new(1);
        SearchHeuristics heuristics = new();
        SearchConfig opts = new() { MaxDepth = 20, TimeLimitMs = 1, Threads = 1 };

        SearchEngine.SearchPosition(b, Player.Red, opts, tt, heuristics, CancellationToken.None);

        SearchConfig opts2 = new() { MaxDepth = 6, TimeLimitMs = 5000, Threads = 1 };
        tt.ResetStats();
        (_, _, SearchStats stats2) = SearchEngine.SearchPosition(b, Player.Red, opts2, tt, heuristics, CancellationToken.None);
        Assert.True(stats2.SearchScore > -Constants.Score.WinScore);
    }
}

public class HeuristicsRetentionTests
{
    [Fact]
    public void HeuristicsAgeForNewMoveHalvesTables()
    {
        SearchHeuristics h = new();
        h.RecordHistory(Player.Red, 7, 7, 10);  // 100
        h.RecordHistory(Player.Blue, 8, 8, 4);  // 16
        h.RecordContHistory(Player.Red, 5, 5, 6, 6, 10);
        h.RecordKiller(4, new Position(3, 3));
        h.RecordCounterMove(Player.Red, 5, 5, 6, 6);

        h.AgeForNewMove();

        Assert.Equal(50, h.HistoryScore(Player.Red, 7, 7));
        Assert.Equal(8, h.HistoryScore(Player.Blue, 8, 8));
        Assert.Equal(150, h.ContHistoryScore(Player.Red, 5, 5, 6, 6));
        Assert.True(h.IsKiller(4, new Position(3, 3)));
        Position cm = h.CounterMoveFor(Player.Red, 5, 5);
        Assert.Equal(6, cm.X);
    }

    [Fact]
    public void GetBestMoveAgesHeuristicsInsteadOfClearing()
    {
        using MinimaxAI ai = new(1, 1);
        ai.Heuristics.RecordHistory(Player.Red, 7, 7, 10);
        int seeded = ai.Heuristics.HistoryScore(Player.Red, 7, 7);

        Board b = Board.NewBoard()
            .PlaceStone(8, 8, Player.Red)
            .PlaceStone(9, 9, Player.Blue);
        SearchOptions opts = new() { TimeRemainingMs = 0, MoveNumber = 1, ThreadCount = 1, TimeFraction = 1.0 };
        ai.GetBestMove(b, Player.Red, opts, CancellationToken.None);

        int after = ai.Heuristics.HistoryScore(Player.Red, 7, 7);
        Assert.True(after >= seeded / 2,
            "game-level ordering knowledge must carry to the next move (aged), not be wiped");
        Assert.True(after <= seeded);
    }

    [Fact]
    public void ParallelSearchPreservesHeuristics()
    {
        SearchHeuristics shared = new();
        shared.RecordHistory(Player.Red, 7, 7, 10);
        int seeded = shared.HistoryScore(Player.Red, 7, 7);

        Board b = Board.NewBoard()
            .PlaceStone(8, 8, Player.Red)
            .PlaceStone(9, 9, Player.Blue)
            .PlaceStone(7, 7, Player.Red)
            .PlaceStone(10, 10, Player.Blue);
        using TranspositionTable tt = new(1);
        SearchConfig opts = new() { MaxDepth = 6, TimeLimitMs = 300, SoftLimitMs = 250, Threads = 4 };

        ParallelSearch.Run(b, Player.Red, opts, tt, shared, CancellationToken.None);

        Assert.True(shared.HistoryScore(Player.Red, 7, 7) >= seeded,
            "the shared heuristics must survive a parallel search (worker 0 evolves it, never wipes it)");
    }
}
