using System.Text.Json;

namespace AUVC.Protocol;

/// <summary>
/// Thrown when a document cannot be understood as a protocol message.
/// </summary>
public sealed class MalformedMessageException(string message) : Exception(message);

/// <summary>
/// Reads and writes protocol messages.
/// </summary>
/// <remarks>
/// The envelope is read first so that a message of an unknown type produces a
/// clear error instead of a half-filled object. This mirrors Decode in
/// bot/pkg/protocol/message.go; the shared fixtures keep the two honest.
/// </remarks>
public static class ProtocolCodec
{
    private static readonly JsonSerializerOptions Options = new()
    {
        // The contract spells every field out. Ignoring case would let a
        // capture build send "Seq" and have it quietly work here but not in Go.
        PropertyNameCaseInsensitive = false,
    };

    /// <summary>Writes a message as JSON.</summary>
    public static string Encode(Message message) =>
        JsonSerializer.Serialize(message, message.GetType(), Options);

    /// <summary>Reads one JSON message into its typed form.</summary>
    /// <exception cref="MalformedMessageException">
    /// The document is not JSON, carries no type, or names a type the contract
    /// does not define.
    /// </exception>
    public static Message Decode(string json)
    {
        JsonDocument document;
        try
        {
            document = JsonDocument.Parse(json);
        }
        catch (JsonException error)
        {
            throw new MalformedMessageException($"not a JSON document: {error.Message}");
        }

        using (document)
        {
            if (!document.RootElement.TryGetProperty("type", out var typeElement) ||
                typeElement.ValueKind != JsonValueKind.String)
            {
                throw new MalformedMessageException("message has no type");
            }

            var type = typeElement.GetString() ?? "";
            return type switch
            {
                ProtocolContract.TypeHello => Read<Hello>(json),
                ProtocolContract.TypeAuthentication => Read<Authentication>(json),
                ProtocolContract.TypeHeartbeat => Read<Heartbeat>(json),
                ProtocolContract.TypeSnapshot => Read<Snapshot>(json),
                ProtocolContract.TypeGameStateChanged => Read<GameStateChanged>(json),
                ProtocolContract.TypePlayerJoined => Read<PlayerJoined>(json),
                ProtocolContract.TypePlayerLeft => Read<PlayerLeft>(json),
                ProtocolContract.TypePlayerChanged => Read<PlayerChanged>(json),
                ProtocolContract.TypePlayerDied => Read<PlayerDied>(json),
                ProtocolContract.TypeGameEnded => Read<GameEnded>(json),
                ProtocolContract.TypeError => Read<ProtocolError>(json),
                "" => throw new MalformedMessageException("message has no type"),
                _ => throw new MalformedMessageException($"unknown message type '{type}'"),
            };
        }
    }

    private static TMessage Read<TMessage>(string json) where TMessage : Message
    {
        try
        {
            return JsonSerializer.Deserialize<TMessage>(json, Options)
                   ?? throw new MalformedMessageException("message is null");
        }
        catch (JsonException error)
        {
            throw new MalformedMessageException(error.Message);
        }
    }
}

/// <summary>
/// The checks capture runs on a message before sending it.
/// </summary>
/// <remarks>
/// These mirror the bot's receiver so that capture finds its own mistakes
/// locally instead of learning about them from a refusal mid-round. The bot
/// still checks everything itself: this is a courtesy, not a security boundary.
/// </remarks>
public static class MessageValidator
{
    /// <summary>
    /// Returns the reason the message would be refused, or null if it is valid.
    /// </summary>
    public static string? Refusal(Message message)
    {
        if (message.Protocol != ProtocolContract.Version)
        {
            return $"this capture speaks protocol {ProtocolContract.Version}, the message says {message.Protocol}";
        }
        if (!ProtocolContract.IsKnownType(message.Type))
        {
            return $"unknown message type '{message.Type}'";
        }
        if (string.IsNullOrEmpty(message.Session))
        {
            return "the message carries no session";
        }
        if (message.Seq == 0)
        {
            return "sequence numbers start at 1";
        }

        return message switch
        {
            Hello hello when string.IsNullOrEmpty(hello.Capture) =>
                "hello does not say which capture build it is",
            Authentication authentication when string.IsNullOrEmpty(authentication.Credential) =>
                "authentication carries no credential",
            Snapshot snapshot => SnapshotRefusal(snapshot),
            GameStateChanged changed when !ProtocolContract.IsKnownPhase(changed.Phase) =>
                $"unknown phase '{changed.Phase}'",
            PlayerJoined joined => PlayerRefusal(joined.Player),
            PlayerLeft left => PlayerRefusal(left.Player),
            PlayerChanged changed => PlayerRefusal(changed.Player),
            PlayerDied died => PlayerRefusal(died.Player),
            ProtocolError error when string.IsNullOrEmpty(error.Code) =>
                "error carries no code",
            _ => null,
        };
    }

    private static string? SnapshotRefusal(Snapshot snapshot)
    {
        if (!ProtocolContract.IsKnownPhase(snapshot.Phase))
        {
            return $"unknown phase '{snapshot.Phase}'";
        }
        return snapshot.Players.Any(player => string.IsNullOrEmpty(player.Name))
            ? "a player in the snapshot has no name"
            : null;
    }

    // The name is what links a player to a Discord account, so an event without
    // one cannot be acted on at all.
    private static string? PlayerRefusal(Player player) =>
        string.IsNullOrEmpty(player.Name) ? "the event carries a player with no name" : null;
}
