using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;
using Xunit;
using Xunit.Abstractions;

namespace Caro.Core.Tests.GameLogic;

/// <summary>
/// Benchmarks comparing SearchBoard (mutable) vs Board (immutable) performance.
/// Run with: dotnet test --filter "FullyQualifiedName~SearchBoardBenchmark"
/// </summary>
public class SearchBoardBenchmarks
{
    private readonly ITestOutputHelper _output;

    public SearchBoardBenchmarks(ITestOutputHelper output)
    {
        _output = output;
    }

    /// <summary>
    /// Benchmark: Compare PlaceStone vs MakeMove/UnmakeMove for simulating a search pattern.
    /// This simulates a depth-4 search with 10 moves at each depth.
    /// </summary>
    [Fact]
    public void Benchmark_SearchPattern_ComparePerformance()
    {
        const int iterations = 1000;
        const int depth = 4;
        const int movesPerDepth = 10;
        var testMoves = GenerateTestMoves(movesPerDepth * depth);

        // Warmup
        RunImmutableSearch(testMoves, depth, movesPerDepth);
        RunMutableSearch(testMoves, depth, movesPerDepth);

        // Benchmark immutable Board
        var immutableSw = System.Diagnostics.Stopwatch.StartNew();
        for (int i = 0; i < iterations; i++)
        {
            RunImmutableSearch(testMoves, depth, movesPerDepth);
        }
        immutableSw.Stop();

        // Benchmark mutable SearchBoard
        var mutableSw = System.Diagnostics.Stopwatch.StartNew();
        for (int i = 0; i < iterations; i++)
        {
            RunMutableSearch(testMoves, depth, movesPerDepth);
        }
        mutableSw.Stop();

        var immutableMs = immutableSw.ElapsedMilliseconds;
        var mutableMs = mutableSw.ElapsedMilliseconds;
        var speedup = (double)immutableMs / mutableMs;

        _output.WriteLine($"=== SearchBoard Performance Benchmark ===");
        _output.WriteLine($"Iterations: {iterations}");
        _output.WriteLine($"Depth: {depth}, Moves per depth: {movesPerDepth}");
        _output.WriteLine($"Immutable Board: {immutableMs}ms ({immutableMs * 1000.0 / iterations:F2}μs/iter)");
        _output.WriteLine($"Mutable SearchBoard: {mutableMs}ms ({mutableMs * 1000.0 / iterations:F2}μs/iter)");
        _output.WriteLine($"Speedup: {speedup:F2}x");

        // Mutable should be faster (at least 1.5x expected for this pattern)
        // We use a lower threshold to account for test variance
        Assert.True(speedup > 1.0,
            $"SearchBoard should be faster. Immutable: {immutableMs}ms, Mutable: {mutableMs}ms, Speedup: {speedup:F2}x");
    }

    private static List<(int x, int y)> GenerateTestMoves(int count)
    {
        var moves = new List<(int x, int y)>();
        var start = 5;
        for (int i = 0; i < count; i++)
        {
            int x = start + (i % 8);
            int y = start + ((i / 8) % 8);
            moves.Add((x, y));
        }
        return moves;
    }

    private static void RunImmutableSearch(List<(int x, int y)> moves, int depth, int branchFactor)
    {
        var board = new Board();
        int moveIndex = 0;
        SimulateImmutableSearch(board, moves, ref moveIndex, depth, branchFactor, Player.Red);
    }

    private static int SimulateImmutableSearch(Board board, List<(int x, int y)> moves, ref int moveIndex, int depth, int branchFactor, Player player)
    {
        if (depth == 0 || moveIndex >= moves.Count)
            return 0;

        int totalNodes = 1;

        for (int i = 0; i < branchFactor && moveIndex < moves.Count; i++)
        {
            var (x, y) = moves[moveIndex++];
            if (board.IsEmpty(x, y))
            {
                var newBoard = board.PlaceStone(x, y, player);
                var nextPlayer = player == Player.Red ? Player.Blue : Player.Red;
                totalNodes += SimulateImmutableSearch(newBoard, moves, ref moveIndex, depth - 1, branchFactor, nextPlayer);
            }
        }

        return totalNodes;
    }

    private static void RunMutableSearch(List<(int x, int y)> moves, int depth, int branchFactor)
    {
        var board = new SearchBoard();
        int moveIndex = 0;
        SimulateMutableSearch(board, moves, ref moveIndex, depth, branchFactor, Player.Red);
    }

    private static int SimulateMutableSearch(SearchBoard board, List<(int x, int y)> moves, ref int moveIndex, int depth, int branchFactor, Player player)
    {
        if (depth == 0 || moveIndex >= moves.Count)
            return 0;

        int totalNodes = 1;

        for (int i = 0; i < branchFactor && moveIndex < moves.Count; i++)
        {
            var (x, y) = moves[moveIndex++];
            if (board.IsEmpty(x, y))
            {
                var undo = board.MakeMove(x, y, player);
                var nextPlayer = player == Player.Red ? Player.Blue : Player.Red;
                totalNodes += SimulateMutableSearch(board, moves, ref moveIndex, depth - 1, branchFactor, nextPlayer);
                board.UnmakeMove(undo);
            }
        }

        return totalNodes;
    }
}
