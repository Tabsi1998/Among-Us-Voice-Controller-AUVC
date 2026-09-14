-- The message in the control channel where players choose their crewmate.
--
-- The bot edits one message per guild rather than posting a new one on every
-- change, so it has to remember which message that is. Keeping it here rather
-- than in memory means a bot that did not shut down cleanly finds its old
-- message again instead of leaving it behind with a menu that no longer works.

CREATE TABLE crewmate_board (
    guild_id   TEXT    NOT NULL PRIMARY KEY,
    channel_id TEXT    NOT NULL,
    message_id TEXT    NOT NULL,
    updated_at INTEGER NOT NULL
) STRICT;
