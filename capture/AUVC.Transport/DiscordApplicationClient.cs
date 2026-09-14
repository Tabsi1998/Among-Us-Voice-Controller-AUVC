using System.Globalization;
using System.Net;
using System.Net.Http.Headers;
using System.Net.Http.Json;
using System.Text.Json.Serialization;

namespace AUVC.Transport;

/// <summary>The Discord application a bot token belongs to.</summary>
/// <param name="ApplicationId">The id the invite link needs.</param>
/// <param name="BotName">What the bot is called in Discord.</param>
/// <param name="RequiresCodeGrant">
/// Whether "Requires OAuth2 Code Grant" is switched on, which makes the ordinary
/// invite link fail.
/// </param>
public sealed record BotApplication(string ApplicationId, string BotName, bool RequiresCodeGrant);

/// <summary>Why a token could not be used.</summary>
public enum TokenProblem
{
    /// <summary>Nothing was entered.</summary>
    Empty,

    /// <summary>Discord does not accept the token.</summary>
    Rejected,

    /// <summary>Discord could not be reached.</summary>
    Unreachable,

    /// <summary>Discord answered, but not in a way that describes an application.</summary>
    Unexpected,
}

/// <summary>
/// Raised when a bot token cannot be used. The message never contains the token.
/// </summary>
public sealed class BotTokenException(TokenProblem problem, string message) : Exception(message)
{
    public TokenProblem Problem { get; } = problem;
}

/// <summary>
/// Asks Discord about a bot token, so the setup can say straight away whether it
/// works and build the invite link for it.
/// </summary>
public sealed class DiscordApplicationClient(HttpClient http)
{
    private static readonly Uri ApplicationEndpoint = new("https://discord.com/api/v10/oauth2/applications/@me");

    public const long ViewChannel = 1L << 10;
    public const long SendMessages = 1L << 11;
    public const long EmbedLinks = 1L << 14;
    public const long Connect = 1L << 20;
    public const long MuteMembers = 1L << 22;
    public const long DeafenMembers = 1L << 23;
    public const long MoveMembers = 1L << 24;

    /// <summary>
    /// Every permission AUVC uses, and nothing more: seeing and joining the voice
    /// channels, muting, deafening and moving players, and posting warnings and
    /// the lobby message in the control channel.
    /// </summary>
    public const long RequiredPermissions =
        ViewChannel | SendMessages | EmbedLinks | Connect | MuteMembers | DeafenMembers | MoveMembers;

    /// <summary>
    /// Trims what was pasted and drops a leading "Bot ", which people copy from
    /// examples of the Authorization header.
    /// </summary>
    public static string NormalizeToken(string? pasted)
    {
        var token = pasted?.Trim() ?? "";
        if (token.StartsWith("Bot ", StringComparison.OrdinalIgnoreCase))
        {
            token = token[4..].Trim();
        }
        return token;
    }

    /// <exception cref="BotTokenException">The token cannot be used.</exception>
    public async Task<BotApplication> CheckTokenAsync(string? pasted, CancellationToken cancellationToken = default)
    {
        var token = NormalizeToken(pasted);
        if (token.Length == 0)
        {
            throw new BotTokenException(TokenProblem.Empty, "Paste the bot token first.");
        }

        using var request = new HttpRequestMessage(HttpMethod.Get, ApplicationEndpoint);
        request.Headers.Authorization = new AuthenticationHeaderValue("Bot", token);

        HttpResponseMessage response;
        try
        {
            response = await http.SendAsync(request, cancellationToken);
        }
        catch (HttpRequestException error)
        {
            throw new BotTokenException(TokenProblem.Unreachable,
                $"Discord could not be reached. Check the internet connection. ({error.Message})");
        }
        catch (TaskCanceledException) when (!cancellationToken.IsCancellationRequested)
        {
            throw new BotTokenException(TokenProblem.Unreachable, "Discord did not answer in time.");
        }

        using (response)
        {
            if (response.StatusCode is HttpStatusCode.Unauthorized or HttpStatusCode.Forbidden)
            {
                throw new BotTokenException(TokenProblem.Rejected,
                    "Discord does not accept this token. Copy it again from the Bot page of the application.");
            }
            if (!response.IsSuccessStatusCode)
            {
                throw new BotTokenException(TokenProblem.Unexpected,
                    $"Discord answered {(int)response.StatusCode}. Try again in a moment.");
            }

            ApplicationResponse? body;
            try
            {
                body = await response.Content.ReadFromJsonAsync<ApplicationResponse>(cancellationToken);
            }
            catch (Exception) when (!cancellationToken.IsCancellationRequested)
            {
                body = null;
            }

            if (body is null || string.IsNullOrEmpty(body.Id))
            {
                throw new BotTokenException(TokenProblem.Unexpected, "Discord's answer did not describe an application.");
            }

            var name = string.IsNullOrEmpty(body.Bot?.Username) ? body.Name : body.Bot.Username;
            return new BotApplication(body.Id, name, body.BotRequireCodeGrant);
        }
    }

    /// <summary>
    /// The link that adds the bot to a server with exactly the permissions it
    /// needs, and the slash commands.
    /// </summary>
    public static Uri InviteUrl(string applicationId) => new(
        "https://discord.com/oauth2/authorize?client_id=" + Uri.EscapeDataString(applicationId) +
        "&scope=bot%20applications.commands&permissions=" +
        RequiredPermissions.ToString(CultureInfo.InvariantCulture));

    private sealed record ApplicationResponse
    {
        [JsonPropertyName("id")] public string Id { get; init; } = "";
        [JsonPropertyName("name")] public string Name { get; init; } = "";
        [JsonPropertyName("bot_require_code_grant")] public bool BotRequireCodeGrant { get; init; }
        [JsonPropertyName("bot")] public BotUser? Bot { get; init; }
    }

    private sealed record BotUser
    {
        [JsonPropertyName("username")] public string Username { get; init; } = "";
    }
}
