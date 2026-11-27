package gamedef

import (
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"
	"unicode/utf8"

	"go.yaml.in/yaml/v3"
)

// checkNodeTypes enforces the JSON types of the schema on the YAML document: the decoder
// would otherwise accept 1 for a string, yes for a bool or null for an object.
func checkNodeTypes(node *yaml.Node, t reflect.Type, at string, errs *[]Error) {
	if node.Kind == yaml.AliasNode {
		node = node.Alias
	}
	if node.Tag == "!!null" {
		if at != "/image" { // the only nullable field
			*errs = append(*errs, Error{Path: at, Message: "must not be null"})
		}
		return
	}
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	fail := func(want string) {
		*errs = append(*errs, Error{Path: at, Message: "must be " + want})
	}
	switch t {
	case reflect.TypeFor[I18n](), reflect.TypeFor[ArgItem]():
		if node.Kind == yaml.ScalarNode {
			if node.Tag != "!!str" {
				fail("a string")
			} else if t == reflect.TypeFor[I18n]() && strings.TrimSpace(node.Value) == "" {
				fail("a non-blank text")
			}
			return
		}
		if t == reflect.TypeFor[I18n]() && node.Kind == yaml.MappingNode {
			for i := 1; i < len(node.Content); i += 2 {
				if child := node.Content[i]; child.Tag == "!!str" && strings.TrimSpace(child.Value) == "" {
					*errs = append(*errs, Error{Path: at + "/" + escapePointer(node.Content[i-1].Value), Message: "must be a non-blank text"})
				}
			}
		}
		if t == reflect.TypeFor[ArgItem]() {
			t = reflect.TypeFor[argObject]()
		}
	}
	switch t.Kind() {
	case reflect.Struct:
		if node.Kind != yaml.MappingNode {
			fail("an object")
			return
		}
		fields := map[string]reflect.Type{}
		for i := range t.NumField() {
			name, _, _ := strings.Cut(t.Field(i).Tag.Get("yaml"), ",")
			if name != "" && name != "-" {
				fields[name] = t.Field(i).Type
			}
		}
		for i := 0; i+1 < len(node.Content); i += 2 {
			if field, ok := fields[node.Content[i].Value]; ok {
				checkNodeTypes(node.Content[i+1], field, at+"/"+escapePointer(node.Content[i].Value), errs)
			}
		}
	case reflect.Map:
		if node.Kind != yaml.MappingNode {
			fail("an object")
			return
		}
		for i := 0; i+1 < len(node.Content); i += 2 {
			checkNodeTypes(node.Content[i+1], t.Elem(), at+"/"+escapePointer(node.Content[i].Value), errs)
		}
	case reflect.Slice:
		if node.Kind != yaml.SequenceNode {
			fail("an array")
			return
		}
		for i, child := range node.Content {
			checkNodeTypes(child, t.Elem(), fmt.Sprintf("%s/%d", at, i), errs)
		}
	case reflect.String:
		if node.Kind != yaml.ScalarNode || node.Tag != "!!str" {
			fail("a string")
		}
	case reflect.Bool:
		if node.Kind != yaml.ScalarNode || node.Tag != "!!bool" {
			fail("a boolean")
		}
	case reflect.Int:
		if node.Kind != yaml.ScalarNode || node.Tag != "!!int" {
			fail("an integer")
		}
	case reflect.Float64:
		if node.Kind != yaml.ScalarNode || (node.Tag != "!!int" && node.Tag != "!!float") {
			fail("a number")
		}
	}
}

func escapePointer(segment string) string {
	return strings.NewReplacer("~", "~0", "/", "~1").Replace(segment)
}

// limit bounds the item count of an array, the property count of an object, the length
// of a string (in code points) or the value of a number at a pointer pattern.
type limit struct {
	path         string
	min, max     float64
	exclusiveMin bool
}

func atMost(path string, max float64) limit { return limit{path: path, max: max} }

func between(path string, min, max float64) limit { return limit{path: path, min: min, max: max} }

// limits mirrors the size and range constraints of the JSON schema.
var limits = func() []limit {
	inf := math.Inf(1)
	l := []limit{
		atMost("/tags", 6), atMost("/sources", 8), atMost("/install", 16), atMost("/ports", 16),
		atMost("/settings", 256), atMost("/config", 32), atMost("/protectedFiles", 64),
		atMost("/backupIgnore", 64), atMost("/preconditions", 16), atMost("/run/exec", 256),
		atMost("/run/tmpfs", 8), atMost("/hooks/postInstall", 32), atMost("/hooks/preStart", 32),
		atMost("/install/*/extraArgs", 8), between("/settings/*/values", 1, 512),
		between("/workshop/settings", 1, 8), atMost("/run/env", 64), atMost("/install/*/env", 64),
		between("/config/*/values", 1, 256), between("/run/imageVariants/images", 1, 32),
		between("/settings/*/validation", 1, inf),

		between("/steamAppId", 1, inf), between("/ports/*/default", 1, 65535), between("/ports/*/group", 0, 255),
		between("/run/stop/timeoutSeconds", 1, 600), between("/install/*/appId", 1, inf),
		between("/install/*/extract/stripComponents", 0, 8), between("/query/countLinesSkip", 0, 8),
		between("/settings/*/generate/length", 4, 256), {path: "/requirements/minCpuThreads", max: 64, exclusiveMin: true},
		between("/requirements/minRam", 128, 262144), between("/requirements/minDiskspace", 1, 4096),
		between("/workshop/appId", 1, inf),

		between("/run/shell", 1, 16384), between("/install/*/shell", 1, 65536),
		between("/run/stop/command", 1, 256), between("/query/command", 1, 256), between("/query/countLinesCommand", 1, 256),
		atMost("/settings/*/default", 2048), atMost("/settings/*/values/*/value", 256),
		between("/settings/*/validation/pattern", 1, 512), between("/run/ready/pattern", 1, 512),
		between("/players/joinPattern", 1, 512), between("/players/leavePattern", 1, 512), between("/query/pattern", 1, 512),
		atMost("/query/path", 512), between("/query/paths/*", 1, inf), atMost("/install/*/url", 2048),
		atMost("/sources/*/ref", 512), atMost("/sources/*/license", 64), between("/config/*/delimiter", 1, 4),
		between("/hooks/*/*/replace/pattern", 1, 512),
	}
	for _, p := range []string{"/run/env/*", "/install/*/env/*", "/run/exec/*", "/run/exec/*/arg", "/run/exec/*/value",
		"/config/*/values/*", "/config/*/template", "/hooks/*/*/replace/replacement", "/hooks/*/*/writeFile/content", "/install/*/branch"} {
		l = append(l, atMost(p, 8192))
	}
	for _, p := range []string{"/name", "/description", "/settings/*/name", "/settings/*/description",
		"/settings/*/values/*/label", "/ports/*/name", "/preconditions/*/message"} {
		l = append(l, atMost(p, 4096), atMost(p+"/*", 4096))
	}
	for _, p := range []string{"/run/image", "/run/dataPath", "/run/workingDir", "/run/tmpfs/*", "/run/imageVariants/images/*",
		"/install/*/image", "/install/*/dir", "/install/*/dest", "/config/*/path", "/config/*/createFrom",
		"/protectedFiles/*/path", "/backupIgnore/*", "/workshop/dir", "/hooks/*/*/*/path", "/hooks/*/*/*/from",
		"/hooks/*/*/*/to", "/hooks/*/*/*/target", "/hooks/*/*/*/link"} {
		l = append(l, atMost(p, 512))
	}
	return l
}()

// checkLimits applies the limits table to the source document.
func checkLimits(document any, errs *[]Error) {
	walkPointers(document, nil, func(pointer []string) {
		value := lookupPointer(document, pointer)
		for _, l := range limits {
			if !matchPointer(l.path, pointer) {
				continue
			}
			var size float64
			unit := "items"
			switch v := value.(type) {
			case []any:
				size = float64(len(v))
			case map[string]any:
				size, unit = float64(len(v)), "properties"
			case string:
				size, unit = float64(utf8.RuneCountInString(v)), "characters"
			case int:
				size, unit = float64(v), ""
			case float64:
				size, unit = v, ""
			default:
				continue
			}
			at := "/" + strings.Join(escapeAll(pointer), "/")
			switch {
			case unit == "" && (size < l.min || size > l.max || (l.exclusiveMin && size <= l.min)):
				*errs = append(*errs, Error{Path: at, Message: fmt.Sprintf("must be between %s and %s", formatNumber(l.min), formatNumber(l.max))})
			case unit != "" && size < l.min:
				*errs = append(*errs, Error{Path: at, Message: fmt.Sprintf("must have at least %s %s", formatNumber(l.min), unit)})
			case unit != "" && size > l.max:
				*errs = append(*errs, Error{Path: at, Message: fmt.Sprintf("must have at most %s %s", formatNumber(l.max), unit)})
			}
		}
	})
}

func escapeAll(pointer []string) []string {
	out := make([]string, len(pointer))
	for i, segment := range pointer {
		out[i] = escapePointer(segment)
	}
	return out
}

func lookupPointer(document any, pointer []string) any {
	for _, segment := range pointer {
		switch node := document.(type) {
		case map[string]any:
			document = node[segment]
		case []any:
			index, _ := strconv.Atoi(segment)
			document = node[index]
		default:
			return nil
		}
	}
	return document
}
