package protocol

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// fixtureRoot is the shared directory both implementations read. Keeping the
// examples in one place outside either language is what stops the Go and C#
// sides from drifting: a change that breaks one breaks the other's tests too.
const fixtureRoot = "../../../protocol/fixtures"

func readFixture(t *testing.T, name string) []byte {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(fixtureRoot, name))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return data
}

func fixtureNames(t *testing.T, dir string) []string {
	t.Helper()

	entries, err := os.ReadDir(filepath.Join(fixtureRoot, dir))
	if err != nil {
		t.Fatalf("list fixtures: %v", err)
	}

	var names []string
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) == ".json" {
			names = append(names, entry.Name())
		}
	}
	return names
}

// Every message type in the contract needs an example, or the C# side has
// nothing to check its own encoder against.
func TestEveryMessageTypeHasAFixture(t *testing.T) {
	present := map[string]bool{}
	for _, name := range fixtureNames(t, "messages") {
		present[name] = true
	}

	for _, messageType := range Types {
		if !present[string(messageType)+".json"] {
			t.Errorf("no fixture for %s; add protocol/fixtures/messages/%s.json", messageType, messageType)
		}
	}
	if got, want := len(present), len(Types); got != want {
		t.Errorf("found %d fixtures for %d message types: %v", got, want, present)
	}
}

func TestEveryFixtureDecodesToItsType(t *testing.T) {
	for _, name := range fixtureNames(t, "messages") {
		t.Run(name, func(t *testing.T) {
			message, err := Decode(readFixture(t, filepath.Join("messages", name)))
			if err != nil {
				t.Fatalf("decode: %v", err)
			}

			envelope := Envelope(message)
			if want := name[:len(name)-len(".json")]; string(envelope.Type) != want {
				t.Errorf("fixture %s decoded as type %q", name, envelope.Type)
			}
			if envelope.Protocol != Version {
				t.Errorf("fixture speaks protocol %d, this build speaks %d", envelope.Protocol, Version)
			}
		})
	}
}

// Re-encoding a fixture has to produce the same document. If it does not, this
// build either drops a field capture sent or invents one it did not, and the
// C# side would be writing something Go silently discards.
func TestFixturesSurviveARoundTrip(t *testing.T) {
	for _, name := range fixtureNames(t, "messages") {
		t.Run(name, func(t *testing.T) {
			original := readFixture(t, filepath.Join("messages", name))

			message, err := Decode(original)
			if err != nil {
				t.Fatalf("decode: %v", err)
			}
			encoded, err := Encode(message)
			if err != nil {
				t.Fatalf("encode: %v", err)
			}

			var before, after any
			if err := json.Unmarshal(original, &before); err != nil {
				t.Fatalf("read fixture as json: %v", err)
			}
			if err := json.Unmarshal(encoded, &after); err != nil {
				t.Fatalf("read re-encoded message as json: %v", err)
			}
			if !reflect.DeepEqual(before, after) {
				t.Errorf("round trip changed the message:\nfixture: %s\nencoded: %s", original, encoded)
			}
		})
	}
}

// The invalid fixtures are the other half of the contract: both implementations
// have to refuse the same documents, rather than one of them quietly coping.
func TestEveryInvalidFixtureIsRefused(t *testing.T) {
	for _, name := range fixtureNames(t, "invalid") {
		t.Run(name, func(t *testing.T) {
			data := readFixture(t, filepath.Join("invalid", name))

			message, err := Decode(data)
			if err != nil {
				return // refused at the door, which is a valid way to refuse it
			}

			var receiver Receiver
			if _, refusal := receiver.Accept(message); refusal == nil {
				t.Errorf("%s was accepted; it must be refused by Decode or by the receiver", name)
			}
		})
	}
}
