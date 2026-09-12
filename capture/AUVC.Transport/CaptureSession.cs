using AUVC.Protocol;

namespace AUVC.Transport;

/// <summary>
/// Builds the messages a capture session sends, in the order the bot accepts
/// them.
/// </summary>
/// <remarks>
/// This holds no socket and no clock. The rules it follows are the ones in
/// protocol/README.md, and keeping them separate from the connection is what
/// lets a reconnect be tested by reading what the session produced rather than
/// by unplugging a network cable.
///
/// The bot enforces all of this itself. A session that got it wrong would be
/// refused rather than believed, so this class exists to make capture correct,
/// not to make the bot safe.
/// </remarks>
public sealed class CaptureSession
{
    private readonly string _captureBuild;
    private readonly Func<string> _newSessionId;

    private string _session = "";
    private ulong _seq;
    private bool _snapshotSent;

    public CaptureSession(string captureBuild, Func<string>? newSessionId = null)
    {
        if (string.IsNullOrWhiteSpace(captureBuild))
        {
            throw new ArgumentException("capture has to say which build it is", nameof(captureBuild));
        }

        _captureBuild = captureBuild;
        _newSessionId = newSessionId ?? (() => Guid.NewGuid().ToString());
    }

    /// <summary>The session currently being spoken in, empty before the first hello.</summary>
    public string SessionId => _session;

    /// <summary>The sequence number of the last message produced.</summary>
    public ulong LastSeq => _seq;

    /// <summary>
    /// Whether a snapshot has been sent for the current session. Until it has,
    /// the bot refuses every incremental event.
    /// </summary>
    public bool SnapshotSent => _snapshotSent;

    /// <summary>
    /// Starts a new session and returns the hello that opens it.
    /// </summary>
    /// <remarks>
    /// Every connection is a new session, including a reconnect. The bot drops
    /// everything it knew about the previous one, which is why the caller has
    /// to follow this with authentication and a complete snapshot.
    /// </remarks>
    public Hello Open()
    {
        _session = _newSessionId();
        _seq = 0;
        _snapshotSent = false;

        return new Hello { Session = _session, Seq = Next(), Capture = _captureBuild };
    }

    /// <summary>Presents the stored credential.</summary>
    public Authentication Authenticate(string credential)
    {
        RequireOpenSession();

        if (string.IsNullOrEmpty(credential))
        {
            throw new InvalidOperationException("capture has no credential; pair first");
        }
        return new Authentication { Session = _session, Seq = Next(), Credential = credential };
    }

    /// <summary>
    /// Sends the complete state of the round.
    /// </summary>
    /// <remarks>
    /// This is what makes a reconnect work. The bot cannot rebuild a round that
    /// was already running from incremental events, so it refuses them until a
    /// snapshot arrives.
    /// </remarks>
    public Snapshot Snapshot(string phase, IReadOnlyList<Player> players)
    {
        RequireOpenSession();

        _snapshotSent = true;
        return new Snapshot { Session = _session, Seq = Next(), Phase = phase, Players = players };
    }

    public Heartbeat Heartbeat()
    {
        RequireOpenSession();
        return new Heartbeat { Session = _session, Seq = Next() };
    }

    public GameStateChanged PhaseChanged(string phase)
    {
        RequireEvent();
        return new GameStateChanged { Session = _session, Seq = Next(), Phase = phase };
    }

    public PlayerJoined PlayerJoined(Player player)
    {
        RequireEvent();
        return new PlayerJoined { Session = _session, Seq = Next(), Player = player };
    }

    public PlayerLeft PlayerLeft(Player player)
    {
        RequireEvent();
        return new PlayerLeft { Session = _session, Seq = Next(), Player = player };
    }

    public PlayerChanged PlayerChanged(Player player)
    {
        RequireEvent();
        return new PlayerChanged { Session = _session, Seq = Next(), Player = player };
    }

    public PlayerDied PlayerDied(Player player)
    {
        RequireEvent();
        return new PlayerDied { Session = _session, Seq = Next(), Player = player };
    }

    public GameEnded GameEnded()
    {
        RequireEvent();
        return new GameEnded { Session = _session, Seq = Next() };
    }

    private ulong Next() => ++_seq;

    private void RequireOpenSession()
    {
        if (_session.Length == 0)
        {
            throw new InvalidOperationException("open a session with Open() before sending anything");
        }
    }

    // Producing an event before the snapshot would build a message the bot is
    // certain to refuse. Failing here names the mistake where it was made,
    // instead of turning it into a protocol error from the far end.
    private void RequireEvent()
    {
        RequireOpenSession();

        if (!_snapshotSent)
        {
            throw new InvalidOperationException(
                "send a complete snapshot before any incremental event");
        }
    }
}
