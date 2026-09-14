using System.Globalization;
using AUVC.Transport;
using Gu.Localization;

namespace AUCapture_WPF
{
    /// <summary>
    /// The words of the setup, in German or English.
    /// </summary>
    /// <remarks>
    /// Kept in code rather than in the resource files, because the setup is
    /// read once, carefully, by somebody who has never seen AUVC, and every step
    /// has to read as one piece. German when the app runs in German, English
    /// otherwise.
    /// </remarks>
    internal static class SetupText
    {
        private static bool German =>
            (Translator.Culture ?? CultureInfo.CurrentUICulture).TwoLetterISOLanguageName == "de";

        private static string T(string german, string english) => German ? german : english;

        public static string WindowTitle => T("AUVC einrichten", "Set up AUVC");
        public static string SetupButton => T("Einrichten", "Set up");
        public static string SetupButtonTooltip => T("Den Discord-Bot auf diesem PC einrichten", "Set up the Discord bot on this PC");
        public static string Back => T("Zurück", "Back");
        public static string Next => T("Weiter", "Next");
        public static string Cancel => T("Abbrechen", "Cancel");
        public static string Finish => T("Fertig", "Finish");
        public static string Refresh => T("Aktualisieren", "Refresh");

        // Step 1: the token.
        public static string TokenTitle => T("1 von 4 · Deinen Discord-Bot verbinden", "1 of 4 · Connect your Discord bot");

        public static string TokenIntro => T(
            "AUVC braucht einen eigenen Discord-Bot. Das kostet nichts und dauert zwei Minuten:\n\n" +
            "1. Klicke auf „Developer Portal öffnen“ und melde dich mit deinem Discord-Konto an.\n" +
            "2. Klicke auf „New Application“, gib einen Namen ein (zum Beispiel AUVC) und bestätige.\n" +
            "3. Öffne links „Bot“, klicke auf „Reset Token“ und dann auf „Copy“.\n" +
            "4. Füge den Token unten ein und klicke auf „Token prüfen“.\n\n" +
            "Eine Redirect- oder Callback-URL brauchst du nicht.\n\n" +
            "Der Token ist wie ein Passwort. Gib ihn niemandem weiter. AUVC speichert ihn verschlüsselt und nur auf diesem PC.",
            "AUVC needs a Discord bot of its own. It is free and takes two minutes:\n\n" +
            "1. Click “Open the Developer Portal” and sign in with your Discord account.\n" +
            "2. Click “New Application”, enter a name (for example AUVC) and confirm.\n" +
            "3. Open “Bot” on the left, click “Reset Token”, then “Copy”.\n" +
            "4. Paste the token below and click “Check token”.\n\n" +
            "You do not need a redirect or callback URL.\n\n" +
            "The token is like a password. Do not share it. AUVC keeps it encrypted, on this PC only.");

        public static string OpenPortal => T("Developer Portal öffnen", "Open the Developer Portal");
        public static string TokenPlaceholder => T("Bot-Token hier einfügen", "Paste the bot token here");
        public static string CheckToken => T("Token prüfen", "Check token");
        public static string Checking => T("Wird geprüft …", "Checking …");
        public static string TokenStored => T("Ein Token ist bereits gespeichert. Du kannst direkt auf „Weiter“ klicken oder einen neuen einfügen.",
            "A token is already stored. You can click “Next” straight away, or paste a new one.");

        public static string TokenOk(string botName) =>
            T($"✔ Token passt: Bot „{botName}“ gefunden.", $"✔ The token works: found the bot “{botName}”.");

        public static string CodeGrantWarning => T(
            "⚠ Im Developer Portal ist unter „Bot“ die Option „Requires OAuth2 Code Grant“ eingeschaltet. Schalte sie aus, sonst klappt die Einladung nicht.",
            "⚠ “Requires OAuth2 Code Grant” is switched on under “Bot” in the Developer Portal. Switch it off, or the invite will not work.");

        public static string TokenProblemText(TokenProblem problem) => problem switch
        {
            TokenProblem.Empty => T("Füge zuerst den Token ein.", "Paste the token first."),
            TokenProblem.Rejected => T(
                "✖ Discord akzeptiert diesen Token nicht. Kopiere ihn im Developer Portal unter „Bot“ noch einmal, notfalls nach „Reset Token“.",
                "✖ Discord does not accept this token. Copy it again under “Bot” in the Developer Portal, after “Reset Token” if needed."),
            TokenProblem.Unreachable => T("✖ Discord ist nicht erreichbar. Prüfe die Internetverbindung.",
                "✖ Discord cannot be reached. Check the internet connection."),
            _ => T("✖ Discord hat unerwartet geantwortet. Versuche es gleich noch einmal.",
                "✖ Discord answered unexpectedly. Try again in a moment."),
        };

        // Step 2: invite and server.
        public static string ServerTitle => T("2 von 4 · Bot einladen und Server wählen", "2 of 4 · Invite the bot and choose a server");

        public static string ServerIntro => T(
            "Klicke auf „Bot einladen“. Im Browser wählst du deinen Discord-Server und klickst auf „Autorisieren“.\n\n" +
            "Danach erscheint der Server hier in der Liste. Wähle ihn aus und klicke auf „Weiter“.",
            "Click “Invite the bot”. In the browser, choose your Discord server and click “Authorize”.\n\n" +
            "The server then appears in the list below. Select it and click “Next”.");

        public static string Invite => T("Bot einladen", "Invite the bot");
        public static string StartingBot => T("Der Bot wird gestartet …", "Starting the bot …");
        public static string BotOnline(string name) => T($"✔ Der Bot „{name}“ ist online.", $"✔ The bot “{name}” is online.");
        public static string NoServersYet => T("Der Bot ist noch in keinem Server. Lade ihn ein – die Liste aktualisiert sich von selbst.",
            "The bot is not in any server yet. Invite it — the list refreshes by itself.");

        public static string BotStartFailed(BotHostException error) => error.Problem switch
        {
            BotHostProblem.Missing => T(
                "✖ Das Bot-Programm fehlt (bot\\auvc.exe). Installiere AUVC neu oder entpacke die portable Version vollständig.",
                "✖ The bot program is missing (bot\\auvc.exe). Reinstall AUVC, or unpack the portable version completely."),
            BotHostProblem.Exited => T(
                "✖ Der Bot hat sich gleich wieder beendet. Meist ist der Token falsch. Gehe zurück und prüfe ihn.",
                "✖ The bot stopped right after starting. The token is usually the cause. Go back and check it."),
            BotHostProblem.NotReady => T(
                "✖ Der Bot hat sich nicht rechtzeitig mit Discord verbunden. Prüfe die Internetverbindung und versuche es noch einmal.",
                "✖ The bot did not connect to Discord in time. Check the internet connection and try again."),
            _ => error.Message,
        };

        // Step 3: channels.
        public static string ChannelTitle => T("3 von 4 · Kanäle wählen", "3 of 4 · Choose the channels");

        public static string ChannelIntro => T(
            "Hauptkanal: Hier sind alle, die noch leben.\n" +
            "Geisterkanal: Hierhin kommen Tote, sobald ihr Tod im Meeting bekannt wird.\n" +
            "Textkanal: Optional. Hier schreibt AUVC Hinweise, zum Beispiel wenn das Spiel nicht mehr erkannt wird.",
            "Main channel: everyone still alive.\n" +
            "Ghost channel: the dead go here once a meeting has announced their death.\n" +
            "Text channel: optional. AUVC posts notices here, for example when the game is no longer detected.");

        public static string MainChannel => T("Hauptkanal (Sprachkanal)", "Main channel (voice)");
        public static string GhostChannel => T("Geisterkanal (Sprachkanal)", "Ghost channel (voice)");
        public static string ControlChannel => T("Textkanal für die Crewmate-Auswahl und Hinweise (empfohlen)", "Text channel for choosing crewmates and for notices (recommended)");
        public static string NoControlChannel => T("(keiner)", "(none)");
        public static string AutoStart => T("AUVC automatisch starten, sobald ein Spiel erkannt wird", "Start AUVC automatically when a game is detected");
        public static string Saving => T("Wird gespeichert …", "Saving …");
        public static string NeedTwoVoiceChannels => T("✖ Wähle zwei verschiedene Sprachkanäle.", "✖ Choose two different voice channels.");
        public static string TooFewVoiceChannels => T(
            "Der Server hat noch keine zwei Sprachkanäle. Lege sie in Discord an und klicke auf „Aktualisieren“.",
            "The server does not have two voice channels yet. Create them in Discord and click “Refresh”.");

        // Step 4: done.
        public static string DoneTitle => T("4 von 4 · Fertig", "4 of 4 · Done");

        public static string DoneIntro => T(
            "AUVC ist eingerichtet. Der Bot startet ab jetzt mit diesem Programm und geht offline, wenn du es schließt.\n\n" +
            "Eins ist noch zu tun: Jeder Spieler wählt im Textkanal einmal seine Figur aus, oder tippt in Discord /au link.",
            "AUVC is set up. From now on the bot starts with this program and goes offline when you close it.\n\n" +
            "One thing is left: every player picks their crewmate once in the text channel, or types /au link in Discord.");

        public static string ChecksHeading => T("Prüfung", "Checks");
        public static string BotNotRunning => T("✖ Der Bot läuft gerade nicht. Gehe zurück zu Schritt 2.", "✖ The bot is not running. Go back to step 2.");

        // Settings, once the bot is set up.
        public static string SettingsWindowTitle => T("AUVC · Bot-Einstellungen", "AUVC · Bot settings");
        public static string SettingsButton => T("Bot", "Bot");
        public static string SettingsButtonTooltip => T("Token, Server, Kanäle und Status des Bots ändern", "Change the bot's token, server, channels and status");
        public static string SectionToken => T("Token", "Token");
        public static string SectionServer => T("Server", "Server");
        public static string SectionChannels => T("Kanäle", "Channels");
        public static string SectionStatus => T("Status", "Status");

        public static string SettingsTitle(int section) => section switch
        {
            0 => T("Bot-Token ändern", "Change the bot token"),
            1 => T("Server wechseln", "Change the server"),
            2 => T("Kanäle ändern", "Change the channels"),
            _ => T("Status des Bots", "Bot status"),
        };

        public static string UseServer => T("Diesen Server verwenden", "Use this server");

        public static string ServerSaved(string name) =>
            T($"✔ AUVC verwendet jetzt den Server „{name}“. Wähle als Nächstes unter „Kanäle“ die Kanäle.",
                $"✔ AUVC now uses the server “{name}”. Choose its channels next, under “Channels”.");

        public static string Save => T("Speichern", "Save");
        public static string Saved => T("✔ Gespeichert.", "✔ Saved.");
        public static string ChooseServerFirst => T("Wähle zuerst unter „Server“ einen Server.", "Choose a server under “Server” first.");
        public static string RestartBot => T("Bot neu starten", "Restart the bot");
        public static string Restarting => T("Der Bot wird neu gestartet …", "Restarting the bot …");
        public static string DisableBot => T("Bot auf diesem PC ausschalten", "Stop running the bot on this PC");

        public static string DisableBotQuestion => T(
            "Der Bot startet dann nicht mehr mit AUVC. Token und Einstellungen bleiben gespeichert – über „Einrichten“ kannst du ihn jederzeit wieder einschalten.",
            "The bot will no longer start with AUVC. The token and the settings stay stored — you can switch it back on at any time with “Set up”.");

        public static string BotDisabled => T(
            "Der Bot läuft nicht mehr auf diesem PC. Über „Einrichten“ im Hauptfenster schaltest du ihn wieder ein.",
            "The bot no longer runs on this PC. Use “Set up” in the main window to switch it back on.");

        public static string StatusIntro => T(
            "Hier siehst du, ob der Bot läuft und was er über deinen Server meldet. Die anderen Bereiche oben ändern Token, Server und Kanäle.",
            "This shows whether the bot is running and what it reports about your server. The other sections above change the token, the server and the channels.");

        public static string BotRunning => T("✔ Der Bot läuft auf diesem PC.", "✔ The bot is running on this PC.");
        public static string BotStopped => T("Der Bot läuft gerade nicht. „Bot neu starten“ startet ihn.", "The bot is not running. “Restart the bot” starts it.");

        public static string TokenChangedRestart => T(
            "Der Bot wird mit dem neuen Token neu gestartet. Ist es ein anderer Bot, lade ihn unter „Server“ ein und wähle den Server dort neu.",
            "The bot restarts with the new token. If it is a different bot, invite it under “Server” and choose the server there again.");

        // Main window.
        public static string BotFailedTitle => T("Der Bot auf diesem PC konnte nicht starten", "The bot on this PC could not start");
        public static string OpenSetup => T("Einrichtung öffnen", "Open setup");
        public static string Close => T("Schließen", "Close");
    }
}
