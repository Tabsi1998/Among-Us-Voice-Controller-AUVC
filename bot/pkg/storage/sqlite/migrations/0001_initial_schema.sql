-- Guild configuration and player links, the two things AUVC must not lose
-- across a bot or container restart.
--
-- STRICT tables are used so a wrong Go type is rejected at write time instead
-- of being silently coerced. Timestamps are Unix seconds as INTEGER: unambiguous
-- across time zones and trivially comparable.

CREATE TABLE guild_config (
    guild_id                TEXT    NOT NULL PRIMARY KEY,
    enabled                 INTEGER NOT NULL DEFAULT 1,
    main_voice_channel_id   TEXT    NOT NULL DEFAULT '',
    ghost_voice_channel_id  TEXT    NOT NULL DEFAULT '',
    control_text_channel_id TEXT    NOT NULL DEFAULT '',
    admin_role_id           TEXT    NOT NULL DEFAULT '',
    voice_policy            TEXT    NOT NULL DEFAULT 'ghost-chat',
    auto_move_ghosts        INTEGER NOT NULL DEFAULT 1,
    -- The requirements set enforce_channels to true by default.
    enforce_channels        INTEGER NOT NULL DEFAULT 1,
    capture_timeout_seconds INTEGER NOT NULL DEFAULT 60,
    -- Recovery defaults to fail-open: on a capture timeout players are released
    -- rather than left muted.
    capture_timeout_action  TEXT    NOT NULL DEFAULT 'fail-open',
    auto_start              INTEGER NOT NULL DEFAULT 0,
    config_version          INTEGER NOT NULL DEFAULT 1,
    created_at              INTEGER NOT NULL,
    updated_at              INTEGER NOT NULL
) STRICT;

-- One Among Us player name maps to at most one Discord user per guild, which is
-- what the primary key enforces. Restart recovery reads this back.
CREATE TABLE player_link (
    guild_id        TEXT    NOT NULL,
    in_game_name    TEXT    NOT NULL,
    discord_user_id TEXT    NOT NULL,
    updated_at      INTEGER NOT NULL,
    PRIMARY KEY (guild_id, in_game_name)
) STRICT;

CREATE INDEX player_link_by_user ON player_link (guild_id, discord_user_id);
