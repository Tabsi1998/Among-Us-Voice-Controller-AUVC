using System;
using System.Linq;
using AUOffsetManager;
using Xunit;

namespace AUVC.Capture.Tests
{
    /// <summary>
    /// Another build of a known version (Epic, Microsoft Store) has its classes at other
    /// addresses. The code that loads them is the same, so a signature finds them again.
    /// </summary>
    public class SignatureIndexTests
    {
        private const string Steam = "615A95C9E09B6FFD45570883CB8459311C2B457533A38B37ED1704D6205AEDF9";

        // mov rcx, [rip + disp]; call ?; test rax, rax: the load of one class.
        private static void Plant(byte[] image, int at, int target, byte marker)
        {
            var code = new byte[] { 0x48, 0x8B, 0x0D, 0, 0, 0, 0, 0xE8, 1, 2, 3, 4, 0x48, 0x85, 0xC0, marker };
            BitConverter.GetBytes(target - (at + 7)).CopyTo(code, 3);
            code.CopyTo(image, at);
        }

        [Fact]
        public void APatternGivesTheAddressItLoads()
        {
            var image = new byte[0x1000];
            Plant(image, 0x100, 0x0800, 0x7A);

            Assert.Equal(0x800, SignatureIndex.Find(image, "48 8B 0D ? ? ? ? E8 ? ? ? ? 48 85 C0 7A", 3));
        }

        [Fact]
        public void APatternFoundTwiceGivesNothing()
        {
            var image = new byte[0x1000];
            Plant(image, 0x100, 0x0800, 0x7A);
            Plant(image, 0x300, 0x0900, 0x7A);

            Assert.Null(SignatureIndex.Find(image, "48 8B 0D ? ? ? ? E8 ? ? ? ? 48 85 C0 7A", 3));
            Assert.Null(SignatureIndex.Find(image, "48 8B 0D ? ? ? ? E8 ? ? ? ? 48 85 C0 7B", 3));
        }

        [Fact]
        public void TheDerivedEntryIsTheTemplateWithEveryClassMoved()
        {
            var set = SignatureIndex.LoadBundled()["x64"];
            var index = OffsetManager.LoadBundledIndex();
            var template = index[Steam];
            var image = new byte[0x40000];
            var names = set.Classes.Keys.ToList();
            // Each class one page further than in the template's build, each load in its own place.
            var classes = names.Select((name, i) => (name, address: 0x20000 + i * 0x100)).ToList();
            var simple = new SignatureIndex.Set
            {
                Template = Steam,
                Classes = classes.ToDictionary(c => c.name, c => new SignatureIndex.Signature
                {
                    Pattern = $"48 8B 0D ? ? ? ? E8 ? ? ? ? 48 85 C0 {0x70 + names.IndexOf(c.name):X2}",
                    Displacement = 3,
                }),
            };
            foreach (var (name, address) in classes) Plant(image, 0x1000 + names.IndexOf(name) * 0x40, address, (byte)(0x70 + names.IndexOf(name)));

            var derived = SignatureIndex.Derive(image, index, simple);

            Assert.Equal(0x20000 + names.IndexOf("AmongUsClientOffset") * 0x100, derived.AmongUsClientOffset);
            Assert.Equal(derived.AmongUsClientOffset, derived.GameStateOffsets[0]);
            Assert.Equal(template.GameStateOffsets.Skip(1), derived.GameStateOffsets.Skip(1));
            Assert.Equal(derived.GameStartManagerOffset, derived.GameCodeOffsets[0]);
            Assert.Equal(derived.TempDataOffset, derived.WinningPlayersPtrOffsets[0]);
            Assert.Equal(template.PlayerInfoStructOffsets.ObjectOffset, derived.PlayerInfoStructOffsets.ObjectOffset);
            Assert.Contains("signature", derived.Description);
            // The template itself is untouched.
            Assert.Equal(0x34078C0, index[Steam].GameStateOffsets[0]);
        }

        [Fact]
        public void OneClassNotFoundIsNoEntry()
        {
            var index = OffsetManager.LoadBundledIndex();
            var set = SignatureIndex.LoadBundled()["x64"];

            Assert.Null(SignatureIndex.Derive(new byte[0x1000], index, set));
        }

        [Fact]
        public void TheBundledSignaturesCoverEveryClassOfTheTemplate()
        {
            var set = SignatureIndex.LoadBundled()["x64"];

            Assert.Equal(Steam, set.Template);
            Assert.Equal(8, set.Classes.Count);
            Assert.All(set.Classes.Keys, name => Assert.NotNull(typeof(GameOffsets).GetProperty(name)));
        }

        /// <summary>Against the real Steam DLL when it is at hand: the signatures find the entry's own addresses.</summary>
        [Fact]
        public void OnTheRealSteamBuildTheSignaturesFindTheEntry()
        {
            var path = Environment.GetEnvironmentVariable("AUVC_GAMEASSEMBLY_IMAGE");
            if (string.IsNullOrEmpty(path)) return;
            var index = OffsetManager.LoadBundledIndex();

            var derived = SignatureIndex.Derive(System.IO.File.ReadAllBytes(path), index, SignatureIndex.LoadBundled()["x64"]);

            var steam = index[Steam];
            Assert.Equal(
                new[] { steam.AmongUsClientOffset, steam.GameDataOffset, steam.MeetingHudOffset, steam.GameStartManagerOffset, steam.HudManagerOffset, steam.ServerManagerOffset, steam.TempDataOffset, steam.GameOptionsOffset },
                new[] { derived.AmongUsClientOffset, derived.GameDataOffset, derived.MeetingHudOffset, derived.GameStartManagerOffset, derived.HudManagerOffset, derived.ServerManagerOffset, derived.TempDataOffset, derived.GameOptionsOffset });
        }
    }
}
