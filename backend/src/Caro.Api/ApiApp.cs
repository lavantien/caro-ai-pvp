using Caro.Persistence;
using Microsoft.AspNetCore.Builder;
using Microsoft.AspNetCore.Http;
using Microsoft.Extensions.DependencyInjection;

namespace Caro.Api;

/// <summary>
/// Service and pipeline wiring shared by the real host and the in-test
/// host, so both serve identical behavior.
/// </summary>
public static class CaroApp
{
    public static IServiceCollection AddCaroApi(this IServiceCollection services, MatchStore? matches = null, GameStore? store = null)
    {
        services.AddCors(o => o.AddPolicy("loopback", p => p
            .SetIsOriginAllowed(origin => LocalOrigin.IsLocalOrigin(origin))
            .AllowCredentials()
            .WithMethods("GET", "POST", "DELETE", "OPTIONS")
            .WithHeaders("Content-Type")));
        if (store != null)
        {
            services.AddSingleton(store);
        }
        else
        {
            services.AddSingleton<GameStore>();
        }
        if (matches != null)
        {
            services.AddSingleton(matches);
        }
        services.AddSingleton<GameHandlers>();
        return services;
    }

    public static WebApplication UseCaroPipeline(this WebApplication app)
    {
        app.UseMiddleware<ErrorMappingMiddleware>();
        app.UseCors("loopback");
        // Bare OPTIONS requests (no preflight headers) answer 204 like the
        // Go CORS middleware did, instead of falling through to routing.
        app.Use(async (ctx, next) =>
        {
            if (HttpMethods.IsOptions(ctx.Request.Method))
            {
                ctx.Response.StatusCode = 204;
                return;
            }
            await next();
        });
        app.UseMiddleware<RequestLoggingMiddleware>();
        app.UseWebSockets();
        app.MapGameEndpoints();
        app.MapUciWebSocket();
        return app;
    }
}
