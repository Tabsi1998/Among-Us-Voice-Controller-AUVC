using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Text.Json.Nodes;
using AUVC.Protocol;
using Xunit;

namespace AUVC.Capture.Tests
{
    /// <summary>
    /// Checks the C# half of the capture-to-bot contract against the same
    /// fixtures the Go tests read. Keeping the examples in one place outside
    /// either language is what stops the two implementations from drifting: a
    /// change that breaks one breaks the other's tests too.
    /// </summary>
    public class ProtocolContractTests
    {
        private static string FixtureRoot =>
            Path.Combine(AppContext.BaseDirectory, "Fixtures", "protocol");

        private static IEnumerable<string> FixtureFiles(string directory) =>
            Directory.EnumerateFiles(Path.Combine(FixtureRoot, directory), "*.json").Order();

        public static TheoryData<string> ValidFixtures()
        {
            var data = new TheoryData<string>();
            foreach (var path in FixtureFiles("messages"))
            {
                data.Add(Path.GetFileName(path));
            }
            return data;
        }

        public static TheoryData<string> InvalidFixtures()
        {
            var data = new TheoryData<string>();
            foreach (var path in FixtureFiles("invalid"))
            {
                data.Add(Path.GetFileName(path));
            }
            return data;
        }

        private static string ReadFixture(string directory, string name) =>
            File.ReadAllText(Path.Combine(FixtureRoot, directory, name));

        /// <summary>
        /// Every message type in the contract needs an example, or one side can
        /// gain a message the other has never seen.
        /// </summary>
        [Fact]
        public void EveryMessageTypeHasAFixture()
        {
            var present = FixtureFiles("messages").Select(Path.GetFileName).ToHashSet();

            foreach (var type in ProtocolContract.Types)
            {
                Assert.True(present.Contains($"{type}.json"),
                    $"no fixture for {type}; add protocol/fixtures/messages/{type}.json");
            }
            Assert.Equal(ProtocolContract.Types.Count, present.Count);
        }

        [Theory]
        [MemberData(nameof(ValidFixtures))]
        public void EveryFixtureDecodesToItsType(string name)
        {
            var message = ProtocolCodec.Decode(ReadFixture("messages", name));

            Assert.Equal(Path.GetFileNameWithoutExtension(name), message.Type);
            Assert.Equal(ProtocolContract.Version, message.Protocol);
        }

        /// <summary>
        /// Re-encoding a fixture has to produce the same document. If it does
        /// not, this build either drops a field the bot sent or invents one it
        /// did not, and the Go side would be reading something else.
        /// </summary>
        [Theory]
        [MemberData(nameof(ValidFixtures))]
        public void FixturesSurviveARoundTrip(string name)
        {
            var original = ReadFixture("messages", name);

            var encoded = ProtocolCodec.Encode(ProtocolCodec.Decode(original));

            Assert.True(JsonNode.DeepEquals(JsonNode.Parse(original), JsonNode.Parse(encoded)),
                $"round trip changed the message:\nfixture: {original}\nencoded: {encoded}");
        }

        /// <summary>
        /// The invalid fixtures are the other half of the contract: both
        /// implementations have to refuse the same documents, rather than one
        /// of them quietly coping.
        /// </summary>
        [Theory]
        [MemberData(nameof(InvalidFixtures))]
        public void EveryInvalidFixtureIsRefused(string name)
        {
            var json = ReadFixture("invalid", name);

            Message message;
            try
            {
                message = ProtocolCodec.Decode(json);
            }
            catch (MalformedMessageException)
            {
                return; // refused at the door, which is a valid way to refuse it
            }

            Assert.NotNull(MessageValidator.Refusal(message));
        }

        /// <summary>
        /// A valid message must not be reported as a refusal, or capture would
        /// refuse to send the very messages the contract asks for.
        /// </summary>
        [Theory]
        [MemberData(nameof(ValidFixtures))]
        public void ValidFixturesAreNotRefused(string name)
        {
            var message = ProtocolCodec.Decode(ReadFixture("messages", name));

            Assert.Null(MessageValidator.Refusal(message));
        }

        /// <summary>
        /// The two implementations agree on the version number itself. Without
        /// this, a mismatch would only surface as a refused handshake at a
        /// player's machine.
        /// </summary>
        [Fact]
        public void TheVersionMatchesTheFixtures()
        {
            var hello = ProtocolCodec.Decode(ReadFixture("messages", "hello.json"));

            Assert.Equal(ProtocolContract.Version, hello.Protocol);
        }

        /// <summary>
        /// The lobby code is posted in a Discord channel, so capture refuses to send
        /// anything that is not a code the game uses, as the bot would.
        /// </summary>
        [Theory]
        [InlineData("ABCD", true)]
        [InlineData("ABCDEF", true)]
        [InlineData("******", true)]
        [InlineData("", true)]
        [InlineData("abcd", false)]
        [InlineData("ABCDE", false)]
        [InlineData("<b>hi</b>", false)]
        public void OnlyALobbyCodeTheGameUsesIsSent(string code, bool valid)
        {
            var lobby = new Lobby { Code = code, Map = ProtocolContract.MapPolus };

            Assert.Equal(valid, MessageValidator.Refusal(new GameStateChanged
            {
                Session = "s",
                Seq = 1,
                Phase = ProtocolContract.PhaseLobby,
                Lobby = lobby,
            }) is null);
            Assert.Equal(valid, MessageValidator.Refusal(new Snapshot
            {
                Session = "s",
                Seq = 1,
                Phase = ProtocolContract.PhaseLobby,
                Lobby = lobby,
            }) is null);
        }

        [Fact]
        public void AnUnknownMessageTypeIsRefusedRatherThanIgnored()
        {
            var json = """{"protocol":1,"type":"player_promoted","session":"s","seq":1}""";

            Assert.Throws<MalformedMessageException>(() => ProtocolCodec.Decode(json));
        }

        /// <summary>
        /// Capture builds messages itself, so the validator has to catch what
        /// capture can get wrong locally rather than let the bot refuse it
        /// mid-round.
        /// </summary>
        [Fact]
        public void CaptureFindsItsOwnMistakesBeforeSending()
        {
            Assert.NotNull(MessageValidator.Refusal(
                new Hello { Session = "s", Seq = 1, Capture = "" }));
            Assert.NotNull(MessageValidator.Refusal(
                new Heartbeat { Session = "", Seq = 1 }));
            Assert.NotNull(MessageValidator.Refusal(
                new Heartbeat { Session = "s", Seq = 0 }));
            Assert.NotNull(MessageValidator.Refusal(
                new GameStateChanged { Session = "s", Seq = 1, Phase = "voting" }));
            Assert.NotNull(MessageValidator.Refusal(
                new PlayerDied { Session = "s", Seq = 1, Player = new Player { Color = 2 } }));

            Assert.Null(MessageValidator.Refusal(
                new Heartbeat { Session = "s", Seq = 1 }));
        }

        /// <summary>
        /// Every phase the contract defines has to be accepted on both sides,
        /// or capture cannot report a state the bot already handles.
        /// </summary>
        [Fact]
        public void EveryDefinedPhaseIsAccepted()
        {
            foreach (var phase in ProtocolContract.Phases)
            {
                var snapshot = new Snapshot
                {
                    Session = "s",
                    Seq = 1,
                    Phase = phase,
                    Players = [new Player { Name = "Red" }],
                };

                Assert.Null(MessageValidator.Refusal(snapshot));
            }
        }
    }
}
