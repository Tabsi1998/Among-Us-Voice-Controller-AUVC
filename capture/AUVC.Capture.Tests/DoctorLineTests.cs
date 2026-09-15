using System;
using AUVC.Transport;
using Xunit;

namespace AUVC.Capture.Tests
{
    /// <summary>
    /// The bot writes its checks for Discord. The app shows them in the status line
    /// and in Bot → Status, where Discord markdown would appear as typed.
    /// </summary>
    public class DoctorLineTests
    {
        [Theory]
        [InlineData("Set it with `/au setup channels`.", "Set it with /au setup channels.")]
        [InlineData("**Mute Members** on the main voice channel", "Mute Members on the main voice channel")]
        [InlineData("<#1234567890> is configured but AUVC cannot see it", "#1234567890 is configured but AUVC cannot see it")]
        [InlineData("AUVC is missing Discord permissions:\n- **Move Members**\n", "AUVC is missing Discord permissions:\n- Move Members")]
        [InlineData("plain", "plain")]
        public void DiscordMarkdownIsLeftOut(string markdown, string plain)
        {
            Assert.Equal(plain, DoctorLine.Plain(markdown));
        }

        [Fact]
        public void TheFixFollowsWhatIsWrong()
        {
            var check = new LocalCheck
            {
                Name = "Main voice channel",
                Level = LocalCheck.Fail,
                Detail = "<#42> is configured but AUVC cannot see it",
                Fix = "Set it again with `/au setup channels`.",
            };

            Assert.Equal("#42 is configured but AUVC cannot see it" + Environment.NewLine + "Set it again with /au setup channels.",
                DoctorLine.Explain(check));
        }

        [Fact]
        public void WithoutAFixOnlyWhatIsWrongIsSaid()
        {
            var check = new LocalCheck
            {
                Name = "Configuration",
                Level = LocalCheck.Fail,
                Detail = "the configuration cannot be read",
            };

            Assert.Equal("the configuration cannot be read", DoctorLine.Explain(check));
        }
    }
}
