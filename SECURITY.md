# Security

## Supported versions

AUVC is in bootstrap development. No supported AUVC release or production-ready
security boundary exists yet. The imported applications retain legacy upstream
behavior and dependencies. Do not treat the planned pairing or WSS design as
implemented protection.

## Reporting a vulnerability

Use the repository's **Security > Report a vulnerability** feature if available.
If unavailable, contact the repository owner privately through an existing
trusted channel to arrange disclosure. Do not post credentials, exploit details
or personal data in public issues. A dedicated reporting channel must be
established before the first supported release.

## Contributor rules

- Never commit real Discord tokens, capture credentials, pairing secrets,
  private keys, populated environment files or runtime databases.
- Keep runtime configuration outside source control; use placeholders in examples.
- Redact credentials in logs, screenshots, issue reports and test artifacts.
- Run Gitleaks with `--redact` before pushing. Investigate findings; do not
  exempt entire source trees.
- If a secret is exposed, revoke/rotate it immediately and notify the owner
  privately. Do not rewrite main without explicit incident authorization.
- Never weaken or remove tests to make a build pass.

## Planned security boundaries

Expiring single-use pairing codes, revocable long-term credentials, secure
Windows credential storage, WSS authentication, protocol validation and
role-restricted administration are required before deployment.
Only linked human players are managed by default. Capture timeout defaults to
fail-open: unmute, undeafen, pause and warn. See [requirements](docs/requirements.md).

## Privacy

The target design stores per-guild configuration, player links and capture/session
state needed to control Discord voice. It does not require audio recording.
Data retention, diagnostic redaction, export and deletion behavior must be
documented and verified before release. The imported bot's historical
[privacy document](bot/PRIVACY.md) describes upstream and is not an AUVC policy.
