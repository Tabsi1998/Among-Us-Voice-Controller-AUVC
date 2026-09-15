using System.Globalization;
using System.Net.Http.Headers;
using System.Net.Http.Json;
using System.Text.Json.Serialization;

namespace AUVC.Transport;

/// <summary>The bot as a whole, as the local control interface reports it.</summary>
public sealed record LocalStatus
{
    [JsonPropertyName("connected")] public bool Connected { get; init; }
    [JsonPropertyName("bot_id")] public string BotId { get; init; } = "";
    [JsonPropertyName("bot_name")] public string BotName { get; init; } = "";
    [JsonPropertyName("version")] public string Version { get; init; } = "";
    [JsonPropertyName("guilds")] public IReadOnlyList<LocalServer> Servers { get; init; } = [];
}

/// <summary>One server the bot is in.</summary>
public sealed record LocalServer
{
    [JsonPropertyName("id")] public string Id { get; init; } = "";
    [JsonPropertyName("name")] public string Name { get; init; } = "";
}

/// <summary>One channel the setup can choose.</summary>
public sealed record LocalChannel
{
    public const string Voice = "voice";
    public const string Text = "text";

    [JsonPropertyName("id")] public string Id { get; init; } = "";
    [JsonPropertyName("name")] public string Name { get; init; } = "";
    [JsonPropertyName("kind")] public string Kind { get; init; } = "";
    [JsonPropertyName("position")] public int Position { get; init; }
}

/// <summary>One server: how it is set up, and how the bot's doctor judges it.</summary>
public sealed record LocalGuild
{
    [JsonPropertyName("id")] public string Id { get; init; } = "";
    [JsonPropertyName("name")] public string Name { get; init; } = "";
    [JsonPropertyName("main_voice_channel_id")] public string MainVoiceChannelId { get; init; } = "";
    [JsonPropertyName("ghost_voice_channel_id")] public string GhostVoiceChannelId { get; init; } = "";
    [JsonPropertyName("control_text_channel_id")] public string ControlTextChannelId { get; init; } = "";
    [JsonPropertyName("auto_start")] public bool AutoStart { get; init; }
    public const string SessionRunning = "running";
    public const string SessionPaused = "paused";
    public const string SessionStopped = "stopped";

    [JsonPropertyName("capture_connections")] public int CaptureConnections { get; init; }

    /// <summary>Running, paused or stopped, as /au session status reports it; empty from an older bot.</summary>
    [JsonPropertyName("session")] public string Session { get; init; } = "";

    [JsonPropertyName("checks")] public IReadOnlyList<LocalCheck> Checks { get; init; } = [];
}

/// <summary>One line of the doctor's report.</summary>
public sealed record LocalCheck
{
    public const string Ok = "ok";
    public const string Warn = "warn";
    public const string Fail = "fail";

    [JsonPropertyName("name")] public string Name { get; init; } = "";
    [JsonPropertyName("level")] public string Level { get; init; } = "";
    [JsonPropertyName("detail")] public string Detail { get; init; } = "";
    [JsonPropertyName("fix")] public string Fix { get; init; } = "";
}

/// <summary>What the setup chose for a server.</summary>
public sealed record LocalSetup
{
    [JsonPropertyName("main_voice_channel_id")] public string MainVoiceChannelId { get; init; } = "";
    [JsonPropertyName("ghost_voice_channel_id")] public string GhostVoiceChannelId { get; init; } = "";
    [JsonPropertyName("control_text_channel_id")] public string ControlTextChannelId { get; init; } = "";
    [JsonPropertyName("auto_start")] public bool AutoStart { get; init; }
}

/// <summary>Who plays in the lobby, and whom the app can link them to.</summary>
public sealed record LocalCrewmates
{
    [JsonPropertyName("players")] public IReadOnlyList<LocalCrewmate> Players { get; init; } = [];
    [JsonPropertyName("members")] public IReadOnlyList<LocalMember> Members { get; init; } = [];
}

/// <summary>One player in the lobby. <see cref="UserId"/> is empty while nobody is linked.</summary>
public sealed record LocalCrewmate
{
    [JsonPropertyName("name")] public string Name { get; init; } = "";
    [JsonPropertyName("color")] public string Color { get; init; } = "";
    [JsonPropertyName("user_id")] public string UserId { get; init; } = "";
}

/// <summary>A Discord member a crewmate can be linked to.</summary>
public sealed record LocalMember
{
    [JsonPropertyName("id")] public string Id { get; init; } = "";
    [JsonPropertyName("name")] public string Name { get; init; } = "";
}

/// <summary>A crewmate linked to a member, or unlinked with an empty member id.</summary>
public sealed record LocalLink
{
    [JsonPropertyName("player")] public string Player { get; init; } = "";
    [JsonPropertyName("user_id")] public string UserId { get; init; } = "";
}

/// <summary>
/// Raised when the bot refused a request. The message is the bot's, written for a
/// person.
/// </summary>
public sealed class LocalControlException(string code, string message) : Exception(message)
{
    public string Code { get; } = code;
}

/// <summary>
/// Talks to the local control interface of a bot this app started.
/// </summary>
/// <remarks>
/// The secret is the one the app gave the bot when it started it. It goes into
/// the Authorization header of every request and nowhere else.
/// </remarks>
public sealed class LocalControlClient
{
    private readonly HttpClient _http;
    private readonly string _secret;

    public LocalControlClient(HttpClient http, Uri botAddress, string secret)
    {
        if (string.IsNullOrEmpty(secret))
        {
            throw new ArgumentException("the local control secret is required", nameof(secret));
        }

        _http = http;
        Address = botAddress;
        _secret = secret;
    }

    /// <summary>Where the bot listens, for capture as well as for this client.</summary>
    public Uri Address { get; }

    public Task<LocalStatus> GetStatusAsync(CancellationToken cancellationToken = default) =>
        SendAsync<LocalStatus>(HttpMethod.Get, "/local/status", null, cancellationToken);

    public Task<LocalGuild> GetGuildAsync(string guildId, CancellationToken cancellationToken = default) =>
        SendAsync<LocalGuild>(HttpMethod.Get, GuildPath(guildId), null, cancellationToken);

    public async Task<IReadOnlyList<LocalChannel>> GetChannelsAsync(
        string guildId, CancellationToken cancellationToken = default) =>
        await SendAsync<List<LocalChannel>>(HttpMethod.Get, GuildPath(guildId) + "/channels", null, cancellationToken);

    /// <summary>Saves a setup and returns the server as the bot now has it.</summary>
    public Task<LocalGuild> SetupAsync(string guildId, LocalSetup setup, CancellationToken cancellationToken = default) =>
        SendAsync<LocalGuild>(HttpMethod.Put, GuildPath(guildId) + "/setup", setup, cancellationToken);

    /// <summary>The players in the lobby, and the members they can be linked to.</summary>
    public Task<LocalCrewmates> GetCrewmatesAsync(string guildId, CancellationToken cancellationToken = default) =>
        SendAsync<LocalCrewmates>(HttpMethod.Get, GuildPath(guildId) + "/crewmates", null, cancellationToken);

    /// <summary>
    /// Links a crewmate to a member, or unlinks it for an empty member id, and
    /// returns the lobby as the bot now has it.
    /// </summary>
    public Task<LocalCrewmates> LinkAsync(string guildId, string player, string userId, CancellationToken cancellationToken = default) =>
        SendAsync<LocalCrewmates>(HttpMethod.Put, GuildPath(guildId) + "/links",
            new LocalLink { Player = player, UserId = userId }, cancellationToken);

    /// <summary>Obtains a capture credential for a server, without a pairing code.</summary>
    public async Task<string> IssueCredentialAsync(string guildId, CancellationToken cancellationToken = default)
    {
        var answer = await SendAsync<CredentialAnswer>(HttpMethod.Post, GuildPath(guildId) + "/credential", null, cancellationToken);
        if (string.IsNullOrEmpty(answer.Credential))
        {
            throw new LocalControlException("empty", "The bot issued no credential.");
        }
        return answer.Credential;
    }

    /// <summary>Asks the bot to release everyone and stop.</summary>
    public async Task ShutdownAsync(CancellationToken cancellationToken = default) =>
        await SendAsync<StopAnswer>(HttpMethod.Post, "/local/shutdown", null, cancellationToken);

    private static string GuildPath(string guildId) => "/local/guilds/" + Uri.EscapeDataString(guildId);

    /// <exception cref="HttpRequestException">The bot is not listening.</exception>
    /// <exception cref="LocalControlException">The bot refused the request.</exception>
    private async Task<T> SendAsync<T>(HttpMethod method, string path, object? body, CancellationToken cancellationToken)
    {
        using var request = new HttpRequestMessage(method, new Uri(Address, path));
        request.Headers.Authorization = new AuthenticationHeaderValue("Bearer", _secret);
        // The bot writes its checks in this language, so they match the app around them.
        request.Headers.AcceptLanguage.Add(new StringWithQualityHeaderValue(CultureInfo.CurrentUICulture.TwoLetterISOLanguageName));
        if (body is not null)
        {
            request.Content = JsonContent.Create(body, body.GetType());
        }

        using var response = await _http.SendAsync(request, cancellationToken);

        if (!response.IsSuccessStatusCode)
        {
            ErrorAnswer? error = null;
            try
            {
                error = await response.Content.ReadFromJsonAsync<ErrorAnswer>(cancellationToken);
            }
            catch (Exception) when (!cancellationToken.IsCancellationRequested)
            {
                // Not the bot's error format; the status code says enough.
            }

            throw new LocalControlException(
                string.IsNullOrEmpty(error?.Error) ? $"http_{(int)response.StatusCode}" : error.Error,
                string.IsNullOrEmpty(error?.Message) ? $"The bot answered {(int)response.StatusCode}." : error.Message);
        }

        return await response.Content.ReadFromJsonAsync<T>(cancellationToken)
               ?? throw new LocalControlException("empty", "The bot sent an empty answer.");
    }

    private sealed record CredentialAnswer
    {
        [JsonPropertyName("credential")] public string Credential { get; init; } = "";
    }

    private sealed record StopAnswer
    {
        [JsonPropertyName("status")] public string Status { get; init; } = "";
    }

    private sealed record ErrorAnswer
    {
        [JsonPropertyName("error")] public string Error { get; init; } = "";
        [JsonPropertyName("message")] public string Message { get; init; } = "";
    }
}
