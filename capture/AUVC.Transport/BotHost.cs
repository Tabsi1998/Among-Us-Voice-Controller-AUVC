using System.Diagnostics;
using System.Net;
using System.Net.Sockets;
using System.Runtime.InteropServices;
using System.Runtime.Versioning;
using System.Security.Cryptography;

namespace AUVC.Transport;

/// <summary>How the bot is started when it runs on this PC.</summary>
public static class BotLaunch
{
    /// <summary>
    /// A fresh local control secret: 256 random bits, as 64 hex characters. A new
    /// one for every start, so a secret from an earlier run is worth nothing.
    /// </summary>
    public static string NewSecret() => Convert.ToHexString(RandomNumberGenerator.GetBytes(32));

    /// <summary>
    /// A port on the loopback interface that nothing is using right now.
    /// </summary>
    /// <remarks>
    /// A fixed port would collide with a bot that is already running, a second
    /// copy of the app, or anything else that happens to have it. The port is
    /// released again before the bot takes it, which leaves a window of
    /// milliseconds in which something else could; if that happens the bot does
    /// not answer and the start fails with a message rather than silently.
    /// </remarks>
    public static int FreeLoopbackPort()
    {
        var listener = new TcpListener(IPAddress.Loopback, 0);
        listener.Start();
        try
        {
            return ((IPEndPoint)listener.LocalEndpoint).Port;
        }
        finally
        {
            listener.Stop();
        }
    }

    /// <summary>The address capture and the app reach a bot on this PC at.</summary>
    public static Uri AddressFor(int port) => new($"http://127.0.0.1:{port}/");

    /// <summary>Where the bot keeps its database and logs on this PC.</summary>
    public static string DataDirectory() =>
        Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData), "AUVC");

    /// <summary>
    /// The environment the bot starts with. A null value removes a variable the
    /// app itself may have inherited, so nothing from the surroundings changes
    /// how this bot behaves.
    /// </summary>
    public static IReadOnlyDictionary<string, string?> EnvironmentFor(
        string token, string secret, int port, string dataDirectory, string botDirectory) =>
        new Dictionary<string, string?>
        {
            ["DISCORD_BOT_TOKEN"] = token,
            ["AUVC_LOCAL_CONTROL_SECRET"] = secret,
            // Loopback only: capture runs on the same PC, and nothing else has
            // any business reaching this bot.
            ["AUVC_CAPTURE_ADDR"] = $"127.0.0.1:{port}",
            // Every server the bot is in, so /au appears the moment it is invited.
            ["SLASH_COMMAND_GUILD_IDS"] = "*",
            ["AUVC_DATABASE_PATH"] = Path.Combine(dataDirectory, "amongus.db"),
            ["LOG_PATH"] = Path.Combine(dataDirectory, "logs"),
            ["LOCALE_PATH"] = Path.Combine(botDirectory, "locales"),
            ["AUVC_CAPTURE_TLS_CERT"] = null,
            ["AUVC_CAPTURE_TLS_KEY"] = null,
            ["DISABLE_LOG_FILE"] = null,
        };
}

/// <summary>Why the bot could not be started.</summary>
public enum BotHostProblem
{
    /// <summary>The bot program is not where it should be.</summary>
    Missing,

    /// <summary>The bot stopped right after starting, usually because of the token.</summary>
    Exited,

    /// <summary>The bot started but did not connect to Discord in time.</summary>
    NotReady,

    /// <summary>A bot is already running from this app.</summary>
    AlreadyRunning,
}

/// <summary>Raised when the bot could not be started.</summary>
public sealed class BotHostException(BotHostProblem problem, string message, string logTail = "") : Exception(message)
{
    public BotHostProblem Problem { get; } = problem;

    /// <summary>The last lines the bot logged, which usually say why.</summary>
    public string LogTail { get; } = logTail;
}

/// <summary>A started bot process, as the host needs to see it.</summary>
public interface IBotProcess : IDisposable
{
    bool HasExited { get; }

    int ExitCode { get; }

    Task WaitForExitAsync(CancellationToken cancellationToken);

    void Kill();
}

public sealed record BotHostOptions
{
    /// <summary>How long connecting to Discord may take.</summary>
    public TimeSpan StartTimeout { get; init; } = TimeSpan.FromSeconds(45);

    /// <summary>How long the bot has to release everyone and stop before it is killed.</summary>
    public TimeSpan StopTimeout { get; init; } = TimeSpan.FromSeconds(15);

    public TimeSpan PollInterval { get; init; } = TimeSpan.FromMilliseconds(500);
}

/// <summary>
/// Runs the bot on this PC for as long as the app runs.
/// </summary>
/// <remarks>
/// The bot is the same program the release ships for servers, started with a
/// fresh secret and a free loopback port. Stopping asks it to stop, so it
/// releases the players in voice first, and kills it only if it does not.
/// </remarks>
public sealed class BotHost : IAsyncDisposable
{
    private readonly string _executable;
    private readonly string _dataDirectory;
    private readonly HttpClient _http;
    private readonly Func<ProcessStartInfo, IBotProcess> _start;
    private readonly BotHostOptions _options;

    private IBotProcess? _process;
    private LocalControlClient? _control;

    public BotHost(
        string executable,
        string dataDirectory,
        HttpClient http,
        Func<ProcessStartInfo, IBotProcess> start,
        BotHostOptions? options = null)
    {
        _executable = executable;
        _dataDirectory = dataDirectory;
        _http = http;
        _start = start;
        _options = options ?? new BotHostOptions();
    }

    public bool IsRunning => _process is { HasExited: false };

    /// <summary>The running bot's control interface, or null when none is running.</summary>
    public LocalControlClient? Control => IsRunning ? _control : null;

    /// <summary>
    /// Starts the bot and waits until it is connected to Discord.
    /// </summary>
    /// <exception cref="BotHostException">The bot could not be started.</exception>
    public async Task<LocalStatus> StartAsync(string token, CancellationToken cancellationToken = default)
    {
        if (IsRunning)
        {
            throw new BotHostException(BotHostProblem.AlreadyRunning, "The bot is already running.");
        }
        if (!File.Exists(_executable))
        {
            throw new BotHostException(BotHostProblem.Missing, $"The bot program is missing: {_executable}");
        }

        // The bot refuses to start when it cannot create its log file.
        Directory.CreateDirectory(Path.Combine(_dataDirectory, "logs"));

        var secret = BotLaunch.NewSecret();
        var port = BotLaunch.FreeLoopbackPort();
        var botDirectory = Path.GetDirectoryName(Path.GetFullPath(_executable)) ?? ".";

        var info = new ProcessStartInfo(_executable)
        {
            UseShellExecute = false,
            CreateNoWindow = true,
            WorkingDirectory = botDirectory,
        };
        foreach (var (name, value) in BotLaunch.EnvironmentFor(token, secret, port, _dataDirectory, botDirectory))
        {
            if (value is null)
            {
                info.Environment.Remove(name);
            }
            else
            {
                info.Environment[name] = value;
            }
        }

        _process?.Dispose();
        _process = _start(info);
        _control = new LocalControlClient(_http, BotLaunch.AddressFor(port), secret);

        var deadline = DateTime.UtcNow + _options.StartTimeout;
        while (true)
        {
            cancellationToken.ThrowIfCancellationRequested();

            if (_process.HasExited)
            {
                var exitCode = _process.ExitCode;
                throw new BotHostException(BotHostProblem.Exited,
                    $"The bot stopped right after starting (exit code {exitCode}).", ReadLogTail());
            }

            try
            {
                var status = await _control.GetStatusAsync(cancellationToken);
                if (status.Connected)
                {
                    return status;
                }
            }
            catch (HttpRequestException)
            {
                // Not listening yet.
            }
            catch (LocalControlException)
            {
                // Listening, but not ready to answer yet.
            }

            if (DateTime.UtcNow > deadline)
            {
                var tail = ReadLogTail();
                await StopAsync();
                throw new BotHostException(BotHostProblem.NotReady,
                    "The bot did not connect to Discord in time.", tail);
            }

            await Task.Delay(_options.PollInterval, cancellationToken);
        }
    }

    /// <summary>
    /// Asks the bot to release everyone and stop, and kills it if it does not.
    /// </summary>
    public async Task StopAsync()
    {
        var process = _process;
        if (process is null)
        {
            return;
        }

        if (!process.HasExited)
        {
            try
            {
                if (_control is not null)
                {
                    using var asking = new CancellationTokenSource(TimeSpan.FromSeconds(5));
                    await _control.ShutdownAsync(asking.Token);
                }
            }
            catch (Exception)
            {
                // A bot that cannot be asked is killed below.
            }

            using var waiting = new CancellationTokenSource(_options.StopTimeout);
            try
            {
                await process.WaitForExitAsync(waiting.Token);
            }
            catch (OperationCanceledException)
            {
                process.Kill();
            }
        }

        process.Dispose();
        _process = null;
        _control = null;
    }

    public async ValueTask DisposeAsync() => await StopAsync();

    private string ReadLogTail()
    {
        var log = Path.Combine(_dataDirectory, "logs", "logs.txt");
        try
        {
            using var stream = new FileStream(log, FileMode.Open, FileAccess.Read, FileShare.ReadWrite | FileShare.Delete);
            using var reader = new StreamReader(stream);
            var lines = reader.ReadToEnd().Split('\n', StringSplitOptions.RemoveEmptyEntries);
            return string.Join(Environment.NewLine, lines.TakeLast(15).Select(line => line.TrimEnd('\r')));
        }
        catch (IOException)
        {
            return "";
        }
        catch (UnauthorizedAccessException)
        {
            return "";
        }
    }
}

/// <summary>
/// The bot as a real Windows process, tied to this app by a job object.
/// </summary>
/// <remarks>
/// Closing the app stops the bot through <see cref="BotHost.StopAsync"/>. A job
/// object with kill-on-close covers the case where the app does not get that
/// far: when the app dies, Windows closes its handles, the job closes with them,
/// and the bot goes too. Without it a crashed app would leave an invisible bot
/// running with nobody able to stop it.
/// </remarks>
[SupportedOSPlatform("windows")]
public sealed class WindowsBotProcess : IBotProcess
{
    private readonly Process _process;
    private IntPtr _job;

    private WindowsBotProcess(Process process, IntPtr job)
    {
        _process = process;
        _job = job;
    }

    public static WindowsBotProcess Start(ProcessStartInfo info)
    {
        var job = CreateKillOnCloseJob();
        Process process;
        try
        {
            process = Process.Start(info) ?? throw new InvalidOperationException("the bot process did not start");
        }
        catch
        {
            CloseJob(job);
            throw;
        }

        AssignProcessToJobObject(job, process.Handle);
        return new WindowsBotProcess(process, job);
    }

    public bool HasExited => _process.HasExited;

    public int ExitCode => _process.ExitCode;

    public int Id => _process.Id;

    public Task WaitForExitAsync(CancellationToken cancellationToken) => _process.WaitForExitAsync(cancellationToken);

    public void Kill()
    {
        try
        {
            _process.Kill(entireProcessTree: true);
        }
        catch (InvalidOperationException)
        {
            // Already gone.
        }
    }

    public void Dispose()
    {
        CloseJob(_job);
        _job = IntPtr.Zero;
        _process.Dispose();
    }

    private static IntPtr CreateKillOnCloseJob()
    {
        var job = CreateJobObject(IntPtr.Zero, null);
        if (job == IntPtr.Zero)
        {
            return IntPtr.Zero;
        }

        var limits = new JobObjectExtendedLimitInformation
        {
            BasicLimitInformation = new JobObjectBasicLimitInformation { LimitFlags = JobObjectLimitKillOnJobClose },
        };
        if (!SetInformationJobObject(job, JobObjectExtendedLimitInformationClass, ref limits,
                (uint)Marshal.SizeOf<JobObjectExtendedLimitInformation>()))
        {
            CloseJob(job);
            return IntPtr.Zero;
        }
        return job;
    }

    private static void CloseJob(IntPtr job)
    {
        if (job != IntPtr.Zero)
        {
            CloseHandle(job);
        }
    }

    private const int JobObjectExtendedLimitInformationClass = 9;
    private const uint JobObjectLimitKillOnJobClose = 0x2000;

    [StructLayout(LayoutKind.Sequential)]
    private struct JobObjectBasicLimitInformation
    {
        public long PerProcessUserTimeLimit;
        public long PerJobUserTimeLimit;
        public uint LimitFlags;
        public UIntPtr MinimumWorkingSetSize;
        public UIntPtr MaximumWorkingSetSize;
        public uint ActiveProcessLimit;
        public UIntPtr Affinity;
        public uint PriorityClass;
        public uint SchedulingClass;
    }

    [StructLayout(LayoutKind.Sequential)]
    private struct IoCounters
    {
        public ulong ReadOperationCount;
        public ulong WriteOperationCount;
        public ulong OtherOperationCount;
        public ulong ReadTransferCount;
        public ulong WriteTransferCount;
        public ulong OtherTransferCount;
    }

    [StructLayout(LayoutKind.Sequential)]
    private struct JobObjectExtendedLimitInformation
    {
        public JobObjectBasicLimitInformation BasicLimitInformation;
        public IoCounters IoInfo;
        public UIntPtr ProcessMemoryLimit;
        public UIntPtr JobMemoryLimit;
        public UIntPtr PeakProcessMemoryUsed;
        public UIntPtr PeakJobMemoryUsed;
    }

    [DllImport("kernel32.dll", CharSet = CharSet.Unicode, SetLastError = true)]
    private static extern IntPtr CreateJobObject(IntPtr jobAttributes, string? name);

    [DllImport("kernel32.dll", SetLastError = true)]
    [return: MarshalAs(UnmanagedType.Bool)]
    private static extern bool SetInformationJobObject(
        IntPtr job, int infoClass, ref JobObjectExtendedLimitInformation info, uint length);

    [DllImport("kernel32.dll", SetLastError = true)]
    [return: MarshalAs(UnmanagedType.Bool)]
    private static extern bool AssignProcessToJobObject(IntPtr job, IntPtr process);

    [DllImport("kernel32.dll", SetLastError = true)]
    [return: MarshalAs(UnmanagedType.Bool)]
    private static extern bool CloseHandle(IntPtr handle);
}
