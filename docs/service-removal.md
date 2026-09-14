# Public service and premium removal

Inventory and record for
[issue #4](https://github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/issues/4).

Upstream AutoMuteUs ran as a large public multi-tenant service: many guilds, a
worker fleet, paid tiers, a public HTTP API and Kubernetes/Prometheus operations.
AUVC is a single self-hosted bot for one community. Everything that exists only
to operate that public deployment has no counterpart here.

This document began as the inventory for phase 4. Galactus, Redis and PostgreSQL
were out of scope for that phase because they still did real work. Phase 14
removed them once SQLite and the authenticated WebSocket had taken over:
PostgreSQL in
[#82](https://github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/pull/82), and
Redis, the Galactus token provider and the legacy commands in
[#86](https://github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/pull/86).

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
| Premium tiers | `pkg/premium/`, `bot/command/premium.go`, `pkg/storage/premium.go` | Paid feature gating | Removed |
| Premium storage | `pkg/storage/postgres.go`, `types.go` | Tier and expiry records | Removed |
| top.gg integration | `bot/bot.go`, `pkg/storage/postgres.go` | Grant premium for bot-list votes | Removed |
| Official-mode switch | `AUTOMUTEUS_OFFICIAL` in `main.go`, `bot.official` | Separate the hosted bot from self-hosters | Removed |
| Sharding | `NUM_SHARDS`, `SHARDS`, `parseShards` in `main.go` | Spread guilds across gateway shards | Removed |
| Galactus / Redis | `pkg/rediskey/`, `storage/redis.go`, `common/redis.go` | State, locking, capture transport | Phase 14 |
| PostgreSQL | `pkg/storage/` | Guild config, stats, premium | Phase 14 |

These were entangled with each other: the premium lookup took both the official
flag and the top.gg client, and the sharding guards shared a condition with the
official flag. They were therefore removed together in a second step rather than
split into partial, non-compiling ones.

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

`EXPOSE 5000 8080 2112` was dropped from `deploy/Dockerfile` because all
three ports are gone.

### Worker bot token pool

Mute and deafen work was spread across a pool of extra bot tokens, sized per
premium tier by `PremiumBotConstraints`. `ModifyUsers` now attempts the mute on
the capture bot and otherwise applies it on the primary session. The capture
path stays until phases 12 and 13 replace the transport.

`MAX_REQ_5_SEC` is deliberately kept: `maxRequests5Seconds` still bounds
`IncrAndTestGuildTokenComboLock` and `BlacklistTokenForDuration` on that path.

## Removed in the second step

### Premium

Upstream gated features behind paid tiers. The decisive detail is that
`GetGuildOrUserPremiumStatus` opened with `if !official { return SelfHostTier,
NoExpiryCode }`: **self-hosters already received the full feature set
unconditionally.** Removing the gates is therefore behaviour-preserving for
AUVC's target deployment. Every gate was resolved to the branch a self-hoster
already took, so no feature disappears:

| Gate | Previous self-host behaviour | Now |
| --- | --- | --- |
| `/settings` premium settings | All settings available | Unconditional |
| Settings embed | Split into free and 💎 premium sections | One flat list |
| `/stats` detailed stats | Shown | Unconditional |
| `/download` Gold requirement | Passed | Unconditional |
| `/new` active-game lockout | Never triggered (only free tier) | Removed |
| `/premium` command | Displayed tier info | Removed |

Deleted: `pkg/premium/`, `bot/command/premium.go`, `pkg/storage/premium.go` and
its tests, the premium lookup chain in `pkg/storage/postgres.go`
(`isUserPremium`, `GetGuildOrUserPremiumStatus`, `guildOrUserPremium`,
`getGuildPremiumStatus`), the `Premium` field on `task.UserModifyRequest`, the
`Premium` flag on `setting.Setting`, and the `isPrem` parameters on the three
stats embeds.

### top.gg

Premium could also be earned by voting on top.gg. `TOP_GG_TOKEN`, the `dbl`
client, `TopGGID` and `setUserVoteTime` are gone, and `github.com/top-gg/go-dbl`
left the module.

### Official-mode switch and sharding

`AUTOMUTEUS_OFFICIAL` separated the hosted bot from self-hosters. For a
self-hoster it was always unset, so every guard it controlled took the same
branch: the schema was always applied, and slash commands were always registered
and deregistered. Those guards are now unconditional.

`NUM_SHARDS`, `SHARD_ID` and `SHARDS` spread guilds across gateway shards. AUVC
serves one community from one process, so `main.go` now starts a single bot
instead of a slice, and `parseShards`, `defaultShard` and `isPrimaryShard` are
gone.

### Tests

Three tests covering deleted functions were removed:
`TestIsUserPremium`, `TestIsUserPremium_nilTopGG` and `TestIsUserOrGuildPremium`.
No remaining test was weakened. `TestPostgresGuild_ToCSV` keeps its exact
assertions; only the tier constant became the literal `5` it already expected.

### Deliberately kept

`PostgresGuild.Premium` stays as a struct field. The `guilds` table still has a
`premium` column and `pgxscan` reads it with `SELECT *`, so removing the field
would break scanning. The column disappears with the schema in phase 14.

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
is untouched.

One legacy string is knowingly left in place. `bot/setting/muteSpectators.go`
still warns about delays "when not self-hosting, or using a Premium worker bot".
The message exists in ten locale files, so rewriting only the English fallback
would leave nine stale translations. Product-facing legacy wording is the subject
of [#33](https://github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/issues/33),
which reworks these strings as a set. Self-hosting still requires Redis and PostgreSQL, so "self-hosting
without these services" in the issue's acceptance criteria cannot be demonstrated
until phase 14. Runtime verification against a live Discord guild needs the
owner's bot token and is not part of automated validation.
