using System;
using System.Net.Http;
using System.Reflection;
using System.Threading;
using System.Threading.Tasks;
using AUVC.Transport;

namespace AUCapture_WPF
{
    /// <summary>
    /// Asks GitHub which AUVC releases exist. Only the public list of releases is
    /// read: nothing is downloaded, and nothing about this PC, the server or the
    /// game is sent.
    /// </summary>
    internal static class ReleaseCheck
    {
        private static readonly HttpClient Http = new() { Timeout = TimeSpan.FromSeconds(10) };

        /// <summary>
        /// The version this app was built as. Releases and the local check stamp it;
        /// any other build carries 0.0.0-dev from the project file.
        /// </summary>
        public static AppVersion Current { get; } = AppVersion.Parse(
            Assembly.GetEntryAssembly()?.GetCustomAttribute<AssemblyInformationalVersionAttribute>()?.InformationalVersion);

        /// <summary>The newer release to tell about, or null when there is none.</summary>
        public static async Task<NewerRelease> FindAsync(AppVersion current, CancellationToken token = default)
        {
            using var request = new HttpRequestMessage(HttpMethod.Get, ReleaseFeed.Address);
            // GitHub refuses requests without a User-Agent.
            request.Headers.UserAgent.ParseAdd("AUVC/" + current);
            request.Headers.Accept.ParseAdd("application/vnd.github+json");

            using var response = await Http.SendAsync(request, token);
            response.EnsureSuccessStatusCode();
            var json = await response.Content.ReadAsStringAsync(token);
            return ReleaseFeed.Newer(current, ReleaseFeed.Parse(json));
        }
    }
}
