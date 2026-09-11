using System;
using System.Globalization;
using System.Reflection;
using System.Runtime.Versioning;
using System.Threading;
using AUOffsetManager;
using Xunit;

namespace AUVC.Capture.Tests
{
    /// <summary>
    /// Guards the .NET LTS migration. A project left behind on an older target
    /// framework still builds and still runs, so nothing fails until a user
    /// without that runtime installed tries to start the app.
    /// </summary>
    public class FrameworkBaselineTests
    {
        private const string ExpectedFramework = ".NETCoreApp,Version=v10.0";

        [Theory]
        [InlineData(typeof(OffsetManager))]
        [InlineData(typeof(GameOffsets))]
        public void ShippedAssembliesTargetTheSupportedLts(Type fromAssembly)
        {
            var assembly = fromAssembly.Assembly;
            var attribute = assembly.GetCustomAttribute<TargetFrameworkAttribute>();

            Assert.NotNull(attribute);
            Assert.Equal(ExpectedFramework, attribute!.FrameworkName);
        }

        [Fact]
        public void TestsRunOnTheSupportedLts()
        {
            Assert.Equal(10, Environment.Version.Major);
        }

        /// <summary>
        /// Offsets are memory addresses, and a decimal separator that follows the
        /// operating system's language would turn one into a different address.
        /// Framework upgrades are exactly when culture handling changes, so the
        /// index is parsed under a comma-decimal culture here.
        /// </summary>
        [Fact]
        public void TheBundledIndexParsesTheSameUnderAnyCulture()
        {
            var invariant = OffsetManager.LoadBundledIndex();

            var previous = Thread.CurrentThread.CurrentCulture;
            try
            {
                Thread.CurrentThread.CurrentCulture = new CultureInfo("de-DE");
                var german = OffsetManager.LoadBundledIndex();

                Assert.Equal(invariant.Count, german.Count);
                foreach (var (hash, offsets) in invariant)
                {
                    Assert.True(german.ContainsKey(hash), $"{hash} disappeared under de-DE");
                    Assert.Equal(offsets.AmongUsClientOffset, german[hash].AmongUsClientOffset);
                    Assert.Equal(offsets.GameDataOffset, german[hash].GameDataOffset);
                    Assert.Equal(offsets.MeetingHudOffset, german[hash].MeetingHudOffset);
                    Assert.Equal(offsets.HudManagerOffset, german[hash].HudManagerOffset);
                }
            }
            finally
            {
                Thread.CurrentThread.CurrentCulture = previous;
            }
        }

    }
}
