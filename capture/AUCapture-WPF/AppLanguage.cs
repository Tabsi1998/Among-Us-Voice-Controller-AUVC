#nullable enable
using System.Globalization;
using System.Linq;

namespace AUCapture_WPF
{
    /// <summary>
    /// Which language the app shows.
    /// </summary>
    /// <remarks>
    /// Without a choice of their own, people get the display language of Windows,
    /// read again on every start, so changing Windows changes the app too. Only
    /// languages with every text translated are offered: a half-translated one
    /// shows two languages at once, which is worse than English throughout.
    /// Nothing here needs WPF, so the tests compile this file directly.
    /// </remarks>
    public static class AppLanguage
    {
        /// <summary>The saved choice for following Windows.</summary>
        public const string SameAsWindows = "";

        /// <summary>The language for a Windows language the app does not have.</summary>
        public const string Fallback = "en";

        /// <summary>The languages every text of the app is translated into.</summary>
        public static readonly string[] Supported = ["de", "en"];

        /// <summary>
        /// The saved choice as one of <see cref="Supported"/>, or
        /// <see cref="SameAsWindows"/> for anything the app does not offer (any more).
        /// </summary>
        public static string Normalize(string? choice)
        {
            var language = choice?.Split('-')[0].ToLowerInvariant();
            return language is not null && Supported.Contains(language) ? language : SameAsWindows;
        }

        /// <summary>The language to show for the saved choice, on a Windows set to <paramref name="windows"/>.</summary>
        public static CultureInfo Resolve(string? choice, CultureInfo windows)
        {
            var language = Normalize(choice);
            if (language == SameAsWindows)
            {
                language = Supported.Contains(windows.TwoLetterISOLanguageName) ? windows.TwoLetterISOLanguageName : Fallback;
            }
            return CultureInfo.GetCultureInfo(language);
        }
    }
}
