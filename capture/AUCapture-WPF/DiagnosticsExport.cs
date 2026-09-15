using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Net.Http;
using System.Threading.Tasks;
using AUVC.Transport;

namespace AUCapture_WPF
{
    /// <summary>
    /// Gathers what goes into a diagnostics export on this PC. Which files may go
    /// in, and the redaction, are decided in <see cref="DiagnosticsBundle"/>.
    /// </summary>
    internal static class DiagnosticsExport
    {
        private static readonly string Roaming = Environment.GetFolderPath(Environment.SpecialFolder.ApplicationData);
        private static readonly string Local = Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData);

        public static async Task<IReadOnlyList<DiagnosticsFile>> CollectAsync(IAppSettings settings)
        {
            var files = new List<DiagnosticsFile>();

            var botVersion = "not running on this PC";
            IReadOnlyList<LocalCheck> checks = [];
            var control = LocalBot.Control;
            if (control is not null)
            {
                try
                {
                    var status = await control.GetStatusAsync();
                    botVersion = string.IsNullOrEmpty(status.Version) ? "unknown" : status.Version;
                    if (!string.IsNullOrEmpty(settings.botGuildId))
                    {
                        checks = (await control.GetGuildAsync(settings.botGuildId)).Checks;
                    }
                }
                catch (Exception error) when (error is LocalControlException or HttpRequestException or TaskCanceledException)
                {
                    botVersion = "could not be asked: " + error.Message;
                }
            }

            files.Add(new DiagnosticsFile("summary.txt", DiagnosticsBundle.Summary(
                ReleaseCheck.Current?.ToString() ?? "unknown", botVersion, Environment.OSVersion.VersionString,
                Environment.Version.ToString(), checks, DateTimeOffset.Now)));

            AddNewest(files, App.LogFolder, "*.log", 5, "app-logs/");
            AddNewest(files, Path.Combine(Local, "AUVC", "logs"), "*.*", 3, "bot-logs/");
            AddFile(files, Path.Combine(Roaming, "AmongUsCapture", "AmongUsGUI", "Settings.json"), "settings/app-settings.json");
            AddFile(files, Path.Combine(Roaming, "AmongUsCapture", "Settings.json"), "settings/capture-settings.json");
            return files;
        }

        private static void AddNewest(List<DiagnosticsFile> files, string folder, string pattern, int count, string prefix)
        {
            if (!Directory.Exists(folder)) return;
            foreach (var file in new DirectoryInfo(folder).GetFiles(pattern).OrderByDescending(file => file.LastWriteTimeUtc).Take(count))
            {
                AddFile(files, file.FullName, prefix + file.Name);
            }
        }

        private static void AddFile(List<DiagnosticsFile> files, string path, string name)
        {
            if (!DiagnosticsBundle.MayInclude(path) || !File.Exists(path)) return;
            try
            {
                // The logs are open for writing by NLog and the bot, so they are read
                // shared rather than locked.
                using var stream = new FileStream(path, FileMode.Open, FileAccess.Read, FileShare.ReadWrite | FileShare.Delete);
                files.Add(new DiagnosticsFile(name, DiagnosticsBundle.ReadTail(stream)));
            }
            catch (Exception error) when (error is IOException or UnauthorizedAccessException)
            {
                files.Add(new DiagnosticsFile(name + ".unreadable.txt", "Could not be read: " + error.Message));
            }
        }
    }
}
