package gamedef

//go:generate go run ./internal/gen

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
)

// EngineBaseline is the engine version that supports every field without a feature entry.
const EngineBaseline = 1

// EngineFeature maps a JSON pointer pattern to the engine version that introduced it. In
// patterns, "*" matches any single segment (array index or map key).
type EngineFeature struct {
	Path   string `json:"path"`
	Engine int    `json:"engine"`
}

// EngineFeatures is the feature table of the definition format. Within apiVersion 1, a new
// field gets an entry with the next engine version. Exported as
// schema/engine-features.v1.json.
var EngineFeatures = []EngineFeature{
	{Path: "/run/imageVariants", Engine: 1},
	{Path: "/run/machineId", Engine: 1},
	{Path: "/run/tmpfs", Engine: 1},
	{Path: "/install/*/beta", Engine: 1},
	{Path: "/install/*/steamSdk", Engine: 1},
	{Path: "/settings/*/generate", Engine: 1},
	{Path: "/protectedFiles", Engine: 1},
	{Path: "/hooks/*/*/writeFile/ifMissing", Engine: 1},
}

// EngineFeaturesJSON returns the content of schema/engine-features.v1.json.
func EngineFeaturesJSON() []byte {
	return marshalGenerated(map[string]any{"baseline": EngineBaseline, "features": EngineFeatures})
}

// MinEngine returns the minimum engine version that can run the definition: the highest
// engine of all feature patterns that match a value present in the source document, at
// least EngineBaseline. It is defined on the source document only; for a definition that
// was not produced by Parse it conservatively returns EngineVersion.
func MinEngine(d *Definition) int {
	if d.raw == nil {
		return EngineVersion
	}
	engine := EngineBaseline
	walkPointers(d.raw, nil, func(pointer []string) {
		for _, feature := range EngineFeatures {
			if feature.Engine > engine && matchPointer(feature.Path, pointer) {
				engine = feature.Engine
			}
		}
	})
	return engine
}

// walkPointers calls visit with the pointer segments of every non-null value below the
// root (the root itself excluded).
func walkPointers(value any, pointer []string, visit func([]string)) {
	if value == nil {
		return
	}
	if len(pointer) > 0 {
		visit(pointer)
	}
	switch node := value.(type) {
	case map[string]any:
		for key, child := range node {
			walkPointers(child, append(pointer[:len(pointer):len(pointer)], key), visit)
		}
	case []any:
		for i, child := range node {
			walkPointers(child, append(pointer[:len(pointer):len(pointer)], strconv.Itoa(i)), visit)
		}
	}
}

// matchPointer matches unescaped pointer segments against a pattern like /install/*/beta.
func matchPointer(pattern string, pointer []string) bool {
	segments := strings.Split(strings.TrimPrefix(pattern, "/"), "/")
	if len(segments) != len(pointer) {
		return false
	}
	for i, segment := range segments {
		segment = strings.NewReplacer("~1", "/", "~0", "~").Replace(segment)
		if segment != "*" && segment != pointer[i] {
			return false
		}
	}
	return true
}

// PanelFlags are the panel flags of a definition after derivation and overrides.
type PanelFlags struct {
	Installable     bool `json:"installable"`
	ShowMap         bool `json:"showMap"`
	ShowPlayerCount bool `json:"showPlayerCount"`
	SteamConnect    bool `json:"steamConnect"`
	ConsoleCommands bool `json:"consoleCommands"`
}

// EffectiveFlags derives the panel flags (SPEC §8) and applies the overrides of d.Flags.
// Installable is always derived.
func EffectiveFlags(d *Definition) PanelFlags {
	q := d.Query
	flags := PanelFlags{
		Installable: len(d.Install) > 0,
		ShowMap: q.Protocol == QueryA2S ||
			(q.Protocol == QueryRcon && hasNamedGroup(q.Pattern, "map")) ||
			(q.Protocol == QueryHTTP && q.Paths != nil && q.Paths.Map != ""),
		ShowPlayerCount: (q.Protocol != "" && q.Protocol != QueryNone) ||
			(d.Players != nil && (d.Players.JoinPattern != "" || d.Players.LeavePattern != "")),
		SteamConnect:    d.SteamAppID != 0 && q.Protocol == QueryA2S,
		ConsoleCommands: d.Run.Console.Via != ConsoleNone,
	}
	override := func(target *bool, value *bool) {
		if value != nil {
			*target = *value
		}
	}
	override(&flags.ShowMap, d.Flags.ShowMap)
	override(&flags.ShowPlayerCount, d.Flags.ShowPlayerCount)
	override(&flags.SteamConnect, d.Flags.SteamConnect)
	override(&flags.ConsoleCommands, d.Flags.ConsoleCommands)
	return flags
}

func hasNamedGroup(pattern string, name string) bool {
	re, err := regexp.Compile(pattern)
	if err != nil || pattern == "" {
		return false
	}
	return re.SubexpIndex(name) >= 0
}

// marshalGenerated encodes a generated JSON file (indented, trailing newline).
func marshalGenerated(value any) []byte {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		panic(err)
	}
	return append(data, '\n')
}
