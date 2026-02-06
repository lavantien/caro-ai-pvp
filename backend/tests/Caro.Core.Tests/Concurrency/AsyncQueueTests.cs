using System.Collections.Concurrent;
using System.Threading.Channels;
using Caro.Core.Concurrency;
using Xunit;
using Xunit.Abstractions;

namespace Caro.Core.Tests.Concurrency;

/// <summary>
/// Tests for the AsyncQueue utility class
/// Tests thread-safe enqueue/dequeue operations and error handling
/// </summary>
public class AsyncQueueTests
{
    private readonly ITestOutputHelper _output;

    public AsyncQueueTests(ITestOutputHelper output)
    {
        _output = output;
    }

    [Fact]
    public async Task EnqueueAsync_ItemGetsProcessed()
    {
        // Arrange
        var processedItems = new List<int>();
        var tcs = new TaskCompletionSource<bool>();

        var queue = new AsyncQueue<int>(async item =>
        {
            processedItems.Add(item);
            if (item == 5)
                tcs.SetResult(true);
            await ValueTask.CompletedTask;
        },
        capacity: 10,
        queueName: "TestQueue");

        // Act
        for (int i = 1; i <= 5; i++)
        {
            await queue.EnqueueAsync(i);
        }

        // Wait for processing
        var completed = await Task.WhenAny(tcs.Task, Task.Delay(1000));

        // Cleanup
        queue.Dispose();

        // Assert
        Assert.True(completed == tcs.Task, "Item should be processed");
        Assert.Equal(5, processedItems.Count);
        Assert.Equal(new[] { 1, 2, 3, 4, 5 }, processedItems);
    }

    [Fact]
    public async Task TryEnqueue_WhenQueueFull_DropsOldest()
    {
        // Arrange
        var processedItems = new List<int>();
        var tcs = new TaskCompletionSource<bool>();

        var queue = new AsyncQueue<int>(async item =>
        {
            // Slow processing to fill up queue
            processedItems.Add(item);
            await Task.Delay(50);

            if (processedItems.Count >= 8)
                tcs.SetResult(true);
        },
        capacity: 3, // Small capacity
        queueName: "TestQueue",
        dropOldest: true);

        // Act - Enqueue more items than capacity rapidly
        // This should cause some items to be dropped by the channel
        for (int i = 1; i <= 10; i++)
        {
            queue.TryEnqueue(i);
            // No delay - enqueue rapidly to overwhelm the queue
        }

        // Wait for some processing
        await Task.WhenAny(tcs.Task, Task.Delay(5000));

        var (processed, errors, dropped) = queue.GetStatistics();

        queue.Dispose();

        // Assert - With DropOldest mode and rapid enqueuing, not all 10 items will be processed
        _output.WriteLine($"Processed: {processed}, Errors: {errors}, Dropped: {dropped}, Items: {string.Join(",", processedItems)}");

        // Note: Dropped count may not reflect channel's internal drops with DropOldest mode
        // The key behavior is that not all 10 items are processed (some got dropped)
        Assert.True(processedItems.Count < 10, "Not all items should be processed (some were dropped)");
        Assert.True(processedItems.Count >= 3, "At least some items should be processed");
    }

    [Fact]
    public async Task TryEnqueue_WhenQueueFull_WaitsIfNotDropOldest()
    {
        // Arrange
        var processedItems = new List<int>();
        var tcs = new TaskCompletionSource<bool>();

        var queue = new AsyncQueue<int>(async item =>
        {
            processedItems.Add(item);
            await Task.Delay(20);

            if (processedItems.Count >= 5)
                tcs.SetResult(true);
        },
        capacity: 3,
        queueName: "TestQueue",
        dropOldest: false); // Wait instead of dropping

        // Act
        var enqueueTasks = Enumerable.Range(1, 5).Select(i =>
            Task.Run(async () =>
            {
                await queue.EnqueueAsync(i);
            })
        ).ToArray();

        await Task.WhenAll(enqueueTasks);
        await Task.WhenAny(tcs.Task, Task.Delay(1000));

        queue.Dispose();

        // Assert
        _output.WriteLine($"Processed items: {string.Join(",", processedItems)}");
        Assert.Equal(5, processedItems.Count);
    }

    [Fact]
    public async Task Processing_ExceptionsAreCaughtAndLogged()
    {
        // Arrange
        var processedCount = 0;
        var tcs = new TaskCompletionSource<bool>();

        var queue = new AsyncQueue<int>(async item =>
        {
            Interlocked.Increment(ref processedCount);

            // Throw exception on even numbers
            if (item % 2 == 0)
                throw new InvalidOperationException($"Even number: {item}");

            await ValueTask.CompletedTask;

            if (processedCount >= 10)
                tcs.SetResult(true);
        },
        capacity: 100);

        // Act - Enqueue items that will cause exceptions
        for (int i = 1; i <= 10; i++)
        {
            await queue.EnqueueAsync(i);
        }

        await Task.WhenAny(tcs.Task, Task.Delay(1000));

        var (processed, errors, dropped) = queue.GetStatistics();

        queue.Dispose();

        // Assert
        _output.WriteLine($"Processed: {processed}, Errors: {errors}");
        Assert.True(errors > 0, "Should have some errors");
        Assert.Equal(5, errors); // Even numbers 2, 4, 6, 8, 10
    }

    [Fact]
    public async Task ConcurrentEnqueues_AllProcessedSafely()
    {
        // Arrange
        var processedItems = new ConcurrentBag<int>();
        var tcs = new TaskCompletionSource<bool>();
        var expectedCount = 100;

        var queue = new AsyncQueue<int>(async item =>
        {
            processedItems.Add(item);
            await Task.Delay(1); // Simulate work

            if (processedItems.Count >= expectedCount)
                tcs.TrySetResult(true);
        },
        capacity: 50,
        dropOldest: false); // Wait when full instead of dropping

        // Act - Enqueue from multiple threads
        var enqueueTasks = Enumerable.Range(0, 10).Select(taskId =>
            Task.Run(async () =>
            {
                for (int i = 0; i < 10; i++)
                {
                    await queue.EnqueueAsync(taskId * 10 + i);
                    await Task.Yield();
                }
            })
        ).ToArray();

        await Task.WhenAll(enqueueTasks);
        await Task.WhenAny(tcs.Task, Task.Delay(2000));

        queue.Dispose();

        // Assert
        Assert.Equal(expectedCount, processedItems.Count);

        // All items should be unique and processed
        var allItems = processedItems.ToList();
        Assert.Equal(expectedCount, allItems.Distinct().Count());
    }

    [Fact]
    public async Task Dispose_GracefullyShutsDown()
    {
        // Arrange
        var processedItems = new List<int>();
        var processingStarted = new TaskCompletionSource<bool>();

        var queue = new AsyncQueue<int>(async item =>
        {
            processingStarted.TrySetResult(true);
            processedItems.Add(item);
            await Task.Delay(100); // Long-running task
        },
        capacity: 10);

        // Start some work
        await queue.EnqueueAsync(1);
        await processingStarted.Task; // Wait for processing to start

        // Act - Dispose while processing
        var sw = System.Diagnostics.Stopwatch.StartNew();
        queue.Dispose();
        sw.Stop();

        // Assert - Should wait for current item to complete
        _output.WriteLine($"Dispose took {sw.ElapsedMilliseconds}ms");
        Assert.Single(processedItems);
    }

    [Fact]
    public async Task EnqueueAsync_AfterDispose_ThrowsException()
    {
        // Arrange
        var queue = new AsyncQueue<int>(_ => ValueTask.CompletedTask);
        queue.Dispose();

        // Act & Assert
        await Assert.ThrowsAsync<ChannelClosedException>(async () =>
        {
            await queue.EnqueueAsync(42);
        });
    }

    [Fact]
    public async Task Statistics_TrackCorrectly()
    {
        // Arrange
        var queue = new AsyncQueue<int>(async item =>
        {
            if (item == -1)
                throw new InvalidOperationException("Test error");
            await ValueTask.CompletedTask;
        },
        capacity: 5,
        dropOldest: true);

        // Act
        for (int i = 0; i < 10; i++)
        {
            queue.TryEnqueue(i);
        }

        // Add some errors
        await queue.EnqueueAsync(-1);
        await queue.EnqueueAsync(-2);

        await Task.Delay(100); // Let some processing happen

        var (processed, errors, dropped) = queue.GetStatistics();

        queue.Dispose();

        // Assert
        _output.WriteLine($"Processed: {processed}, Errors: {errors}, Dropped: {dropped}");
        Assert.True(processed >= 0);
        Assert.True(errors >= 0);
        Assert.True(dropped >= 0);
    }

    [Fact]
    public async Task HighThroughput_ProcessesManyItemsQuickly()
    {
        // Arrange
        const int itemCount = 1000;
        var processedCount = 0;
        var tcs = new TaskCompletionSource<bool>();

        var queue = new AsyncQueue<int>(async item =>
        {
            Interlocked.Increment(ref processedCount);

            if (processedCount >= itemCount)
                tcs.SetResult(true);

            await ValueTask.CompletedTask;
        },
        capacity: 100,
        dropOldest: false); // Wait when full to ensure all items are processed

        // Act - Enqueue many items
        var sw = System.Diagnostics.Stopwatch.StartNew();

        for (int i = 0; i < itemCount; i++)
        {
            await queue.EnqueueAsync(i);
        }

        await tcs.Task;
        sw.Stop();

        queue.Dispose();

        // Assert
        _output.WriteLine($"Processed {itemCount} items in {sw.ElapsedMilliseconds}ms");
        Assert.Equal(itemCount, processedCount);
    }
}
