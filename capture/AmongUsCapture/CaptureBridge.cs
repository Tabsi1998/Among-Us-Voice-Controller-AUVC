using System;
using System.Linq;
using AmongUsCapture.Memory.Structs;
using AUVC.Protocol;
using AUVC.Transport;

namespace AmongUsCapture
{
    /// <summary>
    /// Passes what the memory reader sees on to the AUVC bot.
    /// </summary>
    /// <remarks>
    /// The reader's events describe the game as the game models it; the protocol
    /// describes a round the way the bot needs it. This is the one place the two
    /// meet, so the whole mapping can be read, and tested, here.
    /// </remarks>
    public static class CaptureBridge
    {
        public static void Attach(GameMemReader reader, IRoundReporter report)
        {
            reader.GameStateChanged += (_, e) => Forward(report, e);
            reader.PlayerChanged += (_, e) => Forward(report, e);
            reader.GameOver += (_, e) => report.ReportGameEnded(ResultFor(e));
            reader.JoinedLobby += (_, e) => Forward(report, e);
        }

        /// <summary>
        /// Who won the round and who the impostors were, for the post in Discord. The
        /// game's reasons name the winning side; the hide-and-seek ones too.
        /// </summary>
        public static RoundResult ResultFor(GameOverEventArgs e)
        {
            var (winner, reason) = e.GameOverReason switch
            {
                GameOverReason.HumansByVote => (RoundResult.Crewmates, RoundResult.ByVote),
                GameOverReason.HumansByTask => (RoundResult.Crewmates, RoundResult.ByTasks),
                GameOverReason.ImpostorByVote => (RoundResult.Impostors, RoundResult.ByVote),
                GameOverReason.ImpostorByKill => (RoundResult.Impostors, RoundResult.ByKill),
                GameOverReason.ImpostorBySabotage => (RoundResult.Impostors, RoundResult.BySabotage),
                GameOverReason.ImpostorDisconnect => (RoundResult.Crewmates, RoundResult.ByDisconnect),
                GameOverReason.HumansDisconnect => (RoundResult.Impostors, RoundResult.ByDisconnect),
                GameOverReason.HideAndSeekCrewmatesByTimer => (RoundResult.Crewmates, RoundResult.ByTimer),
                GameOverReason.HideAndSeekImpostorsByKills => (RoundResult.Impostors, RoundResult.ByKill),
                _ => (null, null),
            };
            if (winner is null) return null;
            var impostors = (e.PlayerInfos ?? [])
                .Where(player => player.IsImpostor && !string.IsNullOrEmpty(player.Name))
                .Select(player => player.Name)
                .OrderBy(name => name, StringComparer.CurrentCultureIgnoreCase)
                .Take(RoundResult.MaxImpostors)
                .ToList();
            return new RoundResult { Winner = winner, Reason = reason, ImpostorNames = impostors };
        }

        /// <summary>
        /// The protocol name of a map, or empty for a map the protocol has no name for.
        /// </summary>
        public static string MapFor(PlayMap map) => map switch
        {
            PlayMap.Skeld => ProtocolContract.MapTheSkeld,
            PlayMap.Mira => ProtocolContract.MapMiraHQ,
            PlayMap.Polus => ProtocolContract.MapPolus,
            PlayMap.dlekS => ProtocolContract.MapDleks,
            PlayMap.Airship => ProtocolContract.MapAirship,
            PlayMap.Fungle => ProtocolContract.MapFungle,
            _ => "",
        };

        public static void Forward(IRoundReporter report, LobbyEventArgs e)
        {
            // The reader raises this only with a code the game uses: four or six
            // capital letters, or six asterisks when the host hides it.
            report.ReportLobby(new Lobby { Code = e.LobbyCode ?? "", Map = MapFor(e.Map) });
        }

        /// <summary>
        /// The protocol phase for a game state, or null for a state the protocol
        /// has no word for.
        /// </summary>
        public static string PhaseFor(GameState state) => state switch
        {
            GameState.LOBBY => ProtocolContract.PhaseLobby,
            GameState.TASKS => ProtocolContract.PhaseTasks,
            GameState.DISCUSSION => ProtocolContract.PhaseDiscussion,
            GameState.MENU => ProtocolContract.PhaseMenu,
            GameState.ENDED => ProtocolContract.PhaseEnded,
            _ => null,
        };

        public static void Forward(IRoundReporter report, GameStateChangedEventArgs e)
        {
            // UNKNOWN is the reader failing to tell, not a state the game is in.
            // Reporting it would silence or release players on a guess.
            var phase = PhaseFor(e.NewState);
            if (phase is not null)
            {
                report.ReportPhase(phase);
            }
        }

        public static void Forward(IRoundReporter report, PlayerChangedEventArgs e)
        {
            var player = new Player
            {
                Name = e.Name ?? "",
                Color = (int)e.Color,
                Dead = e.IsDead,
                Disconnected = e.Disconnected,
            };

            switch (e.Action)
            {
                case PlayerAction.Joined:
                    report.ReportPlayerJoined(player);
                    break;

                case PlayerAction.Left:
                    report.ReportPlayerLeft(player);
                    break;

                // An exile is a death. The reader reports it before the game
                // flags the player as dead, so the event alone would say alive.
                case PlayerAction.Died:
                case PlayerAction.Exiled:
                    report.ReportPlayerDied(player);
                    break;

                case PlayerAction.ChangedColor:
                case PlayerAction.Disconnected:
                case PlayerAction.ForceUpdated:
                    report.ReportPlayerChanged(player);
                    break;
            }
        }
    }
}
