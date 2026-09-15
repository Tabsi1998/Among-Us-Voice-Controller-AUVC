using System.Text.RegularExpressions;

namespace AUVC.Transport;

/// <summary>
/// A doctor check as plain text for the app. The bot writes its checks for Discord,
/// with bold names, code spans and channel mentions, which a window would show
/// exactly as they are typed.
/// </summary>
public static partial class DoctorLine
{
    /// <summary>
    /// The text without Discord markdown. Bold and code marks go, and a channel
    /// mention, which only Discord can turn into a name, becomes <c>#id</c>.
    /// </summary>
    public static string Plain(string markdown) =>
        ChannelMention().Replace(markdown.Replace("**", "").Replace("`", ""), "#$1").Trim();

    /// <summary>What is wrong and, when the doctor knows, what to do about it, for the status line.</summary>
    public static string Explain(LocalCheck check) =>
        string.IsNullOrEmpty(check.Fix)
            ? Plain(check.Detail)
            : Plain(check.Detail) + Environment.NewLine + Plain(check.Fix);

    [GeneratedRegex(@"<#(\d+)>")]
    private static partial Regex ChannelMention();
}
