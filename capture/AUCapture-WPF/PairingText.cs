using System;
using AUCapture_WPF.Properties;
using AUVC.Transport;

namespace AUCapture_WPF
{
    /// <summary>
    /// What the app tells the person who paired, in the app's language.
    /// </summary>
    internal static class PairingText
    {
        public static string Paired(PairingOutcome outcome) =>
            outcome.SendsCredentialInClear
                ? Resources.PairedMessage + Environment.NewLine + Environment.NewLine + Resources.PairedInClearWarning
                : Resources.PairedMessage;

        /// <summary>
        /// Why pairing did not work. Each reason has its own next step; an answer the
        /// app does not know keeps the bot's own words.
        /// </summary>
        public static string Refused(PairingRefusedException refused, string typedAddress) => refused.Problem switch
        {
            PairingProblem.EmptyCode => Resources.PairingEmptyCode,
            PairingProblem.InvalidAddress => string.Format(Resources.PairingInvalidAddress, BotAddress.Default),
            PairingProblem.Unreachable => string.Format(Resources.PairingUnreachable, typedAddress),
            PairingProblem.Expired => Resources.PairingExpired,
            PairingProblem.Rejected => Resources.PairingRejected,
            _ => refused.Message,
        };
    }
}
