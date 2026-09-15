using System.Text.Json.Serialization;

namespace AUVC.Protocol;

/// <summary>
/// The constants of the capture-to-bot contract. They mirror
/// bot/pkg/protocol/message.go, and both sides are tested against the shared
/// fixtures in protocol/fixtures so the two cannot drift apart silently.
/// </summary>
public static class ProtocolContract
{
    /// <summary>The protocol version this build speaks.</summary>
    public const int Version = 1;

    public const string TypeHello = "hello";
    public const string TypeAuthentication = "authentication";
    public const string TypeHeartbeat = "heartbeat";
    public const string TypeSnapshot = "snapshot";
    public const string TypeGameStateChanged = "game_state_changed";
    public const string TypePlayerJoined = "player_joined";
    public const string TypePlayerLeft = "player_left";
    public const string TypePlayerChanged = "player_changed";
    public const string TypePlayerDied = "player_died";
    public const string TypeGameEnded = "game_ended";
    public const string TypeError = "error";

    /// <summary>
    /// Every message type in the contract, in handshake-then-events order.
    /// Tests walk it so a new type cannot be added on one side only.
    /// </summary>
    public static readonly IReadOnlyList<string> Types =
    [
        TypeHello,
        TypeAuthentication,
        TypeHeartbeat,
        TypeSnapshot,
        TypeGameStateChanged,
        TypePlayerJoined,
        TypePlayerLeft,
        TypePlayerChanged,
        TypePlayerDied,
        TypeGameEnded,
        TypeError,
    ];

    public const string PhaseLobby = "lobby";
    public const string PhaseTasks = "tasks";
    public const string PhaseDiscussion = "discussion";
    public const string PhaseMenu = "menu";
    public const string PhaseEnded = "ended";

    /// <summary>Every phase value the contract defines.</summary>
    public static readonly IReadOnlyList<string> Phases =
        [PhaseLobby, PhaseTasks, PhaseDiscussion, PhaseMenu, PhaseEnded];

    // Error codes are part of the contract, so both sides can react to a
    // refusal without parsing prose.
    public const string CodeIncompatibleProtocol = "incompatible_protocol";
    public const string CodeExpectedHello = "expected_hello";
    public const string CodeUnauthenticated = "unauthenticated";
    public const string CodeSnapshotRequired = "snapshot_required";
    public const string CodeMalformed = "malformed";

    // Map names as they travel on the wire. The bot ignores a map it does not
    // know rather than refusing it, so a newer capture works with an older bot.
    public const string MapTheSkeld = "the_skeld";
    public const string MapMiraHQ = "mira_hq";
    public const string MapPolus = "polus";
    public const string MapDleks = "dleks";
    public const string MapAirship = "airship";
    public const string MapFungle = "fungle";

    /// <summary>Every map name the contract defines.</summary>
    public static readonly IReadOnlyList<string> Maps =
        [MapTheSkeld, MapMiraHQ, MapPolus, MapDleks, MapAirship, MapFungle];

    /// <summary>
    /// Whether a lobby code is one the game uses: four or six capital letters, or
    /// six asterisks when the host hides it.
    /// </summary>
    public static bool IsLobbyCode(string code) =>
        code == "******" || (code.Length is 4 or 6 && code.All(letter => letter is >= 'A' and <= 'Z'));

    public static bool IsKnownPhase(string phase) => Phases.Contains(phase);

    public static bool IsKnownType(string type) => Types.Contains(type);
}

/// <summary>One player as capture sees them in the game.</summary>
/// <remarks>
/// The in-game name is what links a player to a Discord account. There is no
/// Discord identity here on purpose: capture reads the game and must not need
/// to know anything about Discord.
/// </remarks>
public sealed record Player
{
    [JsonPropertyName("name")] public string Name { get; init; } = "";
    [JsonPropertyName("color")] public int Color { get; init; }
    [JsonPropertyName("dead")] public bool Dead { get; init; }
    [JsonPropertyName("disconnected")] public bool Disconnected { get; init; }
}

/// <summary>
/// The lobby capture is in: its code and its map. It travels on a snapshot and on
/// a phase change, and is left out while capture knows neither.
/// </summary>
public sealed record Lobby
{
    /// <summary>The lobby code, or six asterisks when the host hides it.</summary>
    [JsonPropertyName("code")] public string Code { get; init; } = "";

    /// <summary>One of <see cref="ProtocolContract.Maps"/>, or empty when capture does not know the map.</summary>
    [JsonPropertyName("map")] public string Map { get; init; } = "";
}

/// <summary>
/// The envelope every message carries.
/// </summary>
/// <remarks>
/// The session travels on every message rather than only on hello, which is
/// what lets the bot check the rules from the messages alone, with no reference
/// to a connection.
/// </remarks>
public abstract record Message
{
    [JsonPropertyName("protocol")] public int Protocol { get; init; } = ProtocolContract.Version;
    [JsonPropertyName("type")] public string Type { get; init; } = "";
    [JsonPropertyName("session")] public string Session { get; init; } = "";
    [JsonPropertyName("seq")] public ulong Seq { get; init; }
}

/// <summary>Opens a session. Always the first message.</summary>
public sealed record Hello : Message
{
    public Hello() => Type = ProtocolContract.TypeHello;

    /// <summary>The capture build, for diagnostics and /au version.</summary>
    [JsonPropertyName("capture")] public string Capture { get; init; } = "";
}

/// <summary>
/// Presents the long-term credential issued by /au capture pair. The credential
/// is a secret and must never reach a log.
/// </summary>
public sealed record Authentication : Message
{
    public Authentication() => Type = ProtocolContract.TypeAuthentication;

    [JsonPropertyName("credential")] public string Credential { get; init; } = "";
}

/// <summary>Capture reporting that it is alive and still reading the game.</summary>
public sealed record Heartbeat : Message
{
    public Heartbeat() => Type = ProtocolContract.TypeHeartbeat;
}

/// <summary>
/// The complete state of the round. Capture sends one after every reconnect,
/// because incremental events alone cannot rebuild what the bot missed.
/// </summary>
public sealed record Snapshot : Message
{
    public Snapshot() => Type = ProtocolContract.TypeSnapshot;

    [JsonPropertyName("phase")] public string Phase { get; init; } = "";
    [JsonPropertyName("players")] public IReadOnlyList<Player> Players { get; init; } = [];

    [JsonPropertyName("lobby")]
    [JsonIgnore(Condition = JsonIgnoreCondition.WhenWritingNull)]
    public Lobby? Lobby { get; init; }
}

/// <summary>Reports a phase transition.</summary>
public sealed record GameStateChanged : Message
{
    public GameStateChanged() => Type = ProtocolContract.TypeGameStateChanged;

    [JsonPropertyName("phase")] public string Phase { get; init; } = "";

    /// <summary>
    /// The lobby, once capture has read it. The game reads it only after the
    /// phase has changed, so joining a lobby is a change into the same phase.
    /// </summary>
    [JsonPropertyName("lobby")]
    [JsonIgnore(Condition = JsonIgnoreCondition.WhenWritingNull)]
    public Lobby? Lobby { get; init; }
}

/// <summary>Reports a player entering the lobby.</summary>
public sealed record PlayerJoined : Message
{
    public PlayerJoined() => Type = ProtocolContract.TypePlayerJoined;

    [JsonPropertyName("player")] public Player Player { get; init; } = new();
}

/// <summary>Reports a player leaving.</summary>
public sealed record PlayerLeft : Message
{
    public PlayerLeft() => Type = ProtocolContract.TypePlayerLeft;

    [JsonPropertyName("player")] public Player Player { get; init; } = new();
}

/// <summary>Reports a change that is neither death nor departure.</summary>
public sealed record PlayerChanged : Message
{
    public PlayerChanged() => Type = ProtocolContract.TypePlayerChanged;

    [JsonPropertyName("player")] public Player Player { get; init; } = new();
}

/// <summary>
/// Reports a death. Separate from <see cref="PlayerChanged"/> because it is the
/// event the voice policy reacts to.
/// </summary>
public sealed record PlayerDied : Message
{
    public PlayerDied() => Type = ProtocolContract.TypePlayerDied;

    [JsonPropertyName("player")] public Player Player { get; init; } = new();
}

/// <summary>Reports that the round is over.</summary>
public sealed record GameEnded : Message
{
    public GameEnded() => Type = ProtocolContract.TypeGameEnded;
}

/// <summary>
/// Reports a refusal. The code is stable and machine-readable; the message is
/// written for a person reading a log or a Discord warning.
/// </summary>
public sealed record ProtocolError : Message
{
    public ProtocolError() => Type = ProtocolContract.TypeError;

    [JsonPropertyName("code")] public string Code { get; init; } = "";
    [JsonPropertyName("message")] public string Message { get; init; } = "";
}
