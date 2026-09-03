using Caro.Domain;
using Xunit;

namespace Caro.Engine.Tests;

/// <summary>
/// Regression fixtures for the phantom VCF forced wins recorded in the
/// round-robin archives: the eventual loser played [VCF] chain moves
/// (score 30000) that never converted (docs/artifacts/tournaments/
/// ANOMALIES.md, finding 1). Positions are rebuilt from the game seeds
/// plus the recorded moves; L5 runs the solver at its configured depth 12.
/// The phantom positions must never again return Win; the converting
/// position must keep returning Win.
/// </summary>
public class TournamentVcfRegressionTests
{
    private const int L5VcfDepth = 12;

    // l1v5-100 game 67 (seed 20260888): blue L5 claimed chain-12 wins from
    // move 35 on and lost the game at move 75.
    private const string Game67Moves =
        "8,10;5,11;6,10;4,10;3,9;6,11;7,11;7,10;8,9;5,9;3,11;6,9;3,8;3,10;8,11;8,8;8,13;8,14;7,13;6,13;2,10;5,12;4,11;4,12;3,12;7,7;4,8;5,7;2,8;5,8;2,13;1,14;4,7";

    // initial-5 game 28 (seed 20260849): blue L5 claimed chain 6 at move 31
    // and chain 12 at moves 95-105, and lost at move 109.
    private const string Initial5Game28Moves =
        "9,3;8,2;8,6;9,2;10,2;9,4;8,4;7,5;7,6;10,6;10,5;8,7;9,5;10,4;10,1;11,5;9,1;12,4;9,7;11,4;13,4;11,6;9,8;9,9;5,6;6,6;11,3;12,6;10,8;11,7;11,9;12,10;12,5;12,7;12,3;13,7;14,8;12,8;12,9;13,6;14,6;13,3;14,2;14,5;13,9;10,7;14,7;10,9;11,8;8,8;7,7;6,8;6,7;7,9;8,9;7,8;14,10;14,9;5,7;5,8;4,8;4,7;8,1;7,1;11,10;10,11;11,1;12,1;11,12;11,11;8,11;7,2;7,10;11,2;13,0;10,10;12,12;10,12;10,13;6,2;5,2;9,13;8,14;9,12;6,11;5,12;9,11;5,3;7,0;4,11;4,4;3,10;2,9";

    // l1v5-100 game 45 (seed 20260866): blue L5's chain converted and blue
    // won; the solver must keep finding this win.
    private const string Game45Moves =
        "8,10;7,9;8,9;8,8;9,7;7,10;7,8;6,7;6,6;5,8;8,11;5,9;8,12;8,13;7,6;6,10;9,6;3,6;4,7;3,9;4,9";

    [Fact]
    public void Game67Move35BlueHasNoForcedVcfWin()
    {
        Board b = TournamentReplay.BoardAt(20260888L, Game67Moves, 33, out Player mover);
        Assert.Equal(Player.Blue, mover);

        // The archived run claimed a chain-12 win here in ~630k nodes and
        // the game refuted it. With end-block replies explored the claim
        // must not reappear; a full refutation can exceed the budget, in
        // which case Timeout (the fail-safe used in play) is acceptable.
        VcfSearchResult r = Vcf.SolveVCFWithDepth(b, Player.Blue, L5VcfDepth, 3_000, CancellationToken.None);
        Assert.NotEqual(VCFResult.Win, r.Result);
    }

    [Fact]
    public void Initial5Game28Move95BlueHasNoForcedVcfWin()
    {
        Board b = TournamentReplay.BoardAt(20260849L, Initial5Game28Moves, 93, out Player mover);
        Assert.Equal(Player.Blue, mover);

        VcfSearchResult r = Vcf.SolveVCFWithDepth(b, Player.Blue, L5VcfDepth, 3_000, CancellationToken.None);
        Assert.NotEqual(VCFResult.Win, r.Result);
    }

    [Fact]
    public void Game45Move23BlueForcedVcfWinStillConverts()
    {
        Board b = TournamentReplay.BoardAt(20260866L, Game45Moves, 21, out Player mover);
        Assert.Equal(Player.Blue, mover);

        VcfSearchResult r = Vcf.SolveVCFWithDepth(b, Player.Blue, L5VcfDepth, 10_000, CancellationToken.None);
        Assert.Equal(VCFResult.Win, r.Result);
    }
}
