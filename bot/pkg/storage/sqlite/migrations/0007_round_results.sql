-- The result posted in the control channel after a round, on by default and
-- switched off with /au settings round, and what every player did in the
-- rounds AUVC saw, for /au stats. Players are kept by their in-game name, the
-- name the round knew them by.

ALTER TABLE guild_config ADD COLUMN post_round_results INTEGER NOT NULL DEFAULT 1;

CREATE TABLE player_stats (
    guild_id    TEXT    NOT NULL,
    player_name TEXT    NOT NULL COLLATE NOCASE,
    rounds      INTEGER NOT NULL DEFAULT 0,
    wins        INTEGER NOT NULL DEFAULT 0,
    impostor    INTEGER NOT NULL DEFAULT 0,
    updated_at  INTEGER NOT NULL,
    PRIMARY KEY (guild_id, player_name)
);
