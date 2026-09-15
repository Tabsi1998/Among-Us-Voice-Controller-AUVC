-- The language AUVC writes in for everybody in a server: the crewmate message
-- and the notices in the text channel. Empty follows the server language set in
-- Discord. Replies only one member sees use that member's own Discord language
-- and do not look at this.

ALTER TABLE guild_config ADD COLUMN language TEXT NOT NULL DEFAULT '';
