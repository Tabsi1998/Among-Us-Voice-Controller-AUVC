# Self-hosting

Deployment files will be introduced after the runtime and persistence contracts
are established. There is no supported AUVC installation command yet.

Target: one self-hosted bot, SQLite persisted as `/data/amongus.db` in a Docker
volume, and direct authenticated WSS from one Windows capture computer.
No Galactus, Redis, PostgreSQL, premium services or public worker bots in v1.0.0.

The historical Dockerfile remains at [bot/Dockerfile](../bot/Dockerfile).
The build-only [Dockerfile.baseline](Dockerfile.baseline) supports the monorepo
without requiring a nested Git checkout or an upstream release tag. CI builds it
and checks its non-root user and license files. It still runs the legacy bot;
this is not the final standalone deployment or a published image.

Release packaging must provide a bot image, signed Windows Setup EXE/MSI,
`AmongUsVoiceCapture-win-x64.zip`, checksums, signed update manifests,
SBOM/provenance, release notes and upgrade/migration instructions. GHCR repository paths
must be lowercase; intended image path:
`ghcr.io/tabsi1998/amongus-voice-controller`.
No image has been published.

The [Windows installation and release contract](../docs/windows-installation-and-releases.md)
defines the graphical installer/first-run experience, Stable/Preview channels,
release triggers, update recovery, signing and download access. No installer or
release workflow exists yet. Resolve private-repository artifact access before
community distribution without embedding a shared download token.
