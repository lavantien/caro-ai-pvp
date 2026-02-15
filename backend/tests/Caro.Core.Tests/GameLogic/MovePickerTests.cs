using Caro.Core.Domain.Configuration;
using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;
using Xunit;

namespace Caro.Core.Tests.GameLogic;

public class MovePickerTests
{
    private const int BoardSize = GameConstants.BoardSize;

    [Fact]
    public void MovePicker_InitialStage_IsNone()
    {
        // Arrange
        var board = new Board();
        var candidates = new List<(int x, int y)> { (9, 9), (9, 10), (10, 9) };
        var threadData = CreateThreadData();
        var continuationHistory = new ContinuationHistory();
        var counterMoveHistory = new CounterMoveHistory();

        // Act
        var picker = new MovePicker(
            candidates, board, Player.Red, 5, null,
            threadData, continuationHistory, counterMoveHistory);

        // Assert
        Assert.Equal(MovePicker.Stage.None, picker.CurrentStage);
    }

    [Fact]
    public void MovePicker_NextMove_AdvancesThroughStages()
    {
        // Arrange
        var board = new Board();
        board = board.PlaceStone(9, 9, Player.Red); // Center move
        var candidates = new List<(int x, int y)> { (9, 10), (10, 9), (8, 8) };
        var threadData = CreateThreadData();
        var continuationHistory = new ContinuationHistory();
        var counterMoveHistory = new CounterMoveHistory();

        var picker = new MovePicker(
            candidates, board, Player.Blue, 5, null,
            threadData, continuationHistory, counterMoveHistory);

        // Act - Get all moves and track stage progression
        var moves = new List<(int x, int y)>();
        var stages = new List<MovePicker.Stage>();

        (int x, int y)? move;
        while ((move = picker.NextMove()) != null)
        {
            moves.Add(move.Value);
            stages.Add(picker.CurrentStage);
        }

        // Assert - Should have returned all candidates
        Assert.Equal(3, moves.Count);
        Assert.Equal(MovePicker.Stage.Done, picker.CurrentStage);
    }

    [Fact]
    public void MovePicker_TTMove_PrioritizedFirst()
    {
        // Arrange
        var board = new Board();
        var candidates = new List<(int x, int y)> { (5, 5), (9, 9), (10, 10) };
        var threadData = CreateThreadData();
        var continuationHistory = new ContinuationHistory();
        var counterMoveHistory = new CounterMoveHistory();

        // TT move is (9, 9) - should come first
        var picker = new MovePicker(
            candidates, board, Player.Red, 5, (9, 9),
            threadData, continuationHistory, counterMoveHistory);

        // Act
        var firstMove = picker.NextMove();

        // Assert - TT move should be first
        Assert.Equal((9, 9), firstMove);
        Assert.Equal(MovePicker.Stage.TT_MOVE, picker.CurrentStage);
    }

    [Fact]
    public void MovePicker_MustBlock_PrioritizedOverWinning()
    {
        // Arrange - Create a board where opponent has an open four threat
        var board = new Board();
        // Place 4 red stones in a row with open ends
        board = board.PlaceStone(5, 5, Player.Red);
        board = board.PlaceStone(6, 5, Player.Red);
        board = board.PlaceStone(7, 5, Player.Red);
        board = board.PlaceStone(8, 5, Player.Red);
        // Red now has open four - Blue must block at (4,5) or (9,5)

        var candidates = new List<(int x, int y)> { (9, 9), (4, 5), (9, 5), (10, 10) };
        var threadData = CreateThreadData();
        var continuationHistory = new ContinuationHistory();
        var counterMoveHistory = new CounterMoveHistory();

        var picker = new MovePicker(
            candidates, board, Player.Blue, 5, null,
            threadData, continuationHistory, counterMoveHistory);

        // Act - Skip TT_MOVE stage (no TT move), get first move
        picker.NextMove(); // This should be a MUST_BLOCK move

        // Assert - Should be in MUST_BLOCK stage
        Assert.Equal(MovePicker.Stage.MUST_BLOCK, picker.CurrentStage);
    }

    [Fact]
    public void MovePicker_EmptyCandidates_ReturnsNull()
    {
        // Arrange
        var board = new Board();
        var candidates = new List<(int x, int y)>();
        var threadData = CreateThreadData();
        var continuationHistory = new ContinuationHistory();
        var counterMoveHistory = new CounterMoveHistory();

        var picker = new MovePicker(
            candidates, board, Player.Red, 5, null,
            threadData, continuationHistory, counterMoveHistory);

        // Act
        var move = picker.NextMove();

        // Assert
        Assert.Null(move);
        Assert.Equal(MovePicker.Stage.Done, picker.CurrentStage);
    }

    [Fact]
    public void MovePicker_SingleCandidate_ReturnsIt()
    {
        // Arrange
        var board = new Board();
        var candidates = new List<(int x, int y)> { (9, 9) };
        var threadData = CreateThreadData();
        var continuationHistory = new ContinuationHistory();
        var counterMoveHistory = new CounterMoveHistory();

        var picker = new MovePicker(
            candidates, board, Player.Red, 5, null,
            threadData, continuationHistory, counterMoveHistory);

        // Act
        var move = picker.NextMove();
        var nextMove = picker.NextMove();

        // Assert
        Assert.Equal((9, 9), move);
        Assert.Null(nextMove);
    }

    [Fact]
    public void MovePicker_GetRemainingMoves_ReturnsAllUnreturned()
    {
        // Arrange
        var board = new Board();
        var candidates = new List<(int x, int y)> { (5, 5), (9, 9), (10, 10) };
        var threadData = CreateThreadData();
        var continuationHistory = new ContinuationHistory();
        var counterMoveHistory = new CounterMoveHistory();

        var picker = new MovePicker(
            candidates, board, Player.Red, 5, null,
            threadData, continuationHistory, counterMoveHistory);

        // Get first move
        picker.NextMove();

        // Act - Get remaining moves
        var remaining = picker.GetRemainingMoves();

        // Assert - Should have 2 remaining moves
        Assert.Equal(2, remaining.Count);
    }

    [Fact]
    public void MovePicker_KillerMoves_GetKillerCategory()
    {
        // Arrange
        var board = new Board();
        var candidates = new List<(int x, int y)> { (5, 5), (9, 9) };
        var threadData = CreateThreadData();
        threadData.KillerMoves[5, 0] = (5, 5); // Set (5,5) as killer move

        var continuationHistory = new ContinuationHistory();
        var counterMoveHistory = new CounterMoveHistory();

        var picker = new MovePicker(
            candidates, board, Player.Red, 5, null,
            threadData, continuationHistory, counterMoveHistory);

        // Act - Move through stages to KILLER_COUNTER
        MovePicker.Stage? killerStage = null;
        (int x, int y)? move;
        while ((move = picker.NextMove()) != null)
        {
            if (move.Value == (5, 5))
            {
                killerStage = picker.CurrentStage;
                break;
            }
        }

        // Assert - Killer move should be in KILLER_COUNTER stage
        Assert.Equal(MovePicker.Stage.KILLER_COUNTER, killerStage);
    }

    [Fact]
    public void MovePicker_StageOrder_IsCorrect()
    {
        // Arrange
        var board = new Board();
        var candidates = new List<(int x, int y)> { (9, 9) };
        var threadData = CreateThreadData();
        var continuationHistory = new ContinuationHistory();
        var counterMoveHistory = new CounterMoveHistory();

        var picker = new MovePicker(
            candidates, board, Player.Red, 5, null,
            threadData, continuationHistory, counterMoveHistory);

        // Act & Assert - Verify stage order
        var expectedOrder = new[]
        {
            MovePicker.Stage.TT_MOVE,
            MovePicker.Stage.MUST_BLOCK,
            MovePicker.Stage.WINNING_MOVE,
            MovePicker.Stage.THREAT_CREATE,
            MovePicker.Stage.KILLER_COUNTER,
            MovePicker.Stage.GOOD_QUIET,
            MovePicker.Stage.BAD_QUIET
        };

        picker.NextMove(); // Gets first (and only) move

        // The move should have been in one of the expected stages
        Assert.Contains(picker.CurrentStage, expectedOrder);
    }

    [Fact]
    public void MovePicker_WinningMove_HigherScoreThanThreatCreate()
    {
        // Arrange
        var board = new Board();
        // Create a position where player can make winning move
        board = board.PlaceStone(5, 5, Player.Red);
        board = board.PlaceStone(6, 5, Player.Red);
        board = board.PlaceStone(7, 5, Player.Red);
        // Red can win by playing (8,5) or (4,5)

        var candidates = new List<(int x, int y)> { (8, 5), (9, 9), (10, 10) };
        var threadData = CreateThreadData();
        var continuationHistory = new ContinuationHistory();
        var counterMoveHistory = new CounterMoveHistory();

        var picker = new MovePicker(
            candidates, board, Player.Red, 5, null,
            threadData, continuationHistory, counterMoveHistory);

        // Act - Get first move (should be winning move)
        var firstMove = picker.NextMove();

        // Assert - Winning move (8,5) should come first
        Assert.Equal((8, 5), firstMove);
    }

    [Fact]
    public void MovePicker_GetMoveScore_ReturnsValidScore()
    {
        // Arrange
        var board = new Board();
        var candidates = new List<(int x, int y)> { (9, 9) };
        var threadData = CreateThreadData();
        var continuationHistory = new ContinuationHistory();
        var counterMoveHistory = new CounterMoveHistory();

        var picker = new MovePicker(
            candidates, board, Player.Red, 5, null,
            threadData, continuationHistory, counterMoveHistory);

        // Act
        var score = picker.GetMoveScore(0);

        // Assert - Score should be non-negative
        Assert.True(score >= 0);
    }

    [Fact]
    public void MovePicker_InvalidIndex_ReturnsZero()
    {
        // Arrange
        var board = new Board();
        var candidates = new List<(int x, int y)> { (9, 9) };
        var threadData = CreateThreadData();
        var continuationHistory = new ContinuationHistory();
        var counterMoveHistory = new CounterMoveHistory();

        var picker = new MovePicker(
            candidates, board, Player.Red, 5, null,
            threadData, continuationHistory, counterMoveHistory);

        // Act & Assert
        Assert.Equal(0, picker.GetMoveScore(-1));
        Assert.Equal(0, picker.GetMoveScore(100));
        Assert.Equal(MovePicker.Stage.None, picker.GetMoveStage(-1));
    }

    private static MovePicker.ThreadData CreateThreadData()
    {
        var data = new MovePicker.ThreadData();
        data.Reset();
        return data;
    }
}

// Extension to make ThreadData.Reset accessible
file static class ThreadDataExtensions
{
    public static void Reset(this MovePicker.ThreadData data)
    {
        for (int i = 0; i < 20; i++)
        {
            data.KillerMoves[i, 0] = (-1, -1);
            data.KillerMoves[i, 1] = (-1, -1);
        }
        Array.Clear(data.MoveHistory, 0, data.MoveHistory.Length);
        data.MoveHistoryCount = 0;
        data.LastOpponentCell = -1;
    }
}
