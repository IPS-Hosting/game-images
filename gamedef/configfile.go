package gamedef

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
	"go.yaml.in/yaml/v3"
)

// ApplyConfig computes the new content of a managed config file (SPEC §5.3). existing is
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
		return applyJSON(existing, file.Values)
	case FormatYAML:
		return applyYAML(existing, file.Values)
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

var jsonNumberPattern = regexp.MustCompile(`^-?(0|[1-9][0-9]*)(\.[0-9]+)?([eE][+-]?[0-9]+)?$`)

var iniSectionPattern = regexp.MustCompile(`^\s*\[([^\]]*)\]\s*$`)

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

// jsonLiteral returns the raw JSON for a rendered value: numbers, booleans and null stay
// literals unless the existing value at that location is a string.
func jsonLiteral(value string, existingIsString bool) (string, bool) {
	if existingIsString {
		return "", false
	}
	switch value {
	case "true", "false", "null":
		return value, true
	}
	if jsonNumberPattern.MatchString(value) {
		return value, true
	}
	return "", false
}

func applyJSON(existing []byte, values map[string]string) ([]byte, error) {
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
		current := gjson.GetBytes(document, key)
		if raw, ok := jsonLiteral(value, current.Exists() && current.Type == gjson.String); ok {
			document, err = sjson.SetRawBytes(document, key, []byte(raw))
		} else {
			document, err = sjson.SetBytes(document, key, value)
		}
		if err != nil {
			return nil, fmt.Errorf("set %q: %w", key, err)
		}
	}
	return document, nil
}

func applyYAML(existing []byte, values map[string]string) ([]byte, error) {
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
		if err := setYAMLPath(root.Content[0], strings.Split(key, "."), values[key]); err != nil {
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

func setYAMLPath(mapping *yaml.Node, segments []string, value string) error {
	var child *yaml.Node
	// The first occurrence of a key is managed; later duplicates would override it (or make
	// the file invalid), so they are removed.
	content := mapping.Content[:0]
	for i := 0; i+1 < len(mapping.Content); i += 2 {
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
		if len(segments) == 1 {
			setYAMLScalar(child, value)
			return nil
		}
		if child.Kind != yaml.MappingNode {
			return fmt.Errorf("%q is not a mapping", segments[0])
		}
		return setYAMLPath(child, segments[1:], value)
	}
	keyNode := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: segments[0]}
	if len(segments) == 1 {
		valueNode := &yaml.Node{Kind: yaml.ScalarNode}
		setYAMLScalar(valueNode, value)
		mapping.Content = append(mapping.Content, keyNode, valueNode)
		return nil
	}
	child = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	mapping.Content = append(mapping.Content, keyNode, child)
	return setYAMLPath(child, segments[1:], value)
}

// setYAMLScalar keeps an existing string type (also an empty string); otherwise numbers
// and booleans are typed.
func setYAMLScalar(node *yaml.Node, value string) {
	keepString := node.Kind == yaml.ScalarNode && node.Tag == "!!str"
	*node = yaml.Node{Kind: yaml.ScalarNode, Value: value, Style: node.Style, HeadComment: node.HeadComment, LineComment: node.LineComment, FootComment: node.FootComment}
	if raw, ok := jsonLiteral(value, keepString); ok && raw != "null" {
		node.Tag = map[bool]string{true: "!!bool", false: "!!int"}[raw == "true" || raw == "false"]
		if node.Tag == "!!int" && strings.ContainsAny(raw, ".eE") {
			node.Tag = "!!float"
		}
		return
	}
	node.Tag = "!!str"
	if node.Style == 0 && value == "" {
		node.Style = yaml.DoubleQuotedStyle
	}
}
