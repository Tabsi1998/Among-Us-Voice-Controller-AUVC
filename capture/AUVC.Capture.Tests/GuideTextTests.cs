using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Xml.Linq;
using Xunit;

namespace AUVC.Capture.Tests
{
    /// <summary>
    /// The guides name the app's settings and buttons the way the app shows them (#143).
    /// A guide that names a setting the app does not have sends somebody looking for
    /// a word that is not on the screen.
    /// </summary>
    public class GuideTextTests
    {
        private static readonly string Root = RepositoryRoot();

        private static string RepositoryRoot()
        {
            for (var directory = new DirectoryInfo(AppContext.BaseDirectory); directory is not null; directory = directory.Parent)
            {
                if (File.Exists(Path.Combine(directory.FullName, "docs", "guide.md")) &&
                    File.Exists(Path.Combine(directory.FullName, "capture", "AUCapture-WPF", "Properties", "Resources.resx")))
                {
                    return directory.FullName;
                }
            }
            throw new DirectoryNotFoundException("No docs/guide.md above " + AppContext.BaseDirectory);
        }

        /// <summary>Each guide, the texts of its language, and the heading of its settings table.</summary>
        public static TheoryData<string, string, string> SettingsTables => new()
        {
            { "guide.md", "Resources.resx", "### App settings: the gear button" },
            { "anleitung.md", "Resources.de.resx", "### App-Einstellungen: das Zahnrad" },
        };

        public static TheoryData<string, string> Guides => new()
        {
            { "guide.md", "Resources.resx" },
            { "anleitung.md", "Resources.de.resx" },
        };

        // A label such as "App version:" is named without its colon in a table.
        private static Dictionary<string, string> Texts(string file) =>
            XDocument.Load(Path.Combine(Root, "capture", "AUCapture-WPF", "Properties", file)).Root!.Elements("data")
                .ToDictionary(data => (string)data.Attribute("name")!,
                    data => ((string?)data.Element("value") ?? "").Trim().TrimEnd(':').Trim());

        private static string Guide(string file) =>
            File.ReadAllText(Path.Combine(Root, "docs", file)).Replace("\r\n", "\n");

        /// <summary>The cells of the first table after <paramref name="heading"/>, without its header rows.</summary>
        private static List<string[]> TableAfter(string guide, string heading)
        {
            var start = guide.IndexOf(heading + "\n", StringComparison.Ordinal);
            if (start < 0) return [];

            var rows = new List<string[]>();
            foreach (var line in guide[start..].Split('\n').SkipWhile(line => !line.StartsWith('|')))
            {
                if (!line.StartsWith('|')) break;
                rows.Add(line.Trim().Trim('|').Split('|').Select(cell => cell.Trim()).ToArray());
            }
            return rows.Skip(2).ToList();
        }

        [Theory]
        [MemberData(nameof(SettingsTables))]
        public void EverySettingTheGuideNamesIsOneTheAppShows(string guide, string resources, string heading)
        {
            var texts = Texts(resources);
            var tabs = new[] { "SettingsGeneralTabHeader", "SettingsDebugTabHeader", "SettingsAboutTabHeader" }
                .Select(key => texts[key]).ToHashSet();
            var shown = texts.Values.ToHashSet();
            var rows = TableAfter(Guide(guide), heading);

            Assert.True(rows.Count >= 10, $"{guide}: the table after '{heading}' has {rows.Count} rows");
            var wrongTabs = rows.Where(row => !tabs.Contains(row[0])).Select(row => row[0]).ToList();
            Assert.True(wrongTabs.Count == 0, $"{guide}: tabs the app does not have: {string.Join("; ", wrongTabs)}");
            var unknown = rows.Where(row => !shown.Contains(row[1])).Select(row => row[1]).ToList();
            Assert.True(unknown.Count == 0, $"{guide}: settings the app does not show: {string.Join("; ", unknown)}");
        }

        [Theory]
        [MemberData(nameof(Guides))]
        public void PairingIsDescribedWithTheAppsWords(string guide, string resources)
        {
            var texts = Texts(resources);
            var text = Guide(guide);
            var start = text.IndexOf("## 11.", StringComparison.Ordinal);
            Assert.True(start >= 0, $"{guide} has no section 11 about a bot on another computer");
            var section = text[start..];

            Assert.Contains("**" + texts["ManualConnectTooltip"] + "**", section);
            Assert.Contains("**" + texts["ManualConnectionSubmitButton"] + "**", section);
        }
    }
}
