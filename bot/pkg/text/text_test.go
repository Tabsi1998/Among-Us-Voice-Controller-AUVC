package text

import (
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// verb matches one fmt placeholder: an optional explicit argument index, flags,
// width, precision and the verb itself.
var verb = regexp.MustCompile(`%(?:\[(\d+)\])?[-+# 0]*\d*(?:\.\d+)?([a-zA-Z%])`)

// placeholders maps each argument position to its verb, so two translations can
// be compared even when one reorders its arguments with %[2]s.
func placeholders(format string) map[int]string {
	found := map[int]string{}
	next := 1
	for _, match := range verb.FindAllStringSubmatch(format, -1) {
		if match[2] == "%" {
			continue
		}
		if match[1] != "" {
			next, _ = strconv.Atoi(match[1])
		}
		found[next] = match[2]
		next++
	}
	return found
}

// A sentence missing from one language shows up in English among the others,
// which is the mixed-language reply #112 is about.
func TestEveryLanguageHasEverySentence(t *testing.T) {
	for _, language := range Languages {
		catalog := catalogs[language]
		for key := range english {
			if strings.TrimSpace(catalog[key]) == "" {
				t.Errorf("%s has no sentence for %q", language, key)
			}
		}
		for key := range catalog {
			if _, ok := english[key]; !ok {
				t.Errorf("%s has a sentence for %q, which English does not", language, key)
			}
		}
	}
}

// A translation with another placeholder prints a number where a name belongs,
// or %!d(MISSING), in front of the one person who asked.
func TestTranslationsKeepThePlaceholders(t *testing.T) {
	for _, language := range Languages {
		for key, format := range catalogs[language] {
			want := placeholders(english[key])
			if got := placeholders(format); !maps.Equal(got, want) {
				t.Errorf("%s %q has placeholders %v, English has %v", language, key, got, want)
			}
		}
	}
}

func TestEverySentenceRendersWithItsArguments(t *testing.T) {
	for _, language := range Languages {
		for key, format := range catalogs[language] {
			found := placeholders(format)
			args := make([]any, len(found))
			for position, kind := range found {
				if position < 1 || position > len(args) {
					t.Errorf("%s %q uses argument %d of %d", language, key, position, len(args))
					continue
				}
				switch kind {
				case "d":
					args[position-1] = 7
				case "t":
					args[position-1] = true
				default:
					args[position-1] = "x"
				}
			}
			if rendered := language.Say(key, args...); strings.Contains(rendered, "%!") {
				t.Errorf("%s %q renders as %q", language, key, rendered)
			}
		}
	}
}

// Every constant in keys.go needs a sentence, and every sentence a constant.
// The constants are read from the source, because Go cannot list them at run
// time.
func TestEveryKeyHasASentence(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "keys.go", nil, 0)
	if err != nil {
		t.Fatalf("parse keys.go: %v", err)
	}

	var declared []Key
	ast.Inspect(file, func(node ast.Node) bool {
		spec, ok := node.(*ast.ValueSpec)
		if !ok {
			return true
		}
		if kind, ok := spec.Type.(*ast.Ident); !ok || kind.Name != "Key" {
			t.Errorf("%s is not declared as a Key", spec.Names[0].Name)
			return true
		}
		for _, value := range spec.Values {
			literal, ok := value.(*ast.BasicLit)
			if !ok {
				t.Errorf("%s is not a plain string", spec.Names[0].Name)
				continue
			}
			unquoted, _ := strconv.Unquote(literal.Value)
			declared = append(declared, Key(unquoted))
		}
		return true
	})

	for _, key := range declared {
		if _, ok := english[key]; !ok {
			t.Errorf("%q is declared but has no English sentence", key)
		}
	}
	for key := range english {
		if !slices.Contains(declared, key) {
			t.Errorf("%q has a sentence but no constant", key)
		}
	}
	if len(declared) != len(english) {
		t.Errorf("%d constants for %d sentences; is a value used twice?", len(declared), len(english))
	}
}

func TestDiscordLocalesPickTheLanguage(t *testing.T) {
	for locale, want := range map[string]Language{
		"de":    German,
		"DE":    German,
		"en-US": English,
		"en-GB": English,
		"fr":    English,
		"pt-BR": English,
		"":      English,
	} {
		if got := FromDiscord(locale); got != want {
			t.Errorf("FromDiscord(%q) = %s, want %s", locale, got, want)
		}
	}
}

func TestALanguageAUVCDoesNotSpeakSaysItInEnglish(t *testing.T) {
	if got, want := Language("fr").Say(Linked, "1", "Red"), English.Say(Linked, "1", "Red"); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if got, want := Language("").Say(NotInServer), English.Say(NotInServer); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
