using System.Text.Json;

namespace AUVC.Transport;

/// <summary>A release as GitHub lists it.</summary>
/// <param name="Tag">The tag, such as <c>v0.1.2-beta</c>.</param>
/// <param name="PreRelease">Whether it was published as a pre-release.</param>
/// <param name="Draft">Whether it is a draft nobody can download yet.</param>
/// <param name="Page">Its page, as the list gives it.</param>
public sealed record GitHubRelease(string Tag, bool PreRelease, bool Draft, string Page);

/// <summary>A release newer than the running app, and the page to get it from.</summary>
public sealed record NewerRelease(AppVersion Version, string Page);

/// <summary>
/// Which AUVC release somebody should hear about. The app only tells: it
/// downloads and installs nothing, so all there is to decide is which version to
/// name and which page to link.
/// </summary>
public static class ReleaseFeed
{
    public const string ReleasesPage = "https://github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/releases";

    /// <summary>
    /// The public list of releases, newest first. Reading it needs no account.
    /// GitHub's "latest release" leaves pre-releases out, so the list is read instead.
    /// </summary>
    public static readonly Uri Address =
        new("https://api.github.com/repos/Tabsi1998/Among-Us-Voice-Controller-AUVC/releases?per_page=30");

    /// <summary>
    /// Reads the list GitHub answers with. Entries without a tag are skipped; an
    /// answer that is no list at all, such as a rate-limit message, is a
    /// <see cref="FormatException"/>.
    /// </summary>
    public static IReadOnlyList<GitHubRelease> Parse(string json)
    {
        JsonDocument document;
        try
        {
            document = JsonDocument.Parse(json);
        }
        catch (JsonException error)
        {
            throw new FormatException("the release list is not JSON", error);
        }

        using (document)
        {
            if (document.RootElement.ValueKind != JsonValueKind.Array)
            {
                throw new FormatException("the answer is not a list of releases");
            }

            var releases = new List<GitHubRelease>();
            foreach (var entry in document.RootElement.EnumerateArray())
            {
                if (entry.ValueKind != JsonValueKind.Object ||
                    !entry.TryGetProperty("tag_name", out var tag) || tag.ValueKind != JsonValueKind.String)
                {
                    continue;
                }
                var page = entry.TryGetProperty("html_url", out var url) && url.ValueKind == JsonValueKind.String
                    ? url.GetString()!
                    : "";
                releases.Add(new GitHubRelease(tag.GetString()!, Flag(entry, "prerelease"), Flag(entry, "draft"), page));
            }
            return releases;
        }
    }

    /// <summary>
    /// The newest release somebody running <paramref name="current"/> should hear
    /// about, or null. On a pre-release that is any newer pre-release or release;
    /// on a release only a newer release, because a release was chosen for being
    /// finished. A development build hears of nothing.
    /// </summary>
    public static NewerRelease? Newer(AppVersion current, IEnumerable<GitHubRelease> releases)
    {
        if (current.IsDevelopment)
        {
            return null;
        }

        NewerRelease? newest = null;
        foreach (var release in releases)
        {
            var version = AppVersion.Parse(release.Tag);
            if (release.Draft || version is null)
            {
                continue;
            }
            // A pre-release by its tag or by how it was published, whichever says so.
            if ((release.PreRelease || version.IsPreRelease) && !current.IsPreRelease)
            {
                continue;
            }
            if (version.CompareTo(current) <= 0 || (newest is not null && version.CompareTo(newest.Version) <= 0))
            {
                continue;
            }
            newest = new NewerRelease(version, PageOf(release.Page));
        }
        return newest;
    }

    /// <summary>
    /// The page to open for a release: its own when that is one of AUVC's release
    /// pages, otherwise the list of releases. The link is opened in the browser, so
    /// it must not lead anywhere else, whatever the answer said. The page is
    /// compared after <see cref="Uri"/> has resolved any <c>../</c>, and the
    /// prefix it must start with already fixes HTTPS and the host.
    /// </summary>
    public static string PageOf(string page)
    {
        if (Uri.TryCreate(page, UriKind.Absolute, out var uri) &&
            uri.AbsoluteUri.StartsWith(ReleasesPage + "/", StringComparison.Ordinal))
        {
            return uri.AbsoluteUri;
        }
        return ReleasesPage;
    }

    private static bool Flag(JsonElement entry, string name) =>
        entry.TryGetProperty(name, out var value) && value.ValueKind == JsonValueKind.True;
}
