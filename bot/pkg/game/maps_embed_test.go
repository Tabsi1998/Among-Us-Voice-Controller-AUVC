package game

import (
	"bytes"
	"testing"
)

var pngMagic = []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}

// Every selectable map must be answerable from the binary alone. If an image
// stops being embedded, /map would silently fall back to nothing.
func TestEveryNamedMapHasABundledImage(t *testing.T) {
	for playMap, name := range MapNames {
		for _, detailed := range []bool{false, true} {
			file, data, ok := MapImage(playMap, detailed)
			if !ok {
				t.Fatalf("no bundled image for %s (detailed=%v)", name, detailed)
			}
			if !bytes.HasPrefix(data, pngMagic) {
				t.Errorf("bundled image %s is not a PNG", file)
			}
		}
	}
}

// dleks only ever existed as the simple variant upstream.
func TestDleksHasNoDetailedVariant(t *testing.T) {
	simple := MapFileName(DLEKS, false)
	detailed := MapFileName(DLEKS, true)
	if simple != detailed {
		t.Errorf("expected dleks to reuse the simple image, got %q and %q", simple, detailed)
	}
}

func TestEmptyMapHasNoImage(t *testing.T) {
	if name := MapFileName(EMPTYMAP, false); name != "" {
		t.Errorf("expected no file name for EMPTYMAP, got %q", name)
	}
	if _, _, ok := MapImage(EMPTYMAP, false); ok {
		t.Error("expected no bundled image for EMPTYMAP")
	}
}

// AUVC ships no default base URL; an unset BASE_MAP_URL must not silently point
// at somebody else's host.
func TestFormMapUrlWithoutBaseUrlReturnsEmpty(t *testing.T) {
	if url := FormMapUrl("", SKELD, false); url != "" {
		t.Errorf("expected an empty URL without a configured base, got %q", url)
	}
}

func TestFormMapUrlUsesTheConfiguredBase(t *testing.T) {
	got := FormMapUrl("https://maps.example/", POLUS, true)
	if want := "https://maps.example/polus_detailed.png"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
