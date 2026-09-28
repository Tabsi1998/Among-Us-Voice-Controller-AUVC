using System;
using System.Collections.Generic;
using System.Linq;
using AmongUsCapture;
using AUVC.Capture.Tests.Recorded;
using Xunit;

namespace AUVC.Capture.Tests
{
    /// <summary>
    /// Reading players out of a recording instead of a running game (#146).
    ///
    /// <para>Everything below the reader was untested: the walk over the player slots, the
    /// pointer chains of <see cref="PlayerInfo"/> and the strings behind them only ever ran
    /// against Among Us itself. A recording puts the same bytes in front of the same code,
    /// so the read error this lobby caused can be shown and stays shown.</para>
    /// </summary>
    public class RecordedMemoryTests
    {
        /// <summary>The expression the reader uses in GetPlayers, on a given memory.</summary>
        private static List<PlayerInfo> PlayersFrom(ProcessMemory memory)
        {
            var offsets = LobbyRecording.Offsets();
            var count = memory.Read<int>((IntPtr)LobbyRecording.AllPlayersPtr, offsets.PlayerCountOffsets);
            return PlayerSlots.Read(count, (IntPtr)LobbyRecording.FirstSlot(), offsets.AddPlayerPtr,
                address => new PlayerInfo(address, memory, offsets),
                player => !string.IsNullOrEmpty(player.GetPlayerName()) &&
                          Enum.IsDefined(typeof(PlayerColor), player.GetPlayerColor()));
        }

        [Fact]
        public void APlayerStillLoadingDoesNotTakeTheOthersWithThem()
        {
            var players = PlayersFrom(LobbyRecording.Build());

            Assert.Equal(new[] { "Paula", "Leon", "Mira" }, players.Select(player => player.GetPlayerName()));
        }

        /// <summary>
        /// The error this recording is here for: stepping only past the players that were
        /// usable read the loading slot again for every player after it.
        /// </summary>
        [Fact]
        public void TheOldWayOfSteppingLosesEverybodyAfterTheLoadingPlayer()
        {
            var memory = LobbyRecording.Build();
            var offsets = LobbyRecording.Offsets();
            var count = memory.Read<int>((IntPtr)LobbyRecording.AllPlayersPtr, offsets.PlayerCountOffsets);

            var found = new List<string>();
            var slot = (IntPtr)LobbyRecording.FirstSlot();
            for (var index = 0; index < count; index++)
            {
                var player = new PlayerInfo(slot, memory, offsets);
                if (string.IsNullOrEmpty(player.GetPlayerName()))
                {
                    continue;   // the old code stepped only when it kept a player
                }
                found.Add(player.GetPlayerName());
                slot += offsets.AddPlayerPtr;
            }

            Assert.Equal(new[] { "Paula" }, found);
        }

        [Fact]
        public void EveryPlayerIsReadWithNameColourAndWhetherTheyAreDead()
        {
            var players = PlayersFrom(LobbyRecording.Build());

            var paula = players[0];
            Assert.Equal((byte)1, paula.PlayerId);
            Assert.Equal(PlayerColor.Red, paula.GetPlayerColor());
            Assert.False(paula.GetIsDead());
            Assert.False(paula.GetIsImposter());

            var leon = players[1];
            Assert.Equal("Leon", leon.GetPlayerName());
            Assert.True(leon.GetIsDead());

            var mira = players[2];
            Assert.True(mira.GetIsImposter(), "team 1 is the impostor team");
            Assert.False(mira.GetIsDisconnected());
        }

        /// <summary>
        /// A recording with a hole says so. Answering zeros would look like a player who
        /// left, which is exactly the kind of error a recording is meant to catch.
        /// </summary>
        [Fact]
        public void AReadTheRecordingHasNothingForIsNotASilentZero()
        {
            var memory = LobbyRecording.Build();

            var gap = Assert.Throws<RecordingGap>(() => memory.Read((IntPtr)0x7FFFFFFF, 4));
            Assert.Contains("0x7FFFFFFF", gap.Message);

            memory.ZeroFill = true;
            Assert.Equal(new byte[4], memory.Read((IntPtr)0x7FFFFFFF, 4));
        }

        /// <summary>
        /// A recording is a file, so a capture from a real game can replace this one without
        /// touching the tests.
        /// </summary>
        [Fact]
        public void ARecordingSurvivesBeingWrittenAndReadBack()
        {
            var json = LobbyRecording.Build().ToJson();

            var players = PlayersFrom(RecordedMemory.FromJson(json));

            Assert.Equal(new[] { "Paula", "Leon", "Mira" }, players.Select(player => player.GetPlayerName()));
            Assert.Contains("\"is_64_bit\": false", json);
        }
    }
}
