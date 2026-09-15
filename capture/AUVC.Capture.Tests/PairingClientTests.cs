using System;
using System.Net;
using System.Net.Http;
using System.Threading;
using System.Threading.Tasks;
using AUVC.Transport;
using Xunit;

namespace AUVC.Capture.Tests
{
    /// <summary>
    /// Checks what capture tells the person who typed a pairing code. Each
    /// failure leads to a different next step, so they must not all read the
    /// same way.
    /// </summary>
    public class PairingClientTests
    {
        private sealed class StubHandler(Func<HttpRequestMessage, HttpResponseMessage> respond)
            : HttpMessageHandler
        {
            public string? LastBody { get; private set; }
            public Uri? LastUri { get; private set; }

            protected override async Task<HttpResponseMessage> SendAsync(
                HttpRequestMessage request, CancellationToken cancellationToken)
            {
                LastUri = request.RequestUri;
                LastBody = request.Content is null
                    ? null
                    : await request.Content.ReadAsStringAsync(cancellationToken);
                return respond(request);
            }
        }

        private static HttpResponseMessage Json(HttpStatusCode status, string body) =>
            new(status) { Content = new StringContent(body, System.Text.Encoding.UTF8, "application/json") };

        private static readonly Uri Bot = new("http://localhost:8123");

        [Fact]
        public async Task ASuccessfulPairingReturnsTheCredential()
        {
            var handler = new StubHandler(_ => Json(HttpStatusCode.OK,
                """{"guild":"guild-1","credential":"0011.SECRET"}"""));
            var client = new PairingClient(new HttpClient(handler));

            var result = await client.PairAsync(Bot, "AUVC-ABCD-1234");

            Assert.Equal("guild-1", result.Guild);
            Assert.Equal("0011.SECRET", result.Credential);
        }

        /// <summary>
        /// Capture sends the code and nothing else. It never learns a guild id,
        /// because nobody is going to copy a Discord snowflake out of a
        /// developer menu.
        /// </summary>
        [Fact]
        public async Task OnlyTheCodeIsSent()
        {
            var handler = new StubHandler(_ => Json(HttpStatusCode.OK,
                """{"guild":"guild-1","credential":"0011.SECRET"}"""));
            var client = new PairingClient(new HttpClient(handler));

            await client.PairAsync(Bot, "  AUVC-ABCD-1234  ");

            Assert.Equal("""{"code":"AUVC-ABCD-1234"}""", handler.LastBody);
            Assert.Equal("/capture/pair", handler.LastUri?.AbsolutePath);
        }

        /// <summary>
        /// Expired and wrong lead to different next steps: one means ask for a
        /// new code, the other means check what you typed.
        /// </summary>
        [Fact]
        public async Task AnExpiredCodeSaysItExpired()
        {
            var handler = new StubHandler(_ => Json(HttpStatusCode.Forbidden,
                """{"error":"expired","message":"that pairing code has expired"}"""));
            var client = new PairingClient(new HttpClient(handler));

            var error = await Assert.ThrowsAsync<PairingRefusedException>(
                () => client.PairAsync(Bot, "AUVC-ABCD-1234"));

            Assert.Equal(PairingProblem.Expired, error.Problem);
            Assert.Contains("expired", error.Message, StringComparison.OrdinalIgnoreCase);
            Assert.Contains("/au capture pair", error.Message, StringComparison.Ordinal);
        }

        [Fact]
        public async Task ARejectedCodeSaysToAskForANewOne()
        {
            var handler = new StubHandler(_ => Json(HttpStatusCode.Forbidden,
                """{"error":"rejected","message":"that pairing code is not valid"}"""));
            var client = new PairingClient(new HttpClient(handler));

            var error = await Assert.ThrowsAsync<PairingRefusedException>(
                () => client.PairAsync(Bot, "AUVC-0000-0000"));

            Assert.Equal(PairingProblem.Rejected, error.Problem);
            Assert.Contains("/au capture pair", error.Message, StringComparison.Ordinal);
        }

        /// <summary>
        /// A bot that cannot be reached is not a rejected code. Telling a user
        /// to ask for a new one when the address is simply wrong sends them
        /// down entirely the wrong path.
        /// </summary>
        [Fact]
        public async Task AnUnreachableBotIsNotReportedAsABadCode()
        {
            var handler = new StubHandler(_ => throw new HttpRequestException("connection refused"));
            var client = new PairingClient(new HttpClient(handler));

            var error = await Assert.ThrowsAsync<PairingRefusedException>(
                () => client.PairAsync(Bot, "AUVC-ABCD-1234"));

            Assert.Equal(PairingProblem.Unreachable, error.Problem);
            Assert.Contains("reach", error.Message, StringComparison.OrdinalIgnoreCase);
            Assert.DoesNotContain("not valid", error.Message, StringComparison.OrdinalIgnoreCase);
        }

        [Fact]
        public async Task AnEmptyCodeIsRefusedWithoutAskingTheBot()
        {
            var handler = new StubHandler(_ => throw new InvalidOperationException("must not be called"));
            var client = new PairingClient(new HttpClient(handler));

            var error = await Assert.ThrowsAsync<PairingRefusedException>(() => client.PairAsync(Bot, "   "));

            Assert.Equal(PairingProblem.EmptyCode, error.Problem);
            Assert.Null(handler.LastUri);
        }

        /// <summary>
        /// An answer that is not the expected JSON usually means a proxy or a
        /// wrong address rather than the bot, and the user still needs to be
        /// told something they can act on.
        /// </summary>
        [Fact]
        public async Task AnUnexpectedAnswerStillProducesAReadableMessage()
        {
            var handler = new StubHandler(_ => new HttpResponseMessage(HttpStatusCode.BadGateway)
            {
                Content = new StringContent("<html>nginx</html>"),
            });
            var client = new PairingClient(new HttpClient(handler));

            var error = await Assert.ThrowsAsync<PairingRefusedException>(
                () => client.PairAsync(Bot, "AUVC-ABCD-1234"));

            Assert.Equal(PairingProblem.Other, error.Problem);
            Assert.Contains("502", error.Message, StringComparison.Ordinal);
        }
    }
}
