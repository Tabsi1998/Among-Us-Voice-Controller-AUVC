using System.Net.WebSockets;
using System.Text;
using AUVC.Protocol;

namespace AUVC.Transport;

/// <summary>
/// One duplex channel of protocol messages. The WebSocket implementation is
/// below; the interface exists so the connection logic can be tested without a
/// socket, the same way the bot's protocol rules are tested without one.
/// </summary>
public interface IMessageChannel : IAsyncDisposable
{
    Task SendAsync(Message message, CancellationToken cancellationToken);

    /// <summary>
    /// The next message from the bot, or null when the connection has closed.
    /// </summary>
    Task<Message?> ReceiveAsync(CancellationToken cancellationToken);
}

/// <summary>
/// Raised when the bot refused the connection. The message is written for the
/// person running capture.
/// </summary>
public sealed class CaptureRefusedException(string code, string message) : Exception(message)
{
    /// <summary>The protocol error code, stable and machine-readable.</summary>
    public string Code { get; } = code;
}

/// <summary>
/// Carries protocol messages over a WebSocket.
/// </summary>
public sealed class WebSocketMessageChannel(WebSocket socket) : IMessageChannel
{
    // A snapshot of a full lobby is a few kilobytes. This bounds what one
    // message may be so a confused or hostile peer cannot make capture
    // allocate on demand; the bot applies the same limit in the other
    // direction.
    private const int MaxMessageBytes = 256 * 1024;

    private readonly SemaphoreSlim _writeLock = new(1, 1);

    /// <summary>
    /// Connects to a bot and returns the channel.
    /// </summary>
    /// <remarks>
    /// No Origin header is sent, which is deliberate: the bot upgrades only
    /// requests without one, so that a web page cannot reach the handshake.
    /// </remarks>
    public static async Task<WebSocketMessageChannel> ConnectAsync(
        Uri botAddress, CancellationToken cancellationToken = default)
    {
        var builder = new UriBuilder(botAddress)
        {
            Scheme = botAddress.Scheme is "https" or "wss" ? "wss" : "ws",
            Path = "/capture/link",
        };

        var socket = new ClientWebSocket();
        await socket.ConnectAsync(builder.Uri, cancellationToken);
        return new WebSocketMessageChannel(socket);
    }

    public async Task SendAsync(Message message, CancellationToken cancellationToken)
    {
        var payload = Encoding.UTF8.GetBytes(ProtocolCodec.Encode(message));

        // One writer at a time. A WebSocket cannot interleave two sends, and a
        // heartbeat timer racing a game event would otherwise corrupt both.
        await _writeLock.WaitAsync(cancellationToken);
        try
        {
            await socket.SendAsync(payload, WebSocketMessageType.Text, true, cancellationToken);
        }
        finally
        {
            _writeLock.Release();
        }
    }

    public async Task<Message?> ReceiveAsync(CancellationToken cancellationToken)
    {
        var buffer = new byte[8192];
        using var assembled = new MemoryStream();

        while (true)
        {
            WebSocketReceiveResult result;
            try
            {
                result = await socket.ReceiveAsync(buffer, cancellationToken);
            }
            catch (WebSocketException)
            {
                return null; // the far end went away, which is a closed connection
            }

            if (result.MessageType == WebSocketMessageType.Close)
            {
                return null;
            }

            assembled.Write(buffer, 0, result.Count);
            if (assembled.Length > MaxMessageBytes)
            {
                throw new CaptureRefusedException(
                    ProtocolContract.CodeMalformed, "The AUVC bot sent a message that is too large to be one.");
            }
            if (result.EndOfMessage)
            {
                break;
            }
        }

        var text = Encoding.UTF8.GetString(assembled.ToArray());
        return ProtocolCodec.Decode(text);
    }

    public async ValueTask DisposeAsync()
    {
        _writeLock.Dispose();

        if (socket.State == WebSocketState.Open)
        {
            using var timeout = new CancellationTokenSource(TimeSpan.FromSeconds(5));
            try
            {
                await socket.CloseAsync(
                    WebSocketCloseStatus.NormalClosure, "capture is stopping", timeout.Token);
            }
            catch (Exception)
            {
                // A close that fails is a connection that is already gone.
            }
        }

        socket.Dispose();
    }
}
