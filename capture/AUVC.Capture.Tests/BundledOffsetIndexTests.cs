using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using AUOffsetManager;
using Newtonsoft.Json;
using Xunit;

namespace AUVC.Capture.Tests
{
    /// <summary>
    /// AUVC must be able to read the game without contacting a third-party host.
    /// These tests fail if the offset index stops being shipped inside the assembly.
    /// </summary>
    public class BundledOffsetIndexTests
    {
        [Fact]
        public void IndexIsEmbeddedInTheAssembly()
        {
            var index = OffsetManager.LoadBundledIndex();

            Assert.NotEmpty(index);
            Assert.True(
                index.Count >= 60,
                $"Expected at least the 60 game versions present at import, found {index.Count}. " +
                "The embedded resource is probably missing from AUOffsetManager.csproj.");
        }

        [Fact]
        public void EmbeddedIndexMatchesTheRepositoryFile()
        {
            var repository = JsonConvert.DeserializeObject<Dictionary<string, GameOffsets>>(
                File.ReadAllText(Path.Combine(AppContext.BaseDirectory, "Fixtures", "Offsets.json")))!;

            var embedded = OffsetManager.LoadBundledIndex();

            Assert.Equal(repository.Count, embedded.Count);
            Assert.Equal(
                repository.Keys.OrderBy(k => k, StringComparer.Ordinal),
                embedded.Keys.OrderBy(k => k, StringComparer.Ordinal));
        }

        [Fact]
        public void EmbeddedEntriesCarryUsableOffsets()
        {
            var index = OffsetManager.LoadBundledIndex();
            var entry = index.First();

            Assert.False(string.IsNullOrWhiteSpace(entry.Key));
            Assert.NotEqual(0, entry.Value.AmongUsClientOffset);
            Assert.NotEqual(0, entry.Value.GameDataOffset);
        }

        [Fact]
        public void ManagerResolvesAnEmbeddedHashWithoutAnyRemote()
        {
            var expected = OffsetManager.LoadBundledIndex().First();

            // Empty index URL: no network access is attempted at all.
            var manager = new OffsetManager("");
            var offsets = manager.FetchForHash(expected.Key);

            Assert.NotNull(offsets);
            Assert.Equal(expected.Value.AmongUsClientOffset, offsets!.AmongUsClientOffset);
        }

        /// <summary>
        /// Among Us 2026.9.29 on Steam, the first 64-bit build (#198), worked out from the
        /// game's own GameAssembly.dll and global-metadata.dat with Il2CppDumper.
        /// </summary>
        [Fact]
        public void TheFirst64BitSteamBuildIsKnown()
        {
            var offsets = OffsetManager.LoadBundledIndex()["615A95C9E09B6FFD45570883CB8459311C2B457533A38B37ED1704D6205AEDF9"];

            Assert.True(OffsetManager.IsUsable(offsets));
            Assert.Equal(new[] { 0x34078C0, 0xB8, 0x0, 0xAC }, offsets.GameStateOffsets);
            Assert.Equal(new[] { 0x10, 0x14 }, offsets.StringOffsets);
            Assert.Equal((8, 0x20), (offsets.AddPlayerPtr, offsets.PlayerListPtr));
            // GameStartManager keeps Instance in DestroyableSingleton<T>: MethodInfo, klass, statics.
            Assert.Equal(new[] { 0x340B9A8, 0x20, 0xB8, 0x0, 0x70, 0xE0 }, offsets.GameCodeOffsets);
        }
    }
}
