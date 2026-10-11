namespace AUVC.Transport;

/// <summary>What a player wears, as Among Us names it: ids such as "hat_police".</summary>
public sealed record PlayerOutfit(string? Hat, string? Skin, string? Pet, string? Visor);

public enum CosmeticKind
{
    Hat,
    Skin,
    Pet,
    Visor,
}

/// <summary>
/// Turns the game's cosmetic ids into the words under a player in the main window
/// (#155). The game's pictures for every hat are not part of AUVC, so it names them.
/// </summary>
public static class PlayerCosmetics
{
    private static readonly string[] Prefixes = ["hat_", "skin_", "pet_", "visor_"];

    // What the game uses for "nothing worn".
    private static readonly HashSet<string> Nothing = new(StringComparer.OrdinalIgnoreCase)
    {
        "", "0", "none", "missing", "empty", "nohat", "noskin", "nopet", "novisor", "emptypet", "emptyvisor",
    };

    /// <summary>What the player wears, in a fixed order, without what is empty.</summary>
    public static IReadOnlyList<(CosmeticKind Kind, string Name)> Worn(PlayerOutfit outfit)
    {
        var worn = new List<(CosmeticKind, string)>();
        foreach (var (kind, id) in new[]
                 {
                     (CosmeticKind.Hat, outfit.Hat), (CosmeticKind.Skin, outfit.Skin),
                     (CosmeticKind.Pet, outfit.Pet), (CosmeticKind.Visor, outfit.Visor),
                 })
        {
            if (Name(id) is { } name)
            {
                worn.Add((kind, name));
            }
        }
        return worn;
    }

    /// <summary>"hat_police" becomes "Police"; an empty or "nothing" id becomes null.</summary>
    public static string? Name(string? id)
    {
        var name = (id ?? "").Trim();
        foreach (var prefix in Prefixes)
        {
            if (name.StartsWith(prefix, StringComparison.OrdinalIgnoreCase))
            {
                name = name[prefix.Length..];
                break;
            }
        }
        if (Nothing.Contains(name) || name.Any(char.IsControl))
        {
            return null;
        }
        name = name.Replace('_', ' ').Trim();
        return char.ToUpperInvariant(name[0]) + name[1..];
    }
}
