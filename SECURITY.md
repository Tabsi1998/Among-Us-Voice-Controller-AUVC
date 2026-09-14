# Security

## Supported versions

No AUVC release has been published yet. `main` is under active development and
security fixes land there. From the first release on, the latest release is
supported.

## Reporting a vulnerability

Use the repository's **Security > Report a vulnerability** feature. If it is
unavailable, contact the repository owner privately through an existing trusted
channel to arrange disclosure. Do not post credentials, exploit details or
personal data in public issues.

## What is protected, and how

**Pairing.** `/au capture pair` issues an eight-character code from a 32-letter
alphabet (40 bits) that works once and expires after ten minutes. Only its
SHA-256 hash is stored. Requesting a new code replaces the previous one, and
`/au capture revoke` cancels an outstanding code as well as every credential.

**Credentials.** Redeeming a code issues a credential with 256 bits from
`crypto/rand`. Only its SHA-256 hash is stored, and comparison is constant time.
A plain hash is the right tool here rather than a password hash: there is nothing
to guess in 256 random bits, and password hashes exist to slow down guessing
cheap secrets. Revocation takes effect on the next connection attempt.

**Nothing secret in logs or replies.** Secrets are held in a type that prints and
serialises as `[redacted]`; reading the value takes an explicit call that is easy
to find in review. Tests assert that neither pairing codes nor credentials reach
the log, `/au doctor`, `/au capture status` or the configuration export.

**The capture connection.** Every message is validated against a versioned
protocol. A failed or revoked credential closes the connection. Requests carrying
an `Origin` header are refused, so a web page cannot reach the handshake. Frames
are size-limited and idle connections time out.

**Transport security.** `AUVC_CAPTURE_TLS_CERT` and `AUVC_CAPTURE_TLS_KEY` make
the bot terminate TLS itself. Without them it serves plain HTTP and says so in
its log on every start. The listener binds to `127.0.0.1` by default.

**On the capture PC.** The credential is encrypted with the Windows Data
Protection API for the current user account, with application-specific entropy.

**Administration.** `/au` changes require the guild owner, Discord Administrator,
or the role set with `/au setup permissions`. Every reply is ephemeral.

**Voice.** Only linked human players are managed. When capture stops responding,
the default fail-safe unmutes and undeafens everyone rather than leaving a room
unable to speak.

**Supply chain.** CI fails on Go code that can reach a known vulnerability
(`govulncheck`), on any vulnerable NuGet package, and on secrets found by
Gitleaks. GitHub Actions are pinned to commit SHAs.

## Known limitations

- **The capture app does not use the authenticated transport yet.** It still
  contains the upstream socket connection, which the bot no longer serves, so it
  cannot connect at all. The protections above apply to the bot and to the
  capture libraries; they reach a real capture install once the app is wired to
  them.
- **Revoking does not end a connection that is already open.** The credential is
  checked at the handshake, so a capture that is connected when
  `/au capture revoke` runs keeps working until its connection drops, and
  heartbeats keep an active connection alive indefinitely. Restart the bot after
  revoking a credential you believe is compromised. Closing open connections on
  revoke is the next fix.
- **Artifacts are unsigned** (#24). Windows shows "Unknown publisher".
  `SHA256SUMS` on the release page proves a download matches the release; it does
  not prove who built it.
- **Plain HTTP without TLS or a proxy** sends the credential in the clear. Use
  TLS whenever capture and the bot are not on the same machine.
- **The pairing endpoint is not rate limited.** A wrong code does not use up the
  real one, so that a typing mistake costs nothing. Somebody who can reach the
  endpoint can therefore guess repeatedly during a code's ten-minute life. The
  defence is the code's entropy, its short life, and the listener defaulting to
  localhost; put a rate-limiting proxy in front of an exposed listener.
- **Data Protection API encryption does not protect against a compromised user
  account** on the capture PC. That is what `/au capture revoke` is for.

## Privacy

What AUVC stores, where and for how long is in [docs/privacy.md](docs/privacy.md).
The imported bot's [privacy document](bot/PRIVACY.md) describes the hosted
upstream service and is not an AUVC policy.

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
