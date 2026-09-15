using System;
using AUVC.Transport;
using Xunit;

namespace AUVC.Capture.Tests
{
    /// <summary>
    /// Which newer AUVC release the app tells about, and where its link leads.
    /// </summary>
    public class ReleaseFeedTests
    {
        private const string TagPage = ReleaseFeed.ReleasesPage + "/tag/";

        private static GitHubRelease Release(string tag, bool preRelease = false, bool draft = false) =>
            new(tag, preRelease, draft, TagPage + tag);

        private static NewerRelease? Newer(string current, params GitHubRelease[] releases) =>
            ReleaseFeed.Newer(AppVersion.Parse(current)!, releases);

        /// <summary>
        /// GitHub lists releases by when they were made, so the newest version need not
        /// come first: a fix for an older line can be published after it.
        /// </summary>
        [Fact]
        public void APreReleaseHearsOfTheNewestPreReleaseOrRelease()
        {
            var newer = Newer("0.1.2-beta",
                Release("v0.1.1-beta", preRelease: true),
                Release("v0.1.3-beta", preRelease: true),
                Release("v0.1.4-beta", preRelease: true),
                Release("v0.1.2-beta", preRelease: true));

            Assert.Equal("0.1.4-beta", newer?.Version.ToString());
            Assert.Equal(TagPage + "v0.1.4-beta", newer?.Page);
            Assert.Equal("0.1.2", Newer("0.1.2-beta", Release("v0.1.2"))?.Version.ToString());
        }

        /// <summary>A release was chosen for being finished, so it hears only of releases.</summary>
        [Fact]
        public void AReleaseHearsOnlyOfReleases()
        {
            Assert.Equal("0.1.4", Newer("0.1.3", Release("v0.2.0-beta", preRelease: true), Release("v0.1.4"))?.Version.ToString());
            Assert.Null(Newer("0.1.3", Release("v0.2.0-beta", preRelease: true)));
            // Published as a pre-release, even without a suffix in its tag.
            Assert.Null(Newer("0.1.3", Release("v0.2.0", preRelease: true)));
            // A suffix in its tag, even when it was not published as a pre-release.
            Assert.Null(Newer("0.1.3", Release("v0.2.0-rc.1")));
        }

        [Fact]
        public void TheSameOrAnOlderVersionIsNoNews()
        {
            Assert.Null(Newer("0.1.2-beta",
                Release("v0.1.2-beta", preRelease: true),
                Release("v0.1.1"),
                Release("v0.1.2-alpha", preRelease: true)));
        }

        [Fact]
        public void DraftsAndTagsThatAreNoVersionAreIgnored()
        {
            Assert.Null(Newer("0.1.2", Release("v9.9.9", draft: true), Release("nightly"), Release("v1.x")));
        }

        [Fact]
        public void ADevelopmentBuildHearsOfNothing()
        {
            Assert.Null(Newer("0.0.0-dev", Release("v0.1.2"), Release("v0.1.3-beta", preRelease: true)));
        }

        /// <summary>The link is opened in the browser, so it leads to AUVC's releases whatever the list says.</summary>
        [Theory]
        [InlineData(ReleaseFeed.ReleasesPage + "/tag/v0.2.0", ReleaseFeed.ReleasesPage + "/tag/v0.2.0")]
        [InlineData("https://evil.example/Tabsi1998/Among-Us-Voice-Controller-AUVC/releases/tag/v0.2.0", ReleaseFeed.ReleasesPage)]
        [InlineData(ReleaseFeed.ReleasesPage + "/../../../somebody/else", ReleaseFeed.ReleasesPage)]
        [InlineData("http://github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/releases/tag/v0.2.0", ReleaseFeed.ReleasesPage)]
        [InlineData(ReleaseFeed.ReleasesPage + "-elsewhere/tag/v0.2.0", ReleaseFeed.ReleasesPage)]
        [InlineData("", ReleaseFeed.ReleasesPage)]
        public void TheLinkLeadsToAuvcReleases(string page, string opened)
        {
            var newer = ReleaseFeed.Newer(AppVersion.Parse("0.1.0")!, [new GitHubRelease("v0.2.0", false, false, page)]);

            Assert.Equal(opened, newer?.Page);
        }

        [Fact]
        public void TheReleaseListIsReadAsGitHubSendsIt()
        {
            const string json = """
                [
                  {
                    "url": "https://api.github.com/repos/Tabsi1998/Among-Us-Voice-Controller-AUVC/releases/1",
                    "html_url": "https://github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/releases/tag/v0.1.2-beta",
                    "tag_name": "v0.1.2-beta",
                    "draft": false,
                    "prerelease": true,
                    "assets": []
                  },
                  { "tag_name": "v0.1.1-beta", "prerelease": true, "draft": true },
                  { "name": "no tag" },
                  "not a release"
                ]
                """;

            var releases = ReleaseFeed.Parse(json);

            Assert.Equal(
                [
                    new GitHubRelease("v0.1.2-beta", true, false, ReleaseFeed.ReleasesPage + "/tag/v0.1.2-beta"),
                    new GitHubRelease("v0.1.1-beta", true, true, ""),
                ],
                releases);
        }

        /// <summary>A rate-limit message or a broken answer fails the check rather than meaning "up to date".</summary>
        [Theory]
        [InlineData("""{ "message": "API rate limit exceeded" }""")]
        [InlineData("not json")]
        public void AnAnswerThatIsNoReleaseListFailsTheCheck(string json)
        {
            Assert.Throws<FormatException>(() => ReleaseFeed.Parse(json));
        }
    }
}
