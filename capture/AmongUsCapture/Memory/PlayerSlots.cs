using System;
using System.Collections.Generic;

namespace AmongUsCapture
{
    /// <summary>
    /// Walks the game's list of players.
    /// </summary>
    /// <remarks>
    /// Kept apart from <see cref="GameMemReader"/> so it can be tested without a
    /// running game. What it gets wrong decides which players capture sees at all,
    /// and a player capture does not see is, for the bot, a player who left.
    /// </remarks>
    public static class PlayerSlots
    {
        /// <summary>
        /// Reads <paramref name="count"/> slots, stepping to the next slot after
        /// every one, skipped or not, and returns the players that are usable.
        /// </summary>
        public static List<T> Read<T>(int count, IntPtr first, int step, Func<IntPtr, T> read, Func<T, bool> usable)
        {
            var players = new List<T>(Math.Max(count, 0));
            var slot = first;
            for (var i = 0; i < count; i++, slot += step)
            {
                var player = read(slot);
                if (usable(player))
                {
                    players.Add(player);
                }
            }
            return players;
        }

        /// <summary>
        /// Players by name. A player whose name is already taken is left out,
        /// rather than failing the whole read and losing every other player with it.
        /// </summary>
        public static Dictionary<string, T> ByName<T>(IEnumerable<T> players, Func<T, string> name)
        {
            var byName = new Dictionary<string, T>(StringComparer.Ordinal);
            foreach (var player in players)
            {
                byName.TryAdd(name(player), player);
            }
            return byName;
        }
    }
}
