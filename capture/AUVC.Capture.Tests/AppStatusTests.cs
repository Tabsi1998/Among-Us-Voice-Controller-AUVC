using AUVC.Protocol;
using AUVC.Transport;
using Xunit;

namespace AUVC.Capture.Tests
{
    /// <summary>
    /// The status line in the main window. It always names the most urgent thing,
    /// so a person who follows it one step at a time ends up in a managed round.
    /// </summary>
    public class AppStatusTests
    {
        private static readonly AppSituation Playing = new()
        {
            RunsBotOnThisPc = true,
            Paired = true,
            LocalBotRunning = true,
            Link = new LinkStatus(LinkState.Connected),
            Game = GameView.Round,
            Session = LocalGuild.SessionRunning,
            Players = 10,
            LinkedPlayers = 7,
        };

        [Fact]
        public void ARoundAUVCManagesIsReadyAndCountsTheLinkedPlayers() =>
            Assert.Equal(new AppStatus(AppStatusKind.Ready, 7, 10, true), AppStatus.For(Playing));

        [Fact]
        public void NothingSetUpComesBeforeEverythingElse()
        {
            var fresh = Playing with
            {
                RunsBotOnThisPc = false,
                Paired = false,
                LocalBotRunning = false,
                Link = new LinkStatus(LinkState.NotPaired),
                Game = GameView.NotRunning,
            };

            Assert.Equal(AppStatusKind.NotSetUp, AppStatus.For(fresh).Kind);
        }

        [Fact]
        public void ABotOnThisPcThatIsNotRunningComesBeforeTheLink() =>
            Assert.Equal(AppStatusKind.BotNotRunning,
                AppStatus.For(Playing with { LocalBotRunning = false, Link = new LinkStatus(LinkState.Retrying) }).Kind);

        // A bot on another computer is not this PC's to run, and cannot be asked
        // about its session or the lobby.
        [Fact]
        public void AnAppPairedWithABotElsewhereNeedsNoBotOnThisPc() =>
            Assert.Equal(new AppStatus(AppStatusKind.Ready),
                AppStatus.For(Playing with
                {
                    RunsBotOnThisPc = false,
                    LocalBotRunning = false,
                    Session = null,
                    Players = null,
                    LinkedPlayers = null,
                }));

        [Theory]
        [InlineData(LinkState.Stopped, AppStatusKind.Connecting)]
        [InlineData(LinkState.Connecting, AppStatusKind.Connecting)]
        [InlineData(LinkState.Retrying, AppStatusKind.Connecting)]
        [InlineData(LinkState.NotPaired, AppStatusKind.NotPaired)]
        public void TheLinkComesBeforeTheGame(LinkState state, AppStatusKind expected) =>
            Assert.Equal(expected, AppStatus.For(Playing with { Link = new LinkStatus(state), Game = GameView.NotRunning }).Kind);

        [Fact]
        public void ABotThatSpeaksAnotherProtocolIsAWrongVersion() =>
            Assert.Equal(AppStatusKind.WrongVersion, AppStatus.For(Playing with
            {
                Link = new LinkStatus(LinkState.Refused, "incompatible", ProtocolContract.CodeIncompatibleProtocol),
            }).Kind);

        [Fact]
        public void AnyOtherRefusalIsARefusal() =>
            Assert.Equal(AppStatusKind.Refused, AppStatus.For(Playing with
            {
                Link = new LinkStatus(LinkState.Refused, "no", ProtocolContract.CodeUnauthenticated),
            }).Kind);

        [Theory]
        [InlineData(GameView.NotRunning, AppStatusKind.WaitingForGame)]
        [InlineData(GameView.Menu, AppStatusKind.InMenu)]
        public void WithoutALobbyTheGameIsTheNextStep(GameView game, AppStatusKind expected) =>
            Assert.Equal(expected, AppStatus.For(Playing with { Game = game, Session = LocalGuild.SessionPaused }).Kind);

        [Theory]
        [InlineData(GameView.Lobby)]
        [InlineData(GameView.Round)]
        public void APausedSessionIsSaidInALobbyAndInARound(GameView game) =>
            Assert.Equal(AppStatusKind.SessionPaused, AppStatus.For(Playing with { Game = game, Session = LocalGuild.SessionPaused }).Kind);

        // Automatic start waits for the round, so a stopped session in a lobby is
        // how it should be. In a round nobody is being muted, which needs saying.
        [Fact]
        public void AStoppedSessionMattersOnlyInARound()
        {
            Assert.Equal(AppStatusKind.SessionStopped,
                AppStatus.For(Playing with { Session = LocalGuild.SessionStopped }).Kind);
            Assert.Equal(AppStatusKind.Ready,
                AppStatus.For(Playing with { Game = GameView.Lobby, Session = LocalGuild.SessionStopped }).Kind);
        }

        // AUVC manages only linked players, and a public lobby has strangers in it,
        // so some unlinked players are normal. Nobody linked at all is not.
        [Fact]
        public void NobodyLinkedIsAProblemButSomeUnlinkedPlayersAreNot()
        {
            Assert.Equal(new AppStatus(AppStatusKind.NobodyLinked, 0, 10, true),
                AppStatus.For(Playing with { LinkedPlayers = 0 }));
            Assert.Equal(new AppStatus(AppStatusKind.Ready, 1, 10, true),
                AppStatus.For(Playing with { LinkedPlayers = 1 }));
            Assert.Equal(new AppStatus(AppStatusKind.Ready, 0, 0, true),
                AppStatus.For(Playing with { Players = 0, LinkedPlayers = 0 }));
        }

        [Fact]
        public void AnOlderBotWithoutASessionIsTakenAsReady() =>
            Assert.Equal(AppStatusKind.Ready, AppStatus.For(Playing with { Session = "" }).Kind);
    }
}
