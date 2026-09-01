using Microsoft.Extensions.Logging;

namespace Caro.Api;

internal static partial class Log
{
    [LoggerMessage(Level = LogLevel.Information, Message = "request {Method} {Path} {Duration}")]
    public static partial void Request(this ILogger logger, string method, string path, double duration);

    [LoggerMessage(Level = LogLevel.Error, Message = "panic recovered {Path}")]
    public static partial void Panic(this ILogger logger, System.Exception exception, string path);

    [LoggerMessage(Level = LogLevel.Information, Message = "move-statline {GameId} {Line}")]
    public static partial void MoveStatline(this ILogger logger, string gameId, string line);

    [LoggerMessage(Level = LogLevel.Error, Message = "match store {Operation} failed for {GameId}")]
    public static partial void StoreFailure(this ILogger logger, System.Exception exception, string operation, string gameId);
}
