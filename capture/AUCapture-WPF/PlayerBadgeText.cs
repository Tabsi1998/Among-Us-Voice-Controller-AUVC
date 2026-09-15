using System.Collections.Generic;
using System.Linq;
using AUCapture_WPF.Properties;
using AUVC.Transport;

namespace AUCapture_WPF
{
    /// <summary>
    /// The words under a player's name in the main window.
    /// </summary>
    internal static class PlayerBadgeText
    {
        public static string For(IReadOnlyList<PlayerBadge> badges) =>
            string.Join(" · ", badges.Select(badge => badge switch
            {
                PlayerBadge.NotLinked => Resources.BadgeNotLinked,
                PlayerBadge.Linked => Resources.BadgeLinked,
                PlayerBadge.InGhostChannel => Resources.BadgeInGhostChannel,
                PlayerBadge.Muted => Resources.BadgeMuted,
                _ => Resources.BadgeDeafened,
            }));
    }
}
