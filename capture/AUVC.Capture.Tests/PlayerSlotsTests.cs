using System;
using System.Collections.Generic;
using System.Linq;
using AmongUsCapture;
using Xunit;

namespace AUVC.Capture.Tests
{
    /// <summary>
    /// Walking the game's player list. A mistake here makes players vanish from
    /// capture, and to the bot a vanished player is one who left: unmanaged, and
    /// gone from the crewmate message.
    /// </summary>
    public class PlayerSlotsTests
    {
        private const int Step = 8;
        private static readonly IntPtr First = new(0x1000);

        /// <summary>A fake player list: one name per slot, noting every slot that was read.</summary>
        private static Func<IntPtr, string> Slots(List<IntPtr> read, params string[] names) =>
            address =>
            {
                read.Add(address);
                return names[(int)(((long)address - (long)First) / Step)];
            };

        private static bool Named(string name) => name.Length > 0;

        [Fact]
        public void APlayerStillLoadingDoesNotHideThePlayersAfterThem()
        {
            var read = new List<IntPtr>();

            var players = PlayerSlots.Read(4, First, Step, Slots(read, "Ava", "", "Ben", "Cleo"), Named);

            Assert.Equal(new[] { "Ava", "Ben", "Cleo" }, players);
        }

        [Fact]
        public void EverySlotIsReadExactlyOnce()
        {
            var read = new List<IntPtr>();

            PlayerSlots.Read(4, First, Step, Slots(read, "Ava", "", "", "Ben"), Named);

            Assert.Equal(new[] { First, First + Step, First + 2 * Step, First + 3 * Step }, read);
        }

        [Fact]
        public void AFullLobbyIsReadCompletely()
        {
            string[] lobby = ["Ava", "Ben", "", "Cleo", "Dario", "Elif", "", "Finn", "Greta", "Hugo",
                "Ines", "Jonas", "", "Kira", "Luca"];
            var read = new List<IntPtr>();

            var players = PlayerSlots.Read(lobby.Length, First, Step, Slots(read, lobby), Named);

            Assert.Equal(lobby.Where(Named), players);
            Assert.Equal(lobby.Length, read.Count);
        }

        [Theory]
        [InlineData(0)]
        [InlineData(-1)]
        public void AnEmptyListReadsNothing(int count)
        {
            var read = new List<IntPtr>();

            var players = PlayerSlots.Read(count, First, Step, Slots(read, "Ava"), Named);

            Assert.Empty(players);
            Assert.Empty(read);
        }

        [Fact]
        public void TwoPlayersWithTheSameNameDoNotFailTheWholeRead()
        {
            var players = new[] { (Name: "Ava", Id: 1), (Name: "Ava", Id: 2), (Name: "Ben", Id: 3) };

            var byName = PlayerSlots.ByName(players, player => player.Name);

            Assert.Equal(2, byName.Count);
            Assert.Equal(1, byName["Ava"].Id);
            Assert.Equal(3, byName["Ben"].Id);
        }
    }
}
