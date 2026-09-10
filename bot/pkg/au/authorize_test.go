package au

import "testing"

var (
	owner     = Invoker{UserID: "owner", IsGuildOwner: true}
	admin     = Invoker{UserID: "admin", HasAdministrator: true}
	moderator = Invoker{UserID: "mod", RoleIDs: []string{"role-admin", "role-other"}}
	member    = Invoker{UserID: "member", RoleIDs: []string{"role-other"}}
)

func TestEveryoneScopeIsOpenToOrdinaryMembers(t *testing.T) {
	if !Authorized(member, "role-admin", ScopeEveryone) {
		t.Error("an ordinary member must be able to run everyone-scope commands")
	}
}

func TestAdminScopeAcceptsOwnerAdministratorAndConfiguredRole(t *testing.T) {
	for name, invoker := range map[string]Invoker{
		"guild owner":   owner,
		"administrator": admin,
		"admin role":    moderator,
	} {
		if !Authorized(invoker, "role-admin", ScopeAdmin) {
			t.Errorf("%s must be authorized for admin scope", name)
		}
	}
}

func TestAdminScopeRejectsAnOrdinaryMember(t *testing.T) {
	if Authorized(member, "role-admin", ScopeAdmin) {
		t.Error("a member without the admin role must not administer the bot")
	}
}

// Before /au setup permissions has ever run there is no admin role. If the
// owner and administrators did not pass, the bot could never be configured,
// because configuring it is what sets the role.
func TestAdminScopeWorksBeforeAnAdminRoleIsConfigured(t *testing.T) {
	if !Authorized(owner, "", ScopeAdmin) {
		t.Error("the guild owner must be able to configure a fresh bot")
	}
	if !Authorized(admin, "", ScopeAdmin) {
		t.Error("an administrator must be able to configure a fresh bot")
	}
	if Authorized(member, "", ScopeAdmin) {
		t.Error("an ordinary member must not be able to configure a fresh bot")
	}
}

func TestHasRoleIgnoresAnEmptyRoleID(t *testing.T) {
	if moderator.HasRole("") {
		t.Error("an empty role id must never match")
	}
}

func TestScopes(t *testing.T) {
	cases := []struct {
		group, subcommand string
		want              Scope
	}{
		{GroupSetup, SetupChannels, ScopeAdmin},
		{GroupSetup, SetupReset, ScopeAdmin},
		{GroupSettings, SettingsShow, ScopeAdmin},
		{GroupSettings, SettingsExport, ScopeAdmin},
		// Pairing mints a credential and revoking invalidates one.
		{GroupCapture, CapturePair, ScopeAdmin},
		{GroupCapture, CaptureRevoke, ScopeAdmin},
		{GroupSession, SessionStart, ScopeAdmin},
		{GroupSession, SessionStop, ScopeAdmin},
		// Reading the session status is ordinary use.
		{GroupSession, SessionStatus, ScopeEveryone},
		{"", Version, ScopeEveryone},
		{"", Link, ScopeEveryone},
		{"", Unlink, ScopeEveryone},
		{"", Doctor, ScopeAdmin},
	}

	for _, c := range cases {
		if got := ScopeFor(c.group, c.subcommand); got != c.want {
			t.Errorf("ScopeFor(%q, %q) = %v, want %v", c.group, c.subcommand, got, c.want)
		}
	}
}

// A subcommand nobody remembered to classify must refuse ordinary members
// rather than let everyone through.
func TestUnknownSubcommandsDefaultToAdmin(t *testing.T) {
	if got := ScopeFor("", "something-new"); got != ScopeAdmin {
		t.Errorf("unknown subcommand got %v, want ScopeAdmin", got)
	}
	if got := ScopeFor("unknown-group", "whatever"); got != ScopeAdmin {
		t.Errorf("unknown group got %v, want ScopeAdmin", got)
	}
}

// Linking yourself is ordinary use; linking somebody else is administrative.
func TestTargetsAnotherUser(t *testing.T) {
	if TargetsAnotherUser("me", "") {
		t.Error("an omitted target means the invoker themselves")
	}
	if TargetsAnotherUser("me", "me") {
		t.Error("naming yourself is not targeting another user")
	}
	if !TargetsAnotherUser("me", "someone-else") {
		t.Error("naming somebody else must be recognised")
	}
}
