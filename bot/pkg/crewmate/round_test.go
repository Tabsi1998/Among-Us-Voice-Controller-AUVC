package crewmate

import (
	"reflect"
	"testing"

	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/game"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/protocol"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/session"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/text"
	"github.com/bwmarrin/discordgo"
)

var aLobby = []session.GamePlayer{{Name: "Alice", Color: game.Red, Alive: true}}

func TestTheBoardSaysTheMapThePhaseAndTheCode(t *testing.T) {
	board := Render(text.German, Round{Phase: game.TASKS, Map: protocol.MapPolus, Code: "ABCDEF"}, aLobby, nil, nil)

	want := []*discordgo.MessageEmbedField{
		{Name: text.German.Say(text.BoardMap), Value: "Polus", Inline: true},
		{Name: text.German.Say(text.BoardPhase), Value: text.German.Say(text.PhaseTasks), Inline: true},
		{Name: text.German.Say(text.BoardCode), Value: "`ABCDEF`", Inline: true},
	}
	if !reflect.DeepEqual(board.Embed.Fields, want) {
		t.Errorf("fields %+v, want %+v", fieldsOf(board), fieldsOf(Board{Embed: &discordgo.MessageEmbed{Fields: want}}))
	}
}

// The host hid the code for a reason, so the board does not show it either.
func TestAHiddenCodeStaysHidden(t *testing.T) {
	board := Render(text.English, Round{Phase: game.LOBBY, Map: protocol.MapTheSkeld, Code: "******"}, aLobby, nil, nil)

	for _, field := range board.Embed.Fields {
		if field.Name == text.English.Say(text.BoardCode) {
			t.Errorf("the hidden code is shown as %q", field.Value)
		}
	}
}

// A newer capture may report a map this build has no name for. The board shows
// what it knows rather than a raw name.
func TestAMapThisBuildDoesNotKnowIsLeftOut(t *testing.T) {
	board := Render(text.English, Round{Phase: game.LOBBY, Map: "a_map_from_the_future"}, aLobby, nil, nil)

	if got := fieldsOf(board); !reflect.DeepEqual(got, []string{"Phase: Lobby"}) {
		t.Errorf("fields %v", got)
	}
}

func TestEveryProtocolMapHasAName(t *testing.T) {
	for _, name := range protocol.Maps {
		if mapNames[name] == "" {
			t.Errorf("%s has no name on the board", name)
		}
	}
}

// Before capture has reported anything there is nothing to say about the round,
// and a board waiting for players says nothing about it either.
func TestNothingKnownShowsNoFields(t *testing.T) {
	if board := Render(text.English, Round{Phase: game.UNINITIALIZED}, aLobby, nil, nil); len(board.Embed.Fields) != 0 {
		t.Errorf("an unknown round shows %v", fieldsOf(board))
	}
	if board := Render(text.English, Round{Phase: game.LOBBY, Code: "ABCD"}, nil, nil, nil); len(board.Embed.Fields) != 0 {
		t.Errorf("an empty lobby shows %v", fieldsOf(board))
	}
}

func fieldsOf(board Board) []string {
	var fields []string
	for _, field := range board.Embed.Fields {
		fields = append(fields, field.Name+": "+field.Value)
	}
	return fields
}
