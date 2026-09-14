package pairing

import (
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/credential"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/storage/sqlite"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/text"
)

const guild = "guild-1"

// clock is a hand-wound clock, so expiry is tested by moving time rather than
// by sleeping through it.
type clock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *clock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

func testService(t *testing.T) (*Service, *clock) {
	t.Helper()

	db, err := sqlite.Open(filepath.Join(t.TempDir(), "auvc.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	ticking := &clock{now: time.Unix(1_700_000_000, 0).UTC()}
	return NewServiceWithClock(db, ticking.Now), ticking
}

// pairAndRedeem walks the happy path so a test can start from a paired guild.
func pairAndRedeem(t *testing.T, service *Service) credential.Credential {
	t.Helper()

	code, _, err := service.Pair(guild, "admin")
	if err != nil {
		t.Fatalf("pair: %v", err)
	}
	issued, err := service.Redeem(guild, code.Display())
	if err != nil {
		t.Fatalf("redeem: %v", err)
	}
	return issued
}

func TestPairingIssuesAUsableCredential(t *testing.T) {
	service, _ := testService(t)

	issued := pairAndRedeem(t, service)

	got, err := service.Authenticate(issued.Token().Reveal())
	if err != nil {
		t.Fatalf("authenticate: %v", err)
	}
	if got != guild {
		t.Errorf("token authenticated for guild %q, want %q", got, guild)
	}
}

// A pairing code is single use. A second attempt is either a mistake or a
// replay, and both must fail.
func TestAPairingCodeCannotBeRedeemedTwice(t *testing.T) {
	service, _ := testService(t)

	code, _, err := service.Pair(guild, "admin")
	if err != nil {
		t.Fatalf("pair: %v", err)
	}
	if _, err := service.Redeem(guild, code.Display()); err != nil {
		t.Fatalf("first redeem: %v", err)
	}

	if _, err := service.Redeem(guild, code.Display()); !errors.Is(err, credential.ErrUsed) {
		t.Errorf("replaying a code gave %v, want %v", err, credential.ErrUsed)
	}
}

// A code read over someone's shoulder or left in a chat log has to be worthless
// by the time it is tried.
func TestAnExpiredPairingCodeIsRefused(t *testing.T) {
	service, ticking := testService(t)

	code, expires, err := service.Pair(guild, "admin")
	if err != nil {
		t.Fatalf("pair: %v", err)
	}
	if !expires.After(ticking.Now()) {
		t.Fatal("a fresh code should expire in the future")
	}

	ticking.Advance(credential.PairingCodeLifetime + time.Second)

	if _, err := service.Redeem(guild, code.Display()); !errors.Is(err, credential.ErrExpired) {
		t.Errorf("an expired code gave %v, want %v", err, credential.ErrExpired)
	}
}

// Typing errors are ordinary. Burning the code on one would mean a new
// /au capture pair for every slip.
func TestAWrongCodeDoesNotConsumeTheRealOne(t *testing.T) {
	service, _ := testService(t)

	code, _, err := service.Pair(guild, "admin")
	if err != nil {
		t.Fatalf("pair: %v", err)
	}

	if _, err := service.Redeem(guild, "AUVC-0000-0000"); !errors.Is(err, credential.ErrMismatch) {
		t.Fatalf("a wrong code gave %v, want %v", err, credential.ErrMismatch)
	}
	if _, err := service.Redeem(guild, code.Display()); err != nil {
		t.Errorf("the real code stopped working after a typo: %v", err)
	}
}

// Running /au capture pair again is how an administrator cancels a code they
// read out to the wrong person.
func TestANewCodeReplacesTheOutstandingOne(t *testing.T) {
	service, _ := testService(t)

	first, _, err := service.Pair(guild, "admin")
	if err != nil {
		t.Fatalf("first pair: %v", err)
	}
	second, _, err := service.Pair(guild, "admin")
	if err != nil {
		t.Fatalf("second pair: %v", err)
	}

	if _, err := service.Redeem(guild, first.Display()); !errors.Is(err, credential.ErrMismatch) {
		t.Errorf("the replaced code still worked: %v", err)
	}
	if _, err := service.Redeem(guild, second.Display()); err != nil {
		t.Errorf("the current code was refused: %v", err)
	}
}

// The whole point of /au capture revoke.
func TestRevokingStopsAnIssuedCredential(t *testing.T) {
	service, _ := testService(t)
	issued := pairAndRedeem(t, service)

	revoked, err := service.Revoke(guild)
	if err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if revoked != 1 {
		t.Errorf("revoked %d credentials, want 1", revoked)
	}

	if _, err := service.Authenticate(issued.Token().Reveal()); !errors.Is(err, credential.ErrRevoked) {
		t.Errorf("a revoked credential gave %v, want %v", err, credential.ErrRevoked)
	}
}

// Revoking the credentials without dropping the pairing code would leave a way
// back in that the administrator was not told about.
func TestRevokingAlsoCancelsAnOutstandingPairingCode(t *testing.T) {
	service, _ := testService(t)

	code, _, err := service.Pair(guild, "admin")
	if err != nil {
		t.Fatalf("pair: %v", err)
	}
	if _, err := service.Revoke(guild); err != nil {
		t.Fatalf("revoke: %v", err)
	}

	if _, err := service.Redeem(guild, code.Display()); !errors.Is(err, credential.ErrUsed) {
		t.Errorf("a code survived a revoke: %v", err)
	}
}

func TestAnUnknownOrMalformedTokenIsRefused(t *testing.T) {
	service, _ := testService(t)
	pairAndRedeem(t, service)

	cases := map[string]string{
		"unknown identifier": "00112233445566aa.NOTTHESECRET",
		"malformed":          "not-a-token",
		"empty":              "",
	}

	for name, token := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := service.Authenticate(token); err == nil {
				t.Errorf("%q was accepted", token)
			}
		})
	}
}

// A credential from one guild must not open another. Every guild pairs its own
// capture, and a token is the only thing the transport has to go on.
func TestACredentialOnlySpeaksForItsOwnGuild(t *testing.T) {
	service, _ := testService(t)
	issued := pairAndRedeem(t, service)

	otherCode, _, err := service.Pair("guild-2", "admin")
	if err != nil {
		t.Fatalf("pair second guild: %v", err)
	}
	otherIssued, err := service.Redeem("guild-2", otherCode.Display())
	if err != nil {
		t.Fatalf("redeem second guild: %v", err)
	}

	first, err := service.Authenticate(issued.Token().Reveal())
	if err != nil {
		t.Fatalf("authenticate first: %v", err)
	}
	second, err := service.Authenticate(otherIssued.Token().Reveal())
	if err != nil {
		t.Fatalf("authenticate second: %v", err)
	}

	if first == second {
		t.Error("two guilds resolved to the same credential")
	}
	if first != guild || second != "guild-2" {
		t.Errorf("credentials resolved to %q and %q", first, second)
	}
}

// Revoking one guild must not disconnect another guild's capture.
func TestRevokingOneGuildLeavesAnotherAlone(t *testing.T) {
	service, _ := testService(t)
	mine := pairAndRedeem(t, service)

	otherCode, _, err := service.Pair("guild-2", "admin")
	if err != nil {
		t.Fatalf("pair second guild: %v", err)
	}
	theirs, err := service.Redeem("guild-2", otherCode.Display())
	if err != nil {
		t.Fatalf("redeem second guild: %v", err)
	}

	if _, err := service.Revoke(guild); err != nil {
		t.Fatalf("revoke: %v", err)
	}

	if _, err := service.Authenticate(mine.Token().Reveal()); !errors.Is(err, credential.ErrRevoked) {
		t.Errorf("the revoked credential still works: %v", err)
	}
	if _, err := service.Authenticate(theirs.Token().Reveal()); err != nil {
		t.Errorf("an unrelated guild lost its capture: %v", err)
	}
}

func TestStatusReportsWhatAnAdministratorNeeds(t *testing.T) {
	service, ticking := testService(t)

	before, err := service.Status(guild)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if before.Paired || before.PairingOutstanding {
		t.Errorf("a fresh guild reported %+v", before)
	}

	if _, _, err := service.Pair(guild, "admin"); err != nil {
		t.Fatalf("pair: %v", err)
	}
	waiting, err := service.Status(guild)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if !waiting.PairingOutstanding || waiting.Paired {
		t.Errorf("a guild with a code outstanding reported %+v", waiting)
	}

	// An expired code is not outstanding, whatever the row says.
	ticking.Advance(credential.PairingCodeLifetime + time.Second)
	stale, err := service.Status(guild)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if stale.PairingOutstanding {
		t.Errorf("an expired code was still reported as outstanding: %+v", stale)
	}
}

func TestStatusShowsWhenCaptureWasLastSeen(t *testing.T) {
	service, ticking := testService(t)
	issued := pairAndRedeem(t, service)

	unused, err := service.Status(guild)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if !unused.Paired || !unused.LastSeen.IsZero() {
		t.Errorf("a paired but unused capture reported %+v", unused)
	}

	ticking.Advance(time.Minute)
	if _, err := service.Authenticate(issued.Token().Reveal()); err != nil {
		t.Fatalf("authenticate: %v", err)
	}

	used, err := service.Status(guild)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if used.LastSeen.IsZero() {
		t.Error("connecting did not record a last-seen time")
	}
	if !used.LastSeen.Equal(ticking.Now().UTC()) {
		t.Errorf("last seen %s, want %s", used.LastSeen, ticking.Now().UTC())
	}
}

// The last-seen time should say when the credential last worked, not when
// somebody last tried a revoked one.
func TestARevokedCredentialDoesNotUpdateLastSeen(t *testing.T) {
	service, ticking := testService(t)
	issued := pairAndRedeem(t, service)

	if _, err := service.Authenticate(issued.Token().Reveal()); err != nil {
		t.Fatalf("authenticate: %v", err)
	}
	if _, err := service.Revoke(guild); err != nil {
		t.Fatalf("revoke: %v", err)
	}

	ticking.Advance(time.Hour)
	_, _ = service.Authenticate(issued.Token().Reveal())

	status, err := service.Status(guild)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if status.RevokedCredentials != 1 {
		t.Errorf("expected one revoked credential, got %+v", status)
	}
	if status.Paired {
		t.Error("a guild with only revoked credentials is not paired")
	}
}

// The description reaches a Discord channel, so it must never carry a secret.
func TestTheStatusDescriptionCarriesNoSecret(t *testing.T) {
	service, _ := testService(t)
	issued := pairAndRedeem(t, service)

	status, err := service.Status(guild)
	if err != nil {
		t.Fatalf("status: %v", err)
	}

	for _, language := range text.Languages {
		described := status.Describe(language)
		if described == "" {
			t.Fatalf("the %s status described itself as nothing", language)
		}
		for _, secret := range []string{issued.Secret.Reveal(), issued.Token().Reveal()} {
			if strings.Contains(described, secret) {
				t.Errorf("the %s status leaked a secret: %s", language, described)
			}
		}
	}
}
