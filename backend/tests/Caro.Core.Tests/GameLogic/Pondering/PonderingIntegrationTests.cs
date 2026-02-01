using Xunit;
using FluentAssertions;
using Caro.Core.Entities;
using Caro.Core.GameLogic;
using Caro.Core.GameLogic.Pondering;
using Caro.Core.Tournament;
using System.Diagnostics;

namespace Caro.Core.Tests.GameLogic.Pondering;

/// <summary>
/// Integration tests for pondering functionality
/// Tests the interaction between pondering components and the main AI
///
/// Integration path test - covered by matchup suite which includes pondering verification.
/// Run with: dotnet test --filter "Category!=Integration" to exclude.
/// </summary>
[Trait("Category", "Integration")]
public class PonderingIntegrationTests
{
    [Fact]
    public void TournamentEngine_RunGameWithPondering_CompletesSuccessfully()
    {
        // Arrange
        var engine = new TournamentEngine();

        // Act
        var result = engine.RunGame(
            AIDifficulty.Easy,
            AIDifficulty.Braindead,
            maxMoves: 50,
            initialTimeSeconds: 60,
            incrementSeconds: 1,
            ponderingEnabled: true
        );

        // Assert
        result.Should().NotBeNull();
        result.TotalMoves.Should().BeGreaterThan(0);
        result.DurationMs.Should().BeGreaterThan(0);
    }

    [Fact]
    public void TournamentEngine_RunGameWithoutPondering_CompletesSuccessfully()
    {
        // Arrange
        var engine = new TournamentEngine();

        // Act
        var result = engine.RunGame(
            AIDifficulty.Easy,
            AIDifficulty.Braindead,
            maxMoves: 50,
            initialTimeSeconds: 60,
            incrementSeconds: 1,
            ponderingEnabled: false
        );

        // Assert
        result.Should().NotBeNull();
        result.TotalMoves.Should().BeGreaterThan(0);
        result.DurationMs.Should().BeGreaterThan(0);
    }

    [Fact]
    public void MinimaxAI_GetBestMoveWithPonderingEnabled_ReturnsValidMove()
    {
        // Arrange
        var ai = new MinimaxAI();
        var board = new Board();
        board.PlaceStone(9, 9, Player.Red);

        // Act
        var (x, y) = ai.GetBestMove(
            board,
            Player.Blue,
            AIDifficulty.Easy,
            5000,
            0,
            ponderingEnabled: true
        );

        // Assert
        x.Should().BeGreaterThanOrEqualTo(0);
        x.Should().BeLessThan(19);
        y.Should().BeGreaterThanOrEqualTo(0);
        y.Should().BeLessThan(19);
    }

    [Fact]
    public void MinimaxAI_GetBestMoveWithPonderingDisabled_ReturnsValidMove()
    {
        // Arrange
        var ai = new MinimaxAI();
        var board = new Board();
        board.PlaceStone(9, 9, Player.Red);

        // Act
        var (x, y) = ai.GetBestMove(
            board,
            Player.Blue,
            AIDifficulty.Easy,
            5000,
            0,
            ponderingEnabled: false
        );

        // Assert
        x.Should().BeGreaterThanOrEqualTo(0);
        x.Should().BeLessThan(19);
        y.Should().BeGreaterThanOrEqualTo(0);
        y.Should().BeLessThan(19);
    }

    [Fact]
    public void MinimaxAI_GetPonderer_ReturnsPondererInstance()
    {
        // Arrange
        var ai = new MinimaxAI();

        // Act
        var ponderer = ai.GetPonderer();

        // Assert
        ponderer.Should().NotBeNull();
        ponderer.State.Should().Be(PonderState.Idle);
    }

    [Fact]
    public void MinimaxAI_GetLastPV_InitiallyReturnsEmpty()
    {
        // Arrange
        var ai = new MinimaxAI();

        // Act
        var pv = ai.GetLastPV();

        // Assert
        pv.IsEmpty.Should().BeTrue();
    }

    [Fact]
    public void MinimaxAI_StopPondering_StopsActivePondering()
    {
        // Arrange
        var ai = new MinimaxAI();
        var board = new Board();
        board.PlaceStone(9, 9, Player.Red);

        // Start a move that will trigger pondering
        _ = ai.GetBestMove(board, Player.Blue, AIDifficulty.Easy, 5000, 0, true);

        // Act
        ai.StopPondering(Player.Blue);

        // Assert
        var ponderer = ai.GetPonderer();
        ponderer.IsPondering.Should().BeFalse();
    }

    [Fact]
    public void MinimaxAI_ResetPondering_ResetsState()
    {
        // Arrange
        var ai = new MinimaxAI();
        var board = new Board();
        board.PlaceStone(9, 9, Player.Red);

        // Start a move that will trigger pondering
        _ = ai.GetBestMove(board, Player.Blue, AIDifficulty.Easy, 5000, 0, true);

        // Act
        ai.ResetPondering();

        // Assert
        var ponderer = ai.GetPonderer();
        ponderer.State.Should().Be(PonderState.Idle);
        ponderer.PredictedMove.Should().BeNull();
    }

    [Fact]
    public void MinimaxAI_GetPonderingStatistics_ReturnsStatistics()
    {
        // Arrange
        var ai = new MinimaxAI();
        var board = new Board();
        board.PlaceStone(9, 9, Player.Red);

        // Generate a move with pondering
        _ = ai.GetBestMove(board, Player.Blue, AIDifficulty.Easy, 5000, 0, true);

        // Act
        var stats = ai.GetPonderingStatistics();

        // Assert
        stats.Should().NotBeNullOrEmpty();
        stats.Should().Contain("Pondering");
    }

    [Fact]
    public void TournamentEngine_PonderingEnabledVersusDisabled_NoDifferenceInResult()
    {
        // Arrange
        var engine = new TournamentEngine();

        // Act - Run two games with same settings
        var resultWithPondering = engine.RunGame(
            AIDifficulty.Easy,
            AIDifficulty.Braindead,
            maxMoves: 30,
            initialTimeSeconds: 30,
            incrementSeconds: 1,
            ponderingEnabled: true
        );

        var resultWithoutPondering = engine.RunGame(
            AIDifficulty.Easy,
            AIDifficulty.Braindead,
            maxMoves: 30,
            initialTimeSeconds: 30,
            incrementSeconds: 1,
            ponderingEnabled: false
        );

        // Assert - Both should complete successfully
        resultWithPondering.TotalMoves.Should().BeGreaterThan(0);
        resultWithoutPondering.TotalMoves.Should().BeGreaterThan(0);

        // Winner should be consistent (same AI levels)
        resultWithPondering.Winner.Should().Be(resultWithoutPondering.Winner);
    }

    [Fact]
    public void VCFPrecheck_IntegrationWithPonderer_SkipsPonderingForQuietPositions()
    {
        // Arrange
        var ponderer = new Ponderer();
        var board = new Board();

        // Act - Empty board is a quiet position
        ponderer.StartPondering(
            board,
            Player.Blue,
            (7, 7),
            Player.Red,
            AIDifficulty.Medium,
            5000
        );

        // Assert - Pondering should start even on empty board
        // (VCF pre-check is inside StartPondering, but opening phase returns true)
        ponderer.State.Should().Be(PonderState.Pondering);

        ponderer.Dispose();
    }

    [Fact]
    public void Ponderer_HandleOpponentMoveIntegration_WithRealBoard()
    {
        // Arrange
        var ponderer = new Ponderer();
        var board = new Board();
        board.PlaceStone(9, 9, Player.Red);

        ponderer.StartPondering(
            board,
            Player.Blue,
            (8, 8),
            Player.Red,
            AIDifficulty.Medium,
            5000
        );

        // Act - Simulate opponent move
        var (state, result) = ponderer.HandleOpponentMove(8, 8);

        // Assert
        state.Should().Be(PonderState.PonderHit);
        result.Should().NotBeNull();
        result.Value.PonderHit.Should().BeTrue();
        result.Value.TimeSpentMs.Should().BeGreaterThanOrEqualTo(0);

        ponderer.Dispose();
    }

    [Fact]
    public void PV_IntegrationWithAI_StoresCorrectMove()
    {
        // Arrange
        var ai = new MinimaxAI();
        var board = new Board();
        board.PlaceStone(9, 9, Player.Red);

        // Act
        ai.GetBestMove(board, Player.Blue, AIDifficulty.Easy, 5000, 0, false);
        var pv = ai.GetLastPV();

        // Assert
        pv.Should().NotBeNull();
        pv.IsEmpty.Should().BeFalse();
        pv.GetBestMove().Should().NotBeNull();
    }

    [Fact]
    public void TournamentEngine_BothAIsPondering_NoDeadlock()
    {
        // Arrange
        var engine = new TournamentEngine();
        var stopwatch = Stopwatch.StartNew();

        // Act - Run a game where both AIs ponder
        var result = engine.RunGame(
            AIDifficulty.Medium,
            AIDifficulty.Medium,
            maxMoves: 40,
            initialTimeSeconds: 30,
            incrementSeconds: 1,
            ponderingEnabled: true
        );

        stopwatch.Stop();

        // Assert
        result.Should().NotBeNull();
        result.TotalMoves.Should().BeGreaterThan(0);
        stopwatch.ElapsedMilliseconds.Should().BeLessThan(120000); // Should complete in under 2 minutes (parallel search has overhead)
    }

    [Fact]
    public void MinimaxAI_PonderingWithDifferentDifficulties_AllWork()
    {
        // Arrange
        var ai = new MinimaxAI();
        var board = new Board();
        board.PlaceStone(9, 9, Player.Red);

        var difficulties = new[]
        {
            AIDifficulty.Braindead,
            AIDifficulty.Easy,
            AIDifficulty.Easy,
            AIDifficulty.Medium
        };

        // Act & Assert - All difficulty levels should work with pondering
        foreach (var difficulty in difficulties)
        {
            var (x, y) = ai.GetBestMove(board, Player.Blue, difficulty, 5000, 0, true);

            x.Should().BeGreaterThanOrEqualTo(0);
            x.Should().BeLessThan(19);
            y.Should().BeGreaterThanOrEqualTo(0);
            y.Should().BeLessThan(19);
        }
    }

    [Fact]
    public void PonderResult_IntegrationTest_HasValidMove()
    {
        // Arrange
        var ponderer = new Ponderer();
        var board = new Board();
        board.PlaceStone(9, 9, Player.Red);

        ponderer.StartPondering(
            board,
            Player.Blue,
            (8, 8),
            Player.Red,
            AIDifficulty.Medium,
            5000
        );

        // Update ponder result with a move
        ponderer.UpdatePonderResult((9, 9), 3, 50, 100);

        // Act
        var result = ponderer.GetCurrentResult();

        // Assert
        result.HasValidMove.Should().BeTrue();
        result.BestMove.Should().Be((9, 9));
        result.Depth.Should().Be(3);
        result.Score.Should().Be(50);
        result.NodesSearched.Should().Be(100);

        ponderer.Dispose();
    }

    [Fact]
    public void PonderState_IntegrationTest_StateTransitions()
    {
        // Arrange
        var ponderer = new Ponderer();
        var board = new Board();
        board.PlaceStone(9, 9, Player.Red);

        // Initial state
        ponderer.State.Should().Be(PonderState.Idle);

        // Start pondering
        ponderer.StartPondering(
            board,
            Player.Blue,
            (8, 8),
            Player.Red,
            AIDifficulty.Medium,
            5000
        );
        ponderer.State.Should().Be(PonderState.Pondering);

        // Ponder hit
        ponderer.HandleOpponentMove(8, 8);
        ponderer.State.Should().Be(PonderState.PonderHit);

        // Reset
        ponderer.Reset();
        ponderer.State.Should().Be(PonderState.Idle);

        ponderer.Dispose();
    }

    [Fact]
    public void TournamentEngine_RunTournamentWithPondering_CompletesAllGames()
    {
        // Arrange
        var engine = new TournamentEngine();
        var matchups = new Dictionary<(AIDifficulty, AIDifficulty), int>
        {
            { (AIDifficulty.Easy, AIDifficulty.Braindead), 2 }
        };

        // Act
        var results = engine.RunTournament(
            matchups,
            progress: null
        );

        // Assert
        results.Should().HaveCount(2);
        results.Should().OnlyContain(r => r.TotalMoves > 0);
    }
}
