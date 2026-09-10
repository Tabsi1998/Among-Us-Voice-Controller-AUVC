# Self-hosting

Deployment files will be introduced after the runtime and persistence contracts
are established. There is no supported AUVC installation command yet.

Target: one self-hosted bot, SQLite persisted as `/data/amongus.db` in a Docker
volume, and direct authenticated WSS from one Windows capture computer.
No Galactus, Redis, PostgreSQL, premium services or public worker bots in v1.0.0.

The historical Dockerfile remains at [bot/Dockerfile](../bot/Dockerfile).
Its root-relative assumptions and release metadata need review for the monorepo.

Release packaging must provide a bot image, `AmongUsVoiceCapture-win-x64.zip`,
checksums, release notes and upgrade/migration instructions. GHCR repository paths
must be lowercase; intended image path:
`ghcr.io/tabsi1998/amongus-voice-controller`.
No image has been published.
