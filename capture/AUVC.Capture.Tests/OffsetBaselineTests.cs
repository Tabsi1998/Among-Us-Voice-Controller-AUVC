using System;
using System.Collections.Generic;
using System.IO;
using AUOffsetManager;
using Newtonsoft.Json;
using Newtonsoft.Json.Linq;
using Xunit;

namespace AUVC.Capture.Tests
{
    public class OffsetBaselineTests
    {
        [Fact]
        public void HistoricalExportMatchesReviewed2020Fixture()
        {
            var expected = JToken.Parse(File.ReadAllText(Fixture("legacy-2020.json")));
            var actual = JToken.Parse(AUOffsetHelper.Program.ExportLegacySample());
            Assert.True(JToken.DeepEquals(expected, actual), "Historical offset layout changed.");
            Assert.Null(actual["PlayerInfoStructOffsets"]!["OutfitsOffset"]);
            Assert.Null(actual["PlayerInfoStructOffsets"]!["RoleTeamTypeOffset"]);
        }

        [Theory]
        [InlineData("")]
        [InlineData("--current")]
        [InlineData("--legacy-sample extra")]
        public void HelperRefusesImplicitOrUnknownExport(string command)
        {
            using var output = new StringWriter();
            using var error = new StringWriter();
            var args = command.Length == 0 ? Array.Empty<string>() : command.Split(' ');
            Assert.Equal(2, AUOffsetHelper.Program.Run(args, output, error));
            Assert.Equal("", output.ToString());
            Assert.Contains("does not generate current offsets", error.ToString());
        }

        [Fact]
        public void ExplicitHistoricalExportWarnsWithoutPollutingJson()
        {
            using var output = new StringWriter();
            using var error = new StringWriter();
            Assert.Equal(0, AUOffsetHelper.Program.Run(new[] { "--legacy-sample" }, output, error));
            Assert.Equal("v2020.12.9s", (string?)JObject.Parse(output.ToString())["Description"]);
            Assert.Contains("incompatible", error.ToString());
        }

        [Fact]
        public void HelpDoesNotExportOffsets()
        {
            using var output = new StringWriter();
            using var error = new StringWriter();
            Assert.Equal(0, AUOffsetHelper.Program.Run(new[] { "--help" }, output, error));
            Assert.Contains("--legacy-sample", output.ToString());
            Assert.Equal("", error.ToString());
        }

        [Theory]
        [InlineData("1925F256460C80AC017E6B71AC9BC5DE6E0CF1A24F8B822CA9B3494DA2C69D19", 0x22F4F10)]
        [InlineData("BB0E5AEA695C51D5B4CD1E284521F9950B4B9ABA91BEE1D6BF0EF1CED40CE7DA", 0x227024C)]
        public void Bundled2024OffsetsKeepPointerChainsAndAliveState(string hash, int clientOffset)
        {
            // Exercise the actual bundled upstream document (including its hex numbers).
            var index = JsonConvert.DeserializeObject<Dictionary<string, GameOffsets>>(
                File.ReadAllText(Fixture("Offsets.json")))!;
            var offsets = index[hash];
            Assert.Equal(clientOffset, offsets.AmongUsClientOffset);
            Assert.Equal(new[] { clientOffset, 0x5C, 0, 0x88 }, offsets.GameStateOffsets);
            Assert.Equal(new[] { 0x1C, 0xC, 0x1C }, offsets.PlayerInfoStructOffsets.OutfitsOffset);
            Assert.Equal(new[] { 0x28, 0x3C }, offsets.PlayerInfoStructOffsets.RoleTeamTypeOffset);
            Assert.Equal(0x30, offsets.PlayerInfoStructOffsets.IsDeadOffset);
            Assert.Equal(0x34, offsets.PlayerInfoStructOffsets.ObjectOffset);
            Assert.Equal(0x28, offsets.PlayerOutfitStructOffsets.PlayerNameOffset);
            Assert.Equal(0x32, offsets.WinningPlayerDataStructOffsets.IsDeadOffset);

            var recovered = JsonConvert.DeserializeObject<GameOffsets>(
                JsonConvert.SerializeObject(offsets))!;
            Assert.Equal(offsets.GameStateOffsets, recovered.GameStateOffsets);
            Assert.Equal(offsets.PlayerInfoStructOffsets.OutfitsOffset,
                recovered.PlayerInfoStructOffsets.OutfitsOffset);
            Assert.Equal(offsets.PlayerInfoStructOffsets.IsDeadOffset,
                recovered.PlayerInfoStructOffsets.IsDeadOffset);
        }

        [Theory]
        [InlineData("{")]
        [InlineData("{\"hash\": {\"GameStateOffsets\": \"invalid\"}}")]
        [InlineData("{\"hash\": {\"PlayerInfoStructOffsets\": {\"IsDeadOffset\": \"invalid\"}}}")]
        public void MalformedOffsetDocumentsAreNotSilentlyAccepted(string json)
        {
            Assert.ThrowsAny<JsonException>(
                () => JsonConvert.DeserializeObject<Dictionary<string, GameOffsets>>(json));
        }

        private static string Fixture(string name) =>
            Path.Combine(AppContext.BaseDirectory, "Fixtures", name);
    }
}
