# Installer framework decision

The release contract asks for this choice to be made deliberately rather than by
whatever happened to be at hand, and specifically warns against picking a tool
because it can wrap a batch file. This records what was chosen and why.

## Decision

**Inno Setup** builds `AmongUsVoiceCapture-Setup-win-x64.exe`.

## What was weighed

| | Inno Setup | WiX / MSI | MSIX | Squirrel |
| --- | --- | --- | --- | --- |
| Per-user install without elevation | yes | possible, awkward | yes | yes |
| Runs unsigned | yes, with a SmartScreen warning | yes, with a warning | **no** | yes |
| Version string | free text, SemVer fits | three numeric fields only | four numeric fields | free text |
| Uninstall with a data choice | scripted, straightforward | custom actions | limited | limited |
| CI on a Windows runner | one command | one command | needs a signing identity | node toolchain |
| Licence and maintenance | free, actively maintained, no runtime | free, Microsoft-backed | part of Windows | thin, .NET-focused |

**MSIX is out, and that is the decisive point.** Windows refuses to install an
unsigned MSIX at all. The owner has decided to ship unsigned until publisher
identity and code signing are settled (#24), so a format that cannot be
installed unsigned cannot be the format we ship first.

**MSI is deferred rather than rejected.** The release contract lists an MSI for
managed installation, and its `ProductVersion` takes three numeric fields and
ignores a fourth, so `1.2.3-rc.1` has no direct representation. That mapping
needs to be designed and tested, not guessed, and it is not needed for a first
release. Inno Setup's version is free text, so SemVer travels unchanged and the
question can be answered when an MSI is actually required.

**Squirrel** is built around silent self-updating, which is the part AUVC does
not have and deliberately removed in favour of one that verifies AUVC's own
signatures when there are any.

Inno Setup wins on the thing that matters now: it installs per-user without
elevation, it runs unsigned, it scripts the uninstall-time data choice the
contract requires, and building it is one command on a Windows runner.

## Unsigned, and what that means for whoever downloads it

The installer is not signed. Windows SmartScreen shows **"Windows protected your
PC — Unknown publisher"**, and the person has to click **More info** and then
**Run anyway**.

That is a real cost and it is stated plainly here and in the download
instructions, because the failure mode of an undocumented warning is worse than
the warning: somebody who was not expecting it assumes the download is broken or
malicious, and somebody who was expecting it clicks through anything that looks
similar.

A signature does not promise that software is safe. It says who published it and
that nobody changed it since. Until AUVC has a publisher identity, `SHA256SUMS`
published beside the artifact is what a careful person can check instead, which
is weaker — it proves the file matches what the release page says, not who built
it.

Signing is tracked in #24. When it arrives, nothing about the installer changes
except that it is signed.

## What the installer does

- Installs per user, under `%LOCALAPPDATA%\Programs\AUVC Capture`, with no
  elevation prompt.
- Refuses to install over a running capture, rather than replacing files in use
  and failing halfway.
- Upgrades in place and **keeps settings and the paired credential**. An upgrade
  that made a working install pair again would be an upgrade people avoid.
- Uninstall asks, separately and explicitly, whether to delete local data.
  Removing the credential from this PC is **not** the same as revoking access:
  the bot still holds it until somebody runs `/au capture revoke` in Discord.
  The uninstaller says so, because the two are easy to confuse and only one of
  them actually closes the door.

## Portable mode

`AmongUsVoiceCapture-win-x64.zip` stays available and installs nothing. It keeps
its settings and credential in the same per-user locations as the installed
build, so the two are the same application started two ways rather than two
applications. Neither self-updates.
