using System;

namespace AmongUsCapture
{
    /// <summary>
    /// Decides when the reader tells about the lobby it is in.
    /// </summary>
    /// <remarks>
    /// Kept apart from <see cref="GameMemReader"/> so it can be tested without a
    /// running game. The code and the map can be read as soon as a lobby is
    /// entered, but the host can still pick another map in it, and the crewmate
    /// message shows that map. So the lobby is read on every pass inside it and
    /// told again whenever it changed.
    /// </remarks>
    public sealed class LobbyWatch
    {
        private LobbyEventArgs told;
        private bool owed;

        /// <summary>
        /// Called once per pass of the reader. Returns the lobby to tell about, or
        /// null when there is nothing new.
        /// </summary>
        /// <param name="oldState">The state of the previous pass.</param>
        /// <param name="state">The state of this pass.</param>
        /// <param name="read">
        /// Reads the lobby from the game; null while the game has no usable code yet.
        /// </param>
        public LobbyEventArgs Next(GameState oldState, GameState state, Func<LobbyEventArgs> read)
        {
            // Every lobby entered is told once, even the one the last round started from.
            if (state == GameState.LOBBY && oldState != GameState.LOBBY)
            {
                owed = true;
            }

            // Nothing is read during a round, unless the code was still blank when the
            // lobby was entered and the round started before it could be read.
            if (state != GameState.LOBBY && !owed)
            {
                return null;
            }

            var lobby = read();
            if (lobby is null)
            {
                return null;
            }
            if (!owed && Same(told, lobby))
            {
                return null;
            }

            owed = false;
            told = lobby;
            return lobby;
        }

        private static bool Same(LobbyEventArgs before, LobbyEventArgs now) =>
            before is not null
            && before.LobbyCode == now.LobbyCode
            && before.Map == now.Map
            && before.Region == now.Region;
    }
}
