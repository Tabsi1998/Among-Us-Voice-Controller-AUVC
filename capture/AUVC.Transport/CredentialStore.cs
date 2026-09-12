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
/// Stores the credential encrypted with the Windows Data Protection API.
/// </summary>
/// <remarks>
/// The requirements ask for secure Windows storage. DPAPI ties the ciphertext to
/// the current user account, so the file is unreadable to another account on the
/// same machine and to anyone who copies it elsewhere — which is the realistic
/// threat for a file sitting in a profile directory.
///
/// This is not protection against the user's own account being compromised.
/// Nothing stored on a machine can be, and pretending otherwise would be worse
/// than saying so: that is what <c>/au capture revoke</c> is for.
/// </remarks>
[SupportedOSPlatform("windows")]
public sealed class DpapiCredentialStore : ICredentialStore
{
    // Extra entropy mixed into the protection. It is not a secret and does not
    // need to be: it scopes the ciphertext to this application, so a blob
    // protected by some other program for the same user cannot be fed in here.
    private static readonly byte[] Entropy = Encoding.UTF8.GetBytes("AUVC capture credential v1");

    private readonly string _path;

    /// <summary>
    /// Creates a store. The default location is under the user's local
    /// application data, which is per-user and not roamed to other machines.
    /// </summary>
    public DpapiCredentialStore(string? path = null)
    {
        _path = path ?? DefaultPath();
    }

    /// <summary>The file the credential is kept in.</summary>
    public string Path => _path;

    public static string DefaultPath() =>
        System.IO.Path.Combine(
            Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData),
            "AUVC",
            "credential.bin");

    public string? Read()
    {
        if (!File.Exists(_path))
        {
            return null;
        }

        try
        {
            var plaintext = ProtectedData.Unprotect(
                File.ReadAllBytes(_path), Entropy, DataProtectionScope.CurrentUser);
            return Encoding.UTF8.GetString(plaintext);
        }
        catch (CryptographicException)
        {
            // The file belongs to another user or another machine, or it is
            // damaged. Either way there is no credential here, and saying so is
            // more useful than a crash on start: capture asks to pair again.
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
            Encoding.UTF8.GetBytes(credential), Entropy, DataProtectionScope.CurrentUser);

        // Written to a temporary file and moved into place, so a crash halfway
        // through leaves the previous credential intact rather than a truncated
        // file that reads as "never paired".
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
