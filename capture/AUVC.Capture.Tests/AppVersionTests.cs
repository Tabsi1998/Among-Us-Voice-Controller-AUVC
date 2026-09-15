using System;
using System.Linq;
using AUVC.Transport;
using Xunit;

namespace AUVC.Capture.Tests
{
    /// <summary>
    /// The app compares its own version with the released ones. Getting the order
    /// wrong either hides a newer version or announces an older one as new.
    /// </summary>
    public class AppVersionTests
    {
        [Theory]
        [InlineData("v0.1.2-beta", 0, 1, 2, "beta")]
        [InlineData("0.1.2", 0, 1, 2, "")]
        [InlineData("1.2.3-rc.1+4f1c2a9", 1, 2, 3, "rc.1")]
        [InlineData(" V10.20.30 ", 10, 20, 30, "")]
        public void AVersionIsReadFromATagOrABuild(string text, int major, int minor, int patch, string preRelease)
        {
            var version = AppVersion.Parse(text);

            Assert.NotNull(version);
            Assert.Equal((major, minor, patch, preRelease), (version.Major, version.Minor, version.Patch, version.PreRelease));
        }

        [Theory]
        [InlineData(null)]
        [InlineData("")]
        [InlineData("1.2")]
        [InlineData("1.2.3.4")]
        [InlineData("v1.x.3")]
        [InlineData("-1.2.3")]
        [InlineData("1.2.3-")]
        [InlineData("1.2.3-be ta")]
        [InlineData("1.2.3-beta..1")]
        [InlineData("latest")]
        public void AnythingElseIsNoVersion(string? text)
        {
            Assert.Null(AppVersion.Parse(text));
        }

        /// <summary>The order Semantic Versioning defines, oldest first.</summary>
        [Fact]
        public void VersionsAreOrderedAsSemanticVersioningDefines()
        {
            string[] ordered =
            [
                "0.9.9", "1.0.0-alpha", "1.0.0-alpha.1", "1.0.0-alpha.beta", "1.0.0-beta", "1.0.0-beta.2",
                "1.0.0-beta.11", "1.0.0-rc.1", "1.0.0", "1.0.1", "1.2.9", "1.2.10", "2.0.0",
            ];
            var versions = ordered.Select(text => AppVersion.Parse(text)!).ToList();

            for (var i = 0; i < versions.Count; i++)
            {
                for (var j = 0; j < versions.Count; j++)
                {
                    Assert.True(Math.Sign(versions[i].CompareTo(versions[j])) == Math.Sign(i.CompareTo(j)),
                        $"{ordered[i]} compared with {ordered[j]}");
                }
            }
        }

        [Theory]
        [InlineData("0.0.0-dev", true)]
        [InlineData("0.0.0-local", true)]
        [InlineData("0.0.0", true)]
        [InlineData("0.0.1", false)]
        [InlineData("0.1.0-beta", false)]
        public void OnlyZeroIsADevelopmentBuild(string text, bool development)
        {
            Assert.Equal(development, AppVersion.Parse(text)!.IsDevelopment);
        }

        [Theory]
        [InlineData("v0.1.2-beta", "0.1.2-beta")]
        [InlineData("1.2.3+4f1c2a9", "1.2.3")]
        public void AVersionIsShownWithoutPrefixOrBuild(string text, string shown)
        {
            Assert.Equal(shown, AppVersion.Parse(text)!.ToString());
        }
    }
}
