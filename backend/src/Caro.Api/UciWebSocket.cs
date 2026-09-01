using System.Net.WebSockets;
using System.Text;
using Caro.Uci;
using Microsoft.AspNetCore.Builder;
using Microsoft.AspNetCore.Http;
using Microsoft.AspNetCore.Routing;

namespace Caro.Api;

/// <summary>
/// WebSocket writer that serializes sends: the UCI search thread replies
/// while the receive loop is idle.
/// </summary>
internal sealed class WebSocketLineWriter(WebSocket socket) : ILineWriter, IDisposable
{
    private readonly SemaphoreSlim _sendGate = new(1, 1);

    public async void WriteLine(string line)
    {
        byte[] payload = Encoding.UTF8.GetBytes(line + "\n");
        await _sendGate.WaitAsync();
        try
        {
            await socket.SendAsync(payload, WebSocketMessageType.Text, endOfMessage: true,
                CancellationToken.None);
        }
        catch (WebSocketException)
        {
            // Peer disconnected mid-reply; the receive loop notices next.
        }
        catch (ObjectDisposedException)
        {
            // Socket torn down by the receive loop's finally block.
        }
        finally
        {
            _sendGate.Release();
        }
    }

    public void Dispose() => _sendGate.Dispose();
}

public static class UciWebSocket
{
    private const int ReadLimit = 4096;

    public static IEndpointRouteBuilder MapUciWebSocket(this IEndpointRouteBuilder routes)
    {
        routes.Map("/ws/uci", async http =>
        {
            if (!http.WebSockets.IsWebSocketRequest)
            {
                http.Response.StatusCode = 400;
                return;
            }

            string? origin = http.Request.Headers.Origin.ToString();
            if (origin.Length != 0 && !LocalOrigin.IsLocalOrigin(origin))
            {
                http.Response.StatusCode = 403;
                return;
            }

            WebSocket socket = await http.WebSockets.AcceptWebSocketAsync();

            WebSocketLineWriter writer = new(socket);
            using UciHandler handler = new(writer);
            try
            {
                // Bound incoming frame size; commands are short text lines.
                byte[] buffer = new byte[ReadLimit];
                while (true)
                {
                    int received = 0;
                    WebSocketReceiveResult result;
                    do
                    {
                        result = await socket.ReceiveAsync(
                            new ArraySegment<byte>(buffer, received, buffer.Length - received),
                            CancellationToken.None);
                        received += result.Count;
                        if (received > ReadLimit)
                        {
                            await socket.CloseAsync(WebSocketCloseStatus.MessageTooBig,
                                "frame exceeds 4096 bytes", CancellationToken.None);
                            return;
                        }
                    }
                    while (!result.EndOfMessage);

                    if (result.MessageType == WebSocketMessageType.Close)
                    {
                        return;
                    }
                    handler.HandleCommand(Encoding.UTF8.GetString(buffer, 0, received));
                }
            }
            catch (WebSocketException)
            {
                // Client went away; nothing to clean beyond the handler.
            }
            finally
            {
                // Release the handler's engine (TT memory) when the peer
                // disconnects.
                handler.Close();
                socket.Dispose();
            }
        });
        return routes;
    }
}
