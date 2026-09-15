using System;
using System.IO;
using System.IO.Compression;
using System.Linq;
using System.Text;
using AUVC.Transport;
using Xunit;

namespace AUVC.Capture.Tests
{
    /// <summary>
    /// A diagnostics export is shared with whoever helps, so nothing in it may hand
    /// over the bot, the app's access to it, or a pairing code.
    /// </summary>
    public class DiagnosticsTests
    {
        // Shaped like a Discord bot token, and built here rather than written out:
        // GitHub's push protection refuses token-shaped text in a commit, made up or
        // not, and it should keep doing so.
        private static readonly string DiscordToken = string.Join(".", new string('M', 26), "GhIjKl", new string('a', 38));

        // 32 random bytes in base32 without padding are 52 characters.
        private const string Credential = "0123456789abcdef.ABCDEFGHIJKLMNOPQRSTUVWXYZ234567ABCDEFGHIJKLMNOPQRST";

        [Theory]
        [InlineData("Logged in with {token} as AUVC")]
        [InlineData("credential={credential}")]
        [InlineData("Authorization: Bot {token}")]
        public void SecretsAreBlackedOut(string template)
        {
            var line = template.Replace("{token}", DiscordToken).Replace("{credential}", Credential);

            var redacted = DiagnosticsRedactor.Redact(line);

            Assert.DoesNotContain(DiscordToken, redacted);
            Assert.DoesNotContain(Credential, redacted);
            Assert.Contains(DiagnosticsRedactor.Redacted, redacted);
        }

        [Theory]
        [InlineData("Pairing with AUVC-7K3M-Q9ZX")]
        [InlineData("code auvc7k3mq9zx typed")]
        [InlineData("\"code\":\"AUVC-0000-0000\"")]
        public void PairingCodesAreBlackedOut(string line)
        {
            var redacted = DiagnosticsRedactor.Redact(line);

            Assert.Contains(DiagnosticsRedactor.Redacted, redacted);
            Assert.DoesNotMatch("(?i)AUVC-?[0-9A-Z]{4}-?[0-9A-Z]{4}", redacted);
        }

        [Theory]
        [InlineData("Authorization: Bearer abc123", "Authorization: Bearer [redacted]")]
        [InlineData("\"authorization\": \"xyz\"", "\"authorization\": \"[redacted]\"")]
        public void AnAuthorizationValueIsBlackedOutButItsNameStays(string line, string expected)
        {
            Assert.Equal(expected, DiagnosticsRedactor.Redact(line));
        }

        /// <summary>The export is only useful when the ordinary text survives.</summary>
        [Theory]
        [InlineData("The bot is running and AUVC connected to the bot")]
        [InlineData("Loaded AUCapture-WPF.deps.json from C:\\Program Files")]
        [InlineData("Among Us 2025.9.9, guild 123456789012345678, player Red")]
        [InlineData("AUVC bot: Connected ws://127.0.0.1:52000")]
        public void OrdinaryTextStaysAsItIs(string line)
        {
            Assert.Equal(line, DiagnosticsRedactor.Redact(line));
        }

        [Fact]
        public void TheWindowsUserNameInPathsIsReplaced()
        {
            var redacted = DiagnosticsRedactor.Redact(
                "C:\\Users\\Alice\\AppData\\Roaming\\AmongUsCapture\nC:/Users/alice/x\nAlice plays Red\nlog from Alice\nAlice/notes", "Alice");

            // Only a path names the Windows account; the same word elsewhere, such as
            // an in-game name, stays.
            Assert.Equal(
                "C:\\Users\\[user]\\AppData\\Roaming\\AmongUsCapture\nC:/Users/[user]/x\nAlice plays Red\nlog from Alice\nAlice/notes",
                redacted);
        }

        [Theory]
        [InlineData(@"C:\Users\me\AppData\Local\AUVC\bot-token.bin", false)]
        [InlineData(@"C:\Users\me\AppData\Local\AUVC\CREDENTIAL.BIN", false)]
        [InlineData(@"C:\Users\me\AppData\Local\AUVC\amongus.db", false)]
        [InlineData("amongus.db-wal", false)]
        [InlineData(@"C:\Users\me\AppData\Local\AUVC\logs\logs.txt", true)]
        [InlineData("settings/app-settings.json", true)]
        public void TheTokenTheCredentialAndTheDatabaseNeverGoIn(string path, bool included)
        {
            Assert.Equal(included, DiagnosticsBundle.MayInclude(path));
        }

        [Fact]
        public void TheZipHoldsEveryFileRedactedAndNothingForbidden()
        {
            using var output = new MemoryStream();

            DiagnosticsBundle.Write(output,
            [
                new DiagnosticsFile("summary.txt", "App: 0.1.3-beta"),
                new DiagnosticsFile("app-logs/latest.log", "token " + DiscordToken + " in C:\\Users\\Alice\\AppData"),
                new DiagnosticsFile("bot-token.bin", DiscordToken),
            ], "Alice");

            output.Position = 0;
            using var zip = new ZipArchive(output, ZipArchiveMode.Read);
            Assert.Equal(["summary.txt", "app-logs/latest.log"], zip.Entries.Select(entry => entry.FullName).ToArray());
            using var reader = new StreamReader(zip.GetEntry("app-logs/latest.log")!.Open());
            Assert.Equal("token [redacted] in C:\\Users\\[user]\\AppData", reader.ReadToEnd());
        }

        [Fact]
        public void OnlyTheEndOfALongLogGoesInFromAWholeLine()
        {
            var log = string.Join("\n", Enumerable.Range(1, 1000).Select(i => $"line {i}"));
            using var stream = new MemoryStream(Encoding.UTF8.GetBytes(log));

            var tail = DiagnosticsBundle.ReadTail(stream, 100);

            Assert.EndsWith("line 1000", tail);
            Assert.StartsWith("line ", tail);
            Assert.True(Encoding.UTF8.GetByteCount(tail) <= 100);
            Assert.DoesNotContain("line 1\n", tail);
        }

        [Fact]
        public void AShortLogGoesInWhole()
        {
            using var stream = new MemoryStream(Encoding.UTF8.GetBytes("first\nsecond"));

            Assert.Equal("first\nsecond", DiagnosticsBundle.ReadTail(stream, 100));
        }

        [Fact]
        public void TheSummarySaysWhatRunsAndWhatTheBotChecked()
        {
            var discord = new LocalCheck
            {
                Name = "Discord",
                Level = LocalCheck.Ok,
                Detail = "connected to **Crew**",
            };
            var permissions = new LocalCheck
            {
                Name = "Permissions",
                Level = LocalCheck.Fail,
                Detail = "missing Move Members",
                Fix = "Grant `Move Members`.",
            };

            var summary = DiagnosticsBundle.Summary("0.1.3-beta", "v0.1.3-beta", "Windows 11", "10.0.1",
                [discord, permissions], new DateTimeOffset(2026, 9, 15, 13, 0, 0, TimeSpan.FromHours(2)));

            Assert.Contains("App: 0.1.3-beta", summary);
            Assert.Contains("Bot: v0.1.3-beta", summary);
            Assert.Contains("Created: 2026-09-15 13:00:00 +02:00", summary);
            Assert.Contains("[ok] Discord: connected to Crew", summary);
            Assert.Contains("[fail] Permissions: missing Move Members -> Grant Move Members.", summary);
        }

        [Fact]
        public void WithoutTheBotTheSummarySaysSo()
        {
            var summary = DiagnosticsBundle.Summary("0.1.3-beta", "not running on this PC", "Windows 11", "10.0.1", [], DateTimeOffset.UnixEpoch);

            Assert.Contains("could not be asked", summary);
        }
    }
}
