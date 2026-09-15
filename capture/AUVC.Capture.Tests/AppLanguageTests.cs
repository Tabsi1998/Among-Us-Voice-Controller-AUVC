using System.Globalization;
using AUCapture_WPF;
using Xunit;

namespace AUVC.Capture.Tests
{
    /// <summary>
    /// Which language the app shows: Windows' own, until somebody picks another.
    /// </summary>
    public class AppLanguageTests
    {
        private static string Shown(string? choice, string windows) =>
            AppLanguage.Resolve(choice, CultureInfo.GetCultureInfo(windows)).Name;

        [Theory]
        [InlineData("de-DE", "de")]
        [InlineData("de-AT", "de")]
        [InlineData("de-CH", "de")]
        [InlineData("en-US", "en")]
        [InlineData("en-GB", "en")]
        public void WithoutAChoiceTheAppFollowsWindows(string windows, string shown) =>
            Assert.Equal(shown, Shown(AppLanguage.SameAsWindows, windows));

        [Theory]
        [InlineData("fr-FR")]
        [InlineData("ja-JP")]
        [InlineData("pt-BR")]
        public void AWindowsLanguageTheAppDoesNotHaveFallsBackToEnglish(string windows) =>
            Assert.Equal("en", Shown(AppLanguage.SameAsWindows, windows));

        [Theory]
        [InlineData("en", "de-DE", "en")]
        [InlineData("de", "en-US", "de")]
        [InlineData("de", "fr-FR", "de")]
        public void AChoiceOfTheirOwnWinsOverWindows(string choice, string windows, string shown) =>
            Assert.Equal(shown, Shown(choice, windows));

        // Earlier versions offered Japanese and Russian, and saved culture names.
        [Theory]
        [InlineData("ja", "de-DE", "de")]
        [InlineData("ru", "fr-FR", "en")]
        [InlineData("zh-Hans", "de-AT", "de")]
        [InlineData("not a language", "en-US", "en")]
        [InlineData(null, "de-DE", "de")]
        public void AChoiceTheAppDoesNotOfferFollowsWindows(string? choice, string windows, string shown) =>
            Assert.Equal(shown, Shown(choice, windows));

        [Theory]
        [InlineData("", "")]
        [InlineData("de", "de")]
        [InlineData("en-US", "en")]
        [InlineData("DE", "de")]
        [InlineData("ja", "")]
        [InlineData(null, "")]
        public void TheSavedChoiceIsAlwaysOneTheSettingsOffer(string? saved, string offered) =>
            Assert.Equal(offered, AppLanguage.Normalize(saved));

        // F1 opens the guide, in the language the app shows.
        [Theory]
        [InlineData("de", AppLanguage.GermanGuide)]
        [InlineData("de-AT", AppLanguage.GermanGuide)]
        [InlineData("en", AppLanguage.EnglishGuide)]
        [InlineData("fr-FR", AppLanguage.EnglishGuide)]
        public void F1OpensTheGuideInTheLanguageShown(string language, string guide) =>
            Assert.Equal(guide, AppLanguage.GuideFor(CultureInfo.GetCultureInfo(language)));

        // F1 used to open AutoMuteUs's command page.
        [Fact]
        public void TheGuideIsAuvcsOwn()
        {
            Assert.EndsWith("/docs/guide.md", AppLanguage.EnglishGuide);
            Assert.EndsWith("/docs/anleitung.md", AppLanguage.GermanGuide);
            foreach (var guide in new[] { AppLanguage.EnglishGuide, AppLanguage.GermanGuide })
            {
                Assert.StartsWith("https://github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/blob/main/", guide);
            }
        }
    }
}
