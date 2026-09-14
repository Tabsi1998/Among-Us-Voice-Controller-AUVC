-- Voice changes AUVC made to a member that are still in effect.
--
-- Discord never lifts a server mute or deafen on its own. The bot writes down
-- what it is about to set before it sets it, and removes the record once it has
-- lifted it again. A bot that did not stop cleanly, because the app crashed or
-- was killed, therefore finds on its next start exactly whom it left muted,
-- deafened or in the ghost channel, and releases that and nothing else: a
-- member an administrator muted by hand has no record.

CREATE TABLE voice_hold (
    guild_id         TEXT    NOT NULL,
    user_id          TEXT    NOT NULL,
    muted            INTEGER NOT NULL DEFAULT 0 CHECK (muted IN (0, 1)),
    deafened         INTEGER NOT NULL DEFAULT 0 CHECK (deafened IN (0, 1)),
    -- The ghost channel AUVC moved the member into, or empty.
    ghost_channel_id TEXT    NOT NULL DEFAULT '',
    updated_at       INTEGER NOT NULL,
    PRIMARY KEY (guild_id, user_id)
) STRICT;
