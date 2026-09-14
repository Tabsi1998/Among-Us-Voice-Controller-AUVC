using System;
using System.IO;
using System.Text;
using AUVC.Transport;
using Xunit;

namespace AUVC.Capture.Tests
{
    /// <summary>
    /// Checks how capture keeps its credential. The requirement is secure
    /// Windows storage and no plaintext anywhere, so the decisive test is not
    /// that a value comes back out but that it cannot be read off the disk.
    /// </summary>
    public class CredentialStoreTests : IDisposable
    {
        private const string Credential = "0011223344556677.THIS-IS-THE-STORED-VALUE";

        private readonly string _directory =
            Path.Combine(Path.GetTempPath(), "auvc-credential-" + Guid.NewGuid().ToString("N"));

        private string StorePath => Path.Combine(_directory, "credential.bin");

        public void Dispose()
        {
            try
            {
                if (Directory.Exists(_directory))
                {
                    Directory.Delete(_directory, recursive: true);
                }
            }
            catch (IOException)
            {
                // A leftover temporary directory is not worth failing a test.
            }
        }

        [Fact]
        public void AStoredCredentialComesBack()
        {
            if (!OperatingSystem.IsWindows())
            {
                return; // DPAPI is the Windows data protection API; capture is Windows-only.
            }

            var store = new DpapiCredentialStore(StorePath);
            store.Write(Credential);

            Assert.Equal(Credential, store.Read());
        }

        /// <summary>
        /// The point of the exercise. A credential sitting in a profile
        /// directory in plaintext would be readable by anything that can open
        /// the file, which is the realistic threat here.
        /// </summary>
        [Fact]
        public void TheFileOnDiskDoesNotContainTheCredential()
        {
            if (!OperatingSystem.IsWindows())
            {
                return;
            }

            var store = new DpapiCredentialStore(StorePath);
            store.Write(Credential);

            var raw = File.ReadAllBytes(StorePath);

            Assert.DoesNotContain(Credential, Encoding.UTF8.GetString(raw), StringComparison.Ordinal);
            Assert.DoesNotContain(Credential, Encoding.Unicode.GetString(raw), StringComparison.Ordinal);
            Assert.DoesNotContain("THIS-IS-THE-STORED-VALUE",
                Encoding.UTF8.GetString(raw), StringComparison.Ordinal);
        }

        [Fact]
        public void AnInstallThatNeverPairedHasNothingStored()
        {
            if (!OperatingSystem.IsWindows())
            {
                return;
            }

            Assert.Null(new DpapiCredentialStore(StorePath).Read());
        }

        [Fact]
        public void ClearingForgetsTheCredential()
        {
            if (!OperatingSystem.IsWindows())
            {
                return;
            }

            var store = new DpapiCredentialStore(StorePath);
            store.Write(Credential);
            store.Clear();

            Assert.Null(store.Read());
            Assert.False(File.Exists(StorePath));
        }

        [Fact]
        public void WritingTwiceKeepsTheNewerCredential()
        {
            if (!OperatingSystem.IsWindows())
            {
                return;
            }

            var store = new DpapiCredentialStore(StorePath);
            store.Write(Credential);
            store.Write("8877665544332211.A-DIFFERENT-VALUE");

            Assert.Equal("8877665544332211.A-DIFFERENT-VALUE", store.Read());
        }

        /// <summary>
        /// A file copied from another account or another machine cannot be
        /// decrypted here. Capture has to ask the user to pair again rather than
        /// crash on start.
        /// </summary>
        [Fact]
        public void ADamagedFileReadsAsNotPairedRatherThanCrashing()
        {
            if (!OperatingSystem.IsWindows())
            {
                return;
            }

            Directory.CreateDirectory(_directory);
            File.WriteAllBytes(StorePath, [0x00, 0x01, 0x02, 0x03, 0x04]);

            Assert.Null(new DpapiCredentialStore(StorePath).Read());
        }

        /// <summary>
        /// Storing an empty value would read back as "never paired" and send the
        /// user round the pairing loop with no explanation.
        /// </summary>
        [Fact]
        public void AnEmptyCredentialIsRefused()
        {
            if (!OperatingSystem.IsWindows())
            {
                return;
            }

            Assert.Throws<ArgumentException>(() => new DpapiCredentialStore(StorePath).Write(""));
        }

        [Fact]
        public void TheDefaultLocationIsPerUserAndNotRoamed()
        {
            if (!OperatingSystem.IsWindows())
            {
                return;
            }

            var local = Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData);

            Assert.StartsWith(local, DpapiCredentialStore.DefaultPath(), StringComparison.Ordinal);
        }

        /// <summary>
        /// The bot token and the capture credential live side by side. A file
        /// written for one purpose must not read back as the other.
        /// </summary>
        [Fact]
        public void ASecretStoredForOnePurposeCannotBeReadAsAnother()
        {
            if (!OperatingSystem.IsWindows())
            {
                return;
            }

            new DpapiCredentialStore(StorePath, DpapiCredentialStore.BotTokenPurpose).Write(Credential);

            Assert.Null(new DpapiCredentialStore(StorePath, DpapiCredentialStore.CapturePurpose).Read());
            Assert.Equal(Credential, new DpapiCredentialStore(StorePath, DpapiCredentialStore.BotTokenPurpose).Read());
        }

        [Fact]
        public void TheBotTokenHasItsOwnFile()
        {
            if (!OperatingSystem.IsWindows())
            {
                return;
            }

            Assert.NotEqual(DpapiCredentialStore.DefaultPath(), DpapiCredentialStore.BotTokenPath());
            Assert.Throws<ArgumentException>(() => new DpapiCredentialStore(StorePath, " "));
        }

        [Fact]
        public void TheInMemoryStoreKeepsNothingBehind()
        {
            var store = new InMemoryCredentialStore();

            Assert.Null(store.Read());
            store.Write(Credential);
            Assert.Equal(Credential, store.Read());
            store.Clear();
            Assert.Null(store.Read());
        }
    }
}
