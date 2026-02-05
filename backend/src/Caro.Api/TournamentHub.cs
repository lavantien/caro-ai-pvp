using Microsoft.AspNetCore.SignalR;
using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;
using Caro.Core.Tournament;

namespace Caro.Api;

/// <summary>
/// SignalR hub for real-time tournament updates
/// Clients subscribe to events to receive live updates during tournament execution
/// </summary>
public class TournamentHub : Hub<ITournamentClient>
{
    private readonly ILogger<TournamentHub> _logger;

    public TournamentHub(ILogger<TournamentHub> logger)
    {
        _logger = logger;
    }

    public override async Task OnConnectedAsync()
    {
        _logger.LogInformation("Client connected: {ConnectionId}", Context.ConnectionId);
        await base.OnConnectedAsync();
    }

    public override async Task OnDisconnectedAsync(Exception? exception)
    {
        _logger.LogInformation("Client disconnected: {ConnectionId}", Context.ConnectionId);
        await base.OnDisconnectedAsync(exception);
    }
}
