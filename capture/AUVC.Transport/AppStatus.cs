using AUVC.Protocol;

namespace AUVC.Transport;

/// <summary>Where the game is, as far as the status line is concerned.</summary>
public enum GameView
{
    /// <summary>Among Us is not running, or capture has not found it yet.</summary>
    NotRunning,

    /// <summary>In the menus, with no lobby yet.</summary>
    Menu,

    /// <summary>In a lobby, or between rounds.</summary>
    Lobby,

    /// <summary>A round is being played: tasks or a meeting.</summary>
    Round,
}

/// <summary>What the status line says, in the order it is checked.</summary>
public enum AppStatusKind
{
    NotSetUp,
    BotNotRunning,
    WrongVersion,
    Refused,
    NotPaired,
    Connecting,
    WaitingForGame,
    InMenu,
    SessionPaused,
    SessionStopped,
    NobodyLinked,
    Ready,
}

/// <summary>Everything the status line is decided from.</summary>
public sealed record AppSituation
{
    public bool RunsBotOnThisPc { get; init; }
    public bool Paired { get; init; }
    public bool LocalBotRunning { get; init; }
    public LinkStatus Link { get; init; } = new(LinkState.Stopped);
    public GameView Game { get; init; }

    /// <summary>
    /// The session as the bot on this PC reports it (<see cref="LocalGuild.Session"/>),
    /// or null when the app cannot ask, as with a bot on another computer.
    /// </summary>
    public string? Session { get; init; }

    /// <summary>The players in the lobby and how many of them are linked, or null when unknown.</summary>
    public int? Players { get; init; }

    public int? LinkedPlayers { get; init; }
}

/// <summary>
/// The one thing the main window tells a person right now.
/// </summary>
/// <remarks>
/// Only the most urgent problem is shown, because it is the one to fix first:
/// with no bot there is no point in asking for a lobby. Somebody who follows the
/// line one step at a time ends up in a round that AUVC manages.
/// </remarks>
public sealed record AppStatus(AppStatusKind Kind, int LinkedPlayers = 0, int Players = 0, bool Counted = false)
{
    public static AppStatus For(AppSituation situation)
    {
        if (!situation.RunsBotOnThisPc && !situation.Paired)
        {
            return new(AppStatusKind.NotSetUp);
        }
        if (situation.RunsBotOnThisPc && !situation.LocalBotRunning)
        {
            return new(AppStatusKind.BotNotRunning);
        }

        switch (situation.Link.State)
        {
            case LinkState.Refused when situation.Link.Code == ProtocolContract.CodeIncompatibleProtocol:
                return new(AppStatusKind.WrongVersion);
            case LinkState.Refused:
                return new(AppStatusKind.Refused);
            case LinkState.NotPaired:
                return new(AppStatusKind.NotPaired);
            case not LinkState.Connected:
                return new(AppStatusKind.Connecting);
        }

        switch (situation.Game)
        {
            case GameView.NotRunning:
                return new(AppStatusKind.WaitingForGame);
            case GameView.Menu:
                return new(AppStatusKind.InMenu);
        }

        if (situation.Session == LocalGuild.SessionPaused)
        {
            return new(AppStatusKind.SessionPaused);
        }
        // In a lobby a stopped session is normal: automatic start waits for the round.
        if (situation.Session == LocalGuild.SessionStopped && situation.Game == GameView.Round)
        {
            return new(AppStatusKind.SessionStopped);
        }

        if (situation.Players is { } players && situation.LinkedPlayers is { } linked)
        {
            // Some unlinked players are no problem: AUVC manages only linked ones, and
            // a public lobby has strangers in it. Nobody linked at all is.
            return players > 0 && linked == 0
                ? new(AppStatusKind.NobodyLinked, 0, players, true)
                : new(AppStatusKind.Ready, linked, players, true);
        }
        return new(AppStatusKind.Ready);
    }
}
