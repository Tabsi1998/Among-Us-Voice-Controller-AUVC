// Package permission reports which Discord permissions AUVC is missing and
// what stops working without each of them.
//
// Without this, a missing permission surfaces as an API error per player in the
// middle of a round: the bot appears to work, then silently fails to move or
// mute somebody. Checking up front turns that into one sentence an
// administrator can act on.
//
// The package takes effective permission bitmasks as input and makes no Discord
// calls, so every rule is testable. Computing the effective mask, including
// channel overwrites, is the caller's job.
package permission

import (
	"sort"
	"strings"

	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/text"
	"github.com/bwmarrin/discordgo"
)

// Need is one permission AUVC requires, together with what breaks without it.
// The consequence is written for a guild administrator, not a developer.
type Need struct {
	Bit int64
	// Name is Discord's own English name for the permission.
	Name        string
	Consequence text.Key
}

// The permissions AUVC needs on a voice channel it manages. Players move in both
// directions between the main and ghost channels, so both need the same set.
var (
	ViewChannel = Need{
		Bit:         discordgo.PermissionViewChannel,
		Name:        "View Channel",
		Consequence: text.WithoutViewVoiceChannel,
	}
	Connect = Need{
		Bit:         discordgo.PermissionVoiceConnect,
		Name:        "Connect",
		Consequence: text.WithoutConnect,
	}
	MoveMembers = Need{
		Bit:         discordgo.PermissionVoiceMoveMembers,
		Name:        "Move Members",
		Consequence: text.WithoutMoveMembers,
	}
	MuteMembers = Need{
		Bit:         discordgo.PermissionVoiceMuteMembers,
		Name:        "Mute Members",
		Consequence: text.WithoutMuteMembers,
	}
	DeafenMembers = Need{
		Bit:         discordgo.PermissionVoiceDeafenMembers,
		Name:        "Deafen Members",
		Consequence: text.WithoutDeafenMembers,
	}
)

// VoiceChannelNeeds is the full set required on a managed voice channel.
var VoiceChannelNeeds = []Need{ViewChannel, Connect, MoveMembers, MuteMembers, DeafenMembers}

// The permissions AUVC needs on the control text channel, where players choose
// their crewmate and where AUVC posts its warnings.
var (
	SeeTextChannel = Need{
		Bit:         discordgo.PermissionViewChannel,
		Name:        "View Channel",
		Consequence: text.WithoutViewTextChannel,
	}
	SendMessages = Need{
		Bit:         discordgo.PermissionSendMessages,
		Name:        "Send Messages",
		Consequence: text.WithoutSendMessages,
	}
	EmbedLinks = Need{
		Bit:         discordgo.PermissionEmbedLinks,
		Name:        "Embed Links",
		Consequence: text.WithoutEmbedLinks,
	}
)

// ControlChannelNeeds is the full set required on the control text channel.
var ControlChannelNeeds = []Need{SeeTextChannel, SendMessages, EmbedLinks}

// Missing returns the needs that the effective permissions do not cover, in the
// order they were given.
//
// Administrator is honoured because Discord treats it as granting everything;
// reporting a permission as missing when the bot can in fact use it would send
// an administrator hunting for a problem that is not there.
func Missing(effective int64, needs []Need) []Need {
	if effective&discordgo.PermissionAdministrator != 0 {
		return nil
	}

	var missing []Need
	for _, need := range needs {
		if effective&need.Bit == 0 {
			missing = append(missing, need)
		}
	}
	return missing
}

// Channel is one configured voice channel and the permissions AUVC effectively
// holds on it, overwrites already applied.
type Channel struct {
	// Purpose is how the channel is described to an administrator, for example
	// text.PurposeMainChannel.
	Purpose text.Key
	// ID is the Discord channel id. An empty id means the channel is not
	// configured yet, which is a setup question rather than a permission one.
	ID        string
	Effective int64
}

// Report lists what is missing on one channel.
type Report struct {
	Channel Channel
	Missing []Need
}

// Audit checks every configured channel against the voice needs.
//
// Channels without an id are skipped: "not configured" is answered by
// /au setup channels, and reporting five missing permissions on a channel that
// does not exist would bury that.
func Audit(channels ...Channel) []Report {
	var reports []Report

	for _, channel := range channels {
		if channel.ID == "" {
			continue
		}
		if missing := Missing(channel.Effective, VoiceChannelNeeds); len(missing) > 0 {
			reports = append(reports, Report{Channel: channel, Missing: missing})
		}
	}
	return reports
}

// Summary renders reports as text for a Discord response, in a language. It
// returns an empty string when nothing is missing, so a caller can use it
// directly as a condition.
//
// Permissions are named once with every channel that lacks them, rather than
// repeating the same explanation per channel: an administrator fixes a
// permission once, usually on the role.
func Summary(language text.Language, reports []Report) string {
	if len(reports) == 0 {
		return ""
	}

	// Group the affected channels by permission so each one is explained once.
	channelsByNeed := map[string][]string{}
	consequence := map[string]text.Key{}
	order := map[string]int{}

	for _, report := range reports {
		for i, need := range report.Missing {
			channelsByNeed[need.Name] = append(channelsByNeed[need.Name], language.Say(report.Channel.Purpose))
			consequence[need.Name] = need.Consequence
			if _, seen := order[need.Name]; !seen {
				order[need.Name] = i
			}
		}
	}

	names := make([]string, 0, len(channelsByNeed))
	for name := range channelsByNeed {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		if order[names[i]] != order[names[j]] {
			return order[names[i]] < order[names[j]]
		}
		return names[i] < names[j]
	})

	var builder strings.Builder
	builder.WriteString(language.Say(text.PermissionsMissing))
	for _, name := range names {
		builder.WriteString(language.Say(text.PermissionMissingLine,
			name, strings.Join(channelsByNeed[name], language.Say(text.PermissionChannelsJoin)),
			language.Say(consequence[name])))
	}
	return builder.String()
}
