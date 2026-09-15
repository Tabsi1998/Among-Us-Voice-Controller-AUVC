using System;
using System.Globalization;
using System.Linq;
using System.Net;
using System.Net.Http;
using System.Threading;
using System.Threading.Tasks;
using AUVC.Transport;
using Xunit;

namespace AUVC.Capture.Tests
{
    /// <summary>
    /// The client side of the bot's local control interface. The JSON shapes are
    /// the Go server's; a field spelled differently here would silently read as
    /// empty.
    /// </summary>
    public class LocalControlClientTests
    {
        private sealed class StubHandler(Func<HttpRequestMessage, string?, HttpResponseMessage> respond) : HttpMessageHandler
        {
            public HttpRequestMessage? Last { get; private set; }
            public string? LastBody { get; private set; }

            protected override async Task<HttpResponseMessage> SendAsync(HttpRequestMessage request, CancellationToken cancellationToken)
            {
                Last = request;
                LastBody = request.Content is null ? null : await request.Content.ReadAsStringAsync(cancellationToken);
                return respond(request, LastBody);
            }
        }

        private static readonly string Secret = new('s', 64);
        private static readonly Uri Bot = new("http://127.0.0.1:50123/");

        private static HttpResponseMessage Json(HttpStatusCode status, string body) =>
            new(status) { Content = new StringContent(body, System.Text.Encoding.UTF8, "application/json") };

        private static LocalControlClient Client(StubHandler handler) => new(new HttpClient(handler), Bot, Secret);

        [Fact]
        public async Task EveryRequestCarriesTheSecret()
        {
            var handler = new StubHandler((_, _) => Json(HttpStatusCode.OK, """{"connected":true,"guilds":[]}"""));

            await Client(handler).GetStatusAsync();

            Assert.Equal("Bearer", handler.Last!.Headers.Authorization!.Scheme);
            Assert.Equal(Secret, handler.Last.Headers.Authorization.Parameter);
            Assert.Equal("http://127.0.0.1:50123/local/status", handler.Last.RequestUri!.ToString());
        }

        // The bot writes the checks the app shows in this language, so they match
        // the rest of the window.
        [Theory]
        [InlineData("de-DE", "de")]
        [InlineData("en-GB", "en")]
        public async Task EveryRequestAsksForTheLanguageOfTheApp(string culture, string language)
        {
            var handler = new StubHandler((_, _) => Json(HttpStatusCode.OK, """{"connected":true,"guilds":[]}"""));
            var before = CultureInfo.CurrentUICulture;
            try
            {
                CultureInfo.CurrentUICulture = CultureInfo.GetCultureInfo(culture);
                await Client(handler).GetStatusAsync();
            }
            finally
            {
                CultureInfo.CurrentUICulture = before;
            }

            Assert.Equal(language, handler.Last!.Headers.AcceptLanguage.Single().Value);
        }

        [Fact]
        public async Task TheLobbyIsRead()
        {
            var handler = new StubHandler((_, _) => Json(HttpStatusCode.OK,
                """{"players":[{"name":"Alice","color":"red","user_id":""}],"members":[{"id":"u1","name":"Red Leader"}]}"""));

            var crewmates = await Client(handler).GetCrewmatesAsync("g1");

            Assert.Equal(HttpMethod.Get, handler.Last!.Method);
            Assert.Equal("/local/guilds/g1/crewmates", handler.Last.RequestUri!.AbsolutePath);
            var player = Assert.Single(crewmates.Players);
            Assert.Equal(("Alice", "red", ""), (player.Name, player.Color, player.UserId));
            var member = Assert.Single(crewmates.Members);
            Assert.Equal(("u1", "Red Leader"), (member.Id, member.Name));
        }

        [Fact]
        public async Task ALinkIsSentAndTheLobbyReadBack()
        {
            var handler = new StubHandler((_, _) => Json(HttpStatusCode.OK,
                """{"players":[{"name":"Alice","color":"red","user_id":"u1"}],"members":[{"id":"u1","name":"Red Leader"}]}"""));

            var crewmates = await Client(handler).LinkAsync("g1", "Alice", "u1");

            Assert.Equal(HttpMethod.Put, handler.Last!.Method);
            Assert.Equal("/local/guilds/g1/links", handler.Last.RequestUri!.AbsolutePath);
            Assert.Equal("""{"player":"Alice","user_id":"u1"}""", handler.LastBody);
            Assert.Equal("u1", Assert.Single(crewmates.Players).UserId);
        }

        [Fact]
        public async Task TheStatusIsRead()
        {
            var handler = new StubHandler((_, _) => Json(HttpStatusCode.OK,
                """{"connected":true,"bot_id":"42","bot_name":"AUVC","version":"v1","guilds":[{"id":"g1","name":"The Crew"}]}"""));

            var status = await Client(handler).GetStatusAsync();

            Assert.True(status.Connected);
            Assert.Equal("AUVC", status.BotName);
            var server = Assert.Single(status.Servers);
            Assert.Equal(("g1", "The Crew"), (server.Id, server.Name));
        }

        [Fact]
        public async Task ChannelsAndAGuildAreRead()
        {
            var handler = new StubHandler((request, _) => request.RequestUri!.AbsolutePath.EndsWith("/channels")
                ? Json(HttpStatusCode.OK, """[{"id":"v1","name":"Among Us","kind":"voice","position":1}]""")
                : Json(HttpStatusCode.OK,
                    """{"id":"g1","name":"The Crew","main_voice_channel_id":"v1","ghost_voice_channel_id":"v2","control_text_channel_id":"t1","auto_start":true,"capture_connections":1,"session":"paused","checks":[{"name":"Discord","level":"ok","detail":"connected"}]}"""));

            var channels = await Client(handler).GetChannelsAsync("g1");
            var guild = await Client(handler).GetGuildAsync("g1");

            Assert.Equal(LocalChannel.Voice, Assert.Single(channels).Kind);
            Assert.Equal("v2", guild.GhostVoiceChannelId);
            Assert.True(guild.AutoStart);
            Assert.Equal(1, guild.CaptureConnections);
            Assert.Equal(LocalGuild.SessionPaused, guild.Session);
            Assert.Equal(LocalCheck.Ok, Assert.Single(guild.Checks).Level);
        }

        [Fact]
        public async Task ASetupIsSentInTheBotsFieldNames()
        {
            var handler = new StubHandler((_, _) => Json(HttpStatusCode.OK, """{"id":"g1"}"""));

            await Client(handler).SetupAsync("g1", new LocalSetup
            {
                MainVoiceChannelId = "v1",
                GhostVoiceChannelId = "v2",
                ControlTextChannelId = "t1",
                AutoStart = true,
            });

            Assert.Equal(HttpMethod.Put, handler.Last!.Method);
            Assert.Equal("/local/guilds/g1/setup", handler.Last.RequestUri!.AbsolutePath);
            Assert.Equal(
                """{"main_voice_channel_id":"v1","ghost_voice_channel_id":"v2","control_text_channel_id":"t1","auto_start":true}""",
                handler.LastBody);
        }

        [Fact]
        public async Task AGuildIdCannotReachAnotherRoute()
        {
            var handler = new StubHandler((_, _) => Json(HttpStatusCode.OK, """{"credential":"0011.SECRET"}"""));

            await Client(handler).IssueCredentialAsync("../shutdown");

            Assert.Equal("/local/guilds/..%2Fshutdown/credential", handler.Last!.RequestUri!.AbsolutePath);
        }

        [Fact]
        public async Task ACredentialIsReturned()
        {
            var handler = new StubHandler((_, _) => Json(HttpStatusCode.OK, """{"credential":"0011.SECRET"}"""));

            Assert.Equal("0011.SECRET", await Client(handler).IssueCredentialAsync("g1"));
            Assert.Equal(HttpMethod.Post, handler.Last!.Method);
        }

        /// <summary>
        /// The bot's refusal is written for a person, so it has to reach the
        /// setup window as it was written.
        /// </summary>
        [Fact]
        public async Task ARefusalCarriesTheBotsReason()
        {
            var handler = new StubHandler((_, _) => Json(HttpStatusCode.BadRequest,
                """{"error":"invalid_setup","message":"invalid setup: the ghost channel must be a voice channel in this server"}"""));

            var error = await Assert.ThrowsAsync<LocalControlException>(
                () => Client(handler).SetupAsync("g1", new LocalSetup()));

            Assert.Equal("invalid_setup", error.Code);
            Assert.Contains("ghost channel", error.Message);
        }

        [Fact]
        public async Task AnAnswerThatIsNotTheBotsStillFailsClearly()
        {
            var handler = new StubHandler((_, _) => new HttpResponseMessage(HttpStatusCode.BadGateway));

            var error = await Assert.ThrowsAsync<LocalControlException>(() => Client(handler).GetStatusAsync());

            Assert.Equal("http_502", error.Code);
        }

        [Fact]
        public void AClientWithoutASecretCannotBeBuilt()
        {
            Assert.Throws<ArgumentException>(() => new LocalControlClient(new HttpClient(), Bot, ""));
        }
    }
}
