package gamedef

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"

	"go.yaml.in/yaml/v3"
)

// ValidationError collects all problems of one definition.
type ValidationError struct {
	Errors []Error
}

func (e *ValidationError) Error() string {
	messages := make([]string, 0, len(e.Errors))
	for _, err := range e.Errors {
		messages = append(messages, err.Error())
	}
	return strings.Join(messages, "\n")
}

func invalid(path string, format string, args ...any) *ValidationError {
	return &ValidationError{Errors: []Error{{Path: path, Message: fmt.Sprintf(format, args...)}}}
}

// Parse decodes a game.yaml (or JSON) document strictly, rejecting unknown fields, and runs
// the structural and semantic checks (SPEC §9). The JSON schema check is separate
// (package schema) so that engines do not need the schema library.
func Parse(data []byte) (*Definition, error) {
	var raw any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, invalid("", "invalid YAML: %v", err)
	}
	if raw == nil {
		return nil, invalid("", "document is empty")
	}
	if _, ok := raw.(map[string]any); !ok {
		return nil, invalid("", "document must be a mapping")
	}
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, invalid("", "invalid YAML: %v", err)
	}
	var typeErrs []Error
	checkNodeTypes(root.Content[0], reflect.TypeFor[Definition](), "", &typeErrs)
	if len(typeErrs) > 0 {
		return nil, &ValidationError{Errors: typeErrs}
	}

	var definition Definition
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&definition); err != nil {
		return nil, invalid("", "invalid definition: %v", err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return nil, invalid("", "expected a single YAML document")
	}
	if definition.APIVersion != APIVersion {
		return nil, invalid("/apiVersion", "must be %d", APIVersion)
	}
	definition.raw = raw
	definition.applyDefaults()

	var errs []Error
	checkLimits(raw, &errs)
	if errs = append(errs, definition.validate()...); len(errs) > 0 {
		return nil, &ValidationError{Errors: errs}
	}
	return &definition, nil
}
