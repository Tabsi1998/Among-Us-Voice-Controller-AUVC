using System.Net;

namespace AUVC.Transport;

/// <summary>
/// Turns what somebody typed as the bot's address into one capture can use.
/// </summary>
public static class BotAddress
{
    /// <summary>
    /// Where the bot listens when nothing says otherwise: the same PC.
    /// </summary>
    public const string Default = "http://127.0.0.1:8123";

    /// <summary>
    /// Parses an address. A missing scheme means <c>http</c>, <c>ws</c> and
    /// <c>wss</c> are accepted as their HTTP equivalents, and any path is
    /// dropped, because the bot's endpoints are fixed.
    /// </summary>
    /// <param name="problem">Why the text is not an address, written for the person who typed it.</param>
    public static bool TryParse(string? text, out Uri address, out string problem)
    {
        address = new Uri(Default);
        problem = "";

        var typed = text?.Trim() ?? "";
        if (typed.Length == 0)
        {
            problem = $"Enter the address of the AUVC bot, for example {Default}.";
            return false;
        }

        var withScheme = typed.Contains("://", StringComparison.Ordinal) ? typed : "http://" + typed;
        if (!Uri.TryCreate(withScheme, UriKind.Absolute, out var parsed) || string.IsNullOrEmpty(parsed.Host))
        {
            problem = $"'{typed}' is not an address. Use the form {Default}.";
            return false;
        }

        var scheme = parsed.Scheme switch
        {
            "http" or "ws" => Uri.UriSchemeHttp,
            "https" or "wss" => Uri.UriSchemeHttps,
            _ => null,
        };
        if (scheme is null)
        {
            problem = $"'{typed}' does not start with http:// or https://.";
            return false;
        }

        var port = parsed.IsDefaultPort ? -1 : parsed.Port;
        address = new UriBuilder(scheme, parsed.Host, port).Uri;
        return true;
    }

    /// <summary>
    /// Whether the credential would cross a network unencrypted: plain HTTP to
    /// anything but this PC.
    /// </summary>
    public static bool SendsCredentialInClear(Uri address) =>
        address.Scheme == Uri.UriSchemeHttp && !IsThisComputer(address);

    private static bool IsThisComputer(Uri address) =>
        address.IsLoopback || (IPAddress.TryParse(address.Host, out var ip) && IPAddress.IsLoopback(ip));
}
