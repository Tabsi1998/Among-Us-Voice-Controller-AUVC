-- Whether the living are also deafened while the tasks run, on top of being
-- muted. Muting is what the game needs; deafening additionally hides members
-- AUVC does not manage, so it stays available as a setting and is off by
-- default (#171).

ALTER TABLE guild_config ADD COLUMN deafen_during_tasks INTEGER NOT NULL DEFAULT 0;
