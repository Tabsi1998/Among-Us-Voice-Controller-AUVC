using System.Collections.Generic;
using System.Linq;
using AUVC.Transport;

namespace AUCapture_WPF
{
    /// <summary>
    /// One crewmate in the lobby as the Players section shows it: who it is, and
    /// the members it can be linked to.
    /// </summary>
    public sealed class CrewmateRow
    {
        public string Player { get; init; } = "";
        public string Label { get; init; } = "";
        public IReadOnlyList<LocalMember> Choices { get; init; } = [];

        /// <summary>The member the bot has linked, or the "nobody" choice.</summary>
        public LocalMember Selected { get; set; }

        /// <summary>
        /// A row per crewmate. Every row offers "nobody" first, then every member,
        /// and starts on whoever the bot has linked.
        /// </summary>
        internal static IReadOnlyList<CrewmateRow> From(LocalCrewmates crewmates)
        {
            var nobody = new LocalMember { Id = "", Name = SetupText.Nobody };
            var choices = new List<LocalMember> { nobody };
            choices.AddRange(crewmates.Members);

            return crewmates.Players.Select(player => new CrewmateRow
            {
                Player = player.Name,
                Label = $"{player.Name} · {SetupText.ColorName(player.Color)}",
                Choices = choices,
                Selected = choices.FirstOrDefault(member => member.Id == player.UserId) ?? nobody,
            }).ToList();
        }
    }
}
