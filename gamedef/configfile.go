package gamedef

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
	"go.yaml.in/yaml/v3"
)

// ApplyConfig computes the new content of a managed config file (SPEC §5.4). existing is
// the current content (nil when the file does not exist). Unmanaged content (other keys,
// comments, order) is preserved; every occurrence of a managed key is set, duplicates of a
// managed key after the first occurrence are removed, and missing keys are appended.
// The function is pure: writing (atomic, inside the container) is the caller's job.
func ApplyConfig(file PlanConfigFile, existing []byte) ([]byte, error) {
	switch file.Format {
	case FormatTemplate:
		return []byte(file.Content), nil
	case FormatProperties:
		return applyLines(existing, file.Values, "=", true), nil
	case FormatKV:
		delimiter := file.Delimiter
		if delimiter == "" {
			delimiter = "="
		}
		return applyLines(existing, file.Values, delimiter, false), nil
	case FormatINI:
		return applyINI(existing, file.Values), nil
	case FormatJSON:
		return applyJSON(existing, file.Values, file.Types)
	case FormatYAML:
		return applyYAML(existing, file.Values, file.Types)
	case FormatLines:
		return applyPrefixLines(existing, file.Values), nil
	case FormatXML:
		return applyXML(existing, file.Values)
	}
	return nil, fmt.Errorf("unsupported config format %q", file.Format)
}

// lineKey returns the key of a key/value line, or "" for comments and other lines.
// properties also accepts ':' as separator (java.util.Properties).
func lineKey(line string, delimiter string, properties bool) string {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "!") || strings.HasPrefix(trimmed, ";") || strings.HasPrefix(trimmed, "//") {
		return ""
	}
	index := strings.Index(trimmed, delimiter)
	if properties {
		if colon := strings.Index(trimmed, ":"); colon >= 0 && (index < 0 || colon < index) {
			index = colon
		}
	}
	if index <= 0 {
		return ""
	}
	return strings.TrimSpace(trimmed[:index])
}

func splitLines(content []byte) ([]string, string) {
	text := string(content)
	newline := "\n"
	if strings.Contains(text, "\r\n") {
		newline = "\r\n"
	}
	text = strings.TrimSuffix(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	if text == "" {
		return nil, newline
	}
	return strings.Split(text, "\n"), newline
}

func joinLines(lines []string, newline string) []byte {
	if len(lines) == 0 {
		return nil
	}
	return []byte(strings.Join(lines, newline) + newline)
}

func applyLines(existing []byte, values map[string]string, delimiter string, properties bool) []byte {
	lines, newline := splitLines(existing)
	seen := map[string]bool{}
	out := make([]string, 0, len(lines)+len(values))
	for _, line := range lines {
		key := lineKey(line, delimiter, properties)
		value, managed := values[key]
		if key == "" || !managed {
			out = append(out, line)
			continue
		}
		if seen[key] {
			continue // later duplicates would override the managed value in most parsers
		}
		seen[key] = true
		out = append(out, key+delimiter+value)
	}
	for _, key := range sortedKeys(values) {
		if !seen[key] {
			out = append(out, key+delimiter+values[key])
		}
	}
	return joinLines(out, newline)
}

// applyPrefixLines implements format lines: every line starting with a managed prefix is
// replaced by its full line (the longest matching prefix wins); missing lines are appended.
func applyPrefixLines(existing []byte, values map[string]string) []byte {
	lines, newline := splitLines(existing)
	prefixes := sortedKeys(values)
	seen := map[string]bool{}
	for i, line := range lines {
		match := ""
		for _, prefix := range prefixes {
			if strings.HasPrefix(line, prefix) && len(prefix) > len(match) {
				match = prefix
			}
		}
		if match != "" {
			lines[i] = values[match]
			seen[match] = true
		}
	}
	for _, prefix := range prefixes {
		if !seen[prefix] {
			lines = append(lines, values[prefix])
		}
	}
	return joinLines(lines, newline)
}

var iniSectionPattern = regexp.MustCompile(`^\s*\[([^\]]*)\]\s*([;#].*)?$`)

// splitINIKey splits "Section.key" at the last dot: section names may contain dots
// (Unreal: [/Script/Engine.GameSession]), keys practically never do. A key without dot is
// a global key before the first section.
func splitINIKey(key string) (string, string) {
	index := strings.LastIndex(key, ".")
	if index < 0 {
		return "", key
	}
	return key[:index], key[index+1:]
}

func applyINI(existing []byte, values map[string]string) []byte {
	bySection := map[string]map[string]string{}
	var sectionOrder []string
	for _, full := range sortedKeys(values) {
		section, key := splitINIKey(full)
		if bySection[section] == nil {
			bySection[section] = map[string]string{}
			sectionOrder = append(sectionOrder, section)
		}
		bySection[section][key] = values[full]
	}

	lines, newline := splitLines(existing)
	var out []string
	section := ""
	seen := map[string]map[string]bool{}
	sectionsPresent := map[string]bool{"": true}

	flush := func(name string) {
		wanted := bySection[name]
		for _, key := range sortedKeys(wanted) {
			if !seen[name][key] {
				out = append(out, key+"="+wanted[key])
				if seen[name] == nil {
					seen[name] = map[string]bool{}
				}
				seen[name][key] = true
			}
		}
	}

	for _, line := range lines {
		if match := iniSectionPattern.FindStringSubmatch(line); match != nil {
			flush(section)
			section = strings.TrimSpace(match[1])
			sectionsPresent[section] = true
			out = append(out, line)
			continue
		}
		key := lineKey(line, "=", false)
		wanted := bySection[section]
		value, isManaged := wanted[key]
		if key == "" || !isManaged {
			out = append(out, line)
			continue
		}
		if seen[section] == nil {
			seen[section] = map[string]bool{}
		}
		if seen[section][key] {
			continue
		}
		seen[section][key] = true
		out = append(out, key+"="+value)
	}
	flush(section)

	// Global keys must precede the first section header.
	if globals := bySection[""]; len(globals) > 0 && len(seen[""]) < len(globals) {
		var head []string
		for _, key := range sortedKeys(globals) {
			if !seen[""][key] {
				head = append(head, key+"="+globals[key])
			}
		}
		out = append(head, out...)
	}

	for _, name := range sectionOrder {
		if name == "" || sectionsPresent[name] {
			continue
		}
		out = append(out, "["+name+"]")
		flush(name)
	}
	return joinLines(out, newline)
}

// Value types of json and yaml config values (PlanConfigFile.Types).
const (
	TypeString = "string"
	TypeNumber = "number"
	TypeBool   = "bool"
)

// literalKind returns the type a rendered value can be written as: bool for true/false,
// number for the decimal number syntax, otherwise "".
func literalKind(value string) string {
	switch {
	case value == "true" || value == "false":
		return TypeBool
	case numberPattern.MatchString(value):
		return TypeNumber
	}
	return ""
}

// emitLiteral decides whether a value is written as a number/bool literal: the type hint
// decides when present, otherwise the type of the existing value (SPEC §5.4).
func emitLiteral(value string, hint string, existing string) bool {
	kind := literalKind(value)
	if kind == "" {
		return false
	}
	if hint != "" {
		return hint == kind
	}
	return existing == kind
}

// splitKeyPath splits a json/yaml key at unescaped dots; "\." is a literal dot.
func splitKeyPath(key string) []string {
	var segments []string
	var current strings.Builder
	for i := 0; i < len(key); i++ {
		switch {
		case key[i] == '\\' && i+1 < len(key) && key[i+1] == '.':
			current.WriteByte('.')
			i++
		case key[i] == '.':
			segments = append(segments, current.String())
			current.Reset()
		default:
			current.WriteByte(key[i])
		}
	}
	return append(segments, current.String())
}

func applyJSON(existing []byte, values map[string]string, types map[string]string) ([]byte, error) {
	document := existing
	if len(bytes.TrimSpace(document)) == 0 {
		document = []byte("{}")
	}
	if !gjson.ValidBytes(document) {
		return nil, fmt.Errorf("existing file is not valid JSON")
	}
	var err error
	for _, key := range sortedKeys(values) {
		value := values[key]
		document = dedupJSON(document, splitKeyPath(key))
		existingKind := ""
		switch gjson.GetBytes(document, key).Type {
		case gjson.Number:
			existingKind = TypeNumber
		case gjson.True, gjson.False:
			existingKind = TypeBool
		}
		if emitLiteral(value, types[key], existingKind) {
			document, err = sjson.SetRawBytes(document, key, []byte(value))
		} else {
			document, err = sjson.SetBytes(document, key, value)
		}
		if err != nil {
			return nil, fmt.Errorf("set %q: %w", key, err)
		}
	}
	return document, nil
}

// jsonMember is one member of a JSON object: end of the previous member (or of "{"), the
// start of its value and its end, as byte offsets into the document.
type jsonMember struct {
	key                     string
	previousEnd, valueStart int
	end                     int
}

// jsonMembers lists the members of the object starting at offset at (false if it is not
// an object).
func jsonMembers(document []byte, at int) ([]jsonMember, bool) {
	decoder := json.NewDecoder(bytes.NewReader(document[at:]))
	if token, err := decoder.Token(); err != nil || token != json.Delim('{') {
		return nil, false
	}
	var members []jsonMember
	previous := int(decoder.InputOffset())
	for decoder.More() {
		token, err := decoder.Token()
		key, ok := token.(string)
		if err != nil || !ok {
			return nil, false
		}
		var raw json.RawMessage
		if err := decoder.Decode(&raw); err != nil {
			return nil, false
		}
		end := int(decoder.InputOffset())
		members = append(members, jsonMember{key: key, previousEnd: at + previous, valueStart: at + end - len(raw), end: at + end})
		previous = end
	}
	return members, true
}

// dedupJSON removes later duplicates of every object member along the key path, so that
// the managed value is the only one (most parsers use the last duplicate).
func dedupJSON(document []byte, segments []string) []byte {
	at := len(document) - len(bytes.TrimLeft(document, " \t\r\n"))
	for _, segment := range segments {
		members, ok := jsonMembers(document, at)
		if !ok {
			return document
		}
		var matches []jsonMember
		for _, member := range members {
			if member.key == segment {
				matches = append(matches, member)
			}
		}
		if len(matches) == 0 {
			return document
		}
		for i := len(matches) - 1; i > 0; i-- {
			document = append(document[:matches[i].previousEnd:matches[i].previousEnd], document[matches[i].end:]...)
		}
		at = matches[0].valueStart
	}
	return document
}

func applyYAML(existing []byte, values map[string]string, types map[string]string) ([]byte, error) {
	var root yaml.Node
	if len(bytes.TrimSpace(existing)) > 0 {
		if err := yaml.Unmarshal(existing, &root); err != nil {
			return nil, fmt.Errorf("existing file is not valid YAML: %w", err)
		}
	}
	if root.Kind == 0 {
		root = yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{{Kind: yaml.MappingNode, Tag: "!!map"}}}
	}
	if root.Kind != yaml.DocumentNode || len(root.Content) == 0 || root.Content[0].Kind != yaml.MappingNode {
		return nil, fmt.Errorf("existing YAML file must contain a mapping")
	}
	for _, key := range sortedKeys(values) {
		if err := setYAMLPath(root.Content[0], splitKeyPath(key), values[key], types[key]); err != nil {
			return nil, fmt.Errorf("set %q: %w", key, err)
		}
	}
	var out bytes.Buffer
	encoder := yaml.NewEncoder(&out)
	encoder.SetIndent(2)
	if err := encoder.Encode(&root); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// errYAMLAnchor refuses to manage anchored or aliased nodes: changing them would silently
// change every alias as well.
var errYAMLAnchor = fmt.Errorf("anchored or aliased YAML nodes cannot be managed")

func setYAMLPath(mapping *yaml.Node, segments []string, value string, hint string) error {
	if mapping.Anchor != "" {
		return errYAMLAnchor
	}
	var child *yaml.Node
	// The first occurrence of a key is managed; later duplicates would override it (or make
	// the file invalid), so they are removed.
	content := mapping.Content[:0]
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Tag == "!!merge" {
			return errYAMLAnchor
		}
		if mapping.Content[i].Value == segments[0] {
			if child != nil {
				continue
			}
			child = mapping.Content[i+1]
		}
		content = append(content, mapping.Content[i], mapping.Content[i+1])
	}
	mapping.Content = content
	if child != nil {
		if child.Kind == yaml.AliasNode || child.Anchor != "" {
			return errYAMLAnchor
		}
		if len(segments) == 1 {
			setYAMLScalar(child, value, hint)
			return nil
		}
		if child.Kind != yaml.MappingNode {
			return fmt.Errorf("%q is not a mapping", segments[0])
		}
		return setYAMLPath(child, segments[1:], value, hint)
	}
	keyNode := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: segments[0]}
	if len(segments) == 1 {
		valueNode := &yaml.Node{Kind: yaml.ScalarNode}
		setYAMLScalar(valueNode, value, hint)
		mapping.Content = append(mapping.Content, keyNode, valueNode)
		return nil
	}
	child = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	mapping.Content = append(mapping.Content, keyNode, child)
	return setYAMLPath(child, segments[1:], value, hint)
}

// yaml11Words resolve to booleans or null in YAML 1.1 parsers, so strings are quoted.
var yaml11Words = regexp.MustCompile(`^(?i:y|yes|n|no|on|off|true|false|null|~)$`)

// setYAMLScalar writes a value with the type chosen by emitLiteral; strings that a YAML
// parser could read as another type are double-quoted.
func setYAMLScalar(node *yaml.Node, value string, hint string) {
	existing := ""
	if node.Kind == yaml.ScalarNode {
		switch node.Tag {
		case "!!int", "!!float":
			existing = TypeNumber
		case "!!bool":
			existing = TypeBool
		}
	}
	*node = yaml.Node{Kind: yaml.ScalarNode, Value: value, Style: node.Style, HeadComment: node.HeadComment, LineComment: node.LineComment, FootComment: node.FootComment}
	if emitLiteral(value, hint, existing) {
		node.Tag = map[string]string{TypeBool: "!!bool", TypeNumber: "!!int"}[literalKind(value)]
		if node.Tag == "!!int" && strings.ContainsAny(value, ".eE") {
			node.Tag = "!!float"
		}
		node.Style = 0
		return
	}
	node.Tag = "!!str"
	if node.Style&(yaml.DoubleQuotedStyle|yaml.SingleQuotedStyle|yaml.LiteralStyle|yaml.FoldedStyle) == 0 &&
		(value == "" || literalKind(value) != "" || yaml11Words.MatchString(value)) {
		node.Style = yaml.DoubleQuotedStyle
	}
}
