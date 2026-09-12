// Package credential issues and checks the secrets capture uses to reach the
// bot: a short pairing code a person types once, and the long-lived credential
// that replaces it.
//
// Nothing here touches storage, Discord or a clock beyond what it is handed, so
// every rule below is a pure function that can be tested directly. The storage
// layer keeps hashes; this package decides what a secret is worth.
//
// Secrets in this package cannot be printed by accident. Secret hides itself
// from fmt and encoding/json, and the only way to see the value is to ask for
// it by name with Reveal. The requirements are explicit that credentials must
// never be logged in plaintext, and an API that makes the safe thing automatic
// is worth more than a rule everybody has to remember.
package credential

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base32"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

// PairingCodeLifetime is how long a pairing code stays usable.
//
// Long enough to switch to another window and type it, short enough that a code
// read over someone's shoulder or left in a chat log is worthless by the time
// it is tried.
const PairingCodeLifetime = 10 * time.Minute

var (
	// ErrExpired means the pairing code was valid but is past its lifetime.
	ErrExpired = errors.New("pairing code has expired")
	// ErrUsed means the pairing code was already redeemed. Codes are single
	// use, so a second attempt is either a mistake or a replay.
	ErrUsed = errors.New("pairing code has already been used")
	// ErrRevoked means the credential was withdrawn by /au capture revoke.
	ErrRevoked = errors.New("credential has been revoked")
	// ErrMismatch means the presented secret does not match the stored hash.
	ErrMismatch = errors.New("credential does not match")
	// ErrMalformed means the presented value is not shaped like a credential.
	ErrMalformed = errors.New("credential is malformed")
)

// Redacted is what a secret prints as. It is deliberately not empty: an empty
// string in a log reads like a missing value and sends people looking for a bug
// that is not there.
const Redacted = "[redacted]"

// Secret is a value that must never be printed.
//
// It implements fmt.Stringer, fmt.GoStringer and json.Marshaler so that every
// ordinary way of writing a value out produces Redacted instead. Reading the
// real value takes an explicit Reveal, which is easy to find in review.
type Secret struct {
	value string
}

// NewSecret wraps a value that must not be logged.
func NewSecret(value string) Secret { return Secret{value: value} }

// Reveal returns the secret itself. Every call site is a place where a secret
// escapes, so there should be few and they should be obvious.
func (s Secret) Reveal() string { return s.value }

// Empty reports whether there is no secret at all.
func (s Secret) Empty() bool { return s.value == "" }

func (s Secret) String() string { return Redacted }

func (s Secret) GoString() string { return Redacted }

// MarshalJSON keeps a secret out of any structure that gets serialized,
// including the ones written to disk or sent to Discord.
func (s Secret) MarshalJSON() ([]byte, error) { return []byte(`"` + Redacted + `"`), nil }

// Format catches the verbs fmt handles without consulting String, so that
// neither %q nor %x can print the value.
func (s Secret) Format(state fmt.State, verb rune) {
	switch verb {
	case 'q':
		fmt.Fprintf(state, "%q", Redacted)
	default:
		fmt.Fprint(state, Redacted)
	}
}

// pairingAlphabet is Crockford base32: no I, L, O or U. Removing them takes
// away every character pair a person reading a code aloud can confuse, and
// leaving out U avoids spelling anything unfortunate by chance.
const pairingAlphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// pairingCodeLength is eight characters, about forty bits. That is not much on
// its own, which is why a code is single use and lives for ten minutes: an
// attacker gets one guess per code, not an offline search.
const pairingCodeLength = 8

// PairingCode is the short code a person reads from Discord and types into
// capture. It is rendered in two groups of four, which is how people copy
// numbers accurately.
type PairingCode struct {
	Secret Secret
}

// NewPairingCode generates a fresh code.
func NewPairingCode() (PairingCode, error) {
	buffer := make([]byte, pairingCodeLength)
	if _, err := rand.Read(buffer); err != nil {
		return PairingCode{}, fmt.Errorf("generate pairing code: %w", err)
	}

	var builder strings.Builder
	for _, b := range buffer {
		// The alphabet has 32 entries, so a byte maps onto it without bias.
		builder.WriteByte(pairingAlphabet[b%32])
	}
	return PairingCode{Secret: NewSecret(builder.String())}, nil
}

// Display renders the code the way it is shown to a person: grouped, and
// prefixed so it is recognisable when pasted somewhere unexpected.
//
// This is the one place a pairing code is meant to be readable, which is why it
// is a named method rather than a Stringer: a code reaches a human only where
// the code says Display, never because something got formatted into a log line.
func (c PairingCode) Display() string {
	raw := c.Secret.Reveal()
	if len(raw) != pairingCodeLength {
		return raw
	}
	return "AUVC-" + raw[:4] + "-" + raw[4:]
}

// NormalizePairingCode turns whatever a person typed into the canonical form.
//
// People type lower case, add or drop the dashes, paste the AUVC prefix, and
// reach for the letters that look like digits. Accepting all of that is not
// laxness: rejecting a correctly read code because of a hyphen sends the user
// back to Discord for a new one.
func NormalizePairingCode(input string) string {
	// The prefix is stripped before the letter fixes below, not after. U maps to
	// V, so normalizing first would turn AUVC into AVVC and the prefix would no
	// longer be recognisable.
	upper := strings.ToUpper(strings.TrimSpace(input))
	upper = strings.TrimPrefix(upper, "AUVC-")
	upper = strings.TrimPrefix(upper, "AUVC")

	var builder strings.Builder

	for _, r := range upper {
		switch r {
		case 'O':
			builder.WriteRune('0')
		case 'I', 'L':
			builder.WriteRune('1')
		case 'U':
			builder.WriteRune('V')
		default:
			if strings.ContainsRune(pairingAlphabet, r) {
				builder.WriteRune(r)
			}
		}
	}

	return builder.String()
}

// credentialIDBytes is enough to name a credential without being guessable.
const credentialIDBytes = 8

// credentialSecretBytes is 256 bits of randomness. That is the reason a plain
// hash is enough to store it: see Hash.
const credentialSecretBytes = 32

// Credential is the long-lived secret capture presents on every connection.
// The ID is not secret; it names which stored credential to check against, so
// the bot does not have to compare a presented secret with every row it holds.
type Credential struct {
	ID     string
	Secret Secret
}

// NewCredential issues a credential. It is returned once, at pairing time, and
// only its hash is ever stored.
func NewCredential() (Credential, error) {
	id := make([]byte, credentialIDBytes)
	if _, err := rand.Read(id); err != nil {
		return Credential{}, fmt.Errorf("generate credential id: %w", err)
	}

	secret := make([]byte, credentialSecretBytes)
	if _, err := rand.Read(secret); err != nil {
		return Credential{}, fmt.Errorf("generate credential secret: %w", err)
	}

	return Credential{
		ID:     hex.EncodeToString(id),
		Secret: NewSecret(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(secret)),
	}, nil
}

// Token is the single string capture sends in the protocol authentication
// message: the identifier, a dot, and the secret.
func (c Credential) Token() Secret {
	return NewSecret(c.ID + "." + c.Secret.Reveal())
}

// ParseToken splits a presented token back into its parts.
func ParseToken(token string) (id string, secret Secret, err error) {
	id, rest, found := strings.Cut(strings.TrimSpace(token), ".")
	if !found || id == "" || rest == "" {
		return "", Secret{}, ErrMalformed
	}
	if _, decodeErr := hex.DecodeString(id); decodeErr != nil {
		return "", Secret{}, ErrMalformed
	}
	return id, NewSecret(rest), nil
}

// Hash returns the value stored for a secret.
//
// A single SHA-256 is the right tool here and a password hash would not be.
// Argon2 and bcrypt exist to make guessing cheap secrets expensive; these
// secrets are 256 bits of randomness from crypto/rand, so there is nothing to
// guess and no dictionary to run. Pairing codes are short, which is exactly why
// they are single use and expire in ten minutes: an attacker gets one online
// attempt, never an offline search against a stolen hash.
//
// The hash is hex so it can live in a STRICT TEXT column and be compared as a
// value rather than a blob.
func Hash(secret Secret) string {
	sum := sha256.Sum256([]byte(secret.Reveal()))
	return hex.EncodeToString(sum[:])
}

// Matches reports whether a presented secret hashes to the stored value.
//
// The comparison is constant time. The timing of a mismatch must not reveal how
// much of a secret was correct, which is what turns a guessing attack from
// hopeless into feasible one character at a time.
func Matches(storedHash string, presented Secret) bool {
	if storedHash == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(storedHash), []byte(Hash(presented))) == 1
}
