using AmongUsCapture;
using AUOffsetManager;
using Xunit;

namespace AUVC.Capture.Tests
{
    /// <summary>
    /// The meeting lasts until the game proceeds to the exile. The current game counts
    /// Animating 0, Discussion 1, NotVoted 2, Voted 3, Results 4, Proceeding 5; with the
    /// old threshold of 4 the results with the votes already muted everyone.
    /// </summary>
    public class MeetingPhaseTests
    {
        private const int Started = 2;

        [Theory]
        [InlineData(0)]
        [InlineData(1)]
        [InlineData(3)]
        [InlineData(4)]
        public void UpToTheResultsItIsTheMeeting(int state) =>
            Assert.Equal(GameState.DISCUSSION, GameMemReader.RoundState(Started, state, 5, exileCausesEnd: false));

        [Fact]
        public void ProceedingEndsTheMeeting() =>
            Assert.Equal(GameState.TASKS, GameMemReader.RoundState(Started, 5, 5, exileCausesEnd: false));

        [Fact]
        public void TheEntriesUpToV18KeepTheThresholdTheyWereTestedWith()
        {
            Assert.Equal(4, new GameOffsets().MeetingProceedingState);
            Assert.Equal(GameState.TASKS, GameMemReader.RoundState(Started, 4, 4, exileCausesEnd: false));
        }

        [Fact]
        public void The64BitSteamBuildNamesProceedingAsFive() =>
            Assert.Equal(5, OffsetManager.LoadBundledIndex()["615A95C9E09B6FFD45570883CB8459311C2B457533A38B37ED1704D6205AEDF9"].MeetingProceedingState);

        [Theory]
        [InlineData(0, GameState.MENU)]
        [InlineData(1, GameState.LOBBY)]
        [InlineData(3, GameState.ENDED)]
        public void OutsideARoundTheMeetingDoesNotCount(int gameState, GameState expected) =>
            Assert.Equal(expected, GameMemReader.RoundState(gameState, 1, 5, exileCausesEnd: false));

        [Fact]
        public void AnExileThatEndsTheGameGoesToTheLobby() =>
            Assert.Equal(GameState.LOBBY, GameMemReader.RoundState(Started, 1, 5, exileCausesEnd: true));
    }
}
