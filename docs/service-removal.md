# Public service and premium removal

Inventory and record for
[issue #4](https://github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/issues/4).

Upstream AutoMuteUs ran as a large public multi-tenant service: many guilds, a
worker fleet, paid tiers, a public HTTP API and Kubernetes/Prometheus operations.
AUVC is a single self-hosted bot for one community. Everything that exists only
to operate that public deployment has no counterpart here.

Removal is staged. Galactus, Redis and PostgreSQL are explicitly **not** part of
this phase: they still perform real work and are removed in phase 14, once SQLite
and the direct authenticated WebSocket take over
([#14](https://github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/issues/14)).

## Inventory

| Component | Where | Purpose upstream | Status |
| --- | --- | --- | --- |
| Public HTTP API | `bot/api.go` | Query guild/premium state over HTTP | Removed |
| Generated Swagger | `docs/docs.go` | API documentation, only imported by `bot/api.go` | Removed |
| Prometheus metrics | `internal/server/metrics.go` | Public deployment observability on `:2112` | Removed |
| Request telemetry | `RecordDiscordRequests`, 12 call sites | Redis counters feeding the metrics endpoint | Removed |
| Mute/deafen counters | `MuteDeafenSuccessCounts` | Split mute attribution for those metrics | Removed |
| Kubernetes probes | `internal/server/healthcheck.go` | `/live` and `/ready` on `:8080` | Removed |
| Worker bot token pool | `bot/tokenprovider/` | Extra Discord tokens to raise rate limits | Removed |
| Worker membership check | `bot/tokenprovider/verify.go` | Make surplus worker bots leave guilds | Removed |
| Premium tiers | `pkg/premium/`, `bot/command/premium.go`, `pkg/storage/premium.go` | Paid feature gating | Planned |
| Premium storage | `pkg/storage/postgres.go`, `types.go` | Tier and expiry records | Planned |
| top.gg integration | `bot/bot.go`, `pkg/storage/postgres.go` | Grant premium for bot-list votes | Planned |
| Official-mode switch | `AUTOMUTEUS_OFFICIAL` in `main.go`, `bot.official` | Separate the hosted bot from self-hosters | Planned |
| Sharding | `NUM_SHARDS`, `SHARDS`, `parseShards` in `main.go` | Spread guilds across gateway shards | Planned |
| Galactus / Redis | `pkg/rediskey/`, `storage/redis.go`, `common/redis.go` | State, locking, capture transport | Phase 14 |
| PostgreSQL | `pkg/storage/` | Guild config, stats, premium | Phase 14 |

"Planned" items are entangled with each other: the premium lookup takes both the
official flag and the top.gg client, and the sharding guards share a condition
with the official flag. They are removed together in the follow-up work package
rather than split into partial, non-compiling steps.

## Removed in this step

### Public HTTP API and Swagger

`bot/api.go` served guild and premium state over HTTP for the hosted deployment.
`docs/` held its generated Swagger definition and was imported from nowhere else.

### Prometheus metrics and Kubernetes probes

`internal/server` provided the `:2112` metrics endpoint and the `:8080`
liveness/readiness probes. `RecordDiscordRequests` incremented Redis counters at
twelve call sites so the metrics endpoint had something to report.

Removing the telemetry is behaviour-preserving: no control flow ever read those
counters. Where a counter formed the entire body of a conditional, the
conditional was collapsed instead of being left empty, and the variables that
only fed it (`deleted`, `created`, `edited`) became plain calls.

`EXPOSE 5000 8080 2112` was dropped from `deploy/Dockerfile.baseline` because all
three ports are gone.

### Worker bot token pool

Mute and deafen work was spread across a pool of extra bot tokens, sized per
premium tier by `PremiumBotConstraints`. `ModifyUsers` now attempts the mute on
the capture bot and otherwise applies it on the primary session. The capture
path stays until phases 12 and 13 replace the transport.

`MAX_REQ_5_SEC` is deliberately kept: `maxRequests5Seconds` still bounds
`IncrAndTestGuildTokenComboLock` and `BlacklistTokenForDuration` on that path.

## Dependency effect

Direct modules dropped: `gin-gonic/gin`, `gorilla/mux`, `prometheus/client_golang`,
`swaggo/files`, `swaggo/gin-swagger`, `swaggo/swag`, plus `golang.org/x/exp`
once the pool's generic helper disappeared. Roughly 29 indirect modules followed.

This resolves the `prometheus/client_golang` Dependabot update by deletion, as
[go-modernization.md](go-modernization.md) predicted. That document's note about
`golang.org/x/exp` staying at a 2023 pseudo-version is now superseded: the module
is gone entirely.

## Verification and limits

`gofmt`, `go vet`, `go build` and `go test` are clean, and the capture test suite
is untouched. Self-hosting still requires Redis and PostgreSQL, so "self-hosting
without these services" in the issue's acceptance criteria cannot be demonstrated
until phase 14. Runtime verification against a live Discord guild needs the
owner's bot token and is not part of automated validation.
