// Package assets carries the files the bot ships inside its own executable.
//
// The crewmate pictures come from AutoMuteUs (MIT) and show the Among Us
// characters, which belong to Innersloth. They are embedded rather than read
// from disk so the Windows app has one file to start and nothing to lose.
package assets

import "embed"

// Emojis holds one picture per crewmate colour, alive (aured.png) and dead
// (aureddead.png).
//
//go:embed emojis/*.png
var Emojis embed.FS
