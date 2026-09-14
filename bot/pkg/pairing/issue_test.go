package pairing

import (
	"errors"
	"testing"

	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/credential"
)

func TestAnIssuedCredentialAuthenticatesForItsGuild(t *testing.T) {
	service, _ := testService(t)

	issued, err := service.Issue(guild, "windows-app")
	if err != nil {
		t.Fatalf("issue: %v", err)
	}

	got, err := service.Authenticate(issued.Token().Reveal())
	if err != nil {
		t.Fatalf("authenticate: %v", err)
	}
	if got != guild {
		t.Errorf("the credential speaks for %q, want %q", got, guild)
	}
}

// An issued credential is an ordinary one, so revoking works on it too.
func TestAnIssuedCredentialCanBeRevoked(t *testing.T) {
	service, _ := testService(t)
	issued, err := service.Issue(guild, "windows-app")
	if err != nil {
		t.Fatalf("issue: %v", err)
	}

	if _, err := service.Revoke(guild); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if _, err := service.Authenticate(issued.Token().Reveal()); !errors.Is(err, credential.ErrRevoked) {
		t.Errorf("a revoked issued credential gave %v, want %v", err, credential.ErrRevoked)
	}
}

// Issuing goes through a pairing code, so it replaces one that was waiting,
// exactly as a new /au capture pair would.
func TestIssuingReplacesAnOutstandingCode(t *testing.T) {
	service, _ := testService(t)
	code, _, err := service.Pair(guild, "admin")
	if err != nil {
		t.Fatalf("pair: %v", err)
	}

	if _, err := service.Issue(guild, "windows-app"); err != nil {
		t.Fatalf("issue: %v", err)
	}

	if _, _, err := service.RedeemCode(code.Display()); !errors.Is(err, credential.ErrMismatch) {
		t.Errorf("the replaced code gave %v, want %v", err, credential.ErrMismatch)
	}
}

func TestIssuingNeedsAGuild(t *testing.T) {
	service, _ := testService(t)

	if _, err := service.Issue("", "windows-app"); err == nil {
		t.Error("a credential was issued for no guild")
	}
}
