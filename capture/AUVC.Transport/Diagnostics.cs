using System.IO.Compression;
using System.Text;
using System.Text.RegularExpressions;

namespace AUVC.Transport;

/// <summary>One file of a diagnostics export: its name in the zip, and its text as read.</summary>
public sealed record DiagnosticsFile(string Name, string Text);

/// <summary>
/// Blacks out what must not leave the PC in a diagnostics export. Someone who
/// asks for help shares the export, so a secret in it is a secret handed over.
/// </summary>
/// <remarks>
/// Redacting too much costs a little detail; redacting too little costs a bot
/// or a paired app. When a pattern is unsure, it redacts.
/// </remarks>
public static partial class DiagnosticsRedactor
{
    public const string Redacted = "[redacted]";

    /// <summary>What a Windows user name in a path is replaced with.</summary>
    public const string User = "[user]";

    public static string Redact(string text, string? userName = null)
    {
        text = DiscordToken().Replace(text, Redacted);
        text = Credential().Replace(text, Redacted);
        text = PairingCode().Replace(text, Redacted);
        text = Authorization().Replace(text, match => match.Groups["prefix"].Value + Redacted);
        if (!string.IsNullOrWhiteSpace(userName))
        {
            text = Regex.Replace(text, @"(?<=[\\/]Users[\\/])" + Regex.Escape(userName) + @"(?=[\\/]|$)", User,
                RegexOptions.IgnoreCase | RegexOptions.Multiline);
        }
        return text;
    }

    // A Discord bot token: three base64url parts joined by dots.
    [GeneratedRegex(@"[A-Za-z0-9_-]{24,}\.[A-Za-z0-9_-]{6,}\.[A-Za-z0-9_-]{27,}")]
    private static partial Regex DiscordToken();

    // The app's credential for the bot: a 16-character hex id, a dot, and 52 base32
    // characters. Longer is redacted too, rather than let a variant through.
    [GeneratedRegex(@"\b[0-9a-fA-F]{16}\.[A-Za-z2-7]{52,}\b")]
    private static partial Regex Credential();

    // A pairing code as Discord shows it, with or without its dashes.
    [GeneratedRegex(@"\bAUVC-?[0-9A-Z]{4}-?[0-9A-Z]{4}\b", RegexOptions.IgnoreCase)]
    private static partial Regex PairingCode();

    // The value of an Authorization header, with or without its scheme.
    [GeneratedRegex(@"(?<prefix>authorization[""']?\s*[:=]\s*[""']?(?:(?:bot|bearer|basic)\s+)?)[^\s""',;]+", RegexOptions.IgnoreCase)]
    private static partial Regex Authorization();
}

/// <summary>
/// Puts a diagnostics export together: which files may go in, the end of a long
/// log, a summary on top, and the zip with every text redacted.
/// </summary>
public static class DiagnosticsBundle
{
    /// <summary>
    /// Files that never go into an export, whatever a caller passes: the bot token,
    /// the app's credential and the bot's database with its links and hashes.
    /// </summary>
    public static readonly IReadOnlyList<string> Forbidden = ["bot-token.bin", "credential.bin", "amongus.db"];

    /// <summary>How much of one log goes in: its end, where the problem usually is.</summary>
    public const int MaxLogBytes = 2 * 1024 * 1024;

    public static bool MayInclude(string path)
    {
        var name = Path.GetFileName(path);
        return !Forbidden.Contains(name, StringComparer.OrdinalIgnoreCase)
               && !name.StartsWith("amongus.db", StringComparison.OrdinalIgnoreCase);
    }

    /// <summary>
    /// The end of a text, at most <paramref name="maxBytes"/>. When it had to be
    /// cut, the first, probably partial, line is dropped.
    /// </summary>
    public static string ReadTail(Stream stream, int maxBytes = MaxLogBytes)
    {
        var cut = stream.CanSeek && stream.Length > maxBytes;
        if (cut)
        {
            stream.Seek(-maxBytes, SeekOrigin.End);
        }
        using var reader = new StreamReader(stream, Encoding.UTF8, detectEncodingFromByteOrderMarks: !cut, 4096, leaveOpen: true);
        if (cut)
        {
            reader.ReadLine();
        }
        return reader.ReadToEnd();
    }

    /// <summary>The first file of an export: what runs, and what the bot's checks say.</summary>
    public static string Summary(
        string appVersion, string botVersion, string windows, string runtime,
        IReadOnlyList<LocalCheck> checks, DateTimeOffset created)
    {
        var text = new StringBuilder()
            .AppendLine("AUVC diagnostics")
            .AppendLine($"Created: {created:yyyy-MM-dd HH:mm:ss zzz}")
            .AppendLine($"App: {appVersion}")
            .AppendLine($"Bot: {botVersion}")
            .AppendLine($"Windows: {windows}")
            .AppendLine($".NET: {runtime}")
            .AppendLine();

        if (checks.Count == 0)
        {
            text.AppendLine("Checks from the bot: none, the bot on this PC could not be asked.");
            return text.ToString();
        }

        text.AppendLine("Checks from the bot:");
        foreach (var check in checks)
        {
            var line = $"[{check.Level}] {check.Name}: {DoctorLine.Plain(check.Detail)}";
            text.AppendLine(string.IsNullOrEmpty(check.Fix) ? line : line + " -> " + DoctorLine.Plain(check.Fix));
        }
        return text.ToString();
    }

    /// <summary>Writes the files into a zip, each text redacted on the way in.</summary>
    public static void Write(Stream output, IEnumerable<DiagnosticsFile> files, string? userName)
    {
        using var zip = new ZipArchive(output, ZipArchiveMode.Create, leaveOpen: true);
        foreach (var file in files)
        {
            if (!MayInclude(file.Name))
            {
                continue;
            }
            var entry = zip.CreateEntry(file.Name, CompressionLevel.Optimal);
            using var writer = new StreamWriter(entry.Open(), new UTF8Encoding(false));
            writer.Write(DiagnosticsRedactor.Redact(file.Text, userName));
        }
    }
}
