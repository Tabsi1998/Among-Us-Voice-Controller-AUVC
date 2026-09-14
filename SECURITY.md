# Security

## Supported versions

AUVC is in pre-release. Security fixes land on `main` and in the next
pre-release; only the newest release is supported.

## Reporting a vulnerability

Use the repository's **Security > Report a vulnerability** feature. If it is
unavailable, contact the repository owner privately through an existing trusted
channel to arrange disclosure. Do not post credentials, exploit details or
personal data in public issues.

## What is protected, and how

**The bot token.** The app checks a pasted token with Discord and stores it with
the Windows Data Protection API, for the current Windows account only. It never
shows it again and never writes it to a log.

**The bot on this PC.** The app starts the bot with a new random secret and a
free loopback port on every start, and ties it to itself with a Windows job
object, so the bot ends when the app ends, even when the app crashes. Only when
started this way does the bot offer the `/local/...` routes the app uses to list
servers and channels, save the setup, link players, stop the bot and obtain a capture
credential. Every route requires the secret, compared in constant time, answers
only connections from this computer, and refuses any request with an `Origin`
header, so a web page cannot use it. A secret shorter than 32 characters stops
the bot from starting.

**Credentials between app and bot.** A credential has 256 bits from
`crypto/rand`. The bot stores only its SHA-256 hash and compares in constant
time; a plain hash is right here, because there is nothing to guess in 256 random
bits. The app stores the credential with the Data Protection API, separately from
the token and under a different purpose, so neither file reads back as the other.
`/au capture revoke` takes effect at once: open connections are told why and
closed, new ones are refused.

**Pairing codes**, for a bot on another computer: eight characters from a
32-letter alphabet (40 bits), working once and for ten minutes, stored only as a
hash. A new code replaces the previous one.

**Nothing secret in logs or replies.** Secrets are held in a type that prints and
serialises as `[redacted]`. Tests assert that neither pairing codes nor
credentials reach the log, `/au doctor`, `/au capture status` or the
configuration export.

**The connection.** Every message is validated against a versioned protocol.
Requests with an `Origin` header are refused, frames are size-limited and idle
connections time out. The bot listens on `127.0.0.1` by default.
`AUVC_CAPTURE_TLS_CERT` and `AUVC_CAPTURE_TLS_KEY` make a bot on another computer
terminate TLS itself; without them it warns on every start.

**Administration.** `/au` changes require the server owner, a Discord
Administrator, or the role set with `/au setup permissions`. Every reply is
visible only to whoever ran the command.

**Voice.** Only linked human players are managed. When the app stops responding,
the default fail-safe unmutes and undeafens everyone rather than leaving a room
unable to speak, and a stopping bot releases everyone too.

**Supply chain.** CI fails on Go code that can reach a known vulnerability
(`govulncheck`), on any vulnerable NuGet package, and on secrets found by
Gitleaks. GitHub Actions are pinned to commit SHAs.

## Known limitations

- **Tried in a few live rounds only.** App and bot have been through public
  lobbies with Among Us and Discord, but not yet through the full smoke test in
  `docs/acceptance.md` that `v1.0.0` requires.
- **Files are unsigned.** Windows shows "Unknown publisher". `SHA256SUMS` proves
  a download matches the release; it does not prove who built it.
- **Plain HTTP to a bot on another computer** sends the credential in the clear.
  Use TLS whenever app and bot are not on the same machine.
- **The pairing endpoint is not rate limited.** A wrong code does not use up the
  real one. The defence is the code's entropy, its short life, and the listener
  defaulting to localhost.
- **The Data Protection API does not protect against a compromised Windows
  account.** Reset the bot token in the Discord Developer Portal, and use
  `/au capture revoke`, if the PC is compromised.

## Privacy

What AUVC stores, where and for how long is in [docs/privacy.md](docs/privacy.md).

## Contributor rules

- Never commit real Discord tokens, capture credentials, pairing secrets,
  private keys, populated environment files or runtime databases.
- Keep runtime configuration outside source control; use placeholders in examples.
- Redact credentials in logs, screenshots, issue reports and test artifacts.
- Run Gitleaks with `--redact` before pushing. Investigate findings; do not
  exempt entire source trees.
- If a secret is exposed, revoke or rotate it immediately and notify the owner
  privately. Do not rewrite main without explicit incident authorization.
- Never weaken or remove tests to make a build pass.
