using System;
using System.IO;
using System.Linq;
using System.Text.RegularExpressions;
using System.Xml.Linq;
using Xunit;

namespace AUVC.Capture.Tests
{
    /// <summary>
    /// A button a screen reader cannot name is a button some people cannot use (#135).
    /// Every button in the app's windows has text: in the XAML, set by the window's
    /// code, or as <c>AutomationProperties.Name</c> when it only shows a symbol.
    /// </summary>
    public class XamlAccessibilityTests
    {
        private static readonly XNamespace Presentation = "http://schemas.microsoft.com/winfx/2006/xaml/presentation";
        private static readonly XNamespace Xaml = "http://schemas.microsoft.com/winfx/2006/xaml";

        // Read where the app keeps them, as AppTextTests reads the texts.
        private static string AppFolder()
        {
            for (var directory = new DirectoryInfo(AppContext.BaseDirectory); directory is not null; directory = directory.Parent)
            {
                var app = Path.Combine(directory.FullName, "AUCapture-WPF");
                if (File.Exists(Path.Combine(app, "MainWindow.xaml"))) return app;
            }
            throw new DirectoryNotFoundException("No AUCapture-WPF above " + AppContext.BaseDirectory);
        }

        public static TheoryData<string> Windows => new("MainWindow.xaml", "SetupWindow.xaml", "Contributors.xaml");

        [Theory]
        [MemberData(nameof(Windows))]
        public void EveryButtonHasANameAScreenReaderCanRead(string window)
        {
            var folder = AppFolder();
            var xaml = XDocument.Load(Path.Combine(folder, window));
            var codeFile = Path.Combine(folder, window + ".cs");
            var code = File.Exists(codeFile) ? File.ReadAllText(codeFile) : "";

            var nameless = xaml.Descendants(Presentation + "Button")
                .Where(button => !HasText(button, code))
                .Select(Describe)
                .ToList();

            Assert.True(nameless.Count == 0, $"{window}: buttons without a name a screen reader can read: {string.Join("; ", nameless)}");
        }

        /// <summary>The test itself has to notice a button with only a symbol.</summary>
        [Fact]
        public void AButtonWithOnlyASymbolIsFound()
        {
            var symbol = XElement.Parse(
                $"""<Button xmlns="{Presentation}" Click="Settings"><Button.ContentTemplate><DataTemplate><Path /></DataTemplate></Button.ContentTemplate></Button>""");
            var named = XElement.Parse(
                $"""<Button xmlns="{Presentation}" xmlns:x="{Xaml}" x:Name="NextButton" Click="Next" />""");

            Assert.False(HasText(symbol, ""));
            Assert.False(HasText(named, "OtherButton.Content = \"x\";"));
            Assert.False(HasText(named, "NextButton.IsEnabled = true;"));
            Assert.True(HasText(named, "NextButton.Content = SetupText.Next;"));
        }

        private static bool HasText(XElement button, string code)
        {
            if (button.Attribute("Content") is not null) return true;
            if (button.Attributes().Any(attribute => attribute.Name.LocalName == "AutomationProperties.Name")) return true;
            if (button.Nodes().OfType<XText>().Any(text => !string.IsNullOrWhiteSpace(text.Value))) return true;

            var name = (string?)button.Attribute(Xaml + "Name");
            return name is not null && Regex.IsMatch(code, @"\b" + Regex.Escape(name) + @"\.Content\s*=");
        }

        private static string Describe(XElement button) =>
            (string?)button.Attribute(Xaml + "Name")
            ?? (string?)button.Attribute("Click")
            ?? "a button without x:Name or Click";
    }
}
