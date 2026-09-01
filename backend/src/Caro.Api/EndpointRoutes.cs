using Microsoft.AspNetCore.Builder;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.AspNetCore.Http;
using Microsoft.AspNetCore.Routing;

namespace Caro.Api;

public static class EndpointRoutes
{
    public static IEndpointRouteBuilder MapGameEndpoints(this IEndpointRouteBuilder routes)
    {
        GameHandlers handler = routes.ServiceProvider.GetRequiredService<GameHandlers>();

        routes.MapPost("/api/game/new", handler.CreateGameAsync);
        routes.MapGet("/api/game/{id}", handler.GetGameAsync);
        routes.MapPost("/api/game/{id}/move", handler.MakeMoveAsync);
        routes.MapPost("/api/game/{id}/ai-move", handler.MakeAIMoveAsync);
        routes.MapPost("/api/game/{id}/undo", handler.UndoMoveAsync);
        routes.MapDelete("/api/game/{id}", handler.DeleteGameAsync);
        return routes;
    }
}
