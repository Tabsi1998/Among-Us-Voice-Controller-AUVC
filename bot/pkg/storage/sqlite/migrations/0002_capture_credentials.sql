-- Capture pairing codes and the long-lived credentials they issue.
--
-- Only hashes are stored. A secret that is never written down cannot leak from
-- a database file, a backup or a support bundle, and nothing in AUVC ever needs
-- the original value again: every check hashes what was presented and compares.

CREATE TABLE capture_pairing (
    -- One outstanding code per guild. Running /au capture pair again replaces
    -- the previous code, which is also how an administrator cancels one they
    -- read out to the wrong person.
    guild_id   TEXT    NOT NULL PRIMARY KEY,
    code_hash  TEXT    NOT NULL,
    -- Who asked for the code, so an unexpected pairing can be traced to a
    -- person rather than to the guild as a whole.
    created_by TEXT    NOT NULL,
    created_at INTEGER NOT NULL,
    expires_at INTEGER NOT NULL
) STRICT;

CREATE TABLE capture_credential (
    -- The identifier is not secret. It names which row to check a presented
    -- secret against, so the bot compares against one hash instead of every
    -- credential it holds.
    id           TEXT    NOT NULL PRIMARY KEY,
    guild_id     TEXT    NOT NULL,
    secret_hash  TEXT    NOT NULL,
    created_at   INTEGER NOT NULL,
    -- Zero until the credential is first used, then the last time it was seen.
    -- This is what /au capture status reports.
    last_seen_at INTEGER NOT NULL DEFAULT 0,
    -- Zero while the credential is valid. Revoked credentials are kept rather
    -- than deleted so that a connection attempt with one can be told apart from
    -- a connection attempt with a credential that never existed.
    revoked_at   INTEGER NOT NULL DEFAULT 0
) STRICT;

CREATE INDEX capture_credential_guild ON capture_credential (guild_id);
