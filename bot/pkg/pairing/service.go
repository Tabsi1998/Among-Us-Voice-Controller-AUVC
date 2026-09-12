// Package pairing turns a pairing code into a capture credential and checks the
// credentials capture presents later.
//
// It is the one place that decides whether a capture install may speak for a
// guild. The rules it enforces are the ones in docs/requirements.md: a pairing
// code is single use and expires, the credential it issues is long-lived and
// revocable, and no secret is ever stored or logged in the clear.
package pairing

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/credential"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/storage/sqlite"
)

// Store is the persistence this service needs. It is an interface so the rules
// can be tested against a fake as well as against a real database.
type Store interface {
	SavePairing(pairing sqlite.Pairing) error
	Pairing(guildID string) (sqlite.Pairing, error)
	PairingByCodeHash(codeHash string) (sqlite.Pairing, error)
	DeletePairing(guildID string) error
	RedeemPairing(guildID string, issued sqlite.CaptureCredential) error
	CaptureCredentialByID(id string) (sqlite.CaptureCredential, error)
	CaptureCredentials(guildID string) ([]sqlite.CaptureCredential, error)
	TouchCaptureCredential(id string, at int64) error
	RevokeCaptureAccess(guildID string, at int64) (int, error)
}

// Service issues and checks capture credentials.
//
// The mutex serializes pairing and redemption. Two captures redeeming the same
// code at the same moment must not both come away with a credential, and the
// database transaction alone cannot express that across the read-then-write the
// expiry check needs.
type Service struct {
	store Store
	now   func() time.Time
	mu    sync.Mutex
}

// NewService builds a service on the system clock.
func NewService(store Store) *Service {
	return &Service{store: store, now: time.Now}
}

// NewServiceWithClock builds a service on a caller-supplied clock, so expiry
// can be tested without sleeping.
func NewServiceWithClock(store Store, now func() time.Time) *Service {
	return &Service{store: store, now: now}
}

// Pair issues a fresh pairing code for a guild and returns it together with the
// moment it stops working.
//
// Any previous code for the guild is replaced, which is also how an
// administrator cancels a code they read out to the wrong person.
func (s *Service) Pair(guildID, createdBy string) (credential.PairingCode, time.Time, error) {
	if guildID == "" {
		return credential.PairingCode{}, time.Time{}, errors.New("pair: guild is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	code, err := credential.NewPairingCode()
	if err != nil {
		return credential.PairingCode{}, time.Time{}, err
	}

	now := s.now()
	expires := now.Add(credential.PairingCodeLifetime)

	if err := s.store.SavePairing(sqlite.Pairing{
		GuildID:   guildID,
		CodeHash:  credential.Hash(code.Secret),
		CreatedBy: createdBy,
		CreatedAt: now.Unix(),
		ExpiresAt: expires.Unix(),
	}); err != nil {
		return credential.PairingCode{}, time.Time{}, err
	}

	return code, expires, nil
}

// Redeem exchanges a typed pairing code for a long-lived credential.
//
// The credential is returned exactly once, here. Only its hash is stored, so
// nothing after this call can recover it: a capture install that loses its
// credential has to pair again, which is the intended cost of never keeping a
// recoverable copy.
func (s *Service) Redeem(guildID, typed string) (credential.Credential, error) {
	if guildID == "" {
		return credential.Credential{}, errors.New("redeem: guild is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	pairing, err := s.store.Pairing(guildID)
	if errors.Is(err, sqlite.ErrNoPairing) {
		// No outstanding code is indistinguishable from an already redeemed
		// one, and both mean the same thing to whoever is typing: ask for a
		// new code.
		return credential.Credential{}, credential.ErrUsed
	}
	if err != nil {
		return credential.Credential{}, err
	}

	now := s.now()
	if now.Unix() > pairing.ExpiresAt {
		// Drop it rather than leaving a dead row that a later clock change
		// could bring back to life.
		if err := s.store.DeletePairing(guildID); err != nil {
			return credential.Credential{}, err
		}
		return credential.Credential{}, credential.ErrExpired
	}

	presented := credential.NewSecret(credential.NormalizePairingCode(typed))
	if !credential.Matches(pairing.CodeHash, presented) {
		// A wrong code does not consume the outstanding one. Typing errors are
		// ordinary, and burning the code on one would mean a new /au capture
		// pair for every slip.
		return credential.Credential{}, credential.ErrMismatch
	}

	issued, err := credential.NewCredential()
	if err != nil {
		return credential.Credential{}, err
	}

	if err := s.store.RedeemPairing(guildID, sqlite.CaptureCredential{
		ID:         issued.ID,
		GuildID:    guildID,
		SecretHash: credential.Hash(issued.Secret),
		CreatedAt:  now.Unix(),
	}); err != nil {
		if errors.Is(err, sqlite.ErrNoPairing) {
			// Something redeemed the same code in the moment between the read
			// above and this write. Single use wins.
			return credential.Credential{}, credential.ErrUsed
		}
		return credential.Credential{}, err
	}

	return issued, nil
}

// RedeemCode exchanges a typed pairing code for a credential without being told
// which guild it belongs to, and reports the guild it turned out to be.
//
// Capture only ever sees the code. Requiring a guild id as well would mean
// asking a player to copy a Discord snowflake out of a developer menu, which is
// not a setup flow anybody finishes.
//
// The code is found by its hash, so the stored value is still never compared in
// the clear. Looking it up across guilds is safe for the same reasons the code
// is safe at all: single use, minutes of life, and one online attempt per guess.
func (s *Service) RedeemCode(typed string) (string, credential.Credential, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	normalized := credential.NewSecret(credential.NormalizePairingCode(typed))
	if normalized.Empty() {
		return "", credential.Credential{}, credential.ErrMismatch
	}

	stored, err := s.store.PairingByCodeHash(credential.Hash(normalized))
	if errors.Is(err, sqlite.ErrNoPairing) {
		// An unknown code and an already redeemed one are the same thing to
		// whoever is typing: ask an administrator for a new one.
		return "", credential.Credential{}, credential.ErrMismatch
	}
	if err != nil {
		return "", credential.Credential{}, err
	}

	now := s.now()
	if now.Unix() > stored.ExpiresAt {
		if err := s.store.DeletePairing(stored.GuildID); err != nil {
			return "", credential.Credential{}, err
		}
		return "", credential.Credential{}, credential.ErrExpired
	}

	issued, err := credential.NewCredential()
	if err != nil {
		return "", credential.Credential{}, err
	}

	if err := s.store.RedeemPairing(stored.GuildID, sqlite.CaptureCredential{
		ID:         issued.ID,
		GuildID:    stored.GuildID,
		SecretHash: credential.Hash(issued.Secret),
		CreatedAt:  now.Unix(),
	}); err != nil {
		if errors.Is(err, sqlite.ErrNoPairing) {
			return "", credential.Credential{}, credential.ErrUsed
		}
		return "", credential.Credential{}, err
	}

	return stored.GuildID, issued, nil
}

// Authenticate checks a token capture presented and returns the guild it speaks
// for.
//
// The distinct errors are deliberate. A revoked credential and an unknown one
// mean different things to the person holding it: one says pair again because
// an administrator withdrew this access, the other says this install was never
// paired here. The identifier half of a token is not secret, so telling them
// apart reveals nothing an attacker could not already learn by trying.
func (s *Service) Authenticate(token string) (string, error) {
	id, secret, err := credential.ParseToken(token)
	if err != nil {
		return "", err
	}

	stored, err := s.store.CaptureCredentialByID(id)
	if errors.Is(err, sqlite.ErrNoCredential) {
		return "", credential.ErrMismatch
	}
	if err != nil {
		return "", err
	}

	if stored.Revoked() {
		return "", credential.ErrRevoked
	}
	if !credential.Matches(stored.SecretHash, secret) {
		return "", credential.ErrMismatch
	}

	// Recording the use is a convenience for /au capture status, not part of
	// the decision, so a failure to write it must not refuse a valid capture.
	_ = s.store.TouchCaptureCredential(id, s.now().Unix())

	return stored.GuildID, nil
}

// Revoke withdraws every credential the guild holds and cancels any outstanding
// pairing code, returning how many credentials were withdrawn.
func (s *Service) Revoke(guildID string) (int, error) {
	if guildID == "" {
		return 0, errors.New("revoke: guild is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	return s.store.RevokeCaptureAccess(guildID, s.now().Unix())
}

// Status is what /au capture status reports.
type Status struct {
	// Paired is true when at least one credential is usable.
	Paired bool
	// ActiveCredentials counts the credentials that have not been revoked.
	ActiveCredentials int
	// RevokedCredentials counts the withdrawn ones, so an administrator can see
	// that a revoke actually did something.
	RevokedCredentials int
	// LastSeen is the most recent time any credential was used, zero if none
	// ever has been.
	LastSeen time.Time
	// PairingOutstanding is true while a pairing code is waiting to be typed.
	PairingOutstanding bool
	// PairingExpires is when that code stops working.
	PairingExpires time.Time
}

// Status summarizes a guild's capture access.
func (s *Service) Status(guildID string) (Status, error) {
	if guildID == "" {
		return Status{}, errors.New("status: guild is required")
	}

	credentials, err := s.store.CaptureCredentials(guildID)
	if err != nil {
		return Status{}, err
	}

	var status Status
	var lastSeen int64
	for _, stored := range credentials {
		if stored.Revoked() {
			status.RevokedCredentials++
			continue
		}
		status.ActiveCredentials++
		if stored.LastSeenAt > lastSeen {
			lastSeen = stored.LastSeenAt
		}
	}
	status.Paired = status.ActiveCredentials > 0
	// Zero means never used, which has to stay a zero time rather than becoming
	// 1970 in a Discord message.
	if lastSeen > 0 {
		status.LastSeen = time.Unix(lastSeen, 0).UTC()
	}

	pairing, err := s.store.Pairing(guildID)
	switch {
	case errors.Is(err, sqlite.ErrNoPairing):
	case err != nil:
		return Status{}, err
	default:
		// An expired code is not outstanding, whatever the row says.
		if s.now().Unix() <= pairing.ExpiresAt {
			status.PairingOutstanding = true
			status.PairingExpires = time.Unix(pairing.ExpiresAt, 0).UTC()
		}
	}

	return status, nil
}

// Describe renders a status for Discord.
func (s Status) Describe() string {
	var lines []string

	switch {
	case s.Paired && s.LastSeen.IsZero():
		lines = append(lines, "✅ Capture is paired but has never connected yet.")
	case s.Paired:
		lines = append(lines, fmt.Sprintf("✅ Capture is paired. Last seen %s.",
			s.LastSeen.Format(time.RFC3339)))
	default:
		lines = append(lines, "❌ No capture is paired. Run `/au capture pair` to connect one.")
	}

	if s.PairingOutstanding {
		lines = append(lines, fmt.Sprintf("⚠️ A pairing code is waiting to be used; it expires %s.",
			s.PairingExpires.Format(time.RFC3339)))
	}
	if s.RevokedCredentials > 0 {
		lines = append(lines, fmt.Sprintf("%d revoked credential(s) on record.", s.RevokedCredentials))
	}
	if s.ActiveCredentials > 1 {
		lines = append(lines, fmt.Sprintf("%d capture installs are paired.", s.ActiveCredentials))
	}

	return strings.Join(lines, "\n")
}
