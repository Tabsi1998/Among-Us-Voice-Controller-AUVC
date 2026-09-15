using AUCapture_WPF.Properties;
using AUVC.Transport;

namespace AUCapture_WPF
{
    /// <summary>
    /// The words of the status line in the main window.
    /// </summary>
    /// <remarks>
    /// The icon repeats what the headline says, so the state does not rest on a
    /// colour or a symbol alone.
    /// </remarks>
    internal static class StatusText
    {
        public static (string Icon, string Headline, string NextStep) For(AppStatus status, bool runsBotOnThisPc) => status.Kind switch
        {
            AppStatusKind.NotSetUp => ("⚠", Resources.StatusNotSetUp, Resources.StatusNotSetUpNext),
            AppStatusKind.BotNotRunning => ("✖", Resources.StatusBotNotRunning, Resources.StatusBotNotRunningNext),
            AppStatusKind.WrongVersion => ("✖", Resources.StatusWrongVersion, Resources.StatusWrongVersionNext),
            AppStatusKind.Refused => ("✖", Resources.StatusRefused,
                runsBotOnThisPc ? Resources.StatusRefusedLocalNext : Resources.StatusRefusedRemoteNext),
            AppStatusKind.NotPaired => ("⚠", Resources.StatusNotPaired, Resources.StatusNotPairedNext),
            AppStatusKind.Connecting => ("…", Resources.StatusConnecting,
                runsBotOnThisPc ? "" : Resources.StatusConnectingRemoteNext),
            // The doctor's own words, which the bot on this PC wrote in the app's language.
            AppStatusKind.BotProblem => ("✖", string.Format(Resources.StatusBotProblem, status.Problem.Name),
                DoctorLine.Explain(status.Problem)),
            AppStatusKind.GameNotSupported => ("✖", Resources.StatusGameNotSupported, Resources.StatusGameNotSupportedNext),
            AppStatusKind.WaitingForGame => ("…", Resources.StatusWaitingForGame, Resources.StatusWaitingForGameNext),
            AppStatusKind.InMenu => ("…", Resources.StatusInMenu, Resources.StatusInMenuNext),
            AppStatusKind.SessionPaused => ("⏸", Resources.StatusPaused, Resources.StatusPausedNext),
            AppStatusKind.SessionStopped => ("⚠", Resources.StatusStopped, Resources.StatusStoppedNext),
            AppStatusKind.NobodyLinked => ("⚠", Resources.StatusNobodyLinked, Resources.StatusNobodyLinkedNext),
            _ => ("✔", Resources.StatusReady, status.Counted
                ? string.Format(Resources.StatusReadyCounted, status.LinkedPlayers, status.Players)
                : Resources.StatusReadyNext),
        };
    }
}
