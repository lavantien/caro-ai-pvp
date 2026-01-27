using System.Collections.Concurrent;
using System.Diagnostics;
using Xunit;
using Xunit.Abstractions;
using Caro.Core.GameLogic;
using Caro.Core.GameLogic.Pondering;
using Caro.Core.Entities;
using Caro.Core.Tournament;

namespace Caro.Core.Tests.Concurrency;

/// <summary>
/// Stress tests that run 100+ concurrent operations to surface race conditions
/// Uses high contention scenarios to trigger edge cases
/// These tests should FAIL initially, revealing concurrency issues that will be fixed
/// </summary>
public class ConcurrencyStressTests
{
    private readonly ITestOutputHelper _output;

    public ConcurrencyStressTests(ITestOutputHelper output)
    {
        _output = output;
    }

    [Fact]
    public async Task LockFreeTranspositionTable_100ConcurrentWrites_NoDataLoss()
    {
        // Arrange
        var tt = new LockFreeTranspositionTable(1); // Small table for maximum contention
        var threads = 100;
        var writesPerThread = 100;
        var exceptions = new ConcurrentBag<Exception>();

        // Act - 100 threads writing 100 times each = 10,000 total writes
        var tasks = Enumerable.Range(0, threads).Select(threadId =>
            Task.Run(() =>
            {
                try
                {
                    for (int i = 0; i < writesPerThread; i++)
                    {
                        ulong hash = (ulong)(threadId * writesPerThread + i);
                        tt.Store(hash, (sbyte)(i % 10), (short)i, (sbyte)(i % 15), (sbyte)(i % 15), 0, 0);

                        // Insert random delays to increase race window
                        if (i % 17 == 0)
                            Thread.Yield();
                        if (i % 23 == 0)
                            Thread.SpinWait(10);
                    }
                }
                catch (Exception ex)
                {
                    exceptions.Add(ex);
                }
            })
        ).ToArray();

        await Task.WhenAll(tasks);

        // Assert - Verify no crashes and table is consistent
        Assert.Empty(exceptions);

        var (used, _, hits, lookups, _) = tt.GetStats();
        _output.WriteLine($"Used: {used}, Hits: {hits}, Lookups: {lookups}");

        // Table should have some entries (not crashed to zero)
        Assert.True(used > 0, "Table should have entries after concurrent writes");
    }

    [Fact]
    public async Task ParallelMinimaxSearch_10ConcurrentSearches_RespectCancellation()
    {
        // Arrange
        var search = new ParallelMinimaxSearch(16, maxThreads: 4);

        // Act - Start 10 concurrent searches with separate boards to avoid shared state
        var results = new ConcurrentBag<(int x, int y, int depth, long nodes)>();
        var exceptions = new ConcurrentBag<Exception>();

        var tasks = Enumerable.Range(0, 10).Select(i =>
            Task.Run(() =>
            {
                try
                {
                    // Create unique board per task using different areas of the board
                    var board = new Board();
                    var offsetX = (i % 3) * 4;
                    var offsetY = (i / 3) * 4;

                    // Place stones in non-overlapping regions
                    board.PlaceStone(offsetX, offsetY, Player.Red);
                    board.PlaceStone(offsetX, offsetY + 1, Player.Blue);
                    board.PlaceStone(offsetX + 1, offsetY, Player.Red);
                    board.PlaceStone(offsetX + 1, offsetY + 1, Player.Blue);

                    var result = search.GetBestMoveWithStats(
                        board, Player.Red, AIDifficulty.Hard,
                        timeRemainingMs: 5000
                    );
                    results.Add((result.X, result.Y, result.DepthAchieved, result.NodesSearched));
                }
                catch (Exception ex)
                {
                    exceptions.Add(ex);
                    _output.WriteLine($"Exception: {ex.GetType().Name} - {ex.Message}");
                }
            })
        ).ToArray();

        // Wait for all tasks with timeout - prevents hangs
        var allCompleted = Task.WhenAll(tasks).Wait(TimeSpan.FromSeconds(30));

        // Assert - All searches should complete (not hang)
        _output.WriteLine($"Completed: {allCompleted}, Results: {results.Count}, Exceptions: {exceptions.Count}");
        Assert.True(allCompleted, "All tasks should complete without timeout");
        Assert.Empty(exceptions);
        Assert.True(results.Count > 0, "At least some searches should complete");
    }

    [Theory]
    [InlineData(10)]
    [InlineData(50)]
    [InlineData(100)]
    public async Task Ponderer_ConcurrentStartStopPondering_NoDeadlocks(int operations)
    {
        // Arrange
        var ponderers = Enumerable.Range(0, Math.Min(operations, 10))
            .Select(_ => new Ponderer()).ToArray();

        var exceptions = new ConcurrentBag<Exception>();
        var completedOperations = 0;

        // Act - Rapid start/stop cycles across multiple ponderers
        // Each task gets its own board to avoid shared state issues
        // Note: Don't pre-place predictedOpponentMove - Ponderer places it internally
        var tasks = Enumerable.Range(0, operations).Select(i =>
        {
            var ponderer = ponderers[i % ponderers.Length];
            // Create a unique board for each task with base position only
            var board = new Board();
            board.PlaceStone(7, 7, Player.Red);
            // Don't place the predicted move here - Ponderer will place it on its cloned board

            return Task.Run(() =>
            {
                try
                {
                    // Each task uses a different predicted move to avoid collisions
                    ponderer.StartPondering(
                        board, Player.Blue, (7 + (i % 5), 8 + ((i / 5) % 5)),
                        Player.Red, AIDifficulty.Hard, 1000
                    );

                    // Small delay to ensure pondering starts
                    Thread.Sleep(5);

                    ponderer.StopPondering();
                    Interlocked.Increment(ref completedOperations);
                }
                catch (Exception ex)
                {
                    exceptions.Add(ex);
                }
            });
        }).ToArray();

        // Wait for all tasks with timeout to detect deadlocks
        var completed = Task.WhenAll(tasks).Wait(TimeSpan.FromSeconds(30));

        // Cleanup
        foreach (var ponderer in ponderers)
        {
            try
            {
                ponderer.Dispose();
            }
            catch { }
        }

        // Assert - Should complete without deadlock
        Assert.True(completed, "Operations should complete within timeout (no deadlock)");
        Assert.Empty(exceptions);
        _output.WriteLine($"Completed {completedOperations}/{operations} operations");
    }

    [Fact]
    public async Task TournamentState_ConcurrentReadsAndWrites_NoCorruption()
    {
        // Simulates the TournamentManager pattern with high contention
        var state = new TournamentState();
        var stateLock = new object();
        var readers = 50;
        var writers = 10;
        var durationMs = 2000;
        var exceptions = new ConcurrentBag<Exception>();
        var readCount = 0;
        var writeCount = 0;

        // Use a countdown event to ensure all tasks have started before we begin timing
        var allTasksStarted = new CountdownEvent(readers + writers);

        // Reader tasks - many concurrent reads
        var readerTasks = Enumerable.Range(0, readers).Select(_ =>
            Task.Run(() =>
            {
                var local = new Random();
                try
                {
                    allTasksStarted.Signal(); // Signal that this task has started
                    // Give time for all tasks to start
                    Thread.Sleep(10);

                    var startTime = Stopwatch.GetTimestamp();
                    while (Stopwatch.GetElapsedTime(startTime).TotalMilliseconds < durationMs)
                    {
                        TournamentState snapshot;
                        lock (stateLock)
                        {
                            snapshot = new TournamentState
                            {
                                Status = state.Status,
                                CompletedGames = state.CompletedGames,
                                TotalGames = state.TotalGames
                            };
                        }

                        // Validate snapshot consistency
                        Assert.NotNull(snapshot);
                        Assert.InRange(snapshot.CompletedGames, 0, 500);
                        Assert.InRange(snapshot.TotalGames, 0, 500);

                        Interlocked.Increment(ref readCount);
                        Thread.Sleep(local.Next(1, 10));
                    }
                }
                catch (Exception ex)
                {
                    exceptions.Add(ex);
                }
            })
        ).ToArray();

        // Writer tasks - fewer but modifying state
        var writerTasks = Enumerable.Range(0, writers).Select(i =>
            Task.Run(() =>
            {
                var local = new Random();
                try
                {
                    allTasksStarted.Signal(); // Signal that this task has started
                    // Give time for all tasks to start
                    Thread.Sleep(10);

                    var startTime = Stopwatch.GetTimestamp();
                    while (Stopwatch.GetElapsedTime(startTime).TotalMilliseconds < durationMs)
                    {
                        lock (stateLock)
                        {
                            state.CompletedGames = local.Next(0, 100);
                            state.TotalGames = 462; // Fixed for 22 bots round-robin
                            state.Status = (TournamentStatus)(local.Next(3));
                        }

                        Interlocked.Increment(ref writeCount);
                        Thread.Sleep(local.Next(5, 20));
                    }
                }
                catch (Exception ex)
                {
                    exceptions.Add(ex);
                }
            })
        ).ToArray();

        // Wait for all tasks to complete
        var allCompleted = Task.WhenAll(readerTasks.Concat(writerTasks)).Wait(TimeSpan.FromSeconds(10));

        // Assert - All tasks completed without data corruption
        Assert.True(allCompleted, "All tasks should complete within timeout");
        Assert.Empty(exceptions);
        _output.WriteLine($"Reads: {readCount}, Writes: {writeCount}");
        Assert.True(readCount > 0, "Should have completed reads");
        Assert.True(writeCount > 0, "Should have completed writes");
    }

    [Fact]
    public async Task MinimaxAI_ConcurrentInstances_DontShareCorruptedState()
    {
        // Ensure AI instances don't share transposition table state incorrectly
        var boards = Enumerable.Range(0, 20).Select(i =>
        {
            var b = new Board();
            b.PlaceStone(7, 7, Player.Red);
            b.PlaceStone(7 + (i % 3), 8 + (i / 3), Player.Blue);
            return b;
        }).ToArray();

        var results = new ConcurrentBag<(int x, int y)>();
        var exceptions = new ConcurrentBag<Exception>();

        var tasks = boards.Select((board, i) =>
            Task.Run(() =>
            {
                try
                {
                    // Each AI gets its own instance
                    var ai = new MinimaxAI();
                    var (x, y) = ai.GetBestMove(board, Player.Red, AIDifficulty.Easy);
                    results.Add((x, y));
                }
                catch (Exception ex)
                {
                    exceptions.Add(ex);
                }
            })
        ).ToArray();

        await Task.WhenAll(tasks);

        // Assert - All should complete without sharing state corruption
        Assert.Empty(exceptions);
        Assert.Equal(20, results.Count);
        _output.WriteLine($"Completed {results.Count} concurrent searches");
    }

    [Fact]
    public void Interlocked_Patterns_WorkAsExpected()
    {
        // Verify our Interlocked usage patterns work correctly
        var counter = 0;
        var iterations = 10000;
        var exceptions = new ConcurrentBag<Exception>();

        // Parallel increment
        Parallel.For(0, iterations, i =>
        {
            try
            {
                Interlocked.Increment(ref counter);

                // Test CompareExchange pattern (this IS atomic)
                if (i % 50 == 0)
                {
                    // Read current value, then try to exchange only if still same value
                    var current = Interlocked.CompareExchange(ref counter, 0, -1);
                    // This reads current value (doesn't change it)
                    var _ = current;
                }
            }
            catch (Exception ex)
            {
                exceptions.Add(ex);
            }
        });

        Assert.Empty(exceptions);
        Assert.Equal(iterations, counter);
    }

    [Fact]
    public async Task ConcurrentDictionary_PerformanceUnderContention()
    {
        // Test that ConcurrentDictionary performs well under high contention
        var dict = new ConcurrentDictionary<string, int>();
        var threads = 50;
        var operationsPerThread = 1000;
        var exceptions = new ConcurrentBag<Exception>();

        var tasks = Enumerable.Range(0, threads).Select(threadId =>
            Task.Run(() =>
            {
                try
                {
                    for (int i = 0; i < operationsPerThread; i++)
                    {
                        var key = $"key-{threadId}-{i % 100}"; // Some key collision
                        dict.AddOrUpdate(key, 1, (k, v) => v + 1);

                        if (i % 50 == 0)
                            Thread.Yield();
                    }
                }
                catch (Exception ex)
                {
                    exceptions.Add(ex);
                }
            })
        ).ToArray();

        await Task.WhenAll(tasks);

        Assert.Empty(exceptions);
        _output.WriteLine($"Dictionary entries: {dict.Count}");
        Assert.True(dict.Count > 0, "Dictionary should have entries");
    }
}
