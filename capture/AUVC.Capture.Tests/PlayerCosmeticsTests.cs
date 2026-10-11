using AUVC.Transport;
using Xunit;

namespace AUVC.Capture.Tests
{
    /// <summary>The words for what a player wears, from the game's ids (#155).</summary>
    public class PlayerCosmeticsTests
    {
        [Theory]
        [InlineData("hat_police", "Police")]
        [InlineData("skin_Mech", "Mech")]
        [InlineData("pet_ellie", "Ellie")]
        [InlineData("visor_pk01_Visor", "Pk01 Visor")]
        [InlineData("hat_pkHW01_Witch", "PkHW01 Witch")]
        public void AnIdBecomesAName(string id, string name) => Assert.Equal(name, PlayerCosmetics.Name(id));

        [Theory]
        [InlineData(null)]
        [InlineData("")]
        [InlineData("missing")]
        [InlineData("hat_NoHat")]
        [InlineData("pet_EmptyPet")]
        [InlineData("visor_EmptyVisor")]
        [InlineData("skin_None")]
        public void NothingWornIsNotNamed(string? id) => Assert.Null(PlayerCosmetics.Name(id));

        [Fact]
        public void WhatIsWornComesInAFixedOrderWithoutTheEmptyOnes()
        {
            var worn = PlayerCosmetics.Worn(new PlayerOutfit("hat_police", "", "pet_ellie", "visor_EmptyVisor"));

            Assert.Equal(new[] { (CosmeticKind.Hat, "Police"), (CosmeticKind.Pet, "Ellie") }, worn);
        }

        [Fact]
        public void ARecordingGapIsNoName() => Assert.Null(PlayerCosmetics.Name("hat_\u0001\u0002"));
    }
}
