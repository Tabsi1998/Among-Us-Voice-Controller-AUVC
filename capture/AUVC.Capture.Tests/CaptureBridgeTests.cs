using System;
using System.Collections.Generic;
using AmongUsCapture;
using AUVC.Protocol;
using AUVC.Transport;
using Xunit;

namespace AUVC.Capture.Tests
{
    /// <summary>
    /// The mapping from what the memory reader sees to what the bot is told. A
    /// mistake here is invisible to every other test and decides who can speak.
    /// </summary>
    public class CaptureBridgeTests
    {
        private sealed class Recorder : IRoundReporter
        {
            public List<(string Report, Player? Player, string? Phase)> Reports { get; } = [];

            public void ReportPhase(string phase) => Reports.Add(("phase", null, phase));

            public void ReportPlayerJoined(Player player) => Reports.Add(("joined", player, null));

            public void ReportPlayerLeft(Player player) => Reports.Add(("left", player, null));

            public void ReportPlayerChanged(Player player) => Reports.Add(("changed", player, null));

            public void ReportPlayerDied(Player player) => Reports.Add(("died", player, null));

            public void ReportGameEnded() => Reports.Add(("ended", null, null));
        }

        [Theory]
        [InlineData(GameState.LOBBY, ProtocolContract.PhaseLobby)]
        [InlineData(GameState.TASKS, ProtocolContract.PhaseTasks)]
        [InlineData(GameState.DISCUSSION, ProtocolContract.PhaseDiscussion)]
        [InlineData(GameState.MENU, ProtocolContract.PhaseMenu)]
        [InlineData(GameState.ENDED, ProtocolContract.PhaseEnded)]
        public void EveryGameStateTheProtocolKnowsIsForwarded(GameState state, string phase)
        {
            var recorder = new Recorder();

            CaptureBridge.Forward(recorder, new GameStateChangedEventArgs { NewState = state });

            Assert.Equal(("phase", (Player?)null, phase), Assert.Single(recorder.Reports));
        }

        /// <summary>
        /// UNKNOWN is the reader failing to tell. Forwarding a guess would silence
        /// or release a whole lobby.
        /// </summary>
        [Fact]
        public void AnUnknownStateIsNotForwarded()
        {
            var recorder = new Recorder();

            CaptureBridge.Forward(recorder, new GameStateChangedEventArgs { NewState = GameState.UNKNOWN });

            Assert.Empty(recorder.Reports);
        }

        [Fact]
        public void EveryGameStateIsMappedOrDeliberatelyLeftOut()
        {
            foreach (var state in Enum.GetValues<GameState>())
            {
                Assert.True(CaptureBridge.PhaseFor(state) is not null || state == GameState.UNKNOWN,
                    $"{state} is not mapped to a protocol phase");
            }
        }

        [Theory]
        [InlineData(PlayerAction.Joined, "joined")]
        [InlineData(PlayerAction.Left, "left")]
        [InlineData(PlayerAction.Died, "died")]
        [InlineData(PlayerAction.Exiled, "died")]
        [InlineData(PlayerAction.ChangedColor, "changed")]
        [InlineData(PlayerAction.Disconnected, "changed")]
        [InlineData(PlayerAction.ForceUpdated, "changed")]
        public void EveryPlayerActionBecomesTheMatchingReport(PlayerAction action, string report)
        {
            var recorder = new Recorder();

            CaptureBridge.Forward(recorder, new PlayerChangedEventArgs
            {
                Action = action,
                Name = "Red",
                Color = PlayerColor.Cyan,
                IsDead = false,
                Disconnected = action == PlayerAction.Disconnected,
            });

            var (kind, player, _) = Assert.Single(recorder.Reports);
            Assert.Equal(report, kind);
            Assert.Equal("Red", player!.Name);
            Assert.Equal((int)PlayerColor.Cyan, player.Color);
            Assert.Equal(action == PlayerAction.Disconnected, player.Disconnected);
        }

        [Fact]
        public void EveryPlayerActionIsHandled()
        {
            foreach (var action in Enum.GetValues<PlayerAction>())
            {
                var recorder = new Recorder();
                CaptureBridge.Forward(recorder, new PlayerChangedEventArgs { Action = action, Name = "Red" });
                Assert.True(recorder.Reports.Count == 1, $"{action} is not forwarded");
            }
        }
    }
}
