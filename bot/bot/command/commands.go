// Package command registers the Discord commands AUVC offers.
//
// There is one. The upstream bot had fourteen, most of them built around a
// hosted service with match history, a public worker pool and a per-guild
// settings store in Redis. AUVC is a self-hosted voice controller for one
// server, and everything it does is reachable under /au.
package command

import (
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/au"
	"github.com/bwmarrin/discordgo"
)

// All is the command set registered with Discord.
var All = []*discordgo.ApplicationCommand{
	au.Command(),
}
