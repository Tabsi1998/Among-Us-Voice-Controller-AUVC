using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.IO;
using System.Linq;
using System.Net;
using System.Net.Http;
using System.Threading;
using System.Threading.Tasks;
using AUVC.Transport;
using Xunit;

namespace AUVC.Capture.Tests
{
    /// <summary>
    /// Starting and stopping the bot on this PC, with the process and the bot's
    /// answers faked, plus one test against a real Windows process for the part
    /// that only Windows can do.
    /// </summary>
    public class BotHostTests : IDisposable
    {
        private static readonly string Token = string.Join(".", "pretend", "bot", "token");

        private readonly string _directory = Path.Combine(Path.GetTempPath(), "auvc-host-" + Guid.NewGuid().ToString("N"));

        private static readonly BotHostOptions Fast = new()
        {
            StartTimeout = TimeSpan.FromSeconds(2),
            StopTimeout = TimeSpan.FromMilliseconds(300),
            PollInterval = TimeSpan.FromMilliseconds(10),
        };

        public BotHostTests()
        {
            Directory.CreateDirectory(Path.Combine(_directory, "bot"));
            File.WriteAllText(Executable, "not really a program");
        }

        public void Dispose()
        {
            try
            {
                Directory.Delete(_directory, recursive: true);
            }
            catch (IOException)
            {
            }
        }

        private string Executable => Path.Combine(_directory, "bot", "auvc.exe");

        private string DataDirectory => Path.Combine(_directory, "data");

        private sealed class FakeProcess : IBotProcess
        {
            private readonly TaskCompletionSource _exited = new(TaskCreationOptions.RunContinuationsAsynchronously);

            public ProcessStartInfo? Info { get; init; }
            public bool Killed { get; private set; }
            public bool Disposed { get; private set; }
            public bool HasExited => _exited.Task.IsCompleted;
            public int ExitCode { get; private set; }

            public void Exit(int code)
            {
                ExitCode = code;
                _exited.TrySetResult();
            }

            public Task WaitForExitAsync(CancellationToken cancellationToken) => _exited.Task.WaitAsync(cancellationToken);

            public void Kill()
            {
                Killed = true;
                Exit(-1);
            }

            public void Dispose() => Disposed = true;
        }

        /// <summary>The bot's side: answers the status route, and stops when asked.</summary>
        private sealed class FakeBot : HttpMessageHandler
        {
            private int _statusCalls;

            public int NotListeningFor { get; init; }
            public int NotConnectedFor { get; init; }
            public bool ExitsWhenAsked { get; init; } = true;
            public FakeProcess? Process { get; set; }
            public List<HttpRequestMessage> Requests { get; } = [];

            protected override Task<HttpResponseMessage> SendAsync(HttpRequestMessage request, CancellationToken cancellationToken)
            {
                lock (Requests)
                {
                    Requests.Add(request);
                }

                if (request.RequestUri!.AbsolutePath == "/local/shutdown")
                {
                    if (ExitsWhenAsked)
                    {
                        Process?.Exit(0);
                    }
                    return Task.FromResult(Json(HttpStatusCode.Accepted, """{"status":"stopping"}"""));
                }

                var call = Interlocked.Increment(ref _statusCalls);
                if (call <= NotListeningFor)
                {
                    throw new HttpRequestException("No connection could be made because the target machine actively refused it.");
                }
                var connected = call > NotListeningFor + NotConnectedFor;
                return Task.FromResult(Json(HttpStatusCode.OK,
                    $$"""{"connected":{{(connected ? "true" : "false")}},"bot_name":"AUVC","guilds":[{"id":"g1","name":"The Crew"}]}"""));
            }

            private static HttpResponseMessage Json(HttpStatusCode status, string body) =>
                new(status) { Content = new StringContent(body, System.Text.Encoding.UTF8, "application/json") };
        }

        private BotHost Host(FakeBot bot, Func<ProcessStartInfo, FakeProcess>? start = null) =>
            new(Executable, DataDirectory, new HttpClient(bot), info =>
            {
                var process = start?.Invoke(info) ?? new FakeProcess { Info = info };
                bot.Process = process;
                return process;
            }, Fast);

        [Fact]
        public async Task StartingWaitsUntilTheBotIsConnectedToDiscord()
        {
            var bot = new FakeBot { NotListeningFor = 2, NotConnectedFor = 2 };
            await using var host = Host(bot);

            var status = await host.StartAsync(Token);

            Assert.True(status.Connected);
            Assert.Equal("The Crew", Assert.Single(status.Servers).Name);
            Assert.True(host.IsRunning);
            Assert.NotNull(host.Control);
        }

        /// <summary>
        /// The app and the bot share a secret nobody else sees: it goes into the
        /// bot's environment and into the Authorization header of the app's
        /// requests, and it is new for every start.
        /// </summary>
        [Fact]
        public async Task TheBotAndTheAppShareAFreshSecretAndALoopbackAddress()
        {
            var bot = new FakeBot();
            FakeProcess? started = null;
            await using var host = Host(bot, info => started = new FakeProcess { Info = info });

            await host.StartAsync(Token);

            var environment = started!.Info!.Environment;
            var secret = environment["AUVC_LOCAL_CONTROL_SECRET"];
            Assert.Equal(64, secret!.Length);
            Assert.Equal(secret, bot.Requests[0].Headers.Authorization!.Parameter);

            var address = environment["AUVC_CAPTURE_ADDR"]!;
            Assert.StartsWith("127.0.0.1:", address);
            Assert.Equal("127.0.0.1", host.Control!.Address.Host);
            Assert.Equal(address.Split(':')[1], host.Control.Address.Port.ToString());

            Assert.Equal(Token, environment["DISCORD_BOT_TOKEN"]);
            Assert.Equal("*", environment["SLASH_COMMAND_GUILD_IDS"]);
            Assert.False(started.Info.UseShellExecute);
            Assert.True(started.Info.CreateNoWindow);
        }

        [Fact]
        public void EverySecretIsNew()
        {
            var secrets = Enumerable.Range(0, 20).Select(_ => BotLaunch.NewSecret()).ToHashSet();

            Assert.Equal(20, secrets.Count);
            Assert.All(secrets, secret => Assert.Matches("^[0-9A-F]{64}$", secret));
        }

        /// <summary>
        /// Settings the app happens to have inherited, such as TLS files for a
        /// server bot, must not change how the bot on this PC runs.
        /// </summary>
        [Fact]
        public void TheEnvironmentLeavesNothingToChance()
        {
            var environment = BotLaunch.EnvironmentFor(Token, "secret", 50123, @"C:\data", @"C:\app\bot");

            Assert.Equal(@"C:\data\amongus.db", environment["AUVC_DATABASE_PATH"]);
            Assert.Equal(@"C:\data\logs", environment["LOG_PATH"]);
            Assert.Equal(@"C:\app\bot\locales", environment["LOCALE_PATH"]);
            Assert.Null(environment["AUVC_CAPTURE_TLS_CERT"]);
            Assert.Null(environment["AUVC_CAPTURE_TLS_KEY"]);
            Assert.Null(environment["DISABLE_LOG_FILE"]);
        }

        [Fact]
        public void AFreePortIsOnOffer()
        {
            Assert.InRange(BotLaunch.FreeLoopbackPort(), 1, 65535);
        }

        [Fact]
        public async Task AMissingBotProgramIsReported()
        {
            File.Delete(Executable);
            await using var host = Host(new FakeBot());

            var error = await Assert.ThrowsAsync<BotHostException>(() => host.StartAsync(Token));

            Assert.Equal(BotHostProblem.Missing, error.Problem);
        }

        /// <summary>
        /// A wrong token makes the bot exit at once. What it logged is the only
        /// explanation, so it travels with the error.
        /// </summary>
        [Fact]
        public async Task ABotThatExitsAtOnceIsReportedWithItsLog()
        {
            var bot = new FakeBot { NotListeningFor = int.MaxValue };
            await using var host = Host(bot, info =>
            {
                Directory.CreateDirectory(Path.Combine(DataDirectory, "logs"));
                File.WriteAllText(Path.Combine(DataDirectory, "logs", "logs.txt"),
                    "AUVC starting\nthe bot failed to start; is the Discord bot token valid?\n");
                var process = new FakeProcess { Info = info };
                process.Exit(1);
                return process;
            });

            var error = await Assert.ThrowsAsync<BotHostException>(() => host.StartAsync(Token));

            Assert.Equal(BotHostProblem.Exited, error.Problem);
            Assert.Contains("exit code 1", error.Message);
            Assert.Contains("token valid", error.LogTail);
        }

        [Fact]
        public async Task ABotThatNeverConnectsIsStoppedAndReported()
        {
            var bot = new FakeBot { NotConnectedFor = int.MaxValue };
            await using var host = Host(bot);

            var error = await Assert.ThrowsAsync<BotHostException>(() => host.StartAsync(Token));

            Assert.Equal(BotHostProblem.NotReady, error.Problem);
            Assert.False(host.IsRunning);
        }

        [Fact]
        public async Task ASecondStartIsRefusedWhileTheFirstRuns()
        {
            await using var host = Host(new FakeBot());
            await host.StartAsync(Token);

            var error = await Assert.ThrowsAsync<BotHostException>(() => host.StartAsync(Token));

            Assert.Equal(BotHostProblem.AlreadyRunning, error.Problem);
        }

        /// <summary>
        /// Stopping asks first, because a bot that is asked releases the players
        /// in voice, and a killed one cannot.
        /// </summary>
        [Fact]
        public async Task StoppingAsksTheBotBeforeAnythingHarsher()
        {
            var bot = new FakeBot();
            await using var host = Host(bot);
            await host.StartAsync(Token);
            var process = bot.Process!;

            await host.StopAsync();

            Assert.Contains(bot.Requests, request => request.RequestUri!.AbsolutePath == "/local/shutdown");
            Assert.False(process.Killed);
            Assert.True(process.Disposed);
            Assert.False(host.IsRunning);
            Assert.Null(host.Control);
        }

        [Fact]
        public async Task ABotThatDoesNotStopIsKilled()
        {
            var bot = new FakeBot { ExitsWhenAsked = false };
            await using var host = Host(bot);
            await host.StartAsync(Token);
            var process = bot.Process!;

            await host.StopAsync();

            Assert.True(process.Killed);
            Assert.False(host.IsRunning);
        }

        /// <summary>
        /// The safety net for a crashed app: once the job closes, Windows ends the
        /// bot, even though nobody asked it to stop.
        /// </summary>
        [Fact]
        public async Task ClosingTheJobEndsARealProcess()
        {
            if (!OperatingSystem.IsWindows())
            {
                return;
            }

            var info = new ProcessStartInfo("cmd.exe", "/c ping -n 60 127.0.0.1")
            {
                UseShellExecute = false,
                CreateNoWindow = true,
            };
            var wrapper = WindowsBotProcess.Start(info);
            using var watcher = Process.GetProcessById(wrapper.Id);

            wrapper.Dispose();

            Assert.True(watcher.WaitForExit(10_000), "the process outlived its job");
            await Task.CompletedTask;
        }
    }
}
