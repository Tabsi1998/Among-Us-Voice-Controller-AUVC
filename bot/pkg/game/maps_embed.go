package game

import (
	"embed"
	"fmt"
)

// mapFiles carries the Among Us map images inside the binary. AUVC must be able
// to answer /map without fetching anything from a host it does not control, and
// the runtime image does not ship a data directory, so embedding is the only
// delivery that works for both the Docker image and a bare binary.
//
//go:embed maps/*.png
var mapFiles embed.FS

// MapFileName returns the bundled image file name for a map, or an empty string
// if the map has no image. dleks only exists as the simple variant.
func MapFileName(mapType PlayMap, detailed bool) string {
	if mapType == EMPTYMAP {
		return ""
	}

	name := ""
	for candidate, value := range nameToPlayMap {
		if value == int32(mapType) {
			name = candidate
			break
		}
	}
	if name == "" {
		return ""
	}

	if detailed && mapType != DLEKS {
		return fmt.Sprintf("%s_detailed.png", name)
	}
	return fmt.Sprintf("%s.png", name)
}

// MapImage returns the bundled image for a map together with its file name.
// The second return value is false when the map has no bundled image.
func MapImage(mapType PlayMap, detailed bool) (string, []byte, bool) {
	name := MapFileName(mapType, detailed)
	if name == "" {
		return "", nil, false
	}

	data, err := mapFiles.ReadFile("maps/" + name)
	if err != nil {
		return "", nil, false
	}
	return name, data, true
}
