using System.Threading.Channels;
using AUVC.Protocol;

namespace AUVC.Transport;

/// <summary>Where the link to the bot stands.</summary>
public enum LinkState
{
    /// <summary>Not started, or stopped.</summary>
    Stopped,

    /// <summary>No credential is stored. Capture has to pair first.</summary>
    NotPaired,

    Connecting,

    /// <summary>
    /// The connection is open and the handshake is on its way. The bot answers
    /// only to refuse, so a connection that stays in this state is an
    /// authenticated one.
    /// </summary>
    Connected,

    /// <summary>
    /// The bot could not be reached, or the connection dropped. Capture tries
    /// again by itself.
    /// </summary>
    Retrying,

    /// <summary>
    /// The bot refused this capture for a reason trying again cannot fix: the
    /// credential was revoked or never valid, or the builds are incompatible.
    /// Capture waits until it pairs again or is told to reconnect.
    /// </summary>
    Refused,
}

/// <param name="State">Where the link stands.</param>
/// <param name="Detail">What happened, written for the person running capture. Never contains the credential.</param>
/// <param name="Code">The protocol error code when the bot refused, otherwise empty.</param>
public sealed record LinkStatus(LinkState State, string Detail = "", string Code = "");

/// <summary>What the game reader reports about the round.</summary>
public interface IRoundReporter
{
    void ReportPhase(string phase);

    void ReportPlayerJoined(Player player);

    void ReportPlayerLeft(Player player);

    void ReportPlayerChanged(Player player);

    void ReportPlayerDied(Player player);

    void ReportGameEnded();

    /// <summary>The lobby's code and map, read after the phase changed into it.</summary>
    void ReportLobby(Lobby lobby);
}

public sealed record CaptureLinkOptions
{
    /// <summary>
    /// How often capture says it is alive. Well inside the shortest capture
    /// timeout the bot allows, ten seconds, so a live capture is never taken for
    /// a dead one.
    /// </summary>
    public TimeSpan HeartbeatInterval { get; init; } = TimeSpan.FromSeconds(5);

    /// <summary>
    /// How long to wait before each further attempt to reach the bot. The last
    /// delay repeats.
    /// </summary>
    public IReadOnlyList<TimeSpan> RetryDelays { get; init; } =
    [
        TimeSpan.FromSeconds(1),
        TimeSpan.FromSeconds(2),
        TimeSpan.FromSeconds(5),
        TimeSpan.FromSeconds(10),
        TimeSpan.FromSeconds(30),
    ];

    /// <summary>
    /// How long to keep reading after a send failed. A failed send usually means
    /// the bot closed the connection, and it may have said why just before.
    /// </summary>
    public TimeSpan DrainTimeout { get; init; } = TimeSpan.FromSeconds(2);
}

/// <summary>
/// Keeps capture connected to the bot for as long as it runs.
/// </summary>
/// <remarks>
/// Each connection is a new session: hello, the stored credential, then a
/// complete snapshot of the round, then events as they happen and a heartbeat
/// in between. When a connection ends the link tries again by itself, unless
/// the bot refused this capture for a reason another attempt would not fix.
///
/// The game reader reports from its own thread, and the heartbeat and the
/// connection run on others. Every message is built and queued under one lock,
/// because building a message assigns its sequence number: two threads that
/// built messages 5 and 6 and queued them the other way round would show the
/// bot a gap, and the bot answers a gap by refusing events until the next
/// snapshot.
/// </remarks>
public sealed class CaptureLink : IRoundReporter, IAsyncDisposable
{
    private readonly ICredentialStore _credentials;
    private readonly Func<CancellationToken, Task<IMessageChannel>> _connect;
    private readonly CaptureSession _session;
    private readonly CaptureLinkOptions _options;

    private readonly object _gate = new();
    private readonly GameRound _round = new();
    private Channel<Message>? _outbound;
    private LinkStatus _status = new(LinkState.Stopped);

    private CancellationTokenSource _wake = new();
    private CancellationTokenSource? _stop;
    private Task? _running;

    /// <param name="credentials">Where the credential from pairing is kept. Read afresh on every connection.</param>
    /// <param name="connect">Opens a channel to the bot. Called for every connection attempt.</param>
    /// <param name="captureBuild">The capture build, sent in every hello.</param>
    public CaptureLink(
        ICredentialStore credentials,
        Func<CancellationToken, Task<IMessageChannel>> connect,
        string captureBuild,
        CaptureLinkOptions? options = null,
        Func<string>? newSessionId = null)
    {
        _credentials = credentials;
        _connect = connect;
        _session = new CaptureSession(captureBuild, newSessionId);
        _options = options ?? new CaptureLinkOptions();

        if (_options.RetryDelays.Count == 0)
        {
            throw new ArgumentException("at least one retry delay is required", nameof(options));
        }
    }

    /// <summary>
    /// Raised whenever the state changes, on whichever thread changed it.
    /// </summary>
    public event Action<LinkStatus>? StatusChanged;

    public LinkStatus Status
    {
        get
        {
            lock (_gate)
            {
                return _status;
            }
        }
    }

    /// <summary>Starts connecting. Calling it again does nothing.</summary>
    public void Start()
    {
        lock (_gate)
        {
            if (_running is not null)
            {
                return;
            }

            _stop = new CancellationTokenSource();
            var stop = _stop.Token;
            _running = Task.Run(() => RunAsync(stop));
        }
    }

    /// <summary>
    /// Drops the current connection, if any, and connects again at once. Call it
    /// after pairing or after the bot's address changed.
    /// </summary>
    public void Reconnect()
    {
        lock (_gate)
        {
            _wake.Cancel();
        }
    }

    public async ValueTask DisposeAsync()
    {
        Task? running;
        lock (_gate)
        {
            running = _running;
            _stop?.Cancel();
        }

        if (running is not null)
        {
            await running;
        }
    }

    public void ReportPhase(string phase) =>
        Report(round => round.SetPhase(phase), session => session.PhaseChanged(phase, _round.Lobby));

    public void ReportPlayerJoined(Player player) =>
        Report(round => round.Join(player), session => session.PlayerJoined(player));

    public void ReportPlayerLeft(Player player) =>
        Report(round => round.Leave(player), session => session.PlayerLeft(player));

    public void ReportPlayerChanged(Player player) =>
        Report(round => round.Change(player), session => session.PlayerChanged(player));

    public void ReportPlayerDied(Player player) =>
        Report(round => round.Die(player), session => session.PlayerDied(player with { Dead = true }));

    public void ReportGameEnded() =>
        Report(round => round.End(), session => session.GameEnded());

    /// <summary>
    /// Records the lobby and tells the bot with a change into the phase capture is
    /// already in: the game reads the code and the map only after the phase changed.
    /// </summary>
    public void ReportLobby(Lobby lobby) =>
        Report(round => round.SetLobby(lobby), session => session.PhaseChanged(_round.Phase, _round.Lobby));

    /// <summary>
    /// Records a change and, while connected, queues the event that tells the
    /// bot. While disconnected only the record changes; the next connection's
    /// snapshot carries it.
    /// </summary>
    private void Report(Func<GameRound, bool> apply, Func<CaptureSession, Message> build)
    {
        lock (_gate)
        {
            if (!apply(_round) || _outbound is null)
            {
                return;
            }

            _outbound.Writer.TryWrite(build(_session));
        }
    }

    private async Task RunAsync(CancellationToken stop)
    {
        var failures = 0;

        while (!stop.IsCancellationRequested)
        {
            var wake = RenewWake();

            var credential = _credentials.Read();
            if (string.IsNullOrEmpty(credential))
            {
                SetStatus(new LinkStatus(LinkState.NotPaired,
                    "Pair with the AUVC bot using a code from /au capture pair."));
                await WaitAsync(Timeout.InfiniteTimeSpan, wake, stop);
                continue;
            }

            SetStatus(new LinkStatus(LinkState.Connecting));

            SessionEnd end;
            using (var session = CancellationTokenSource.CreateLinkedTokenSource(stop, wake))
            {
                try
                {
                    end = await RunSessionAsync(credential, session.Token);
                }
                catch (OperationCanceledException) when (session.IsCancellationRequested)
                {
                    continue; // stopped, or asked to reconnect
                }
            }

            if (end.Connected)
            {
                failures = 0;
            }

            if (end.Refusal is { } refusal && EndsRetrying(refusal.Code))
            {
                SetStatus(new LinkStatus(LinkState.Refused, refusal.Message, refusal.Code));
                await WaitAsync(Timeout.InfiniteTimeSpan, wake, stop);
                continue;
            }

            var delay = _options.RetryDelays[Math.Min(failures, _options.RetryDelays.Count - 1)];
            failures++;

            SetStatus(new LinkStatus(LinkState.Retrying, end.Refusal?.Message ?? end.Problem, end.Refusal?.Code ?? ""));
            await WaitAsync(delay, wake, stop);
        }

        SetStatus(new LinkStatus(LinkState.Stopped));
    }

    /// <summary>
    /// Runs one connection until it ends, and says how.
    /// </summary>
    /// <exception cref="OperationCanceledException">The link was stopped or asked to reconnect.</exception>
    private async Task<SessionEnd> RunSessionAsync(string credential, CancellationToken token)
    {
        IMessageChannel channel;
        try
        {
            channel = await _connect(token);
        }
        catch (Exception error) when (error is not OperationCanceledException || !token.IsCancellationRequested)
        {
            return new SessionEnd(false, null, $"Could not reach the AUVC bot. {error.Message}");
        }

        await using (channel)
        {
            var outbound = Channel.CreateUnbounded<Message>(new UnboundedChannelOptions { SingleReader = true });
            lock (_gate)
            {
                outbound.Writer.TryWrite(_session.Open());
                outbound.Writer.TryWrite(_session.Authenticate(credential));
                outbound.Writer.TryWrite(_session.Snapshot(_round.Phase, _round.Players, _round.Lobby));
                _outbound = outbound;
            }
            SetStatus(new LinkStatus(LinkState.Connected));

            using var ending = CancellationTokenSource.CreateLinkedTokenSource(token);
            var reading = ReadAsync(channel, outbound, ending.Token);
            var writing = WriteAsync(channel, outbound.Reader, ending.Token);
            var beating = HeartbeatAsync(outbound, ending.Token);

            try
            {
                if (await Task.WhenAny(reading, writing) == writing)
                {
                    await Task.WhenAny(reading, Task.Delay(_options.DrainTimeout, token));
                }
            }
            finally
            {
                lock (_gate)
                {
                    if (_outbound == outbound)
                    {
                        _outbound = null;
                    }
                }
                outbound.Writer.TryComplete();
                ending.Cancel();
                await SettleAsync(reading, writing, beating);
            }

            token.ThrowIfCancellationRequested();

            return reading.IsCompletedSuccessfully && reading.Result is { } refusal
                ? new SessionEnd(true, refusal, "")
                : new SessionEnd(true, null, "The connection to the AUVC bot closed.");
        }
    }

    /// <summary>
    /// Reads until the bot refuses or the connection closes. A request for a
    /// snapshot is answered here and does not end the session.
    /// </summary>
    private async Task<ProtocolError?> ReadAsync(
        IMessageChannel channel, Channel<Message> outbound, CancellationToken token)
    {
        while (true)
        {
            switch (await channel.ReceiveAsync(token))
            {
                case null:
                    return null;

                case ProtocolError { Code: ProtocolContract.CodeSnapshotRequired }:
                    lock (_gate)
                    {
                        if (_outbound == outbound)
                        {
                            outbound.Writer.TryWrite(_session.Snapshot(_round.Phase, _round.Players, _round.Lobby));
                        }
                    }
                    break;

                case ProtocolError refusal:
                    return refusal;
            }
        }
    }

    private static async Task WriteAsync(
        IMessageChannel channel, ChannelReader<Message> outbound, CancellationToken token)
    {
        await foreach (var message in outbound.ReadAllAsync(token))
        {
            await channel.SendAsync(message, token);
        }
    }

    private async Task HeartbeatAsync(Channel<Message> outbound, CancellationToken token)
    {
        while (true)
        {
            await Task.Delay(_options.HeartbeatInterval, token);

            lock (_gate)
            {
                if (_outbound != outbound)
                {
                    return;
                }
                outbound.Writer.TryWrite(_session.Heartbeat());
            }
        }
    }

    // The session is over by the time these are awaited. How each one ended is
    // read from the task itself, so their exceptions are not news.
    private static async Task SettleAsync(params Task[] tasks)
    {
        try
        {
            await Task.WhenAll(tasks);
        }
        catch (Exception)
        {
        }
    }

    // A revoked or unknown credential stays refused however often it is
    // presented, and an incompatible build stays incompatible. Anything else,
    // including a network failure, can pass.
    private static bool EndsRetrying(string code) =>
        code is ProtocolContract.CodeUnauthenticated or ProtocolContract.CodeIncompatibleProtocol;

    private CancellationToken RenewWake()
    {
        lock (_gate)
        {
            if (_wake.IsCancellationRequested)
            {
                _wake.Dispose();
                _wake = new CancellationTokenSource();
            }
            return _wake.Token;
        }
    }

    private static async Task WaitAsync(TimeSpan delay, CancellationToken wake, CancellationToken stop)
    {
        using var either = CancellationTokenSource.CreateLinkedTokenSource(wake, stop);
        try
        {
            await Task.Delay(delay, either.Token);
        }
        catch (OperationCanceledException)
        {
        }
    }

    private void SetStatus(LinkStatus status)
    {
        lock (_gate)
        {
            if (_status == status)
            {
                return;
            }
            _status = status;
        }

        try
        {
            StatusChanged?.Invoke(status);
        }
        catch (Exception)
        {
            // A window that fails to show a status must not stop capture from
            // reporting the game.
        }
    }

    private sealed record SessionEnd(bool Connected, ProtocolError? Refusal, string Problem);
}
