using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Text.RegularExpressions;
using System.Xml.Linq;
using AUCapture_WPF;
using Xunit;

namespace AUVC.Capture.Tests
{
    /// <summary>
    /// The texts of the main window and its settings. One text missing from a
    /// language shows up in English among the others, the mixed languages #112 is about.
    /// </summary>
    public class AppTextTests
    {
        // Read where the app keeps them rather than copied next to the tests, where a
        // language removed from the app would stay behind.
        private static readonly string Folder = AppProperties();

        private static string AppProperties()
        {
            for (var directory = new DirectoryInfo(AppContext.BaseDirectory); directory is not null; directory = directory.Parent)
            {
                var properties = Path.Combine(directory.FullName, "AUCapture-WPF", "Properties");
                if (File.Exists(Path.Combine(properties, "Resources.resx"))) return properties;
            }
            throw new DirectoryNotFoundException("No AUCapture-WPF/Properties above " + AppContext.BaseDirectory);
        }

        private static Dictionary<string, string> Texts(string file) =>
            XDocument.Load(Path.Combine(Folder, file)).Root!.Elements("data")
                .ToDictionary(data => (string)data.Attribute("name")!, data => (string?)data.Element("value") ?? "");

        private static readonly Dictionary<string, string> Neutral = Texts("Resources.resx");

        public static TheoryData<string> Languages => new(AppLanguage.Supported);

        [Fact]
        public void OnlyTheOfferedLanguagesShip()
        {
            var shipped = Directory.GetFiles(Folder, "Resources.*.resx")
                .Select(file => Path.GetFileName(file).Split('.')[1]);

            Assert.Equal(AppLanguage.Supported.Order(), shipped.Order());
        }

        [Theory]
        [MemberData(nameof(Languages))]
        public void EveryTextIsTranslated(string language)
        {
            var translated = Texts($"Resources.{language}.resx");

            Assert.Equal(Neutral.Keys.Order(), translated.Keys.Order());
            Assert.All(translated, text => Assert.False(string.IsNullOrWhiteSpace(text.Value), text.Key));
        }

        [Fact]
        public void TheEnglishTextsAreTheNeutralOnes() =>
            Assert.Equal(Neutral, Texts("Resources.en.resx"));

        // Visual Studio regenerates this class; nothing else does, so it can fall behind.
        [Fact]
        public void TheResourceClassHasEveryText()
        {
            var designer = File.ReadAllText(Path.Combine(Folder, "Resources.Designer.cs"));

            var listed = Regex.Matches(designer, "GetString\\(\"([^\"]+)\"").Select(match => match.Groups[1].Value);

            Assert.Equal(Neutral.Keys.Order(), listed.Order());
        }
    }
}
