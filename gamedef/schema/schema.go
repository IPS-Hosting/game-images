// Package schema validates game definitions against the JSON schema
// game-definition.v1.json. It is used by the catalog CI and the platform API; engines only
// need gamedef.Parse and do not link the schema library.
package schema

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/ips-hosting/game-images/gamedef"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"go.yaml.in/yaml/v3"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

//go:embed game-definition.v1.json
var schemaJSON []byte

// URL is the $id of the schema.
const URL = "https://github.com/IPS-Hosting/game-images/gamedef/schema/game-definition.v1.json"

// JSON returns the JSON schema of the definition format.
func JSON() []byte {
	return bytes.Clone(schemaJSON)
}

var compile = sync.OnceValues(func() (*jsonschema.Schema, error) {
	document, err := jsonschema.UnmarshalJSON(bytes.NewReader(schemaJSON))
	if err != nil {
		return nil, fmt.Errorf("embedded schema is invalid JSON: %w", err)
	}
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource(URL, document); err != nil {
		return nil, fmt.Errorf("embedded schema: %w", err)
	}
	return compiler.Compile(URL)
})

var printer = message.NewPrinter(language.English)

// ValidateSchema validates a game.yaml (or JSON) document against the JSON schema. It
// returns a *gamedef.ValidationError listing the most specific violations.
func ValidateSchema(data []byte) error {
	compiled, err := compile()
	if err != nil {
		return err
	}
	var document any
	if err := yaml.Unmarshal(data, &document); err != nil {
		return &gamedef.ValidationError{Errors: []gamedef.Error{{Message: "invalid YAML: " + err.Error()}}}
	}
	// Round-trip through JSON so that the validator sees JSON types only.
	encoded, err := json.Marshal(document)
	if err != nil {
		return &gamedef.ValidationError{Errors: []gamedef.Error{{Message: "document cannot be represented as JSON: " + err.Error()}}}
	}
	instance, err := jsonschema.UnmarshalJSON(bytes.NewReader(encoded))
	if err != nil {
		return err
	}
	err = compiled.Validate(instance)
	var validationErr *jsonschema.ValidationError
	if err == nil || !errors.As(err, &validationErr) {
		return err
	}
	seen := map[string]bool{}
	var errs []gamedef.Error
	collectLeaves(validationErr, &errs, seen)
	sort.SliceStable(errs, func(i, j int) bool { return errs[i].Path < errs[j].Path })
	return &gamedef.ValidationError{Errors: errs}
}

// collectLeaves flattens the validation error tree to its most specific causes.
func collectLeaves(err *jsonschema.ValidationError, out *[]gamedef.Error, seen map[string]bool) {
	if len(err.Causes) == 0 {
		path := "/" + strings.Join(err.InstanceLocation, "/")
		text := err.ErrorKind.LocalizedString(printer)
		if !seen[path+"\x00"+text] {
			seen[path+"\x00"+text] = true
			*out = append(*out, gamedef.Error{Path: path, Message: text})
		}
		return
	}
	for _, cause := range err.Causes {
		collectLeaves(cause, out, seen)
	}
}
