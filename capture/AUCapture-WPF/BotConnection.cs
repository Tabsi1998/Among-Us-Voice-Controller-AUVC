using System;
using System.Net.Http;
using System.Reflection;
using System.Threading;
using System.Threading.Tasks;
using AmongUsCapture;
using AUVC.Transport;
using NLog;

namespace AUCapture_WPF
{
    /// <summary>What pairing produced: where the bot is, and what to tell the person who paired.</summary>
    public sealed record PairingOutcome(Uri Address, string Message);

    /// <summary>
    /// The capture window's connection to the AUVC bot: the link, the stored
    /// credential, and pairing.
    /// </summary>
    public static class BotConnection
    {
        private static readonly Logger Logger = LogManager.GetCurrentClassLogger();
        private static readonly ICredentialStore Credentials = new DpapiCredentialStore();
        private static readonly HttpClient Http = new() { Timeout = TimeSpan.FromSeconds(15) };

        // Read by the link on every connection attempt, from its own thread.
        private static volatile Uri address = new(BotAddress.Default);

        /// <summary>
        /// The link to the bot. It records the round from the moment capture
        /// starts, connected or not, so the first connection can open with a
        /// complete snapshot.
        /// </summary>
        public static CaptureLink Link { get; } = new(Credentials, ConnectAsync, CaptureBuild());

        /// <summary>
        /// Starts following the game and connecting to the bot. Call it before the
        /// memory reader starts, so no event is missed.
        /// </summary>
        public static void Start(string configuredAddress)
        {
            if (BotAddress.TryParse(configuredAddress, out var parsed, out _))
            {
                address = parsed;
            }

            // The detail is written for a person and never carries the credential.
            Link.StatusChanged += status => Logger.Info("AUVC bot: {state} {detail}", status.State, status.Detail);

            CaptureBridge.Attach(GameMemReader.getInstance(), Link);
            Link.Start();
        }

        /// <summary>
        /// Redeems a pairing code, stores the credential and reconnects with it.
        /// </summary>
        /// <exception cref="PairingRefusedException">
        /// The address is not one, the bot could not be reached, or it refused the code.
        /// </exception>
        public static async Task<PairingOutcome> PairAsync(string typedAddress, string code)
        {
            if (!BotAddress.TryParse(typedAddress, out var parsed, out var problem))
            {
                throw new PairingRefusedException(problem);
            }

            var result = await new PairingClient(Http).PairAsync(parsed, code);

            Credentials.Write(result.Credential);
            address = parsed;
            Link.Reconnect();
            Logger.Info("Paired with the AUVC bot at {address}", parsed);

            var message = "Capture is paired with the AUVC bot and connects by itself from now on.";
            if (BotAddress.SendsCredentialInClear(parsed))
            {
                message += Environment.NewLine + Environment.NewLine +
                           "Warning: this address uses plain HTTP to another computer, so the credential " +
                           "crosses the network unencrypted. Ask the bot's administrator for an https:// address.";
            }
            return new PairingOutcome(parsed, message);
        }

        private static async Task<IMessageChannel> ConnectAsync(CancellationToken token) =>
            await WebSocketMessageChannel.ConnectAsync(address, token);

        private static string CaptureBuild()
        {
            var version = Assembly.GetEntryAssembly()?.GetName().Version;
            return version is null ? "unknown" : $"{version.Major}.{version.Minor}.{version.Build}";
        }
    }
}
