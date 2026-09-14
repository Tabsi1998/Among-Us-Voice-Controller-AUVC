using AUVC.Protocol;

namespace AUVC.Transport;

/// <summary>
/// What capture knows about the round: the phase and every player in it.
/// </summary>
/// <remarks>
/// Capture keeps this whether or not the bot is connected. A reconnect has to
/// open with a complete snapshot, and the memory reader only reports changes, so
/// without a running record there would be nothing to put in one.
///
/// Not safe for concurrent use; <see cref="CaptureLink"/> serializes access.
/// </remarks>
public sealed class GameRound
{
    private readonly Dictionary<string, Player> _players = new(StringComparer.Ordinal);

    /// <summary>
    /// The current phase. Capture starts in the menu, before any game is joined.
    /// </summary>
    public string Phase { get; private set; } = ProtocolContract.PhaseMenu;

    /// <summary>Every player in the round, ordered by name.</summary>
    public IReadOnlyList<Player> Players =>
        _players.Values.OrderBy(player => player.Name, StringComparer.Ordinal).ToList();

    /// <summary>Records a phase and reports whether it changed.</summary>
    public bool SetPhase(string phase)
    {
        if (!ProtocolContract.IsKnownPhase(phase))
        {
            throw new ArgumentException($"unknown phase '{phase}'", nameof(phase));
        }
        if (Phase == phase)
        {
            return false;
        }

        Phase = phase;

        // Nobody is dead in a lobby or the menu. The game has no event for a
        // player coming back to life, so without this a reconnect in the next
        // lobby would send a snapshot still carrying the last round's deaths.
        if (phase is ProtocolContract.PhaseLobby or ProtocolContract.PhaseMenu)
        {
            Revive();
        }
        return true;
    }

    public bool Join(Player player) => Upsert(player);

    public bool Change(Player player) => Upsert(player);

    /// <summary>
    /// Records a death. The player is marked dead whatever the event said: an
    /// exile is reported before the game flags the player as dead.
    /// </summary>
    public bool Die(Player player) => Upsert(player with { Dead = true });

    public bool Leave(Player player)
    {
        if (string.IsNullOrEmpty(player.Name))
        {
            return false;
        }

        _players.Remove(player.Name);
        return true;
    }

    /// <summary>
    /// Records the end of a round. The players stay: a round ending is not
    /// everybody leaving, and the same lobby carries on.
    /// </summary>
    public bool End()
    {
        Revive();
        return true;
    }

    // A player without a name cannot be linked to anyone and the bot refuses
    // an event carrying one, so it is not recorded at all.
    private bool Upsert(Player player)
    {
        if (string.IsNullOrEmpty(player.Name))
        {
            return false;
        }

        _players[player.Name] = player;
        return true;
    }

    private void Revive()
    {
        foreach (var name in _players.Keys.ToList())
        {
            _players[name] = _players[name] with { Dead = false };
        }
    }
}
