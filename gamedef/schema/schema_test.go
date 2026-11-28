package schema

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ips-hosting/game-images/gamedef"
)

// TestCatalog validates every definition in the repository like the CI does: JSON schema,
// strict parse with semantic checks, and short == directory name.
func TestCatalog(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("..", "..", "games", "*", "game.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Skip("no catalog in this checkout")
	}
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if err := ValidateSchema(data); err != nil {
			t.Errorf("%s: schema: %v", file, err)
		}
		definition, err := gamedef.Parse(data)
		if err != nil {
			t.Errorf("%s: %v", file, err)
			continue
		}
		if dir := filepath.Base(filepath.Dir(file)); definition.Short != dir {
			t.Errorf("%s: short %q != directory %q", file, definition.Short, dir)
		}
	}
}

const valid = `
apiVersion: 1
short: test
name: Test
run:
  image: ipshosting/runtime-test:v1
  tmpfs: [/tmp]
  exec:
    - ./server
    - arg: -x
      when: { all: [{ setting: a, truthy: true }, { not: { setting: a, in: ['2'] } }] }
ports:
  - { identifier: game, protocol: UDP, default: 1 }
query: { protocol: http, port: game, path: /status }
settings:
  - { key: a, name: A, type: NUMBER, default: '1', validation: { min: 0, integer: true }, category: gameplay }
config:
  - { path: a.xml, format: xml, values: { 'Root.A@x': '1' } }
protectedFiles:
  - { path: /server, readOnly: true }
`

// TestSchemaAndParseAgree checks documents that both validators must accept or reject.
func TestSchemaAndParseAgree(t *testing.T) {
	cases := map[string]bool{
		valid:                      true,
		valid + "futureField: 1\n": false,
		strings.Replace(valid, "path: /status", "path: /status, url: 'http://x'", 1):                           false,
		strings.Replace(valid, "{ setting: a, truthy: true }", "{ setting: a, truthy: true, equals: '1' }", 1): false,
		strings.Replace(valid, "{ setting: a, truthy: true }", "'{{ .Setting \"a\" }}'", 1):                    false,
		strings.Replace(valid, "category: gameplay", "category: Allgemein", 1):                                 false,
		strings.Replace(valid, "readOnly: true", "hidden: false", 1):                                           false,
		strings.Replace(valid, "image: ipshosting/runtime-test:v1", "image: '{{ .Setting \"a\" }}'", 1):        false,
		strings.Replace(valid, "format: xml", "format: template", 1):                                           false,
	}
	for doc, ok := range cases {
		schemaErr := ValidateSchema([]byte(doc))
		_, parseErr := gamedef.Parse([]byte(doc))
		if (schemaErr == nil) != ok || (parseErr == nil) != ok {
			t.Errorf("want ok=%v, schema: %v, parse: %v\n%s", ok, schemaErr, parseErr, doc)
		}
	}
}

// TestVocabulariesMatchGo keeps the schema enums in sync with the Go tables.
func TestVocabulariesMatchGo(t *testing.T) {
	var document struct {
		Defs map[string]struct {
			Enum []string `json:"enum"`
		} `json:"$defs"`
	}
	if err := json.Unmarshal(JSON(), &document); err != nil {
		t.Fatal(err)
	}
	var categories []string
	for _, category := range gamedef.Categories {
		categories = append(categories, category.ID)
	}
	if got := document.Defs["category"].Enum; !slices.Equal(got, categories) {
		t.Errorf("schema categories %v != Go %v", got, categories)
	}
	if got := document.Defs["tag"].Enum; !slices.Equal(got, gamedef.Tags) {
		t.Errorf("schema tags %v != Go %v", got, gamedef.Tags)
	}
}

const mutationBase = `
apiVersion: 1
short: test
name: Test
steamAppId: 5
requirements: { minRam: 1024, minCpuThreads: 2 }
run:
  image: ipshosting/runtime-test:v1
  machineId: true
  tmpfs: [/tmp]
  env: { LD_LIBRARY_PATH: ./lib, MODE: '{{ .Setting "a" }}' }
  exec:
    - ./server
    - arg: -x
      when: { setting: a, in: ['1'] }
install:
  - { kind: steamcmd, appId: 5 }
  - { kind: script, shell: 'true', env: { A: b } }
ports:
  - { identifier: game, protocol: UDP, default: 1, group: 1 }
query: { protocol: http, port: game, path: /status }
settings:
  - { key: a, name: A, type: NUMBER, default: '1', category: gameplay }
protectedFiles:
  - { path: /server, readOnly: false, hidden: true }
`

// TestSchemaAndParseAgreeOnMutations applies one mutation per schema constraint to a valid
// document; the schema and Parse must both reject it (or both accept it).
func TestSchemaAndParseAgreeOnMutations(t *testing.T) {
	many := "[" + strings.Repeat("'1', ", 64) + "'1']"
	cases := []struct {
		old, new string
		ok       bool
	}{
		{"", "", true},
		{"readOnly: false, hidden: true", "readOnly: false, hidden: false", false},
		{"readOnly: false, hidden: true", "hidden: false", false},
		{"default: '1', category", "default: 1, category", false},
		{"path: /status }", "path: /status, query: {} }", false},
		{"path: /status }", "path: '/st{a}' }", false},
		{"minRam: 1024", "minRam: 1", false},
		{"minCpuThreads: 2", "minCpuThreads: -1", false},
		{"minCpuThreads: 2", "minCpuThreads: 0", false},
		{"steamAppId: 5", "steamAppId: -5", false},
		{"group: 1", "group: 9999", false},
		{"default: 1, group", "default: 70000, group", false},
		{"machineId: true", "machineId: yes", false},
		{"machineId: true", "machineId: 'true'", false},
		{"when: { setting: a, in: ['1'] }", "when: null", false},
		{"in: ['1']", "in: " + many, false},
		{"in: ['1']", "in: []", false},
		{"{ kind: steamcmd, appId: 5 }", "{ kind: steamcmd, appId: 5, env: { A: b } }", false},
		{"{ kind: steamcmd, appId: 5 }", "{ kind: steamcmd, appId: '5' }", false},
		{"LD_LIBRARY_PATH: ./lib", "LD_LIBRARY_PATH: '{{ .Setting \"a\" }}'", false},
		{"arg: -x", "arg: ''", false},
		{"- ./server", "- ''", true},
		{"name: Test", "name: ' '", false},
		{"name: Test", "name: { de: Test, en: ' ' }", false},
		{"name: Test", "name: ''", false},
		{"name: A,", "name: 1,", false},
		{"tmpfs: [/tmp]", "tmpfs: [/tmp, /tmp]", false},
		{"tmpfs: [/tmp]", "tmpfs: tmp", false},
		{"image: ipshosting/runtime-test:v1", "image: 7", false},
		{"apiVersion: 1", "apiVersion: '1'", false},
		{"protocol: UDP", "protocol: udp", false},
		{"type: NUMBER", "type: FLOAT", false},
		{"key: a,", "key: 1a,", false},
		{"shell: 'true'", "shell: ''", false},
		{"{ kind: steamcmd, appId: 5 }", "{ kind: download, url: 'https://x/y' }", false},
		{"{ kind: steamcmd, appId: 5 }", "{ kind: download, url: 'https://x/y', checksum: '' }", false},
		{"MODE:", "LD_PRELOAD: x, MODE:", true},
	}
	for _, c := range cases {
		doc := strings.Replace(mutationBase, c.old, c.new, 1)
		if c.old != "" && doc == mutationBase {
			t.Fatalf("mutation %q does not apply", c.old)
		}
		schemaErr := ValidateSchema([]byte(doc))
		_, parseErr := gamedef.Parse([]byte(doc))
		if (schemaErr == nil) != c.ok || (parseErr == nil) != c.ok {
			t.Errorf("%q -> %q: want ok=%v, schema: %v, parse: %v", c.old, c.new, c.ok, schemaErr, parseErr)
		}
	}
}
