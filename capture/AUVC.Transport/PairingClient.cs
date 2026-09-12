using System.Net;
using System.Net.Http.Json;
using System.Text.Json.Serialization;

namespace AUVC.Transport;

/// <summary>What the bot answered when capture tried to pair.</summary>
public sealed record PairingResult
{
    /// <summary>The guild the code belonged to.</summary>
    public required string Guild { get; init; }

    /// <summary>
    /// The credential to store. This arrives exactly once; the bot keeps only a
    /// hash and cannot send it again.
    /// </summary>
    public required string Credential { get; init; }
}

/// <summary>
/// Raised when the bot refused a pairing attempt. The message is written for
/// the person who typed the code.
/// </summary>
public sealed class PairingRefusedException(string message) : Exception(message);

/// <summary>
/// Exchanges a pairing code for a credential.
/// </summary>
/// <remarks>
/// Pairing is a single HTTP request rather than part of the WebSocket protocol:
/// it happens once, before there is a session to speak in. Capture sends the
/// code and nothing else — it never learns a guild id, because nobody is going
/// to copy a Discord snowflake out of a developer menu.
/// </remarks>
public sealed class PairingClient(HttpClient http)
{
    private sealed record Request
    {
        [JsonPropertyName("code")] public required string Code { get; init; }
    }

    private sealed record Response
    {
        [JsonPropertyName("guild")] public string Guild { get; init; } = "";
        [JsonPropertyName("credential")] public string Credential { get; init; } = "";
        [JsonPropertyName("error")] public string Error { get; init; } = "";
        [JsonPropertyName("message")] public string Message { get; init; } = "";
    }

    /// <summary>
    /// Redeems a code against the bot at <paramref name="botAddress"/>.
    /// </summary>
    /// <exception cref="PairingRefusedException">
    /// The code was wrong, already used or expired.
    /// </exception>
    public async Task<PairingResult> PairAsync(
        Uri botAddress, string code, CancellationToken cancellationToken = default)
    {
        if (string.IsNullOrWhiteSpace(code))
        {
            throw new PairingRefusedException("Enter the pairing code from /au capture pair.");
        }

        var endpoint = new Uri(botAddress, "/capture/pair");

        HttpResponseMessage response;
        try
        {
            response = await http.PostAsJsonAsync(
                endpoint, new Request { Code = code.Trim() }, cancellationToken);
        }
        catch (HttpRequestException error)
        {
            // A connection failure is not a rejected code, and telling a user to
            // ask for a new one when the bot is simply unreachable sends them
            // down the wrong path entirely.
            throw new PairingRefusedException(
                $"Could not reach the AUVC bot at {botAddress}. Check the address and that the bot is running. ({error.Message})");
        }

        var body = await ReadBodyAsync(response, cancellationToken);

        if (response.StatusCode == HttpStatusCode.OK && body.Credential.Length > 0)
        {
            return new PairingResult { Guild = body.Guild, Credential = body.Credential };
        }

        throw new PairingRefusedException(Explain(response.StatusCode, body));
    }

    private static async Task<Response> ReadBodyAsync(
        HttpResponseMessage response, CancellationToken cancellationToken)
    {
        try
        {
            return await response.Content.ReadFromJsonAsync<Response>(cancellationToken)
                   ?? new Response();
        }
        catch (Exception)
        {
            // An answer that is not the expected JSON is usually a proxy or a
            // wrong address rather than the bot; the status code below says
            // more than a parse error would.
            return new Response();
        }
    }

    private static string Explain(HttpStatusCode status, Response body) => body.Error switch
    {
        "expired" => "That pairing code has expired. Ask an administrator for a new one with /au capture pair.",
        "rejected" => "That pairing code is not valid. Ask an administrator for a new one with /au capture pair.",
        _ when body.Message.Length > 0 => body.Message,
        _ => $"The AUVC bot refused the pairing request ({(int)status}).",
    };
}
