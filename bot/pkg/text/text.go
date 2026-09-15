// Package text holds what AUVC says to people in Discord, in every language it
// speaks.
//
// Each sentence has a key and one entry per language. A test fails when a
// language lacks a sentence or gives it other placeholders, so a German reply
// cannot quietly fall back to English or lose a number on the way. Replies only
// one person sees are written in that person's Discord language.
package text

import (
	"fmt"
	"strings"
)

// Language is a language AUVC speaks.
type Language string

const (
	English Language = "en"
	German  Language = "de"
)

// Languages lists every language AUVC speaks.
var Languages = []Language{English, German}

// Key names one sentence.
type Key string

var catalogs = map[Language]map[Key]string{
	English: english,
	German:  german,
}

// FromDiscord returns the language for a Discord locale such as "de", "en-US"
// or "pt-BR": German for German, English for every language AUVC does not
// speak.
func FromDiscord(locale string) Language {
	primary, _, _ := strings.Cut(strings.ToLower(locale), "-")
	if Language(primary) == German {
		return German
	}
	return English
}

// Parse returns the language a setting names, such as "de", or false for a
// language AUVC does not speak.
func Parse(value string) (Language, bool) {
	for _, language := range Languages {
		if string(language) == value {
			return language, true
		}
	}
	return "", false
}

// Name is how a language calls itself, so it can be found from any other.
func (l Language) Name() string {
	if l == German {
		return "Deutsch"
	}
	return "English"
}

// Say writes a sentence in this language, filling its placeholders from args
// the way fmt.Sprintf does. A language AUVC does not speak says it in English.
func (l Language) Say(key Key, args ...any) string {
	format, ok := catalogs[l][key]
	if !ok {
		format, ok = english[key]
	}
	if !ok {
		// Only a key without any sentence gets here, which the tests rule out.
		return string(key)
	}
	return fmt.Sprintf(format, args...)
}
