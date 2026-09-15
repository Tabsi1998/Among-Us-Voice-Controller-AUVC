package session

// Lobby is the lobby a session is in, as capture last reported it: its code and
// its map by their protocol names. The zero value is a lobby capture has told
// nothing about.
type Lobby struct {
	Code string
	Map  string
}

// Lobby returns the lobby the session is in.
func (l *Live) Lobby() Lobby { return l.lobby }

// SetLobby records the lobby and reports whether it changed. Only the crewmate
// board shows it; voice does not depend on it.
func (l *Live) SetLobby(lobby Lobby) bool {
	if l.lobby == lobby {
		return false
	}
	l.lobby = lobby
	return true
}
