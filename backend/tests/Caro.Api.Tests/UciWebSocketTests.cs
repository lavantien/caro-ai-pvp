using System.Net.WebSockets;
using System.Text;
using Caro.Api;
using Microsoft.AspNetCore.TestHost;
using Xunit;

namespace Caro.Api.Tests;

public class UciWebSocketTests
{
    private static async Task<WebSocket> ConnectAsync(TestApi api, string? origin = null)
    {
        TestServer server = api.App.GetTestServer();
        WebSocketClient client = server.CreateWebSocketClient();
        if (origin != null)
        {
            client.ConfigureRequest = req => req.Headers["Origin"] = origin;
        }
        Uri uri = new("ws://localhost/ws/uci");
        return await client.ConnectAsync(uri, CancellationToken.None);
    }

    private static async Task SendLineAsync(WebSocket socket, string line)
    {
        byte[] payload = Encoding.UTF8.GetBytes(line);
        await socket.SendAsync(payload, WebSocketMessageType.Text, endOfMessage: true,
            CancellationToken.None);
    }

    /// <summary>Reads one whole text message from the socket.</summary>
    private static async Task<string> ReceiveAsync(WebSocket socket, int timeoutMs = 30_000)
    {
        using CancellationTokenSource cts = new(timeoutMs);
        byte[] buffer = new byte[4096];
        StringBuilder sb = new();
        while (true)
        {
            WebSocketReceiveResult result = await socket.ReceiveAsync(buffer, cts.Token);
            if (result.MessageType == WebSocketMessageType.Close)
            {
                throw new InvalidOperationException($"unexpected close ({result.CloseStatus}) after: {sb}");
            }
            sb.Append(Encoding.UTF8.GetString(buffer, 0, result.Count));
            if (result.EndOfMessage)
            {
                return sb.ToString();
            }
        }
    }

    /// <summary>Collects messages until one contains needle; returns them all joined.</summary>
    private static async Task<string> ReceiveUntilAsync(WebSocket socket, string needle)
    {
        DateTime deadline = DateTime.UtcNow.AddSeconds(60);
        StringBuilder seen = new();
        while (DateTime.UtcNow < deadline)
        {
            string message = (await ReceiveAsync(socket)).TrimEnd();
            seen.AppendLine(message);
            if (message.Contains(needle, StringComparison.Ordinal))
            {
                return seen.ToString();
            }
        }
        throw new TimeoutException($"no message containing {needle}, saw: {seen}");
    }

    [Fact]
    public async Task WsUciRejectsNonUpgradeRequest()
    {
        await using TestApi api = TestHostFactory.Create();
        HttpResponseMessage resp = await api.Client.GetAsync("/ws/uci");
        Assert.Equal(System.Net.HttpStatusCode.BadRequest, resp.StatusCode);
    }

    [Fact]
    public async Task WsUciRejectsForeignOrigin()
    {
        await using TestApi api = TestHostFactory.Create();
        // TestHost surfaces the rejected handshake as InvalidOperationException.
        await Assert.ThrowsAnyAsync<InvalidOperationException>(() =>
            ConnectAsync(api, "http://evil.example.com"));
    }

    [Fact]
    public async Task WsUciAcceptsLocalOrigin()
    {
        await using TestApi api = TestHostFactory.Create();
        using WebSocket socket = await ConnectAsync(api, "http://localhost:5173");
        await SendLineAsync(socket, "uci");
        string reply = await ReceiveUntilAsync(socket, "uciok");
        Assert.Contains("id name Caro AI", reply);
    }

    [Fact]
    public async Task WsUciHandshakeAndSearch()
    {
        await using TestApi api = TestHostFactory.Create();
        using WebSocket socket = await ConnectAsync(api);

        await SendLineAsync(socket, "uci");
        await ReceiveUntilAsync(socket, "uciok");
        await SendLineAsync(socket, "isready");
        await ReceiveUntilAsync(socket, "readyok");

        await SendLineAsync(socket, "position startpos moves hh");
        // Both clocks are sent so the side to move always finds its own.
        await SendLineAsync(socket, "go depth 3 wtime 60000 btime 60000");
        string best = await ReceiveUntilAsync(socket, "bestmove");
        Assert.Contains("bestmove ", best);

        // CloseOutput only sends our close frame; the server endpoint
        // returns on the peer's close without negotiating an ack.
        using CancellationTokenSource cts = new(10_000);
        await socket.CloseOutputAsync(WebSocketCloseStatus.NormalClosure, "done", cts.Token);
    }

    [Fact]
    public async Task WsUciRejectsOversizedFrame()
    {
        await using TestApi api = TestHostFactory.Create();
        using WebSocket socket = await ConnectAsync(api);

        byte[] big = new byte[5000];
        await socket.SendAsync(big, WebSocketMessageType.Text, endOfMessage: true,
            CancellationToken.None);

        using CancellationTokenSource cts = new(30_000);
        byte[] buffer = new byte[4096];
        WebSocketReceiveResult result = await socket.ReceiveAsync(buffer, cts.Token);
        Assert.Equal(WebSocketMessageType.Close, result.MessageType);
        Assert.Equal(WebSocketCloseStatus.MessageTooBig, result.CloseStatus);
    }

    [Fact]
    public async Task WsUciToleratesClientAbort()
    {
        // The reply writer must survive the peer vanishing mid-exchange: the
        // catches in WebSocketLineWriter swallow the send failure.
        await using TestApi api = TestHostFactory.Create();
        WebSocket socket = await ConnectAsync(api);
        await SendLineAsync(socket, "uci");
        socket.Abort();
        await Task.Delay(300);
    }
}

/// <summary>A WebSocket whose sends always fail; drives the writer's catches.</summary>
internal sealed class ThrowingWebSocket(Exception exception) : WebSocket
{
    public int SendCalls { get; private set; }

    public override Task SendAsync(ArraySegment<byte> buffer, WebSocketMessageType messageType,
        bool endOfMessage, CancellationToken cancellationToken)
    {
        SendCalls++;
        return Task.FromException(exception);
    }

    public override Task<WebSocketReceiveResult> ReceiveAsync(ArraySegment<byte> buffer,
        CancellationToken cancellationToken) => throw new NotSupportedException();

    public override Task CloseAsync(WebSocketCloseStatus closeStatus, string? statusDescription,
        CancellationToken cancellationToken) => Task.CompletedTask;

    public override Task CloseOutputAsync(WebSocketCloseStatus closeStatus, string? statusDescription,
        CancellationToken cancellationToken) => Task.CompletedTask;

    public override void Abort() { }

    public override void Dispose() { }

    public override WebSocketState State => WebSocketState.Open;
    public override WebSocketCloseStatus? CloseStatus => null;
    public override string? CloseStatusDescription => null;
    public override string? SubProtocol => null;
}

public class WebSocketLineWriterTests
{
    private static async Task WriteLineAsync(WebSocketLineWriter writer)
    {
        writer.WriteLine("info depth 1");
        await Task.Delay(200);
    }

    [Fact]
    public async Task WriterSwallowsWebSocketException()
    {
        ThrowingWebSocket socket = new(new WebSocketException("peer gone"));
        using WebSocketLineWriter writer = new(socket);
        await WriteLineAsync(writer);
        Assert.Equal(1, socket.SendCalls);
    }

    [Fact]
    public async Task WriterSwallowsObjectDisposedException()
    {
        ThrowingWebSocket socket = new(new ObjectDisposedException("socket"));
        using WebSocketLineWriter writer = new(socket);
        await WriteLineAsync(writer);
        Assert.Equal(1, socket.SendCalls);
    }
}
