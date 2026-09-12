package sqlite

import (
	"database/sql"
	"errors"
	"fmt"
)

// ErrNoPairing means the guild has no outstanding pairing code.
var ErrNoPairing = errors.New("no pairing code is outstanding for this guild")

// ErrNoCredential means no credential with that identifier exists.
var ErrNoCredential = errors.New("no such capture credential")

// Pairing is an outstanding pairing code. Only the hash is stored: the code
// itself was shown once, in Discord, and is not recoverable from here.
type Pairing struct {
	GuildID   string
	CodeHash  string
	CreatedBy string
	CreatedAt int64
	ExpiresAt int64
}

// CaptureCredential is a long-lived credential a capture install holds.
type CaptureCredential struct {
	ID         string
	GuildID    string
	SecretHash string
	CreatedAt  int64
	LastSeenAt int64
	// RevokedAt is zero while the credential is valid.
	RevokedAt int64
}

// Revoked reports whether the credential has been withdrawn.
func (c CaptureCredential) Revoked() bool { return c.RevokedAt != 0 }

// SavePairing records an outstanding pairing code, replacing any previous one.
//
// Replacing rather than adding is deliberate: a guild has at most one code in
// flight, so running /au capture pair again is how an administrator cancels a
// code they read out to the wrong person.
func (d *DB) SavePairing(pairing Pairing) error {
	if pairing.GuildID == "" || pairing.CodeHash == "" {
		return errors.New("save pairing: guild and code hash are both required")
	}

	if _, err := d.db.Exec(`
		INSERT INTO capture_pairing (guild_id, code_hash, created_by, created_at, expires_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(guild_id) DO UPDATE SET
			code_hash  = excluded.code_hash,
			created_by = excluded.created_by,
			created_at = excluded.created_at,
			expires_at = excluded.expires_at`,
		pairing.GuildID, pairing.CodeHash, pairing.CreatedBy, pairing.CreatedAt, pairing.ExpiresAt,
	); err != nil {
		return fmt.Errorf("save pairing for guild %s: %w", pairing.GuildID, err)
	}
	return nil
}

// Pairing returns the outstanding pairing code for a guild.
func (d *DB) Pairing(guildID string) (Pairing, error) {
	var pairing Pairing
	err := d.db.QueryRow(`
		SELECT guild_id, code_hash, created_by, created_at, expires_at
		FROM capture_pairing WHERE guild_id = ?`, guildID,
	).Scan(&pairing.GuildID, &pairing.CodeHash, &pairing.CreatedBy, &pairing.CreatedAt, &pairing.ExpiresAt)

	if errors.Is(err, sql.ErrNoRows) {
		return Pairing{}, ErrNoPairing
	}
	if err != nil {
		return Pairing{}, fmt.Errorf("read pairing for guild %s: %w", guildID, err)
	}
	return pairing, nil
}

// PairingByCodeHash finds the guild an outstanding pairing code belongs to.
//
// Capture only ever sees the code, never a guild id, because asking a player to
// copy a Discord snowflake out of a developer menu is not a setup flow anyone
// completes. The code is looked up across guilds instead, which is safe for the
// same reasons it is safe at all: it is single use, it expires in minutes, and
// a wrong guess buys one online attempt rather than an offline search.
func (d *DB) PairingByCodeHash(codeHash string) (Pairing, error) {
	if codeHash == "" {
		return Pairing{}, ErrNoPairing
	}

	var pairing Pairing
	err := d.db.QueryRow(`
		SELECT guild_id, code_hash, created_by, created_at, expires_at
		FROM capture_pairing WHERE code_hash = ?`, codeHash,
	).Scan(&pairing.GuildID, &pairing.CodeHash, &pairing.CreatedBy, &pairing.CreatedAt, &pairing.ExpiresAt)

	if errors.Is(err, sql.ErrNoRows) {
		return Pairing{}, ErrNoPairing
	}
	if err != nil {
		return Pairing{}, fmt.Errorf("look up pairing by code: %w", err)
	}
	return pairing, nil
}

// DeletePairing removes a guild's outstanding pairing code.
//
// A code is single use, so it is deleted the moment it is redeemed rather than
// left to expire. An attacker who learns a code after it was used must find
// nothing to replay.
func (d *DB) DeletePairing(guildID string) error {
	if _, err := d.db.Exec("DELETE FROM capture_pairing WHERE guild_id = ?", guildID); err != nil {
		return fmt.Errorf("delete pairing for guild %s: %w", guildID, err)
	}
	return nil
}

// RedeemPairing deletes the pairing row and stores the credential it issued, in
// one transaction.
//
// The two have to happen together. If the delete succeeded and the insert did
// not, the administrator would be left with a code that no longer works and no
// credential to show for it; the other order would leave a used code redeemable
// a second time.
func (d *DB) RedeemPairing(guildID string, credential CaptureCredential) error {
	if guildID == "" || credential.ID == "" || credential.SecretHash == "" {
		return errors.New("redeem pairing: guild, credential id and hash are all required")
	}

	tx, err := d.db.Begin()
	if err != nil {
		return fmt.Errorf("begin redeeming pairing for guild %s: %w", guildID, err)
	}
	defer tx.Rollback()

	result, err := tx.Exec("DELETE FROM capture_pairing WHERE guild_id = ?", guildID)
	if err != nil {
		return fmt.Errorf("consume pairing for guild %s: %w", guildID, err)
	}
	// If no row was deleted, something redeemed the code between the check and
	// here. Issuing a credential anyway would defeat single use.
	if affected, err := result.RowsAffected(); err == nil && affected == 0 {
		return ErrNoPairing
	}

	if _, err := tx.Exec(`
		INSERT INTO capture_credential (id, guild_id, secret_hash, created_at, last_seen_at, revoked_at)
		VALUES (?, ?, ?, ?, 0, 0)`,
		credential.ID, credential.GuildID, credential.SecretHash, credential.CreatedAt,
	); err != nil {
		return fmt.Errorf("store credential for guild %s: %w", guildID, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit pairing redemption for guild %s: %w", guildID, err)
	}
	return nil
}

// CaptureCredentialByID returns one credential, revoked or not.
//
// Revoked credentials are returned rather than hidden so the caller can tell a
// withdrawn credential apart from one that never existed, and say so.
func (d *DB) CaptureCredentialByID(id string) (CaptureCredential, error) {
	var credential CaptureCredential
	err := d.db.QueryRow(`
		SELECT id, guild_id, secret_hash, created_at, last_seen_at, revoked_at
		FROM capture_credential WHERE id = ?`, id,
	).Scan(&credential.ID, &credential.GuildID, &credential.SecretHash,
		&credential.CreatedAt, &credential.LastSeenAt, &credential.RevokedAt)

	if errors.Is(err, sql.ErrNoRows) {
		return CaptureCredential{}, ErrNoCredential
	}
	if err != nil {
		return CaptureCredential{}, fmt.Errorf("read capture credential: %w", err)
	}
	return credential, nil
}

// CaptureCredentials lists a guild's credentials, newest first.
func (d *DB) CaptureCredentials(guildID string) ([]CaptureCredential, error) {
	rows, err := d.db.Query(`
		SELECT id, guild_id, secret_hash, created_at, last_seen_at, revoked_at
		FROM capture_credential WHERE guild_id = ? ORDER BY created_at DESC, id`, guildID)
	if err != nil {
		return nil, fmt.Errorf("list capture credentials for guild %s: %w", guildID, err)
	}
	defer rows.Close()

	var credentials []CaptureCredential
	for rows.Next() {
		var credential CaptureCredential
		if err := rows.Scan(&credential.ID, &credential.GuildID, &credential.SecretHash,
			&credential.CreatedAt, &credential.LastSeenAt, &credential.RevokedAt); err != nil {
			return nil, fmt.Errorf("read capture credential row: %w", err)
		}
		credentials = append(credentials, credential)
	}
	return credentials, rows.Err()
}

// TouchCaptureCredential records that a credential was just used.
//
// A revoked credential is never touched, so the last-seen time keeps saying
// when the credential last worked rather than when someone last tried it.
func (d *DB) TouchCaptureCredential(id string, at int64) error {
	if _, err := d.db.Exec(
		"UPDATE capture_credential SET last_seen_at = ? WHERE id = ? AND revoked_at = 0", at, id,
	); err != nil {
		return fmt.Errorf("record capture credential use: %w", err)
	}
	return nil
}

// RevokeCaptureAccess withdraws every credential a guild holds and removes any
// outstanding pairing code, returning how many credentials were revoked.
//
// Both halves matter. Revoking the credentials without dropping the pairing
// code would leave a way back in that the administrator was not told about.
func (d *DB) RevokeCaptureAccess(guildID string, at int64) (int, error) {
	if guildID == "" {
		return 0, errors.New("revoke capture access: guild is required")
	}

	tx, err := d.db.Begin()
	if err != nil {
		return 0, fmt.Errorf("begin revoking capture access for guild %s: %w", guildID, err)
	}
	defer tx.Rollback()

	result, err := tx.Exec(
		"UPDATE capture_credential SET revoked_at = ? WHERE guild_id = ? AND revoked_at = 0", at, guildID)
	if err != nil {
		return 0, fmt.Errorf("revoke capture credentials for guild %s: %w", guildID, err)
	}
	revoked, err := result.RowsAffected()
	if err != nil {
		revoked = 0
	}

	if _, err := tx.Exec("DELETE FROM capture_pairing WHERE guild_id = ?", guildID); err != nil {
		return 0, fmt.Errorf("drop pairing for guild %s: %w", guildID, err)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit capture revocation for guild %s: %w", guildID, err)
	}
	return int(revoked), nil
}
