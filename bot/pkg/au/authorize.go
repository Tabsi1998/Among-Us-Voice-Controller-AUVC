package au

// Invoker is everything the authorization rules need to know about whoever ran
// a command. It holds plain values so the rules stay testable without Discord.
type Invoker struct {
	UserID string
	// RoleIDs are the guild roles the member holds.
	RoleIDs []string
	// IsGuildOwner is true for the guild owner, who can always administer AUVC.
	IsGuildOwner bool
	// HasAdministrator reflects the Discord Administrator permission.
	HasAdministrator bool
}

// HasRole reports whether the invoker holds a specific role.
func (i Invoker) HasRole(roleID string) bool {
	if roleID == "" {
		return false
	}
	for _, held := range i.RoleIDs {
		if held == roleID {
			return true
		}
	}
	return false
}

// Scope is how much authority a subcommand needs.
type Scope int

const (
	// ScopeEveryone may be used by any member: reading status, linking yourself.
	ScopeEveryone Scope = iota
	// ScopeAdmin changes configuration, credentials or the running session.
	ScopeAdmin
)

// ScopeFor returns the authority a subcommand needs. group is empty for
// subcommands that are not inside a group, such as link or version.
//
// Anything that changes configuration, issues or revokes capture credentials,
// or steers the running session requires admin. Reading your own status and
// linking yourself do not, because requiring an administrator for every player
// to join a round would make the bot unusable.
func ScopeFor(group, subcommand string) Scope {
	switch group {
	case GroupSetup, GroupSettings, GroupCapture:
		// Includes capture pair and revoke, which mint and invalidate
		// credentials, and settings export, which reveals the configuration.
		return ScopeAdmin

	case GroupSession:
		if subcommand == SessionStatus {
			return ScopeEveryone
		}
		return ScopeAdmin

	case "":
		switch subcommand {
		case Version, Link, Unlink:
			return ScopeEveryone
		case Doctor:
			return ScopeAdmin
		}
	}

	// An unknown subcommand is treated as privileged. A new command that nobody
	// remembered to classify should refuse ordinary members rather than let
	// everyone through.
	return ScopeAdmin
}

// TargetsAnotherUser reports whether a link or unlink names somebody other than
// the invoker. Linking yourself is ordinary use; linking someone else is an
// administrative act, so the scope is raised for it.
func TargetsAnotherUser(invokerID, targetUserID string) bool {
	return targetUserID != "" && targetUserID != invokerID
}

// Authorized reports whether the invoker may run something at the given scope.
//
// adminRoleID is the role configured through /au setup permissions and is empty
// until that has been run. The guild owner and any member with the Discord
// Administrator permission always pass: without that, a freshly invited bot
// could never be configured, because configuring it is what sets the role.
func Authorized(invoker Invoker, adminRoleID string, scope Scope) bool {
	if scope == ScopeEveryone {
		return true
	}
	return invoker.IsGuildOwner || invoker.HasAdministrator || invoker.HasRole(adminRoleID)
}
