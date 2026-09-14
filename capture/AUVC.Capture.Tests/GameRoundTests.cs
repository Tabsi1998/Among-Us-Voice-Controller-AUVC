using System;
using System.Linq;
using AUVC.Protocol;
using AUVC.Transport;
using Xunit;

namespace AUVC.Capture.Tests
{
    public class GameRoundTests
    {
        [Fact]
        public void CaptureStartsInTheMenuWithNobody()
        {
            var round = new GameRound();

            Assert.Equal(ProtocolContract.PhaseMenu, round.Phase);
            Assert.Empty(round.Players);
        }

        [Fact]
        public void APhaseIsRecordedOnlyWhenItChanges()
        {
            var round = new GameRound();

            Assert.True(round.SetPhase(ProtocolContract.PhaseLobby));
            Assert.False(round.SetPhase(ProtocolContract.PhaseLobby));
        }

        [Fact]
        public void AnUnknownPhaseIsAMistakeNotAState()
        {
            Assert.Throws<ArgumentException>(() => new GameRound().SetPhase("voting"));
        }

        /// <summary>
        /// An exile is reported before the game marks the player dead, and it is
        /// a death all the same.
        /// </summary>
        [Fact]
        public void ADeathIsRecordedWhateverTheEventSaid()
        {
            var round = new GameRound();

            round.Die(new Player { Name = "Red", Dead = false });

            Assert.True(Assert.Single(round.Players).Dead);
        }

        [Fact]
        public void APlayerWhoLeftIsGone()
        {
            var round = new GameRound();
            round.Join(new Player { Name = "Red" });

            round.Leave(new Player { Name = "Red" });

            Assert.Empty(round.Players);
        }

        /// <summary>
        /// The game has no event for coming back to life. Without this, a
        /// reconnect in the next lobby would report last round's dead.
        /// </summary>
        [Theory]
        [InlineData(ProtocolContract.PhaseLobby)]
        [InlineData(ProtocolContract.PhaseMenu)]
        public void NobodyIsDeadInALobbyOrTheMenu(string phase)
        {
            var round = new GameRound();
            round.SetPhase(ProtocolContract.PhaseTasks);
            round.Die(new Player { Name = "Red" });

            round.SetPhase(phase);

            Assert.False(Assert.Single(round.Players).Dead);
        }

        [Fact]
        public void TheEndOfARoundRevivesEveryoneAndKeepsTheLobby()
        {
            var round = new GameRound();
            round.SetPhase(ProtocolContract.PhaseTasks);
            round.Join(new Player { Name = "Blue" });
            round.Die(new Player { Name = "Red" });

            round.End();

            Assert.Equal(2, round.Players.Count);
            Assert.All(round.Players, player => Assert.False(player.Dead));
        }

        [Fact]
        public void ANamelessPlayerIsNotRecorded()
        {
            var round = new GameRound();

            Assert.False(round.Join(new Player { Name = "" }));
            Assert.Empty(round.Players);
        }

        [Fact]
        public void PlayersAreOrderedByName()
        {
            var round = new GameRound();
            round.Join(new Player { Name = "White" });
            round.Join(new Player { Name = "Black" });

            Assert.Equal(["Black", "White"], round.Players.Select(player => player.Name));
        }
    }
}
