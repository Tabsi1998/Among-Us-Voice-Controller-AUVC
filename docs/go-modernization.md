# Go and DiscordGo modernization

Phase 3 record for [issue #3](https://github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/issues/3).
It documents the selected toolchain, every dependency that moved, the breaking
changes that had to be resolved, and the dependencies that were deliberately
left at baseline versions.

## Selected toolchain

| Item | Before | After |
| --- | --- | --- |
| `bot/go.mod` directive | `go 1.19` | `go 1.27.0` |
| CI (`.github/workflows/baseline.yml`) | `1.19.13` | `1.27.1` |
| `deploy/Dockerfile` build stage | `golang:1.19.13-alpine` | `golang:1.27.1-alpine` |

Go 1.19 reached end of life long ago and receives no security fixes. Go supports
the two most recent major releases, currently 1.27 and 1.26; 1.27.1 is the newest
patch release and is therefore the supported choice.

All three places move together on purpose. Splitting them is what made the
Dependabot updates unmergeable: Dependabot raised the module directive while CI
kept a pinned 1.19.13 toolchain with `GOTOOLCHAIN=local`, producing
`invalid go version '1.25.0': must match format 1.23`. Keeping the module
directive at `1.27.0` and the toolchain at `1.27.1` lets `GOTOOLCHAIN=local`
resolve without downloading a second toolchain during CI.

## Dependency policy

AUVC removes the public-service, premium, Redis and PostgreSQL infrastructure in
[#4](https://github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/issues/4) and
[#14](https://github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/issues/14).
Updating packages that only serve that code would mean absorbing breaking-change
risk for code with a scheduled deletion date. This phase therefore updates only
what AUVC keeps, plus security-relevant shared libraries.

### Updated

| Module | Before | After | Reason |
| --- | --- | --- | --- |
| `github.com/bwmarrin/discordgo` | v0.27.1 | v0.29.0 | Core Discord library; every later phase builds on it |
| `github.com/nicksnyder/go-i18n/v2` | v2.2.1 | v2.6.1 | Localization stays; German/English UI is a product requirement |
| `github.com/BurntSushi/toml` | v1.1.0 | v1.6.0 | Backs the localization message files |
| `golang.org/x/text` | v0.5.0 | v0.42.0 | Localization; also the module Dependabot flagged |
| `golang.org/x/net` | v0.4.0 | v0.59.0 | Security-relevant shared library |
| `golang.org/x/crypto` | 2022-07-22 pseudo | v0.57.0 | Security-relevant shared library |
| `golang.org/x/sys` | v0.3.0 | v0.48.0 | Security-relevant shared library |
| `golang.org/x/time` | 2019-10-24 pseudo | v0.16.0 | Security-relevant shared library |
| `golang.org/x/tools` | v0.2.0 | v0.49.0 | Transitive, pulled by the above |

### Deliberately not updated

| Module | Used by | Removed in |
| --- | --- | --- |
| `github.com/go-redis/redis/v8` | `bot/eventHandler.go`, `bot/redis.go`, `bot/tokenprovider/`, `common/redis.go`, `internal/server/metrics.go`, `pkg/capture/event.go`, `pkg/rediskey/`, `pkg/task/jobs.go`, `pkg/token/redis.go`, `storage/redis.go` | #4, #14 |
| `github.com/bsm/redislock` | `bot/message_handlers.go`, `bot/redis.go`, `bot/tokenprovider/provider.go`, `bot/voice.go` | #4, #14 |
| `github.com/jackc/*`, `github.com/georgysavva/scany`, `github.com/pashagolub/pgxmock` | `pkg/storage/`, `pkg/rediskey/` | #4, #14 |
| `github.com/prometheus/client_golang` | `internal/server/metrics.go` | #4 |
| `github.com/gin-gonic/gin`, `github.com/swaggo/*` | `bot/api.go`, `docs/docs.go` (generated) | #4 |
| `github.com/gorilla/mux` | `internal/server/healthcheck.go` | #4 |
| `github.com/top-gg/go-dbl` | `bot/bot.go`, `pkg/storage/postgres.go` | #4 |

Note that Redis is not confined to a service package: `bsm/redislock` reaches
into `bot/voice.go` and `go-redis` into `pkg/capture/event.go`. That coupling is
exactly what #4 and the domain separation in
[#7](https://github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/issues/7) have to
unpick. Upgrading `go-redis` v8 to v9 now would mean taking a breaking change in
files that are about to be restructured or deleted, so their known advisories are
resolved by removal rather than by upgrade. This is a recorded decision, not an
oversight; if a later phase changes that plan, these modules move in that phase.

## Breaking changes resolved

`discordgo` v0.28.0 changed `ComponentEmoji` fields and `MessageEdit.Components`
to pointers so that an empty value can be distinguished from an omitted one.
Five call sites needed adjusting:

| Location | Change |
| --- | --- |
| `bot/emoji.go:119` | `Emoji: discordgo.ComponentEmoji{…}` to `&discordgo.ComponentEmoji{…}` |
| `bot/emoji.go:129` | same |
| `bot/slash_commands.go:932` | same |
| `bot/slash_commands.go:941` | same |
| `bot/slash_commands.go:914` | `me.Components = []discordgo.MessageComponent{}` to `&[]discordgo.MessageComponent{}` |

The last one is behaviour-preserving on purpose: a pointer to an empty slice
still clears the components of the parent message, which is what
`deleteComponentInParentMessage` must keep doing so RESET/Cancel buttons cannot
be clicked twice.

No other source change was required. Memory offsets, capture code and the voice
logic are untouched by this phase.

## Validation

Run from `bot/` with Go 1.27.1:

```sh
gofmt -l .
go vet ./...
go test ./...
go build ./...
```

All four were clean on the branch head. `go test -race ./...` runs in Linux CI;
it needs a C toolchain and was not executed on the Windows workstation. The
Docker build stage is likewise verified by CI only, because Docker is not
installed on the workstation used for this phase.

## Known limitations

- The bot still contains the upstream public-service, premium, Redis and
  PostgreSQL code. This phase modernizes the toolchain, not the architecture.
- `golang.org/x/exp` stays at its 2023 pseudo-version; it is only used for
  generic helpers and carries no advisory.
- The capture application is unaffected. Its .NET migration is phase 11
  ([#10](https://github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/issues/10)).
