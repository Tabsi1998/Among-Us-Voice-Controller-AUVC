package game

type PlayMap int

const (
	SKELD PlayMap = iota
	MIRA
	POLUS
	DLEKS // Skeld backwards
	AIRSHIP
	FUNGLE
	EMPTYMAP PlayMap = 10
)

var MapNames = map[PlayMap]string{
	SKELD:   "Skeld",
	MIRA:    "Mira",
	POLUS:   "Polus",
	DLEKS:   "dlekS",
	AIRSHIP: "Airship",
	FUNGLE:  "Fungle",
}

var nameToPlayMap = map[string]int32{
	"the_skeld": (int32)(SKELD),
	"mira_hq":   (int32)(MIRA),
	"polus":     (int32)(POLUS),
	"dleks":     (int32)(DLEKS),
	"airship":   (int32)(AIRSHIP),
	"fungle":    (int32)(FUNGLE),
	"NoMap":     -1,
}

// FormMapUrl builds a map image URL from an operator-provided base URL.
// AUVC ships no default base URL: an empty baseUrl means no external image
// source is configured and callers use the bundled image instead.
func FormMapUrl(baseUrl string, mapType PlayMap, detailed bool) string {
	if baseUrl == "" {
		return ""
	}

	name := MapFileName(mapType, detailed)
	if name == "" {
		return ""
	}
	return baseUrl + name
}
