using AUVC.Transport;
using Xunit;

namespace AUVC.Capture.Tests
{
    /// <summary>
    /// What a player tile says: whether somebody plays the crewmate, and what AUVC
    /// has done to them. It shows what AUVC holds, so a mute that failed is not
    /// shown as done.
    /// </summary>
    public class PlayerBadgesTests
    {
        [Fact]
        public void ACrewmateTheBotDidNotReportShowsNothing() =>
            Assert.Empty(PlayerBadges.For(null));

        [Fact]
        public void AnUnlinkedCrewmateSaysSo() =>
            Assert.Equal([PlayerBadge.NotLinked], PlayerBadges.For(new LocalCrewmate { Name = "Alice" }));

        [Fact]
        public void ALinkedCrewmateAUVCLeftAloneIsOnlyLinked() =>
            Assert.Equal([PlayerBadge.Linked], PlayerBadges.For(new LocalCrewmate { Name = "Alice", UserId = "u1" }));

        [Fact]
        public void EverythingAUVCHoldsIsShownInOneOrder()
        {
            var held = new LocalCrewmate
            {
                Name = "Alice",
                UserId = "u1",
                Muted = true,
                Deafened = true,
                InGhostChannel = true,
            };

            Assert.Equal(
                [PlayerBadge.Linked, PlayerBadge.InGhostChannel, PlayerBadge.Muted, PlayerBadge.Deafened],
                PlayerBadges.For(held));
        }

        [Fact]
        public void ADeadPlayerInTheGhostChannelIsNotMuted()
        {
            var ghost = new LocalCrewmate { Name = "Alice", UserId = "u1", InGhostChannel = true };

            Assert.Equal([PlayerBadge.Linked, PlayerBadge.InGhostChannel], PlayerBadges.For(ghost));
        }

        // Names are matched exactly as capture read them, and of two players with the
        // same name the first counts, as on the crewmate board.
        [Fact]
        public void CrewmatesAreFoundByTheirExactName()
        {
            var byName = PlayerBadges.ByName([
                new LocalCrewmate { Name = "Alice", UserId = "first" },
                new LocalCrewmate { Name = "Alice", UserId = "second" },
                new LocalCrewmate { Name = "alice", UserId = "other" },
            ]);

            Assert.Equal("first", byName["Alice"].UserId);
            Assert.Equal("other", byName["alice"].UserId);
            Assert.Equal(2, byName.Count);
        }
    }
}
