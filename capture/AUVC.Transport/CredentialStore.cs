using System.Runtime.Versioning;
using System.Security.Cryptography;
using System.Text;

namespace AUVC.Transport;

/// <summary>
/// Where capture keeps the credential it was issued when it paired.
/// </summary>
public interface ICredentialStore
{
    /// <summary>The stored credential, or null if this install has never paired.</summary>
    string? Read();

    /// <summary>Replaces the stored credential.</summary>
    void Write(string credential);

    /// <summary>Forgets the credential, so the next start has to pair again.</summary>
    void Clear();
}

/// <summary>
/// Stores a secret encrypted with the Windows Data Protection API.
/// </summary>
/// <remarks>
/// The requirements ask for secure Windows storage. DPAPI ties the ciphertext to
/// the current user account, so the file is unreadable to another account on the
/// same machine and to anyone who copies it elsewhere — which is the realistic
/// threat for a file sitting in a profile directory.
///
/// This is not protection against the user's own account being compromised.
/// Nothing stored on a machine can be, and pretending otherwise would be worse
/// than saying so: that is what <c>/au capture revoke</c> is for, and for the bot
/// token, resetting it in the Discord developer portal.
/// </remarks>
[SupportedOSPlatform("windows")]
public sealed class DpapiCredentialStore : ICredentialStore
{
    /// <summary>The purpose capture's credential is stored under.</summary>
    public const string CapturePurpose = "AUVC capture credential v1";

    /// <summary>The purpose the bot token is stored under when the bot runs on this PC.</summary>
    public const string BotTokenPurpose = "AUVC bot token v1";

    private readonly string _path;
    private readonly byte[] _entropy;

    /// <summary>
    /// Creates a store. The default location is under the user's local
    /// application data, which is per-user and not roamed to other machines.
    /// </summary>
    /// <param name="purpose">
    /// Mixed into the protection as extra entropy. It is not a secret and does not
    /// need to be: it scopes the ciphertext to one use, so a blob protected for
    /// another purpose, by this program or any other for the same user, cannot be
    /// read back as this one.
    /// </param>
    public DpapiCredentialStore(string? path = null, string purpose = CapturePurpose)
    {
        if (string.IsNullOrWhiteSpace(purpose))
        {
            throw new ArgumentException("a purpose is required", nameof(purpose));
        }

        _path = path ?? DefaultPath();
        _entropy = Encoding.UTF8.GetBytes(purpose);
    }

    /// <summary>The file the secret is kept in.</summary>
    public string Path => _path;

    /// <summary>Where capture's credential is kept.</summary>
    public static string DefaultPath() =>
        System.IO.Path.Combine(
            Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData),
            "AUVC",
            "credential.bin");

    /// <summary>Where the bot token is kept when the bot runs on this PC.</summary>
    public static string BotTokenPath() =>
        System.IO.Path.Combine(
            Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData),
            "AUVC",
            "bot-token.bin");

    public string? Read()
    {
        if (!File.Exists(_path))
        {
            return null;
        }

        try
        {
            var plaintext = ProtectedData.Unprotect(
                File.ReadAllBytes(_path), _entropy, DataProtectionScope.CurrentUser);
            return Encoding.UTF8.GetString(plaintext);
        }
        catch (CryptographicException)
        {
            // The file belongs to another user, another machine or another
            // purpose, or it is damaged. Either way there is nothing usable here,
            // and saying so is more useful than a crash on start.
            return null;
        }
        catch (IOException)
        {
            return null;
        }
    }

    public void Write(string credential)
    {
        if (string.IsNullOrEmpty(credential))
        {
            throw new ArgumentException("refusing to store an empty credential", nameof(credential));
        }

        var directory = System.IO.Path.GetDirectoryName(_path);
        if (!string.IsNullOrEmpty(directory))
        {
            Directory.CreateDirectory(directory);
        }

        var ciphertext = ProtectedData.Protect(
            Encoding.UTF8.GetBytes(credential), _entropy, DataProtectionScope.CurrentUser);

        // Written to a temporary file and moved into place, so a crash halfway
        // through leaves the previous value intact rather than a truncated file
        // that reads as "never stored".
        var temporary = _path + ".new";
        File.WriteAllBytes(temporary, ciphertext);
        File.Move(temporary, _path, overwrite: true);
    }

    public void Clear()
    {
        try
        {
            File.Delete(_path);
        }
        catch (IOException)
        {
            // Nothing to forget, or the file is held open. Either way the
            // credential the bot holds is what decides access, and that is
            // revoked from Discord.
        }
    }
}

/// <summary>
/// Keeps a credential in memory only. Useful for tests and for a run that
/// should deliberately not leave anything behind.
/// </summary>
public sealed class InMemoryCredentialStore : ICredentialStore
{
    private string? _credential;

    public string? Read() => _credential;

    public void Write(string credential) => _credential = credential;

    public void Clear() => _credential = null;
}
