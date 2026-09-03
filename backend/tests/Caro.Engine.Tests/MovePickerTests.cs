using Caro.Domain;
using Caro.Engine;
using Xunit;

namespace Caro.Engine.Tests;

public class MovePickerTests
{
    [Fact]
    public void OrderMovesTTFirst()
    {
        SearchBoard sb = new(Board.NewBoard().PlaceStone(8, 8, Player.Red));
        SearchHeuristics h = new();

        List<Position> candidates = [new(7, 7), new(9, 9), new(6, 6)];
        Position? ttMove = new Position(9, 9);

        List<Position> ordered = MoveOrdering.OrderMoves(candidates, sb, Player.Blue, 0, ttMove, h);
        Assert.Equal(new Position(9, 9), ordered[0]);
    }

    [Fact]
    public void OrderMovesWinningMove()
    {
        Board b = Board.NewBoard();
        for (int x = 3; x < 7; x++)
        {
            b = b.PlaceStone(x, 5, Player.Red);
        }
        SearchBoard sb = new(b);
        SearchHeuristics h = new();

        List<Position> candidates = Candidates.GetCandidates(sb, 2);
        List<Position> ordered = MoveOrdering.OrderMoves(candidates, sb, Player.Red, 0, null, h);

        Position top = ordered[0];
        Assert.True((top.X == 2 || top.X == 7) && top.Y == 5,
            $"winning move should be (2,5) or (7,5), got ({top.X},{top.Y})");
    }

    [Fact]
    public void MovePickerWinningBeforeBlocking()
    {
        // Red can win immediately (4 in a row, needs 5th). Blue also has an
        // open four threat. Winning move must be yielded before blocking moves.
        Board b = Board.NewBoard()
            // Red: 4 in a row, need 5th at (7,5)
            .PlaceStone(3, 5, Player.Red).PlaceStone(4, 5, Player.Red)
            .PlaceStone(5, 5, Player.Red).PlaceStone(6, 5, Player.Red)
            // Blue: 4 in a row with both ends open
            .PlaceStone(3, 3, Player.Blue).PlaceStone(4, 3, Player.Blue)
            .PlaceStone(5, 3, Player.Blue).PlaceStone(6, 3, Player.Blue)
            .PlaceStone(10, 10, Player.Red);

        SearchBoard sb = new(b);
        List<Position> candidates = Candidates.GetCandidates(sb, Constants.Board.MaxSearchRadius);
        MovePicker picker = new(candidates, sb, Player.Red, 4, null, new SearchHeuristics(), new Position(-1, -1));

        Assert.True(picker.Next(out Position first), "should yield at least one move");
        Assert.True((first.X == 7 || first.X == 2) && first.Y == 5,
            $"winning move should be yielded before blocking moves, got ({first.X},{first.Y})");
    }

    [Fact]
    public void OrderMovesBlocksThreat()
    {
        Board b = Board.NewBoard();
        for (int x = 3; x < 7; x++)
        {
            b = b.PlaceStone(x, 5, Player.Blue);
        }
        b = b.PlaceStone(8, 8, Player.Red);
        SearchBoard sb = new(b);
        SearchHeuristics h = new();

        List<Position> candidates = Candidates.GetCandidates(sb, 2);
        List<Position> ordered = MoveOrdering.OrderMoves(candidates, sb, Player.Red, 0, null, h);

        Assert.True(ordered.Count > 0);
        Position top = ordered[0];
        Assert.True(top.X == 2 || top.X == 7,
            $"should block opponent four-in-a-row at (2,5) or (7,5), got ({top.X},{top.Y})");
    }
}

public class CandidateTests
{
    // doubleThreatBoard leaves (7,7) empty with red owning the horizontal
    // pair (5,7),(6,7) and the vertical pair (7,5),(7,6): playing the
    // junction creates two open threes at once.
    private static Board DoubleThreatBoard() =>
        Board.NewBoard()
            .PlaceStone(5, 7, Player.Red)
            .PlaceStone(6, 7, Player.Red)
            .PlaceStone(7, 5, Player.Red)
            .PlaceStone(7, 6, Player.Red);

    [Fact]
    public void IsTacticalMoveDoubleThreat()
    {
        SearchBoard sb = new(DoubleThreatBoard());

        Assert.True(Candidates.IsTacticalMove(sb, 7, 7, Player.Red, Player.Blue),
            "a move creating open threes in two directions is forcing");
    }

    [Fact]
    public void IsTacticalMoveBlockingDoubleThreat()
    {
        Board b = DoubleThreatBoard().PlaceStone(7, 7, Player.Red);
        SearchBoard sb = new(b);

        Assert.True(Candidates.IsTacticalMove(sb, 4, 7, Player.Blue, Player.Red),
            "answering an existing double threat is forcing for the defender");
    }

    [Fact]
    public void IsTacticalMoveSingleOpenThreeNotTactical()
    {
        Board b = Board.NewBoard()
            .PlaceStone(5, 7, Player.Red)
            .PlaceStone(6, 7, Player.Red)
            .PlaceStone(10, 10, Player.Blue);
        SearchBoard sb = new(b);

        Assert.False(Candidates.IsTacticalMove(sb, 7, 7, Player.Red, Player.Blue),
            "a lone open three is not forcing");
    }
}
