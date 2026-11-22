package gamedef

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"go.yaml.in/yaml/v3"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

//go:embed schema/game-definition.v1.json
var schemaJSON []byte

// SchemaJSON returns the embedded JSON schema of the definition format.
func SchemaJSON() []byte {
	return bytes.Clone(schemaJSON)
}

const schemaURL = "https://github.com/IPS-Hosting/game-images/gamedef/schema/game-definition.v1.json"

var (
	compiledSchema    *jsonschema.Schema
	compiledSchemaErr error
	compileOnce       sync.Once
)

func schema() (*jsonschema.Schema, error) {
	compileOnce.Do(func() {
		document, err := jsonschema.UnmarshalJSON(bytes.NewReader(schemaJSON))
		if err != nil {
			compiledSchemaErr = fmt.Errorf("embedded schema is invalid JSON: %w", err)
			return
		}
		compiler := jsonschema.NewCompiler()
		if err := compiler.AddResource(schemaURL, document); err != nil {
			compiledSchemaErr = fmt.Errorf("embedded schema: %w", err)
			return
		}
		compiledSchema, compiledSchemaErr = compiler.Compile(schemaURL)
	})
	return compiledSchema, compiledSchemaErr
}

// Mode selects how strictly a definition is parsed.
type Mode int

const (
	// Strict validates against the JSON schema (unknown fields are errors) and runs all
	// semantic checks. Used by the game-images CI and the importer tooling.
	Strict Mode = iota
	// Lenient tolerates unknown fields (additive changes within the same engine version
	// must not break older engines), but still rejects definitions that need a newer engine
	// and runs the semantic checks on everything this engine understands.
	Lenient
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

// ErrUnsupportedEngine is returned when requires.engine exceeds EngineVersion.
var ErrUnsupportedEngine = errors.New("definition requires a newer engine")

// Parse decodes and validates a game.yaml document.
func Parse(data []byte, mode Mode) (*Definition, error) {
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, &ValidationError{Errors: []Error{{Message: "invalid YAML: " + err.Error()}}}
	}
	if len(root.Content) == 0 {
		return nil, &ValidationError{Errors: []Error{{Message: "document is empty"}}}
	}

	var generic any
	if err := root.Decode(&generic); err != nil {
		return nil, &ValidationError{Errors: []Error{{Message: "invalid YAML: " + err.Error()}}}
	}

	// The engine check comes first: a newer definition may legitimately fail everything
	// else, and the caller needs to distinguish "too new" from "broken".
	if object, ok := generic.(map[string]any); ok {
		if requires, ok := object["requires"].(map[string]any); ok {
			if engine, ok := requires["engine"].(int); ok && engine > EngineVersion {
				return nil, fmt.Errorf("%w: requires engine %d, this engine is %d", ErrUnsupportedEngine, engine, EngineVersion)
			}
		}
	}

	if mode == Strict {
		if errs := validateSchema(generic); len(errs) > 0 {
			return nil, &ValidationError{Errors: errs}
		}
	}

	var definition Definition
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(mode == Strict)
	if err := decoder.Decode(&definition); err != nil {
		return nil, &ValidationError{Errors: []Error{{Message: "invalid definition: " + err.Error()}}}
	}
	if definition.APIVersion != APIVersion {
		return nil, &ValidationError{Errors: []Error{{Path: "/apiVersion", Message: fmt.Sprintf("must be %d", APIVersion)}}}
	}
	definition.applyDefaults()

	if errs := definition.validate(); len(errs) > 0 {
		return nil, &ValidationError{Errors: errs}
	}
	return &definition, nil
}

// validateSchema validates a decoded YAML document against the embedded JSON schema.
func validateSchema(document any) []Error {
	compiled, err := schema()
	if err != nil {
		return []Error{{Message: err.Error()}}
	}

	// Round-trip through JSON so that the schema validator sees JSON types only.
	encoded, err := json.Marshal(document)
	if err != nil {
		return []Error{{Message: "document cannot be represented as JSON: " + err.Error()}}
	}
	instance, err := jsonschema.UnmarshalJSON(bytes.NewReader(encoded))
	if err != nil {
		return []Error{{Message: err.Error()}}
	}

	err = compiled.Validate(instance)
	if err == nil {
		return nil
	}
	var validationErr *jsonschema.ValidationError
	if !errors.As(err, &validationErr) {
		return []Error{{Message: err.Error()}}
	}

	seen := map[string]struct{}{}
	var errs []Error
	collectLeaves(validationErr, &errs, seen)
	sort.SliceStable(errs, func(i, j int) bool { return errs[i].Path < errs[j].Path })
	return errs
}

// collectLeaves flattens the validation error tree to its most specific causes.
func collectLeaves(err *jsonschema.ValidationError, out *[]Error, seen map[string]struct{}) {
	if len(err.Causes) == 0 {
		path := "/" + strings.Join(err.InstanceLocation, "/")
		message := err.ErrorKind.LocalizedString(defaultPrinter)
		key := path + "\x00" + message
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		*out = append(*out, Error{Path: path, Message: message})
		return
	}
	for _, cause := range err.Causes {
		collectLeaves(cause, out, seen)
	}
}

func jsonMarshal(value any) ([]byte, error) {
	return json.Marshal(value)
}

var defaultPrinter = message.NewPrinter(language.English)
