using System.Globalization;

namespace AUVC.Transport;

/// <summary>
/// A version as AUVC tags its releases: major.minor.patch with an optional
/// pre-release such as <c>-beta</c> or <c>-rc.1</c>, ordered as Semantic
/// Versioning orders them. Build metadata after <c>+</c> is dropped: it does not
/// change which version a build is.
/// </summary>
public sealed class AppVersion : IComparable<AppVersion>
{
    private AppVersion(int major, int minor, int patch, string preRelease)
    {
        Major = major;
        Minor = minor;
        Patch = patch;
        PreRelease = preRelease;
    }

    public int Major { get; }

    public int Minor { get; }

    public int Patch { get; }

    /// <summary>The pre-release, such as <c>beta</c> or <c>rc.1</c>, or empty for a release.</summary>
    public string PreRelease { get; }

    public bool IsPreRelease => PreRelease.Length > 0;

    /// <summary>
    /// A build that is no release: 0.0.0, which development builds and the local
    /// check carry. There is nothing to update it from.
    /// </summary>
    public bool IsDevelopment => Major == 0 && Minor == 0 && Patch == 0;

    /// <summary>Reads a tag such as <c>v0.1.2-beta</c> or a build version, or returns null.</summary>
    public static AppVersion? Parse(string? text)
    {
        if (text is null)
        {
            return null;
        }

        var rest = text.Trim();
        if (rest.StartsWith('v') || rest.StartsWith('V'))
        {
            rest = rest[1..];
        }
        var plus = rest.IndexOf('+');
        if (plus >= 0)
        {
            rest = rest[..plus];
        }
        var dash = rest.IndexOf('-');
        var core = dash >= 0 ? rest[..dash] : rest;
        var preRelease = dash >= 0 ? rest[(dash + 1)..] : "";

        var numbers = core.Split('.');
        if (numbers.Length != 3)
        {
            return null;
        }
        var parsed = new int[3];
        for (var i = 0; i < parsed.Length; i++)
        {
            if (!int.TryParse(numbers[i], NumberStyles.None, CultureInfo.InvariantCulture, out parsed[i]))
            {
                return null;
            }
        }
        if (dash >= 0 && !preRelease.Split('.').All(IsIdentifier))
        {
            return null;
        }
        return new AppVersion(parsed[0], parsed[1], parsed[2], preRelease);
    }

    public int CompareTo(AppVersion? other)
    {
        if (other is null)
        {
            return 1;
        }

        var order = (Major, Minor, Patch).CompareTo((other.Major, other.Minor, other.Patch));
        if (order != 0)
        {
            return order;
        }
        // A pre-release comes before the release of the same number.
        if (IsPreRelease != other.IsPreRelease)
        {
            return IsPreRelease ? -1 : 1;
        }

        var mine = PreRelease.Split('.');
        var theirs = other.PreRelease.Split('.');
        for (var i = 0; i < Math.Min(mine.Length, theirs.Length); i++)
        {
            order = CompareIdentifiers(mine[i], theirs[i]);
            if (order != 0)
            {
                return order;
            }
        }
        return mine.Length.CompareTo(theirs.Length);
    }

    public override string ToString() =>
        IsPreRelease ? $"{Major}.{Minor}.{Patch}-{PreRelease}" : $"{Major}.{Minor}.{Patch}";

    private static bool IsIdentifier(string identifier) =>
        identifier.Length > 0 && identifier.All(letter => char.IsAsciiLetterOrDigit(letter) || letter == '-');

    // Numeric identifiers compare as numbers and come before alphanumeric ones,
    // which compare as ASCII text: beta.2 before beta.11, and 1 before alpha.
    private static int CompareIdentifiers(string mine, string theirs)
    {
        var mineNumeric = mine.All(char.IsAsciiDigit);
        var theirsNumeric = theirs.All(char.IsAsciiDigit);
        if (mineNumeric && theirsNumeric)
        {
            var a = mine.TrimStart('0');
            var b = theirs.TrimStart('0');
            return a.Length != b.Length ? a.Length.CompareTo(b.Length) : string.CompareOrdinal(a, b);
        }
        if (mineNumeric != theirsNumeric)
        {
            return mineNumeric ? -1 : 1;
        }
        return string.CompareOrdinal(mine, theirs);
    }
}
