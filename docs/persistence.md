# Persistence

Record for
[issue #5](https://github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/issues/5).

AUVC stores guild configuration and Discord player links in SQLite at
`/data/amongus.db`, mounted as a Docker volume. This replaces the upstream
PostgreSQL and Redis storage that phase 14 removes.

## Driver choice

`modernc.org/sqlite`, a pure-Go implementation, not the more common
cgo-based `mattn/go-sqlite3`.

`deploy/Dockerfile.baseline` builds with `CGO_ENABLED=0`. A cgo driver would not
link there at all, and switching the image to cgo would mean shipping a C
toolchain and losing the static binary. The pure-Go driver is verified to build
both with `CGO_ENABLED=0` and cross-compiled to `linux/amd64`.

## Schema

Version 1, in `bot/pkg/storage/sqlite/migrations/0001_initial_schema.sql`.

`guild_config` stores every field the requirements list: `guild_id`, `enabled`,
the main, ghost and control channel ids, `admin_role_id`, `voice_policy`,
`auto_move_ghosts`, `enforce_channels`, `capture_timeout_seconds`,
`capture_timeout_action`, `auto_start`, `config_version`, `created_at` and
`updated_at`.

`player_link` maps an Among Us player name to a Discord user per guild, keyed on
`(guild_id, in_game_name)` so a name resolves to exactly one user. Restart
recovery reads it back.

Both tables are `STRICT`, so a wrong Go type is rejected at write time instead of
being silently coerced. Timestamps are Unix seconds as `INTEGER`: unambiguous
across time zones and directly comparable.

### Defaults

`enforce_channels` defaults to **true**, `voice_policy` to `ghost-chat` and
`capture_timeout_action` to `fail-open`, as the requirements specify.

The defaults exist twice, in the SQL columns and in `DefaultGuildConfig`. A test
inserts a row supplying only the columns that have no default and compares it to
the Go value, so the two cannot drift apart unnoticed.

## Migration policy

Migrations are embedded SQL files named `NNNN_description.sql`. Each runs in its
own transaction and is recorded in `schema_migrations`, so opening an existing
database applies only what is new.

Two guards:

- **Versions must be gapless and start at 1.** A test asserts it, so a
  mis-numbered file fails the build rather than being skipped at runtime.
- **A database newer than the binary is refused.** Running an old build against
  a newer schema is a common way to corrupt data, so `Open` fails with an
  explicit message instead of proceeding.

### Destructive migrations

There are none yet, and none may be added without following this procedure:

1. State in the pull request what data is lost or transformed, and why an
   additive migration cannot achieve the same result.
2. Ship the backup step in the same change. SQLite backs up by copying the
   database file while the bot is stopped, or with `VACUUM INTO 'backup.db'`
   while it runs.
3. Document the recovery path: stop the bot, replace `/data/amongus.db` with the
   backup, start the previous AUVC version. The newer-database guard means a
   restored older file is accepted by the older binary.
4. Never drop or rewrite a column in the same release that stops writing it.
   Stop writing first, release, then remove in a later migration once the
   previous version is no longer in use.

## Backups

The database is a single file in the Docker volume. A backup is a copy of that
file taken while the bot is stopped, or `VACUUM INTO` while it runs; WAL mode
means copying the file alone during operation can miss recent writes.

## Status

The package is complete and tested but not yet wired into the bot. Guild
settings still come from the legacy Redis and PostgreSQL paths; moving them over
happens with the `/au` command work in phase 7 and the final service removal in
phase 14.
