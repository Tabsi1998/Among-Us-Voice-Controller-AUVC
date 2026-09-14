using System;
using System.IO;
using System.Net.Http;
using System.Threading;
using System.Threading.Tasks;
using AUVC.Transport;
using NLog;

namespace AUCapture_WPF
{
    /// <summary>
    /// The bot, when it runs on this PC: its token, starting and stopping it, and
    /// pointing capture at it.
    /// </summary>
    public static class LocalBot
    {
        private static readonly Logger Logger = LogManager.GetCurrentClassLogger();
        private static readonly HttpClient Http = new() { Timeout = TimeSpan.FromSeconds(10) };
        private static readonly ICredentialStore Token =
            new DpapiCredentialStore(DpapiCredentialStore.BotTokenPath(), DpapiCredentialStore.BotTokenPurpose);
        private static readonly SemaphoreSlim Gate = new(1, 1);

        private static readonly BotHost Host = new(
            ExecutablePath, BotLaunch.DataDirectory(), Http, info => WindowsBotProcess.Start(info));

        /// <summary>The bot program, shipped beside the app.</summary>
        public static string ExecutablePath => Path.Combine(AppContext.BaseDirectory, "bot", "auvc.exe");

        public static DiscordApplicationClient Discord { get; } = new(new HttpClient { Timeout = TimeSpan.FromSeconds(15) });

        public static bool IsRunning => Host.IsRunning;

        /// <summary>The running bot's control interface, or null.</summary>
        public static LocalControlClient Control => Host.Control;

        public static bool HasToken => !string.IsNullOrEmpty(Token.Read());

        /// <summary>Stores a token that <see cref="Discord"/> has accepted.</summary>
        public static void SaveToken(string pasted) => Token.Write(DiscordApplicationClient.NormalizeToken(pasted));

        /// <summary>Asks Discord about the stored token again, for the invite link.</summary>
        public static Task<BotApplication> CheckStoredTokenAsync() => Discord.CheckTokenAsync(Token.Read());

        /// <summary>
        /// Starts the bot, or reports on it when it is already running.
        /// </summary>
        /// <exception cref="BotHostException">The bot could not start.</exception>
        /// <exception cref="BotTokenException">No token is stored.</exception>
        public static async Task<LocalStatus> StartAsync()
        {
            await Gate.WaitAsync();
            try
            {
                if (Host.IsRunning)
                {
                    return await Host.Control.GetStatusAsync();
                }

                var token = Token.Read();
                if (string.IsNullOrEmpty(token))
                {
                    throw new BotTokenException(TokenProblem.Empty, "No bot token is stored.");
                }

                Logger.Info("Starting the bot on this PC");
                var status = await Host.StartAsync(token);
                Logger.Info("The bot on this PC is online as {name}", status.BotName);
                return status;
            }
            catch (BotHostException error)
            {
                // The log tail is the bot's own log, which never contains the token.
                Logger.Error("The bot on this PC did not start: {problem} {message}\n{log}", error.Problem, error.Message, error.LogTail);
                throw;
            }
            finally
            {
                Gate.Release();
            }
        }

        /// <summary>Asks the bot to release everyone and stop.</summary>
        public static async Task StopAsync()
        {
            await Gate.WaitAsync();
            try
            {
                if (Host.IsRunning)
                {
                    Logger.Info("Stopping the bot on this PC");
                }
                await Host.StopAsync();
            }
            finally
            {
                Gate.Release();
            }
        }

        /// <summary>
        /// Points capture at the bot on this PC.
        /// </summary>
        /// <param name="freshCredential">
        /// Obtain a new credential even when one is stored, because the stored one
        /// belongs to another bot or was refused.
        /// </param>
        public static async Task ConnectCaptureAsync(string guildId, bool freshCredential)
        {
            var control = Host.Control ?? throw new InvalidOperationException("the bot on this PC is not running");
            await BotConnection.UseLocalBotAsync(control, guildId, freshCredential);
        }
    }
}
