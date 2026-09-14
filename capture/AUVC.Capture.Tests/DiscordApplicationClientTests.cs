using System;
using System.Net;
using System.Net.Http;
using System.Threading;
using System.Threading.Tasks;
using AUVC.Transport;
using Xunit;

namespace AUVC.Capture.Tests
{
    public class DiscordApplicationClientTests
    {
        private sealed class StubHandler(Func<HttpRequestMessage, HttpResponseMessage> respond) : HttpMessageHandler
        {
            public HttpRequestMessage? Last { get; private set; }

            protected override Task<HttpResponseMessage> SendAsync(HttpRequestMessage request, CancellationToken cancellationToken)
            {
                Last = request;
                return Task.FromResult(respond(request));
            }
        }

        // Assembled rather than written out, so a secret scanner sees a
        // construction instead of something shaped like a Discord token.
        private static readonly string Token = string.Join(".", "pretend", "bot", "token");

        private static HttpResponseMessage Json(HttpStatusCode status, string body) =>
            new(status) { Content = new StringContent(body, System.Text.Encoding.UTF8, "application/json") };

        [Fact]
        public async Task AWorkingTokenNamesTheBotAndItsApplication()
        {
            var handler = new StubHandler(_ => Json(HttpStatusCode.OK,
                """{"id":"1234","name":"AUVC app","bot_require_code_grant":false,"bot":{"username":"AUVC"}}"""));

            var application = await new DiscordApplicationClient(new HttpClient(handler)).CheckTokenAsync(Token);

            Assert.Equal("1234", application.ApplicationId);
            Assert.Equal("AUVC", application.BotName);
            Assert.False(application.RequiresCodeGrant);
            Assert.Equal("Bot", handler.Last!.Headers.Authorization!.Scheme);
            Assert.Equal(Token, handler.Last.Headers.Authorization.Parameter);
            Assert.Equal("/api/v10/oauth2/applications/@me", handler.Last.RequestUri!.AbsolutePath);
        }

        /// <summary>
        /// People paste the token with surrounding spaces, or with the "Bot "
        /// prefix copied from an example header.
        /// </summary>
        [Theory]
        [InlineData("  {0}  ")]
        [InlineData("Bot {0}")]
        [InlineData("bot  {0}\n")]
        public async Task WhatWasPastedAroundTheTokenIsIgnored(string pasted)
        {
            var handler = new StubHandler(_ => Json(HttpStatusCode.OK, """{"id":"1234","name":"AUVC"}"""));

            await new DiscordApplicationClient(new HttpClient(handler)).CheckTokenAsync(string.Format(pasted, Token));

            Assert.Equal(Token, handler.Last!.Headers.Authorization!.Parameter);
        }

        [Fact]
        public async Task WithoutABotUserTheApplicationNameIsUsed()
        {
            var handler = new StubHandler(_ => Json(HttpStatusCode.OK, """{"id":"1234","name":"AUVC app"}"""));

            var application = await new DiscordApplicationClient(new HttpClient(handler)).CheckTokenAsync(Token);

            Assert.Equal("AUVC app", application.BotName);
        }

        /// <summary>
        /// With "Requires OAuth2 Code Grant" on, the invite link fails. The setup
        /// has to know so it can say which switch to turn off.
        /// </summary>
        [Fact]
        public async Task ACodeGrantRequirementIsReported()
        {
            var handler = new StubHandler(_ => Json(HttpStatusCode.OK,
                """{"id":"1234","name":"AUVC","bot_require_code_grant":true}"""));

            var application = await new DiscordApplicationClient(new HttpClient(handler)).CheckTokenAsync(Token);

            Assert.True(application.RequiresCodeGrant);
        }

        [Fact]
        public async Task ARejectedTokenSaysSoWithoutRepeatingIt()
        {
            var handler = new StubHandler(_ => Json(HttpStatusCode.Unauthorized, """{"message":"401: Unauthorized"}"""));

            var error = await Assert.ThrowsAsync<BotTokenException>(
                () => new DiscordApplicationClient(new HttpClient(handler)).CheckTokenAsync(Token));

            Assert.Equal(TokenProblem.Rejected, error.Problem);
            Assert.DoesNotContain(Token, error.Message, StringComparison.Ordinal);
        }

        [Fact]
        public async Task NoConnectionIsNotABadToken()
        {
            var handler = new StubHandler(_ => throw new HttpRequestException("No such host is known."));

            var error = await Assert.ThrowsAsync<BotTokenException>(
                () => new DiscordApplicationClient(new HttpClient(handler)).CheckTokenAsync(Token));

            Assert.Equal(TokenProblem.Unreachable, error.Problem);
        }

        [Theory]
        [InlineData("")]
        [InlineData("   ")]
        [InlineData(null)]
        public async Task NothingPastedIsNotSentToDiscord(string? pasted)
        {
            var handler = new StubHandler(_ => throw new InvalidOperationException("nothing should be sent"));

            var error = await Assert.ThrowsAsync<BotTokenException>(
                () => new DiscordApplicationClient(new HttpClient(handler)).CheckTokenAsync(pasted));

            Assert.Equal(TokenProblem.Empty, error.Problem);
            Assert.Null(handler.Last);
        }

        [Fact]
        public async Task AnAnswerThatIsNotAnApplicationIsReported()
        {
            var handler = new StubHandler(_ => Json(HttpStatusCode.OK, "<html>captive portal</html>"));

            var error = await Assert.ThrowsAsync<BotTokenException>(
                () => new DiscordApplicationClient(new HttpClient(handler)).CheckTokenAsync(Token));

            Assert.Equal(TokenProblem.Unexpected, error.Problem);
        }

        /// <summary>
        /// The invite asks for exactly what AUVC uses. Administrator, or anything
        /// else broad, would be a permission nobody needs to hand out.
        /// </summary>
        [Fact]
        public void TheInviteAsksForTheBotTheCommandsAndTheNeededPermissions()
        {
            var invite = DiscordApplicationClient.InviteUrl("1234");
            var query = System.Web.HttpUtility.ParseQueryString(invite.Query);

            Assert.Equal("discord.com", invite.Host);
            Assert.Equal("1234", query["client_id"]);
            Assert.Equal("bot applications.commands", query["scope"]);

            var permissions = long.Parse(query["permissions"]!);
            Assert.Equal(DiscordApplicationClient.RequiredPermissions, permissions);
            foreach (var needed in new[]
                     {
                         DiscordApplicationClient.ViewChannel, DiscordApplicationClient.Connect,
                         DiscordApplicationClient.MuteMembers, DiscordApplicationClient.DeafenMembers,
                         DiscordApplicationClient.MoveMembers, DiscordApplicationClient.SendMessages,
                     })
            {
                Assert.Equal(needed, permissions & needed);
            }

            const long administrator = 1L << 3;
            Assert.Equal(0, permissions & administrator);
        }
    }
}
