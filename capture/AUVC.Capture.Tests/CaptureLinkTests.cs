using System;
using System.Collections.Generic;
using System.Linq;
using System.Net.Http;
using System.Threading;
using System.Threading.Channels;
using System.Threading.Tasks;
using AUVC.Protocol;
using AUVC.Transport;
using Xunit;

namespace AUVC.Capture.Tests
{
    /// <summary>
    /// Checks the link's behaviour without opening a socket, the same way the
    /// bot's protocol rules are checked without one.
    /// </summary>
    public class CaptureLinkTests
    {
        private const string Credential = "0011.SECRET";

        // Retries fast enough to test, and no heartbeat unless a test asks for
        // one, so the messages a test reads are the ones it caused.
        private static readonly CaptureLinkOptions Fast = new()
        {
            HeartbeatInterval = TimeSpan.FromMinutes(10),
            RetryDelays = [TimeSpan.FromMilliseconds(10)],
            DrainTimeout = TimeSpan.FromMilliseconds(200),
        };

        /// <summary>One connection, as the bot's end of it.</summary>
        private sealed class ScriptedChannel : IMessageChannel
        {
            private readonly Channel<Message?> _fromBot = Channel.CreateUnbounded<Message?>();
            private readonly Channel<Message> _sent = Channel.CreateUnbounded<Message>();
            private readonly object _lock = new();
            private readonly List<Message> _all = [];

            public bool Disposed { get; private set; }

            public IReadOnlyList<Message> Sent
            {
                get
                {
                    lock (_lock)
                    {
                        return _all.ToList();
                    }
                }
            }

            /// <summary>Makes the bot say something.</summary>
            public void Say(Message message) => _fromBot.Writer.TryWrite(message);

            /// <summary>Makes the bot hang up.</summary>
            public void Close() => _fromBot.Writer.TryWrite(null);

            /// <summary>The next message capture sent, waiting for it if necessary.</summary>
            public async Task<Message> NextAsync()
            {
                using var timeout = new CancellationTokenSource(TimeSpan.FromSeconds(5));
                return await _sent.Reader.ReadAsync(timeout.Token);
            }

            public Task SendAsync(Message message, CancellationToken cancellationToken)
            {
                lock (_lock)
                {
                    _all.Add(message);
                }
                _sent.Writer.TryWrite(message);
                return Task.CompletedTask;
            }

            public async Task<Message?> ReceiveAsync(CancellationToken cancellationToken) =>
                await _fromBot.Reader.ReadAsync(cancellationToken);

            public ValueTask DisposeAsync()
            {
                Disposed = true;
                return ValueTask.CompletedTask;
            }
        }

        /// <summary>The bot's listener: accepts connections, or refuses to be reached.</summary>
        private sealed class FakeBot
        {
            private readonly Channel<ScriptedChannel> _accepted = Channel.CreateUnbounded<ScriptedChannel>();
            private int _attempts;

            public volatile bool Reachable = true;

            public int Attempts => Volatile.Read(ref _attempts);

            public Task<IMessageChannel> ConnectAsync(CancellationToken cancellationToken)
            {
                Interlocked.Increment(ref _attempts);
                if (!Reachable)
                {
                    throw new HttpRequestException("No connection could be made.");
                }

                var channel = new ScriptedChannel();
                _accepted.Writer.TryWrite(channel);
                return Task.FromResult<IMessageChannel>(channel);
            }

            public async Task<ScriptedChannel> AcceptAsync()
            {
                using var timeout = new CancellationTokenSource(TimeSpan.FromSeconds(5));
                return await _accepted.Reader.ReadAsync(timeout.Token);
            }
        }

        private static InMemoryCredentialStore Paired(string credential = Credential)
        {
            var store = new InMemoryCredentialStore();
            store.Write(credential);
            return store;
        }

        private static CaptureLink NewLink(FakeBot bot, ICredentialStore store, CaptureLinkOptions? options = null) =>
            new(store, bot.ConnectAsync, "1.0.0", options ?? Fast, NewIds("session-a", "session-b", "session-c"));

        private static async Task<Message[]> HandshakeAsync(ScriptedChannel channel) =>
            [await channel.NextAsync(), await channel.NextAsync(), await channel.NextAsync()];

        [Fact]
        public async Task AConnectionOpensWithHelloTheCredentialAndASnapshot()
        {
            var bot = new FakeBot();
            await using var link = NewLink(bot, Paired());
            link.ReportPhase(ProtocolContract.PhaseTasks);
            link.ReportPlayerJoined(new Player { Name = "Red", Color = 0 });

            link.Start();
            var sent = await HandshakeAsync(await bot.AcceptAsync());

            var hello = Assert.IsType<Hello>(sent[0]);
            Assert.Equal(1UL, hello.Seq);
            Assert.Equal("session-a", hello.Session);

            var authentication = Assert.IsType<Authentication>(sent[1]);
            Assert.Equal(Credential, authentication.Credential);
            Assert.Equal(2UL, authentication.Seq);

            var snapshot = Assert.IsType<Snapshot>(sent[2]);
            Assert.Equal(ProtocolContract.PhaseTasks, snapshot.Phase);
            Assert.Equal("Red", Assert.Single(snapshot.Players).Name);

            await Eventually(() => link.Status.State == LinkState.Connected);
        }

        [Fact]
        public async Task EventsFollowTheSnapshot()
        {
            var bot = new FakeBot();
            await using var link = NewLink(bot, Paired());
            link.Start();
            var channel = await bot.AcceptAsync();
            await HandshakeAsync(channel);

            link.ReportPlayerDied(new Player { Name = "Red" });

            var died = Assert.IsType<PlayerDied>(await channel.NextAsync());
            Assert.Equal(4UL, died.Seq);
            Assert.True(died.Player.Dead);
        }

        /// <summary>
        /// The game reader reports on its own thread. A message numbered before
        /// another but sent after it would show the bot a gap, and the bot
        /// answers a gap by refusing events until the next snapshot.
        /// </summary>
        [Fact]
        public async Task ReportsFromManyThreadsLeaveNoGapInTheSequence()
        {
            var bot = new FakeBot();
            await using var link = NewLink(bot, Paired());
            link.Start();
            var channel = await bot.AcceptAsync();
            await HandshakeAsync(channel);

            await Task.WhenAll(Enumerable.Range(0, 8).Select(thread => Task.Run(() =>
            {
                for (var i = 0; i < 50; i++)
                {
                    link.ReportPlayerChanged(new Player { Name = $"P{thread}-{i}" });
                }
            })));

            for (var i = 0; i < 400; i++)
            {
                await channel.NextAsync();
            }

            var sequence = channel.Sent.Select(message => message.Seq).ToList();
            Assert.Equal(Enumerable.Range(1, sequence.Count).Select(n => (ulong)n), sequence);
        }

        [Fact]
        public async Task AnUnchangedPhaseIsNotSentAgain()
        {
            var bot = new FakeBot();
            await using var link = NewLink(bot, Paired());
            link.Start();
            var channel = await bot.AcceptAsync();
            await HandshakeAsync(channel);

            link.ReportPhase(ProtocolContract.PhaseTasks);
            link.ReportPhase(ProtocolContract.PhaseTasks);
            link.ReportPlayerChanged(new Player { Name = "Red" });

            Assert.IsType<GameStateChanged>(await channel.NextAsync());
            Assert.IsType<PlayerChanged>(await channel.NextAsync());
        }

        /// <summary>
        /// Reconnecting is a new session from the bot's point of view, and what
        /// happened while capture was away has to arrive in its snapshot.
        /// </summary>
        [Fact]
        public async Task AReconnectOpensANewSessionWhoseSnapshotCarriesWhatWasMissed()
        {
            var bot = new FakeBot();
            await using var link = NewLink(bot, Paired());
            link.ReportPhase(ProtocolContract.PhaseTasks);
            link.ReportPlayerJoined(new Player { Name = "Red" });
            link.Start();

            var first = await bot.AcceptAsync();
            await HandshakeAsync(first);

            bot.Reachable = false;
            first.Close();
            await Eventually(() => link.Status.State == LinkState.Retrying && bot.Attempts >= 2);

            link.ReportPlayerDied(new Player { Name = "Red" });
            Assert.DoesNotContain(first.Sent, message => message is PlayerDied);

            bot.Reachable = true;
            var sent = await HandshakeAsync(await bot.AcceptAsync());

            var hello = Assert.IsType<Hello>(sent[0]);
            Assert.Equal("session-b", hello.Session);
            Assert.Equal(1UL, hello.Seq);
            Assert.True(Assert.Single(Assert.IsType<Snapshot>(sent[2]).Players).Dead);
            Assert.True(first.Disposed);
        }

        [Fact]
        public async Task AnUnreachableBotIsTriedAgain()
        {
            var bot = new FakeBot { Reachable = false };
            await using var link = NewLink(bot, Paired());
            link.Start();

            await Eventually(() => link.Status.State == LinkState.Retrying && bot.Attempts >= 2);
            Assert.Contains("Could not reach", link.Status.Detail);

            bot.Reachable = true;
            await HandshakeAsync(await bot.AcceptAsync());
            await Eventually(() => link.Status.State == LinkState.Connected);
        }

        /// <summary>
        /// A revoked credential stays revoked however often it is presented, so
        /// trying again would only hammer the bot. The refusal has to reach the
        /// person running capture as something they can act on.
        /// </summary>
        [Fact]
        public async Task ARevokedCredentialStopsRetryingUntilCapturePairsAgain()
        {
            var bot = new FakeBot();
            var store = Paired();
            await using var link = NewLink(bot, store);
            link.Start();

            var channel = await bot.AcceptAsync();
            await HandshakeAsync(channel);
            channel.Say(new ProtocolError
            {
                Code = ProtocolContract.CodeUnauthenticated,
                Message = "this capture credential was revoked; ask an administrator to run /au capture pair again",
            });

            await Eventually(() => link.Status.State == LinkState.Refused);
            Assert.Equal(ProtocolContract.CodeUnauthenticated, link.Status.Code);
            Assert.Contains("revoked", link.Status.Detail);
            Assert.True(channel.Disposed);

            var attempts = bot.Attempts;
            await Task.Delay(200);
            Assert.Equal(attempts, bot.Attempts);

            store.Write("0022.NEWSECRET");
            link.Reconnect();

            var authentication = Assert.IsType<Authentication>((await HandshakeAsync(await bot.AcceptAsync()))[1]);
            Assert.Equal("0022.NEWSECRET", authentication.Credential);
        }

        /// <summary>
        /// Only installing a matching build fixes an incompatible protocol, so the
        /// code has to be kept for the window to say so.
        /// </summary>
        [Fact]
        public async Task AnIncompatibleBotStopsRetrying()
        {
            var bot = new FakeBot();
            await using var link = NewLink(bot, Paired());
            link.Start();

            var channel = await bot.AcceptAsync();
            channel.Say(new ProtocolError
            {
                Code = ProtocolContract.CodeIncompatibleProtocol,
                Message = "this bot speaks protocol 2 and capture speaks 1",
            });

            await Eventually(() => link.Status.State == LinkState.Refused);
            Assert.Equal(ProtocolContract.CodeIncompatibleProtocol, link.Status.Code);
        }

        /// <summary>
        /// A missing snapshot is fixed by sending one. Reconnecting for it would
        /// turn an ordinary recovery into a loop.
        /// </summary>
        [Fact]
        public async Task ARequestForASnapshotIsAnsweredOnTheSameConnection()
        {
            var bot = new FakeBot();
            await using var link = NewLink(bot, Paired());
            link.ReportPlayerJoined(new Player { Name = "Red" });
            link.Start();

            var channel = await bot.AcceptAsync();
            await HandshakeAsync(channel);
            channel.Say(new ProtocolError
            {
                Code = ProtocolContract.CodeSnapshotRequired,
                Message = "send a snapshot",
            });

            var snapshot = Assert.IsType<Snapshot>(await channel.NextAsync());
            Assert.Equal(4UL, snapshot.Seq);
            Assert.Equal("Red", Assert.Single(snapshot.Players).Name);
            Assert.Equal(1, bot.Attempts);
            Assert.Equal(LinkState.Connected, link.Status.State);
        }

        [Fact]
        public async Task WithoutACredentialTheLinkWaitsForPairing()
        {
            var bot = new FakeBot();
            var store = new InMemoryCredentialStore();
            await using var link = NewLink(bot, store);
            link.Start();

            await Eventually(() => link.Status.State == LinkState.NotPaired);
            Assert.Contains("/au capture pair", link.Status.Detail);
            Assert.Equal(0, bot.Attempts);

            store.Write(Credential);
            link.Reconnect();

            await HandshakeAsync(await bot.AcceptAsync());
        }

        /// <summary>
        /// The bot runs its fail-safe when capture goes quiet, and a lobby where
        /// nothing happens is quiet.
        /// </summary>
        [Fact]
        public async Task HeartbeatsKeepAQuietConnectionAlive()
        {
            var bot = new FakeBot();
            await using var link = NewLink(bot, Paired(), Fast with { HeartbeatInterval = TimeSpan.FromMilliseconds(20) });
            link.Start();

            var channel = await bot.AcceptAsync();
            await HandshakeAsync(channel);

            Assert.IsType<Heartbeat>(await channel.NextAsync());
            Assert.IsType<Heartbeat>(await channel.NextAsync());
        }

        [Fact]
        public async Task DisposingStopsTheLinkAndClosesTheChannel()
        {
            var bot = new FakeBot();
            var link = NewLink(bot, Paired());
            link.Start();
            var channel = await bot.AcceptAsync();
            await HandshakeAsync(channel);

            await link.DisposeAsync();

            Assert.True(channel.Disposed);
            Assert.Equal(LinkState.Stopped, link.Status.State);
        }

        /// <summary>
        /// Everything that leaves capture has to satisfy the contract, or the bot
        /// refuses it mid-round.
        /// </summary>
        [Fact]
        public async Task EverythingSentIsValid()
        {
            var bot = new FakeBot();
            await using var link = NewLink(bot, Paired());
            link.Start();
            var channel = await bot.AcceptAsync();
            await HandshakeAsync(channel);

            link.ReportPhase(ProtocolContract.PhaseLobby);
            link.ReportPlayerJoined(new Player { Name = "Red" });
            link.ReportPlayerChanged(new Player { Name = "Red", Color = 3 });
            link.ReportPhase(ProtocolContract.PhaseTasks);
            link.ReportPlayerDied(new Player { Name = "Red" });
            link.ReportGameEnded();
            link.ReportPlayerLeft(new Player { Name = "Red" });
            // A nameless player is recorded nowhere and sent nowhere.
            link.ReportPlayerJoined(new Player { Name = "" });

            for (var i = 0; i < 7; i++)
            {
                await channel.NextAsync();
            }

            Assert.Equal(10, channel.Sent.Count);
            Assert.All(channel.Sent, message => Assert.Null(MessageValidator.Refusal(message)));
        }

        private static async Task Eventually(Func<bool> condition)
        {
            var deadline = DateTime.UtcNow.AddSeconds(5);
            while (DateTime.UtcNow < deadline)
            {
                if (condition())
                {
                    return;
                }
                await Task.Delay(5);
            }
            Assert.Fail("timed out waiting for the link");
        }

        private static Func<string> NewIds(params string[] ids)
        {
            var queue = new Queue<string>(ids);
            return () => queue.Count > 0 ? queue.Dequeue() : "session-z";
        }
    }
}
