using System.Linq;
using AUCapture_WPF.Properties;
using AUVC.Transport;

namespace AUCapture_WPF
{
    /// <summary>The line under a player's badges that says what they wear.</summary>
    internal static class PlayerCosmeticText
    {
        public static string For(PlayerOutfit outfit) =>
            string.Join(" · ", PlayerCosmetics.Worn(outfit).Select(item => (item.Kind switch
            {
                CosmeticKind.Hat => Resources.CosmeticHat,
                CosmeticKind.Skin => Resources.CosmeticSkin,
                CosmeticKind.Pet => Resources.CosmeticPet,
                _ => Resources.CosmeticVisor,
            }) + " " + item.Name));
    }
}
