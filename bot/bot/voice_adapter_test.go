package bot

import (
	"testing"

	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/voice"
	"github.com/bwmarrin/discordgo"
)

func TestObserveVoiceStatesReadsChannelAndServerFlags(t *testing.T) {
	guild := &discordgo.Guild{VoiceStates: []*discordgo.VoiceState{
		{UserID: "a", ChannelID: "main", Mute: true, Deaf: true},
		{UserID: "b", ChannelID: "ghost"},
	}}

	observed := observeVoiceStates(guild)

	if got, want := observed["a"], (voice.Observed{ChannelID: "main", Muted: true, Deafened: true}); got != want {
		t.Errorf("a: got %+v, want %+v", got, want)
	}
	if got, want := observed["b"], (voice.Observed{ChannelID: "ghost"}); got != want {
		t.Errorf("b: got %+v, want %+v", got, want)
	}
}

// SelfMute and SelfDeaf are the member's own choice. Reading them as the
// observed state would make the bot unmute anyone who muted themselves, every
// time it reconciled.
func TestObserveVoiceStatesIgnoresSelfMuteAndSelfDeafen(t *testing.T) {
	guild := &discordgo.Guild{VoiceStates: []*discordgo.VoiceState{
		{UserID: "a", ChannelID: "main", SelfMute: true, SelfDeaf: true},
	}}

	observed := observeVoiceStates(guild)

	if observed["a"].Muted || observed["a"].Deafened {
		t.Errorf("a member who muted themselves must not look server-muted: %+v", observed["a"])
	}
}

func TestObserveVoiceStatesHandlesAnAbsentGuild(t *testing.T) {
	if observed := observeVoiceStates(nil); len(observed) != 0 {
		t.Errorf("expected no observations, got %+v", observed)
	}
}

func TestObserveVoiceStatesSkipsMalformedEntries(t *testing.T) {
	guild := &discordgo.Guild{VoiceStates: []*discordgo.VoiceState{
		nil,
		{UserID: "", ChannelID: "main"},
		{UserID: "a", ChannelID: "main"},
	}}

	observed := observeVoiceStates(guild)

	if len(observed) != 1 {
		t.Errorf("expected only the usable entry, got %+v", observed)
	}
}

// A member edit must carry only what the change asks for. An empty ChannelID in
// particular means "remove from voice" to Discord, so an unset move has to stay
// nil rather than becoming a pointer to an empty string.
func TestMemberParamsOnlyCarriesWhatChanged(t *testing.T) {
	muted := true
	params := memberParams(voice.Change{UserID: "a", Muted: &muted})

	if params.ChannelID != nil {
		t.Errorf("an unset move must not become a channel id, got %q", *params.ChannelID)
	}
	if params.Deaf != nil {
		t.Errorf("an unset deafen must stay nil, got %v", *params.Deaf)
	}
	if params.Mute == nil || !*params.Mute {
		t.Errorf("the mute must be carried, got %v", params.Mute)
	}
}

func TestMemberParamsCarriesAMove(t *testing.T) {
	target := "ghost"
	params := memberParams(voice.Change{UserID: "a", MoveTo: &target})

	if params.ChannelID == nil || *params.ChannelID != "ghost" {
		t.Errorf("the move must be carried, got %v", params.ChannelID)
	}
	if params.Mute != nil || params.Deaf != nil {
		t.Errorf("nothing else should be sent, got mute=%v deaf=%v", params.Mute, params.Deaf)
	}
}

func TestMemberParamsCarriesEverythingWhenEverythingChanged(t *testing.T) {
	target := "main"
	muted, deafened := true, true
	params := memberParams(voice.Change{UserID: "a", MoveTo: &target, Muted: &muted, Deafened: &deafened})

	if params.ChannelID == nil || *params.ChannelID != "main" {
		t.Error("move missing")
	}
	if params.Mute == nil || !*params.Mute {
		t.Error("mute missing")
	}
	if params.Deaf == nil || !*params.Deaf {
		t.Error("deafen missing")
	}
}

// The adapter has to satisfy the reconciler's interface, or the wiring step
// fails to compile far away from here.
func TestDiscordApplierSatisfiesTheReconcilerInterface(t *testing.T) {
	var _ voice.Applier = discordApplier{}
}
