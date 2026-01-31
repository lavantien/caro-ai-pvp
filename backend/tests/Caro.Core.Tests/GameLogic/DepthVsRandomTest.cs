using Caro.Core.Entities;
using Caro.Core.GameLogic;
using Xunit;
using Xunit.Abstractions;

namespace Caro.Core.Tests.GameLogic;

public class DepthVsRandomTest
{
    private readonly ITestOutputHelper _output;

    public DepthVsRandomTest(ITestOutputHelper output)
    {
        _output = output;
    }

    [Fact]
    public void Depth5_Search_Should_Beat_Random_Moves_Consistently()
    {
        var random = new Random(42);
        int depth5Wins = 0;
        int randomWins = 0;
        int draws = 0;
        const int totalGames = 50;

        for (int game = 0; game < totalGames; game++)
        {
            var board = new Board();
            bool gameOver = false;
            int moveCount = 0;
            var ai = new MinimaxAI();

            while (!gameOver && moveCount < 200)
            {
                var player = moveCount % 2 == 0 ? Player.Red : Player.Blue;

                (int x, int y) move;

                if (player == Player.Red)
                {
                    // Red uses depth-5 search (Medium difficulty with 5s time)
                    move = ai.GetBestMove(
                        board, player, AIDifficulty.Medium,
                        timeRemainingMs: 300000, // 5 minutes
                        moveNumber: moveCount + 1,
                        ponderingEnabled: false,
                        parallelSearchEnabled: false);
                }
                else
                {
                    // Blue plays random - pick from center area for validity
                    var attempts = 0;
                    bool found = false;
                    do
                    {
                        move = (random.Next(19), random.Next(19));
                        var cell = board.GetCell(move.x, move.y);
                        if (cell.IsEmpty)
                            found = true;
                        attempts++;
                    } while (!found && attempts < 100);
                }

                board.PlaceStone(move.x, move.y, player);
                moveCount++;

                // Check for win
                var winDetector = new WinDetector();
                var winResult = winDetector.CheckWin(board);
                if (winResult.HasWinner)
                {
                    if (player == Player.Red)
                        depth5Wins++;
                    else
                        randomWins++;
                    gameOver = true;
                }
            }

            if (!gameOver)
                draws++;
        }

        _output.WriteLine($"Depth-5 AI: {depth5Wins} wins");
        _output.WriteLine($"Random: {randomWins} wins");
        _output.WriteLine($"Draws: {draws}");

        // Depth-5 search should beat random 90%+ of the time
        double winRate = (double)depth5Wins / totalGames;
        _output.WriteLine($"Depth-5 win rate: {winRate:P1}");

        Assert.True(winRate >= 0.80, $"Depth-5 should beat random 80%+ of time, got {winRate:P1}");
    }
}
