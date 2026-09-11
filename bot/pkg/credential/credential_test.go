package credential

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

const plaintext = "SUPER-SECRET-VALUE"

// The requirements say credentials must never be logged in plaintext. A rule
// everybody has to remember is worth less than a type that cannot be printed,
// so every ordinary way of writing a value out is checked here.
func TestASecretCannotBePrintedByAccident(t *testing.T) {
	secret := NewSecret(plaintext)

	printed := map[string]string{
		"%v":          fmt.Sprintf("%v", secret),
		"%s":          fmt.Sprintf("%s", secret),
		"%q":          fmt.Sprintf("%q", secret),
		"%#v":         fmt.Sprintf("%#v", secret),
		"%x":          fmt.Sprintf("%x", secret),
		"%+v":         fmt.Sprintf("%+v", secret),
		"Sprint":      fmt.Sprint(secret),
		"Sprintln":    fmt.Sprintln(secret),
		"in a struct": fmt.Sprintf("%v", struct{ Token Secret }{secret}),
	}

	for how, output := range printed {
		if strings.Contains(output, plaintext) {
			t.Errorf("%s leaked the secret: %s", how, output)
		}
		if !strings.Contains(output, Redacted) {
			t.Errorf("%s did not redact: %s", how, output)
		}
	}
}

// Anything serialized can end up in a file, a support bundle or a Discord
// message, so JSON has to redact as well.
func TestASecretIsRedactedInJSON(t *testing.T) {
	encoded, err := json.Marshal(struct {
		Token Secret `json:"token"`
	}{NewSecret(plaintext)})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	if strings.Contains(string(encoded), plaintext) {
		t.Errorf("JSON leaked the secret: %s", encoded)
	}
	if !strings.Contains(string(encoded), Redacted) {
		t.Errorf("JSON did not redact: %s", encoded)
	}
}

// Redaction is worthless if the value cannot be retrieved deliberately.
func TestRevealReturnsTheValue(t *testing.T) {
	if got := NewSecret(plaintext).Reveal(); got != plaintext {
		t.Errorf("Reveal returned %q", got)
	}
	if !NewSecret("").Empty() {
		t.Error("an empty secret should report itself as empty")
	}
}

// An empty redaction placeholder reads like a missing value and sends people
// looking for a bug that is not there.
func TestTheRedactionPlaceholderIsVisible(t *testing.T) {
	if Redacted == "" {
		t.Error("the placeholder must not be empty")
	}
}

func TestPairingCodesUseTheUnambiguousAlphabet(t *testing.T) {
	for i := 0; i < 50; i++ {
		code, err := NewPairingCode()
		if err != nil {
			t.Fatalf("generate: %v", err)
		}

		raw := code.Secret.Reveal()
		if len(raw) != pairingCodeLength {
			t.Fatalf("code %q has length %d, want %d", raw, len(raw), pairingCodeLength)
		}
		for _, r := range raw {
			if !strings.ContainsRune(pairingAlphabet, r) {
				t.Errorf("code %q contains %q, which is not in the alphabet", raw, r)
			}
			if strings.ContainsRune("ILOU", r) {
				t.Errorf("code %q contains %q, which people confuse when reading aloud", raw, r)
			}
		}
	}
}

func TestPairingCodesDiffer(t *testing.T) {
	seen := map[string]bool{}

	for i := 0; i < 200; i++ {
		code, err := NewPairingCode()
		if err != nil {
			t.Fatalf("generate: %v", err)
		}
		raw := code.Secret.Reveal()
		if seen[raw] {
			t.Fatalf("generated %q twice in 200 codes", raw)
		}
		seen[raw] = true
	}
}

func TestDisplayGroupsTheCode(t *testing.T) {
	code := PairingCode{Secret: NewSecret("ABCD1234")}

	if got, want := code.Display(), "AUVC-ABCD-1234"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// Rejecting a correctly read code because of a hyphen or a lower-case letter
// sends the user back to Discord for a new one.
func TestNormalizeAcceptsHowPeopleActuallyType(t *testing.T) {
	const canonical = "ABCD1234"

	cases := map[string]string{
		"canonical":        "ABCD1234",
		"lower case":       "abcd1234",
		"grouped":          "ABCD-1234",
		"with the prefix":  "AUVC-ABCD-1234",
		"prefix no dashes": "AUVCABCD1234",
		"spaced":           " ABCD 1234 ",
		"letter O for 0":   "ABCD123O",
		"letter I for 1":   "ABCDI234",
		"letter L for 1":   "ABCDL234",
	}

	want := map[string]string{
		"letter O for 0": "ABCD1230",
		"letter I for 1": "ABCD1234",
		"letter L for 1": "ABCD1234",
	}

	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			expected := canonical
			if override, ok := want[name]; ok {
				expected = override
			}
			if got := NormalizePairingCode(input); got != expected {
				t.Errorf("normalized %q to %q, want %q", input, got, expected)
			}
		})
	}
}

// U maps to V, so stripping the prefix after that fix would leave AVVC behind
// and a pasted code would never match.
func TestNormalizeStripsThePrefixBeforeFixingLetters(t *testing.T) {
	if got := NormalizePairingCode("AUVC-ABCD-1234"); strings.Contains(got, "V") {
		t.Errorf("the prefix survived normalization: %q", got)
	}
}

func TestHashIsStableAndDistinct(t *testing.T) {
	first := Hash(NewSecret("one"))

	if first != Hash(NewSecret("one")) {
		t.Error("hashing the same secret twice gave different results")
	}
	if first == Hash(NewSecret("two")) {
		t.Error("two different secrets hashed to the same value")
	}
	if strings.Contains(first, "one") {
		t.Errorf("the hash contains the secret: %s", first)
	}
}

func TestMatchesAcceptsOnlyTheRightSecret(t *testing.T) {
	stored := Hash(NewSecret("right"))

	if !Matches(stored, NewSecret("right")) {
		t.Error("the correct secret was rejected")
	}
	if Matches(stored, NewSecret("wrong")) {
		t.Error("a wrong secret was accepted")
	}
	if Matches("", NewSecret("right")) {
		t.Error("an empty stored hash must never match")
	}
	if Matches(stored, NewSecret("")) {
		t.Error("an empty presented secret must not match")
	}
}

func TestCredentialsDiffer(t *testing.T) {
	ids := map[string]bool{}
	secrets := map[string]bool{}

	for i := 0; i < 100; i++ {
		issued, err := NewCredential()
		if err != nil {
			t.Fatalf("issue: %v", err)
		}
		if ids[issued.ID] {
			t.Fatalf("issued id %s twice", issued.ID)
		}
		if secrets[issued.Secret.Reveal()] {
			t.Fatal("issued the same secret twice")
		}
		ids[issued.ID] = true
		secrets[issued.Secret.Reveal()] = true
	}
}

func TestATokenSurvivesARoundTrip(t *testing.T) {
	issued, err := NewCredential()
	if err != nil {
		t.Fatalf("issue: %v", err)
	}

	id, secret, err := ParseToken(issued.Token().Reveal())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if id != issued.ID {
		t.Errorf("parsed id %q, want %q", id, issued.ID)
	}
	if secret.Reveal() != issued.Secret.Reveal() {
		t.Error("the parsed secret does not match the issued one")
	}
}

// A token is the only thing standing between a stranger and a guild's voice
// channels, so anything that is not shaped like one is refused rather than
// coerced into something.
func TestMalformedTokensAreRefused(t *testing.T) {
	cases := map[string]string{
		"empty":          "",
		"no separator":   "abcdef0123456789",
		"no secret":      "abcdef0123456789.",
		"no identifier":  ".SOMESECRET",
		"non-hex id":     "zzzz.SOMESECRET",
		"only separator": ".",
	}

	for name, token := range cases {
		t.Run(name, func(t *testing.T) {
			if _, _, err := ParseToken(token); err == nil {
				t.Errorf("%q was accepted as a token", token)
			}
		})
	}
}

// The token itself is a secret, so building one must not produce a value that
// prints.
func TestATokenIsItselfRedacted(t *testing.T) {
	issued, err := NewCredential()
	if err != nil {
		t.Fatalf("issue: %v", err)
	}

	if printed := fmt.Sprintf("%v", issued.Token()); strings.Contains(printed, issued.ID) {
		t.Errorf("the token printed its contents: %s", printed)
	}
}
