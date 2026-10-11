using System;
using System.IO;
using System.Linq;
using System.Net;
using System.Net.Http;
using System.Threading;
using System.Threading.Tasks;
using AUOffsetManager;
using Newtonsoft.Json;
using Xunit;

namespace AUVC.Capture.Tests
{
    /// <summary>
    /// A new Among Us version needs an entry in the repository's index, not a new release.
    /// What comes from the network only adds versions this AUVC does not know.
    /// </summary>
    public sealed class OffsetFetchTests : IDisposable
    {
        private const string NewHash = "00112233445566778899AABBCCDDEEFF00112233445566778899AABBCCDDEEFF";
        private readonly string folder = Path.Combine(Path.GetTempPath(), "auvc-offsets-" + Guid.NewGuid().ToString("N"));

        public OffsetFetchTests() => Directory.CreateDirectory(folder);

        public void Dispose() => Directory.Delete(folder, recursive: true);

        private sealed class Answer : HttpMessageHandler
        {
            private readonly Func<string> body;
            public int Requests;

            public Answer(Func<string> body) => this.body = body;

            protected override Task<HttpResponseMessage> SendAsync(HttpRequestMessage request, CancellationToken cancellationToken)
            {
                Requests++;
                return Task.FromResult(new HttpResponseMessage(HttpStatusCode.OK) { Content = new StringContent(body()) });
            }
        }

        private static GameOffsets Usable(string description)
        {
            var known = OffsetManager.LoadBundledIndex().Values.Last();
            var copy = JsonConvert.DeserializeObject<GameOffsets>(JsonConvert.SerializeObject(known))!;
            copy.Description = description;
            return copy;
        }

        private static string Index(params (string Hash, GameOffsets Offsets)[] entries) =>
            JsonConvert.SerializeObject(entries.ToDictionary(entry => entry.Hash, entry => entry.Offsets));

        [Fact]
        public async Task AVersionMissingHereIsTakenFromTheRepository()
        {
            var manager = new OffsetManager("", folder);
            var answer = new Answer(() => Index((NewHash, Usable("v2026.9.29s x64"))));

            Assert.True(await manager.FetchMissingAsync(NewHash, "https://example.invalid/index.json", answer));

            Assert.Equal("v2026.9.29s x64", manager.FetchForHash(NewHash).Description);
            Assert.Contains(NewHash, File.ReadAllText(Path.Combine(folder, "indexCache.json")));
            Assert.Equal("v2026.9.29s x64", new OffsetManager("", folder).FetchForHash(NewHash).Description);
        }

        [Fact]
        public async Task ABundledVersionIsNeverOverwrittenFromTheNetwork()
        {
            var bundled = OffsetManager.LoadBundledIndex().First();
            var manager = new OffsetManager("", folder);
            var answer = new Answer(() => Index((bundled.Key, Usable("from the network")), (NewHash, Usable("new"))));

            await manager.FetchMissingAsync(NewHash, "https://example.invalid/index.json", answer);

            Assert.Equal(bundled.Value.Description, manager.FetchForHash(bundled.Key).Description);
            Assert.DoesNotContain("from the network", File.ReadAllText(Path.Combine(folder, "indexCache.json")));
        }

        [Fact]
        public async Task AKnownVersionAsksNobody()
        {
            var manager = new OffsetManager("", folder);
            var answer = new Answer(() => throw new InvalidOperationException("no request expected"));

            Assert.True(await manager.FetchMissingAsync(OffsetManager.LoadBundledIndex().First().Key, "https://example.invalid/", answer));
            Assert.Equal(0, answer.Requests);
        }

        [Fact]
        public async Task AnEntryTheReaderCannotUseIsNotTaken()
        {
            var broken = Usable("broken");
            broken.GameStateOffsets = null;
            var manager = new OffsetManager("", folder);

            Assert.False(await manager.FetchMissingAsync(NewHash, "https://example.invalid/", new Answer(() => Index((NewHash, broken)))));
            Assert.Null(manager.FetchForHash(NewHash));
        }

        [Fact]
        public async Task NoAnswerOrNonsenseLeavesTheIndexAsItWas()
        {
            var manager = new OffsetManager("", folder);

            Assert.False(await manager.FetchMissingAsync(NewHash, "https://example.invalid/", new Answer(() => "<html>not json</html>")));
            Assert.Null(manager.FetchForHash(NewHash));
        }

        [Fact]
        public void TheRepositoryIndexIsTheFileInThisRepository() =>
            Assert.EndsWith("/Tabsi1998/Among-Us-Voice-Controller-AUVC/main/capture/Offsets.json", OffsetManager.RepositoryIndexUrl);
    }
}
