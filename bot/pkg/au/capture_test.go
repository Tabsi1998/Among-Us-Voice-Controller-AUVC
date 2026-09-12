package au

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/pairing"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/storage/sqlite"
)

// pairingService builds a service that can actually issue credentials, as the
// bot does in main.
func pairingService(t *testing.T) (*Service, *sqlite.DB) {
	t.Helper()

	db, err := sqlite.Open(filepath.Join(t.TempDir(), "amongus.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	return NewServiceWithPairing(db, pairing.NewService(db), "v0.1.0-test", "abc123"), db
}

func TestCapturePairReturnsACodeAPersonCanType(t *testing.T) {
	service, _ := pairingService(t)

	reply, err := service.Handle(request(GroupCapture, CapturePair))
	if err != nil {
		t.Fatalf("pair: %v", err)
	}

	if !strings.Contains(reply, "AUVC-") {
		t.Errorf("the reply does not contain a pairing code: %s", reply)
	}
	// Somebody reading this has to learn the two things that surprise people:
	// it stops working, and it only works once.
	if !strings.Contains(reply, "once") {
		t.Errorf("the reply does not say the code is single use: %s", reply)
	}
	if !strings.Contains(strings.ToLower(reply), "expires") {
		t.Errorf("the reply does not say the code expires: %s", reply)
	}
}

func TestCaptureStatusReportsAnUnpairedGuild(t *testing.T) {
	service, _ := pairingService(t)

	reply, err := service.Handle(request(GroupCapture, CaptureStatus))
	if err != nil {
		t.Fatalf("status: %v", err)
	}

	if !strings.Contains(reply, "/au capture pair") {
		t.Errorf("an unpaired guild should be told what to do: %s", reply)
	}
}

// Revoking is not undoable, so it takes the same explicit confirmation the
// configuration reset does.
func TestCaptureRevokeRefusesWithoutConfirmation(t *testing.T) {
	service, _ := pairingService(t)

	if _, err := service.Handle(request(GroupCapture, CaptureRevoke)); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("an unconfirmed revoke gave %v, want %v", err, ErrInvalidInput)
	}

	unconfirmed := request(GroupCapture, CaptureRevoke)
	unconfirmed.Values.Booleans[OptionConfirm] = false
	if _, err := service.Handle(unconfirmed); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("confirm=false gave %v, want %v", err, ErrInvalidInput)
	}
}

func TestCaptureRevokeSaysWhenThereWasNothingToRevoke(t *testing.T) {
	service, _ := pairingService(t)

	confirmed := request(GroupCapture, CaptureRevoke)
	confirmed.Values.Booleans[OptionConfirm] = true

	reply, err := service.Handle(confirmed)
	if err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if !strings.Contains(strings.ToLower(reply), "nothing to revoke") {
		t.Errorf("an empty revoke should say so plainly: %s", reply)
	}
}

// The pairing code reaches Discord in an ephemeral reply, which is the only
// place it is meant to be readable. Nothing else /au says may repeat it.
func TestTheCodeIsNotRepeatedByLaterCommands(t *testing.T) {
	service, _ := pairingService(t)

	paired, err := service.Handle(request(GroupCapture, CapturePair))
	if err != nil {
		t.Fatalf("pair: %v", err)
	}

	code := pairingCodeFrom(t, paired)

	status, err := service.Handle(request(GroupCapture, CaptureStatus))
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if strings.Contains(status, code) {
		t.Errorf("status repeated the pairing code: %s", status)
	}

	exported, err := service.Handle(request(GroupSettings, SettingsExport))
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if strings.Contains(exported, code) {
		t.Errorf("the configuration export leaked the pairing code: %s", exported)
	}
}

// A configuration export is the thing people paste into an issue when asking
// for help, so it must not carry a credential hash either.
func TestTheConfigurationExportCarriesNoCredentialMaterial(t *testing.T) {
	service, db := pairingService(t)

	if _, err := service.Handle(request(GroupCapture, CapturePair)); err != nil {
		t.Fatalf("pair: %v", err)
	}
	stored, err := db.Pairing("guild")
	if err != nil {
		t.Fatalf("read pairing: %v", err)
	}

	exported, err := service.Handle(request(GroupSettings, SettingsExport))
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if strings.Contains(exported, stored.CodeHash) {
		t.Errorf("the export leaked the stored code hash: %s", exported)
	}
}

// A build without a pairing service still has to answer, rather than failing in
// a way that looks like a broken command.
func TestCaptureCommandsReportThemselvesWhenPairingIsUnavailable(t *testing.T) {
	service, _ := testService(t)

	for _, command := range []string{CapturePair, CaptureStatus, CaptureRevoke} {
		reply, err := service.Handle(request(GroupCapture, command))
		if err != nil {
			t.Fatalf("%s: %v", command, err)
		}
		if !strings.Contains(reply, command) {
			t.Errorf("%s did not name itself: %s", command, reply)
		}
	}
}

func TestAnUnknownCaptureCommandIsRefused(t *testing.T) {
	service, _ := pairingService(t)

	if _, err := service.Handle(request(GroupCapture, "teleport")); !errors.Is(err, ErrUnknownPath) {
		t.Errorf("an unknown capture command gave %v, want %v", err, ErrUnknownPath)
	}
}

// pairingCodeFrom pulls the code out of the reply so a test can check that
// nothing else ever prints it.
func pairingCodeFrom(t *testing.T, reply string) string {
	t.Helper()

	start := strings.Index(reply, "AUVC-")
	if start < 0 {
		t.Fatalf("no pairing code in reply: %s", reply)
	}
	code := reply[start:]
	if end := strings.IndexAny(code, "`\n "); end > 0 {
		code = code[:end]
	}
	return code
}
