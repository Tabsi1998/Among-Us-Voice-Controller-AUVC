using System;
using System.IO;
using System.Linq;
using System.Xml.Linq;
using Xunit;

namespace AUVC.Capture.Tests
{
    /// <summary>
    /// The settings panel of the main window (#172) and where the Bot and pairing
    /// buttons live (#173). Nobody can see a window in a test, so what is held here
    /// is what made the panel unreadable: a fixed narrow width, a Viewbox that
    /// scaled the settings down to fit it, and tab names beside the content.
    /// </summary>
    public class SettingsPanelTests
    {
        private static readonly XNamespace Presentation = "http://schemas.microsoft.com/winfx/2006/xaml/presentation";
        private static readonly XNamespace Xaml = "http://schemas.microsoft.com/winfx/2006/xaml";
        private static readonly XNamespace Mah = "clr-namespace:MahApps.Metro.Controls;assembly=MahApps.Metro";

        private static XDocument MainWindow()
        {
            for (var directory = new DirectoryInfo(AppContext.BaseDirectory); directory is not null; directory = directory.Parent)
            {
                var window = Path.Combine(directory.FullName, "AUCapture-WPF", "MainWindow.xaml");
                if (File.Exists(window)) return XDocument.Load(window);
            }
            throw new FileNotFoundException("No AUCapture-WPF/MainWindow.xaml above " + AppContext.BaseDirectory);
        }

        private static XElement SettingsFlyout() =>
            MainWindow().Descendants(Mah + "Flyout").Single(flyout => (string?)flyout.Attribute(Xaml + "Name") == "SettingsFlyout");

        private static XElement Named(string name) =>
            MainWindow().Descendants().Single(element => (string?)element.Attribute(Xaml + "Name") == name);

        /// <summary>A panel of 275 pixels is what the settings were squeezed into.</summary>
        [Fact]
        public void TheSettingsPanelIsWideEnoughToRead()
        {
            var width = (double?)SettingsFlyout().Attribute("Width");

            Assert.NotNull(width);
            Assert.True(width >= 400, $"the settings panel is {width} wide");
        }

        /// <summary>
        /// A Viewbox scales whatever is inside it to the space it gets. In a narrow
        /// panel that turned every setting into unreadably small text.
        /// </summary>
        [Fact]
        public void NothingInTheSettingsPanelIsScaledToFit()
        {
            var boxes = SettingsFlyout().Descendants(Presentation + "Viewbox").Count();

            Assert.True(boxes == 0, $"{boxes} Viewbox elements scale the settings down");
        }

        /// <summary>
        /// The tab names are as long as the settings themselves. Beside them they
        /// left a column too narrow for the content and were cut off.
        /// </summary>
        [Fact]
        public void TheSettingsTabsSitOnTop()
        {
            var tabs = SettingsFlyout().Descendants(Mah + "MetroAnimatedTabControl").Single();

            Assert.Equal("Top", (string?)tabs.Attribute("TabStripPlacement"));
        }

        /// <summary>
        /// The title bar keeps the gear alone; Bot and pairing are settings (#173).
        /// </summary>
        [Fact]
        public void TheTitleBarHoldsOnlyTheGear()
        {
            var commands = MainWindow().Descendants(Mah + "WindowCommands").Single();

            var buttons = commands.Descendants(Presentation + "Button").ToList();
            Assert.Single(buttons);
            Assert.Equal("Settings", (string?)buttons[0].Attribute("Click"));
        }

        [Theory]
        [InlineData("SetupButton")]
        [InlineData("ManualConnectButton")]
        public void TheBotButtonsAreInTheSettings(string button)
        {
            var tab = Named("BotTab");

            Assert.Contains(tab.Descendants(), element => (string?)element.Attribute(Xaml + "Name") == button);
        }

        /// <summary>
        /// Pairing reaches a bot on another computer. Hiding it whenever the bot
        /// runs here has to stay the window's decision, not a static setting.
        /// </summary>
        [Fact]
        public void PairingIsShownAndHiddenByTheWindow()
        {
            var code = File.ReadAllText(Path.Combine(
                Path.GetDirectoryName(MainWindowPath())!, "MainWindow.xaml.cs"));

            Assert.Contains("ManualConnectButton.Visibility", code);
            Assert.Contains("runBotOnThisPc", code);
        }

        private static string MainWindowPath()
        {
            for (var directory = new DirectoryInfo(AppContext.BaseDirectory); directory is not null; directory = directory.Parent)
            {
                var window = Path.Combine(directory.FullName, "AUCapture-WPF", "MainWindow.xaml");
                if (File.Exists(window)) return window;
            }
            throw new FileNotFoundException("No AUCapture-WPF/MainWindow.xaml above " + AppContext.BaseDirectory);
        }
    }
}
