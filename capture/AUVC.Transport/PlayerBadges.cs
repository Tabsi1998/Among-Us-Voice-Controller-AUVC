namespace AUVC.Transport;

/// <summary>One thing a player tile says about the player.</summary>
public enum PlayerBadge
{
    NotLinked,
    Linked,
    InGhostChannel,
    Muted,
    Deafened,
}

/// <summary>
/// What a player tile in the main window says about who plays a crewmate and
/// what AUVC does to them.
/// </summary>
public static class PlayerBadges
{
    /// <summary>
    /// The crewmates the bot reports, by in-game name exactly as capture read it.
    /// Of two players with the same name the first counts, as on the crewmate board.
    /// </summary>
    public static IReadOnlyDictionary<string, LocalCrewmate> ByName(IEnumerable<LocalCrewmate> players)
    {
        var byName = new Dictionary<string, LocalCrewmate>(StringComparer.Ordinal);
        foreach (var player in players)
        {
            byName.TryAdd(player.Name, player);
        }
        return byName;
    }

    /// <summary>
    /// The badges for one crewmate, or none when the bot has not reported it: a
    /// bot on another computer cannot be asked, and a player who just joined shows
    /// up at the next poll.
    /// </summary>
    public static IReadOnlyList<PlayerBadge> For(LocalCrewmate? crewmate)
    {
        if (crewmate is null)
        {
            return [];
        }
        // AUVC manages only linked players, so an unlinked one has nothing held.
        if (string.IsNullOrEmpty(crewmate.UserId))
        {
            return [PlayerBadge.NotLinked];
        }

        var badges = new List<PlayerBadge> { PlayerBadge.Linked };
        if (crewmate.InGhostChannel)
        {
            badges.Add(PlayerBadge.InGhostChannel);
        }
        if (crewmate.Muted)
        {
            badges.Add(PlayerBadge.Muted);
        }
        if (crewmate.Deafened)
        {
            badges.Add(PlayerBadge.Deafened);
        }
        return badges;
    }
}
