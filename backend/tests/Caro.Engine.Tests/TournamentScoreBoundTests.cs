using Caro.Domain;
using Xunit;

namespace Caro.Engine.Tests;

/// <summary>
/// End-to-end guard for the score-bound corruption recorded in the
/// l1v5-100 draw games: replays game 7 (seed 20260828) with a single
/// L5-profile AI holding one transposition table across moves, exactly as
/// the live session did, and asserts no returned score ever leaves the
/// valid range [-WinScore, WinScore] (docs/artifacts/tournaments/
/// ANOMALIES.md, findings 2 and 3).
/// </summary>
public class TournamentScoreBoundTests
{
    private const string Game7Moves =
        "11,7;8,5;7,5;9,7;7,6;9,6;9,8;7,4;10,7;9,4;9,3;8,4;6,4;8,7;5,3;4,2;10,4;10,5;7,8;11,4;12,3;5,2;6,3;6,2;7,3;8,3;7,2;11,6;12,7;11,5;12,5;11,2;11,3;10,3;12,1;12,4;8,8;10,6;13,3;6,8;10,8;11,8;4,6;12,6;13,6;5,5;13,7;14,7;5,6;6,6;4,3;3,3;13,5;13,4;4,4;8,1;8,2;6,7;4,5;4,8;5,4;3,6;3,4;2,4;6,5;3,2;1,2;2,3;7,7;7,9;13,8;13,9;1,5;1,4;2,5;6,9;6,10;9,11;4,1;9,9;8,9;8,11;10,11;7,10;9,12;8,12;8,10;10,10;7,13;3,9;13,2;2,8;3,8;2,9;2,7;11,10;4,9;5,10;1,7;14,10;9,10;11,12;2,6;3,5;12,10;11,11;11,13;12,11;10,13;9,13;7,11;14,9;14,8;12,12;13,13;5,7;2,10;7,1;6,1;3,10;12,13;14,13;13,12;13,14;14,11;11,14;1,9;1,10;1,8;4,11;3,12;6,11;12,14;5,12;6,13;5,11;5,13;4,13;3,14;3,11;2,11;6,12;2,14;2,12;5,14;4,14;1,13;4,15;4,12;3,7;1,11;7,15;2,13;14,14;14,6;3,15;2,1;3,1;5,15;0,12;12,2;8,15;1,1;11,1;2,2;0,8;13,10;0,9;0,10;0,13;15,11;15,9;10,9;15,8;15,7;14,4;9,15;14,3;4,10;13,15;15,12;1,14;14,2;0,7;0,6;12,8;13,1;12,0;10,2;11,0;14,15;3,0;10,0;1,0;15,14;11,15;0,14;15,10;2,0;0,2;2,15;0,1;5,9;15,4;1,15;15,5;15,6;15,1;15,3;3,13;7,12;7,0;9,2;6,0;8,13;9,14;10,14;8,14;7,14;13,11;0,3;14,1;13,0;10,1;6,14;14,5;9,0;14,12;1,3;0,15;15,13;0,5;5,8;0,4;11,9;5,1;4,7;9,1;12,9;15,2;14,0;6,15;0,11;10,12;1,6";

    [Fact]
    public void Game7EndgameScoresStayWithinWinScore()
    {
        DifficultyProfile profile = Difficulty.GetDifficultyProfile(5, CaroConfig.Default);
        using MinimaxAI ai = new(1, profile.TTSizeMB, CaroConfig.Default.TimeManagement);

        string[] cells = Game7Moves.Split(';', StringSplitOptions.RemoveEmptyEntries);
        Board b = TournamentReplay.BoardAt(20260828L, Game7Moves, 0, out _);
        int worst = 0;

        for (int i = 0; i < cells.Length; i++)
        {
            // The corruption recorded in the archive appeared from move 245
            // on; searching the endgame prefix keeps the guard fast while
            // still warming the transposition table across many moves.
            if (i % 2 != 0 && i >= 200)
            {
                // Blue (L5) searches with a persistent TT, like the live session.
                SearchOptions opts = new()
                {
                    TimeRemainingMs = 1000,
                    IncrementMs = 0,
                    MoveNumber = i + 2,
                    ThreadCount = 1,
                    ParallelEnabled = false,
                    TimeFraction = profile.TimeFraction,
                    UseVCF = profile.UseVCF,
                    VCFMaxDepth = profile.VCFDepth,
                    MaxDepth = profile.MaxDepth,
                };
                (_, _, SearchStats stats) = ai.GetBestMove(b, Player.Blue, opts, CancellationToken.None);
                worst = Math.Max(worst, Math.Abs(stats.SearchScore));
                Assert.True(
                    Math.Abs(stats.SearchScore) <= Constants.Score.WinScore,
                    $"move {i + 2}: score {stats.SearchScore} escapes the WinScore bound");
            }

            string[] xy = cells[i].Split(',');
            b = b.PlaceStone(int.Parse(xy[0], System.Globalization.CultureInfo.InvariantCulture), int.Parse(xy[1], System.Globalization.CultureInfo.InvariantCulture), i % 2 == 0 ? Player.Red : Player.Blue);
        }

        Assert.True(worst > 0, "fixture should have run at least one search");
    }
}
