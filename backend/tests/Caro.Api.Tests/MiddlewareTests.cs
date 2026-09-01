using Caro.Api;
using Microsoft.AspNetCore.Http;
using Microsoft.Extensions.Logging;
using Xunit;

namespace Caro.Api.Tests;

public class LocalOriginTests
{
    [Theory]
    [InlineData("http://localhost:5173", true)]
    [InlineData("http://127.0.0.1:5173", true)]
    [InlineData("http://[::1]:5173", true)]
    [InlineData("https://localhost:5173", false)]
    [InlineData("http://evil.example.com", false)]
    [InlineData("not a url", false)]
    [InlineData("", false)]
    public void IsLocalOriginClassifies(string origin, bool expected)
    {
        Assert.Equal(expected, LocalOrigin.IsLocalOrigin(origin));
    }
}

internal sealed class RecordingLogger<T> : ILogger<T>
{
    public List<(LogLevel Level, string Message)> Entries { get; } = [];

    public IDisposable? BeginScope<TState>(TState state) where TState : notnull => null;

    public bool IsEnabled(LogLevel logLevel) => true;

    public void Log<TState>(LogLevel logLevel, EventId eventId, TState state, Exception? exception,
        Func<TState, Exception?, string> formatter)
    {
        Entries.Add((logLevel, formatter(state, exception)));
    }
}

public class RequestLoggingMiddlewareTests
{
    [Fact]
    public async Task LogsRequestWhenInformationEnabled()
    {
        RecordingLogger<RequestLoggingMiddleware> logger = new();
        bool nextCalled = false;
        RequestLoggingMiddleware middleware = new(_ => { nextCalled = true; return Task.CompletedTask; }, logger);

        DefaultHttpContext http = new();
        http.Request.Method = "GET";
        http.Request.Path = "/api/game/xyz";

        await middleware.InvokeAsync(http);

        Assert.True(nextCalled);
        (LogLevel level, string message) = Assert.Single(logger.Entries);
        Assert.Equal(LogLevel.Information, level);
        Assert.Contains("GET", message);
        Assert.Contains("/api/game/xyz", message);
    }
}
