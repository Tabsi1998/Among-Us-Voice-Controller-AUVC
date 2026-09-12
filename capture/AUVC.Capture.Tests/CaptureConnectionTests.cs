using System;
using System.Collections.Generic;
using System.Linq;
using System.Threading;
using System.Threading.Tasks;
using AUVC.Protocol;
using AUVC.Transport;
using Xunit;

namespace AUVC.Capture.Tests
{
    /// <summary>
    /// Checks the connection's behaviour without opening a socket, the same way
    /// the bot's protocol rules are checked without one.
    /// </summary>
    public class CaptureConnectionTests
    {
        /// <summary>
        /// A channel that records what capture sent and hands back whatever the
        /// test wants the bot to have said.
        /// </summary>
        private sealed class FakeChannel : IMessageChannel
        {
            private readonly Queue<Message?> _incoming = new();

            public List<Message> Sent { get; } = [];
            public bool Disposed { get; private set; }

            public void Bot(Message? message) => _incoming.Enqueue(message);

            public Task SendAsync(Message message, CancellationToken cancellationToken)
            {
                Sent.Add(message);
                return Task.CompletedTask;
            }

            public Task<Message?> ReceiveAsync(CancellationToken cancellationToken) =>
                Task.FromResult(_incoming.Count > 0 ? _incoming.Dequeue() : null);

            public ValueTask DisposeAsync()
            {
                Disposed = true;
                return ValueTask.CompletedTask;
            }
        }

        private static CaptureConnection Connect(FakeChannel channel, string sessionId = "session-a") =>
            new(channel, new CaptureSession("1.0.0", () => sessionId));

        [Fact]
        public async Task TheHandshakeIsHelloThenTheCredential()
        {
            var channel = new FakeChannel();
            var connection = Connect(channel);

            await connection.HandshakeAsync("0011.SECRET");

            Assert.Collection(channel.Sent,
                first => Assert.Equal(ProtocolContract.TypeHello, Assert.IsType<Hello>(first).Type),
                second =>
                {
                    var authentication = Assert.IsType<Authentication>(second);
                    Assert.Equal("0011.SECRET", authentication.Credential);
                    Assert.Equal(2UL, authentication.Seq);
                });
        }

        /// <summary>
        /// The bot answers only when it refuses. A refusal has to reach the user
        /// as something they can act on, not as a connection that quietly died.
        /// </summary>
        [Fact]
        public async Task ARefusalFromTheBotSurfaces()
        {
            var channel = new FakeChannel();
            channel.Bot(new ProtocolError
            {
                Session = "session-a",
                Seq = 1,
                Code = ProtocolContract.CodeUnauthenticated,
                Message = "this capture credential was revoked",
            });

            var connection = Connect(channel);
            await connection.HandshakeAsync("0011.SECRET");

            var refused = await Assert.ThrowsAsync<CaptureRefusedException>(
                () => connection.ListenAsync());

            Assert.Equal(ProtocolContract.CodeUnauthenticated, refused.Code);
            Assert.Contains("revoked", refused.Message, StringComparison.OrdinalIgnoreCase);
        }

        /// <summary>
        /// An incompatible protocol has to be named, because the only thing that
        /// fixes it is installing a matching build.
        /// </summary>
        [Fact]
        public async Task AnIncompatibleProtocolIsReportedWithItsCode()
        {
            var channel = new FakeChannel();
            channel.Bot(new ProtocolError
            {
                Session = "session-a",
                Seq = 1,
                Code = ProtocolContract.CodeIncompatibleProtocol,
                Message = "this bot speaks protocol 2 and capture speaks 1",
            });

            var connection = Connect(channel);

            var refused = await Assert.ThrowsAsync<CaptureRefusedException>(
                () => connection.ListenAsync());

            Assert.Equal(ProtocolContract.CodeIncompatibleProtocol, refused.Code);
        }

        /// <summary>
        /// A closed connection is how a session ends normally, so it is not an
        /// error on its own.
        /// </summary>
        [Fact]
        public async Task AClosedConnectionEndsListeningQuietly()
        {
            var channel = new FakeChannel();
            var connection = Connect(channel);

            await connection.ListenAsync();
        }

        /// <summary>
        /// Reconnecting is a new session from the bot's point of view, so the
        /// handshake and a complete snapshot are owed again.
        /// </summary>
        [Fact]
        public async Task ReconnectingStartsANewSessionAndOwesASnapshot()
        {
            var first = new FakeChannel();
            var session = new CaptureSession("1.0.0", NewIds("session-a", "session-b"));

            var connection = new CaptureConnection(first, session);
            await connection.HandshakeAsync("0011.SECRET");
            await connection.SendAsync(session.Snapshot(ProtocolContract.PhaseTasks,
                [new Player { Name = "Red" }]));

            var second = new FakeChannel();
            var reconnected = new CaptureConnection(second, session);
            await reconnected.HandshakeAsync("0011.SECRET");

            Assert.Equal("session-b", session.SessionId);
            Assert.False(session.SnapshotSent);
            Assert.Equal(1UL, second.Sent.First().Seq);
        }

        [Fact]
        public async Task DisposingClosesTheChannel()
        {
            var channel = new FakeChannel();
            var connection = Connect(channel);

            await connection.DisposeAsync();

            Assert.True(channel.Disposed);
        }

        /// <summary>
        /// Everything that leaves capture has to satisfy the contract, or the
        /// bot refuses it mid-round.
        /// </summary>
        [Fact]
        public async Task EverythingSentIsValid()
        {
            var channel = new FakeChannel();
            var connection = Connect(channel);

            await connection.HandshakeAsync("0011.SECRET");
            await connection.SendAsync(connection.Session.Snapshot(
                ProtocolContract.PhaseTasks, [new Player { Name = "Red" }]));
            await connection.SendAsync(connection.Session.PlayerDied(
                new Player { Name = "Red", Dead = true }));

            Assert.All(channel.Sent, message => Assert.Null(MessageValidator.Refusal(message)));
        }

        private static Func<string> NewIds(params string[] ids)
        {
            var queue = new Queue<string>(ids);
            return () => queue.Count > 0 ? queue.Dequeue() : "session-z";
        }
    }
}
