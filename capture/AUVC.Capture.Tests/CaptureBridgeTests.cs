using System;
using System.Collections.Generic;
using AmongUsCapture;
using AmongUsCapture.Memory.Structs;
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

            public RoundResult? Result;

            public void ReportGameEnded(RoundResult? result = null)
            {
                Result = result;
                Reports.Add(("ended", null, null));
            }

            public void ReportLobby(Lobby lobby) => Reports.Add(("lobby", null, lobby.Code + "@" + lobby.Map));
        }

        [Theory]
        [InlineData(PlayMap.Skeld, ProtocolContract.MapTheSkeld)]
        [InlineData(PlayMap.Mira, ProtocolContract.MapMiraHQ)]
        [InlineData(PlayMap.Polus, ProtocolContract.MapPolus)]
        [InlineData(PlayMap.dlekS, ProtocolContract.MapDleks)]
        [InlineData(PlayMap.Airship, ProtocolContract.MapAirship)]
        [InlineData(PlayMap.Fungle, ProtocolContract.MapFungle)]
        public void JoiningALobbyForwardsItsCodeAndMap(PlayMap map, string name)
        {
            var recorder = new Recorder();

            CaptureBridge.Forward(recorder, new LobbyEventArgs { LobbyCode = "ABCDEF", Map = map });

            var report = Assert.Single(recorder.Reports);
            Assert.Equal(("lobby", "ABCDEF@" + name), (report.Report, report.Phase));
        }

        // A map the game adds later must not turn up on the board as a guess.
        [Fact]
        public void EveryMapTheReaderKnowsHasAProtocolName()
        {
            foreach (var map in Enum.GetValues<PlayMap>())
            {
                Assert.Contains(CaptureBridge.MapFor(map), ProtocolContract.Maps);
            }
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

        /// <summary>The end of the round names the winning side, how, and the impostors, for Discord.</summary>
        [Theory]
        [InlineData(GameOverReason.HumansByVote, "crewmates", "vote")]
        [InlineData(GameOverReason.HumansByTask, "crewmates", "tasks")]
        [InlineData(GameOverReason.ImpostorByVote, "impostors", "vote")]
        [InlineData(GameOverReason.ImpostorByKill, "impostors", "kill")]
        [InlineData(GameOverReason.ImpostorBySabotage, "impostors", "sabotage")]
        [InlineData(GameOverReason.ImpostorDisconnect, "crewmates", "disconnect")]
        [InlineData(GameOverReason.HumansDisconnect, "impostors", "disconnect")]
        [InlineData(GameOverReason.HideAndSeekCrewmatesByTimer, "crewmates", "timer")]
        [InlineData(GameOverReason.HideAndSeekImpostorsByKills, "impostors", "kill")]
        public void TheResultNamesTheWinningSide(GameOverReason reason, string winner, string how)
        {
            var result = CaptureBridge.ResultFor(new GameOverEventArgs { GameOverReason = reason, PlayerInfos = [] });

            Assert.Equal((winner, how), (result.Winner, result.Reason));
        }

        [Fact]
        public void TheImpostorsAreNamedInOrder()
        {
            var result = CaptureBridge.ResultFor(new GameOverEventArgs
            {
                GameOverReason = GameOverReason.ImpostorByKill,
                PlayerInfos =
                [
                    new ImmutablePlayer { Name = "Paula", IsImpostor = false },
                    new ImmutablePlayer { Name = "mira", IsImpostor = true },
                    new ImmutablePlayer { Name = "Leon", IsImpostor = true },
                ],
            });

            Assert.Equal(new[] { "Leon", "mira" }, result.ImpostorNames);
        }

        /// <summary>A reason the game reports and AUVC does not know is no result rather than a wrong one.</summary>
        [Fact]
        public void AnUnknownReasonIsNoResult() =>
            Assert.Null(CaptureBridge.ResultFor(new GameOverEventArgs { GameOverReason = GameOverReason.Unknown }));
    }
}
