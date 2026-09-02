using Caro.Domain;
using Microsoft.AspNetCore.Http;
using Microsoft.Extensions.Logging;

namespace Caro.Api;

public static class LocalOrigin
{
    private const string LoopbackScheme = "http";

    // Uri.Host keeps the brackets on IPv6 ("[::1]"); Go's Hostname()
    // stripped them, so both spellings are accepted.
    private static readonly string[] LoopbackHosts = ["localhost", "127.0.0.1", "::1", "[::1]"];

    /// <summary>
    /// Reports whether an Origin header points at a loopback host, which is
    /// the only cross-origin caller this local game server expects.
    /// </summary>
    public static bool IsLocalOrigin(string origin)
    {
        if (!Uri.TryCreate(origin, UriKind.Absolute, out Uri? u))
        {
            return false;
        }
        return u.Scheme == LoopbackScheme && LoopbackHosts.Contains(u.Host);
    }
}

/// <summary>
/// Maps domain exceptions to the HTTP error contract and converts anything
/// unexpected into a 500, mirroring the Go writeError/panic-recovery pair.
/// </summary>
public sealed class ErrorMappingMiddleware(RequestDelegate next, ILogger<ErrorMappingMiddleware> logger)
{
    public async Task InvokeAsync(HttpContext http)
    {
        try
        {
            await next(http);
        }
        catch (Exception e)
        {
            switch (e)
            {
                case GameNotFoundException:
                    await ResponseJson.Write(http, 404, new ErrorResponse { Error = "not_found", Message = e.Message });
                    break;
                case TooManyGamesException:
                    await ResponseJson.Write(http, 429, new ErrorResponse { Error = "too_many_games", Message = e.Message });
                    break;
                case CellOccupiedException or PositionBoundsException or GameOverException
                    or OpenRuleException or InvalidLevelException:
                    await ResponseJson.Write(http, 400, new ErrorResponse { Error = "bad_request", Message = e.Message });
                    break;
                case NotPlayerTurnException:
                    await ResponseJson.Write(http, 409, new ErrorResponse { Error = "not_your_turn", Message = e.Message });
                    break;
                default:
                    logger.Panic(e, http.Request.Path.ToString());
                    await ResponseJson.Write(http, 500, new ErrorResponse { Error = "internal", Message = "Internal server error" });
                    break;
            }
        }
    }
}

public sealed class RequestLoggingMiddleware(RequestDelegate next, ILogger<RequestLoggingMiddleware> logger)
{
    public async Task InvokeAsync(HttpContext http)
    {
        long start = System.Diagnostics.Stopwatch.GetTimestamp();
        await next(http);
        if (logger.IsEnabled(LogLevel.Information))
        {
            double durationMs = System.Diagnostics.Stopwatch.GetElapsedTime(start).TotalMilliseconds;
            logger.Request(http.Request.Method, http.Request.Path.ToString(), durationMs);
        }
    }
}
