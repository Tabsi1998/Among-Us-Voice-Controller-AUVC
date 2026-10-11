-- /au stats counts a linked player by their Discord account, so a new name in
-- the game keeps the rounds. player_key is "user:<id>" for a linked player and
-- "name:<lowercase name>" for anyone else; last_name is what the list shows.
-- impostor_wins makes the impostor win rate possible.

CREATE TABLE round_stats (
    guild_id        TEXT    NOT NULL,
    player_key      TEXT    NOT NULL,
    discord_user_id TEXT    NOT NULL DEFAULT '',
    last_name       TEXT    NOT NULL,
    rounds          INTEGER NOT NULL DEFAULT 0,
    wins            INTEGER NOT NULL DEFAULT 0,
    impostor        INTEGER NOT NULL DEFAULT 0,
    impostor_wins   INTEGER NOT NULL DEFAULT 0,
    updated_at      INTEGER NOT NULL,
    PRIMARY KEY (guild_id, player_key)
) STRICT;

INSERT INTO round_stats (guild_id, player_key, last_name, rounds, wins, impostor, updated_at)
SELECT guild_id, 'name:' || lower(player_name), player_name, rounds, wins, impostor, updated_at
FROM player_stats;

DROP TABLE player_stats;
