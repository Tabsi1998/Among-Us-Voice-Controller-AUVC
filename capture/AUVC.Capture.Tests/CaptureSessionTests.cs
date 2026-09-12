using System;
using System.Collections.Generic;
using System.Linq;
using AUVC.Protocol;
using AUVC.Transport;
using Xunit;

namespace AUVC.Capture.Tests
{
    /// <summary>
    /// Checks that capture speaks in the order the bot accepts, without opening
    /// a socket. The bot enforces all of this itself; these tests exist so a
    /// mistake is caught here rather than as a refusal in the middle of a round.
    /// </summary>
    public class CaptureSessionTests
    {
        private static readonly IReadOnlyList<Player> Lobby =
        [
            new Player { Name = "Red", Color = 0 },
            new Player { Name = "Blue", Color = 1 },
        ];

        private static CaptureSession NewSession(params string[] ids)
        {
            var queue = new Queue<string>(ids.Length > 0 ? ids : ["session-a"]);
            return new CaptureSession("1.0.0", () => queue.Count > 0 ? queue.Dequeue() : "session-z");
        }

        [Fact]
        public void AHandshakeIsNumberedFromOne()
        {
            var session = NewSession();

            var hello = session.Open();
            var authentication = session.Authenticate("id.secret");
            var snapshot = session.Snapshot(ProtocolContract.PhaseTasks, Lobby);

            Assert.Equal(1UL, hello.Seq);
            Assert.Equal(2UL, authentication.Seq);
            Assert.Equal(3UL, snapshot.Seq);
            Assert.Equal("session-a", session.SessionId);
        }

        /// <summary>
        /// Sequence numbers increase by exactly one. A gap costs the session its
        /// snapshot on the bot side, so producing one would be self-inflicted.
        /// </summary>
        [Fact]
        public void SequenceNumbersHaveNoGaps()
        {
            var session = NewSession();

            var produced = new List<Message>
            {
                session.Open(),
                session.Authenticate("id.secret"),
                session.Snapshot(ProtocolContract.PhaseTasks, Lobby),
                session.Heartbeat(),
                session.PlayerDied(new Player { Name = "Red", Dead = true }),
                session.PhaseChanged(ProtocolContract.PhaseDiscussion),
                session.GameEnded(),
            };

            Assert.Equal(
                Enumerable.Range(1, produced.Count).Select(n => (ulong)n),
                produced.Select(message => message.Seq));
        }

        /// <summary>
        /// The bot refuses events until a complete snapshot arrives. Failing
        /// here names the mistake where it was made, instead of turning it into
        /// a protocol error from the far end.
        /// </summary>
        [Fact]
        public void EventsBeforeASnapshotAreRefusedLocally()
        {
            var session = NewSession();
            session.Open();
            session.Authenticate("id.secret");

            Assert.False(session.SnapshotSent);
            Assert.Throws<InvalidOperationException>(
                () => session.PlayerDied(new Player { Name = "Red", Dead = true }));
            Assert.Throws<InvalidOperationException>(
                () => session.PhaseChanged(ProtocolContract.PhaseTasks));
        }

        [Fact]
        public void NothingCanBeSentBeforeTheSessionIsOpened()
        {
            var session = NewSession();

            Assert.Throws<InvalidOperationException>(() => session.Authenticate("id.secret"));
            Assert.Throws<InvalidOperationException>(() => session.Heartbeat());
        }

        /// <summary>
        /// Every connection is a new session. The bot drops everything it knew
        /// about the previous one, so the numbering starts again and a complete
        /// snapshot is owed before any event.
        /// </summary>
        [Fact]
        public void AReconnectStartsANewSessionAndOwesASnapshot()
        {
            var session = NewSession("session-a", "session-b");

            session.Open();
            session.Authenticate("id.secret");
            session.Snapshot(ProtocolContract.PhaseTasks, Lobby);
            session.PlayerDied(new Player { Name = "Red", Dead = true });

            var reconnect = session.Open();

            Assert.Equal("session-b", reconnect.Session);
            Assert.Equal(1UL, reconnect.Seq);
            Assert.False(session.SnapshotSent);
            Assert.Throws<InvalidOperationException>(
                () => session.PlayerDied(new Player { Name = "Blue", Dead = true }));
        }

        [Fact]
        public void EveryMessageCarriesTheSameSession()
        {
            var session = NewSession();

            var produced = new Message[]
            {
                session.Open(),
                session.Authenticate("id.secret"),
                session.Snapshot(ProtocolContract.PhaseLobby, Lobby),
                session.Heartbeat(),
                session.PlayerJoined(new Player { Name = "Green", Color = 2 }),
                session.PlayerChanged(new Player { Name = "Green", Color = 5 }),
                session.PlayerLeft(new Player { Name = "Green", Color = 5, Disconnected = true }),
            };

            Assert.All(produced, message => Assert.Equal("session-a", message.Session));
        }

        /// <summary>
        /// Whatever capture produces has to satisfy the contract, or the bot
        /// refuses it. Checking here means the contract is enforced on both
        /// ends of the same wire.
        /// </summary>
        [Fact]
        public void EverythingCaptureProducesIsValid()
        {
            var session = NewSession();

            var produced = new Message[]
            {
                session.Open(),
                session.Authenticate("id.secret"),
                session.Snapshot(ProtocolContract.PhaseTasks, Lobby),
                session.Heartbeat(),
                session.PhaseChanged(ProtocolContract.PhaseDiscussion),
                session.PlayerJoined(new Player { Name = "Green", Color = 2 }),
                session.PlayerChanged(new Player { Name = "Green", Color = 5 }),
                session.PlayerDied(new Player { Name = "Red", Dead = true }),
                session.PlayerLeft(new Player { Name = "Green", Color = 5, Disconnected = true }),
                session.GameEnded(),
            };

            Assert.All(produced, message => Assert.Null(MessageValidator.Refusal(message)));
        }

        /// <summary>
        /// The messages have to survive the codec, because that is what actually
        /// travels. A field that does not serialize is a field the bot never
        /// sees.
        /// </summary>
        [Fact]
        public void EverythingCaptureProducesSurvivesTheCodec()
        {
            var session = NewSession();
            session.Open();
            session.Authenticate("id.secret");

            var snapshot = session.Snapshot(ProtocolContract.PhaseTasks, Lobby);
            var decoded = Assert.IsType<Snapshot>(ProtocolCodec.Decode(ProtocolCodec.Encode(snapshot)));

            Assert.Equal(snapshot.Session, decoded.Session);
            Assert.Equal(snapshot.Seq, decoded.Seq);
            Assert.Equal(ProtocolContract.PhaseTasks, decoded.Phase);
            Assert.Equal(2, decoded.Players.Count);
            Assert.Equal("Red", decoded.Players[0].Name);
        }

        [Fact]
        public void ASessionRefusesToAuthenticateWithoutACredential()
        {
            var session = NewSession();
            session.Open();

            Assert.Throws<InvalidOperationException>(() => session.Authenticate(""));
        }

        [Fact]
        public void ASessionHasToSayWhichBuildItIs()
        {
            Assert.Throws<ArgumentException>(() => new CaptureSession(""));
        }

        /// <summary>
        /// A heartbeat is liveness and says nothing about the round, so it is
        /// allowed before the snapshot. The bot accepts it there for the same
        /// reason: refusing it would make a live capture look dead.
        /// </summary>
        [Fact]
        public void AHeartbeatIsAllowedBeforeTheSnapshot()
        {
            var session = NewSession();
            session.Open();
            session.Authenticate("id.secret");

            var heartbeat = session.Heartbeat();

            Assert.Equal(3UL, heartbeat.Seq);
            Assert.Null(MessageValidator.Refusal(heartbeat));
        }
    }
}
