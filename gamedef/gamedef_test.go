package gamedef

import (
	"bytes"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

const base = `
apiVersion: 1
short: testgame
name: Test
ports:
  - { identifier: game, protocol: UDP, default: 2456 }
settings:
  - { key: name, name: Name, type: STRING, default: My Server }
  - { key: password, name: Passwort, type: HIDDENSTRING }
  - { key: crossplay, name: Crossplay, type: BOOLEAN, default: '0' }
`

// withRun appends a run section with the test image and the given extra lines (indented
// by two spaces).
func withRun(doc string, run string) string {
	return doc + "run:\n  image: ipshosting/runtime-test:v1\n" + run
}

func mustParse(t *testing.T, doc string) *Definition {
	t.Helper()
	definition, err := Parse([]byte(doc))
	if err != nil {
		t.Fatalf("parse: %v\n%s", err, doc)
	}
	return definition
}

// expectError parses doc and checks that the error contains every fragment.
func expectError(t *testing.T, doc string, fragments ...string) {
	t.Helper()
	_, err := Parse([]byte(doc))
	if err == nil {
		t.Fatalf("expected an error containing %q\n%s", fragments, doc)
	}
	for _, fragment := range fragments {
		if !strings.Contains(err.Error(), fragment) {
			t.Fatalf("error does not contain %q:\n%v", fragment, err)
		}
	}
}

func render(t *testing.T, d *Definition, settings map[string]string) *StartPlan {
	t.Helper()
	values := DefaultValues(d)
	for key, value := range settings {
		values.Settings[key] = value
	}
	plan, err := Render(d, values)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	return plan
}

func renderInstall(t *testing.T, d *Definition, settings map[string]string) *InstallPlan {
	t.Helper()
	values := DefaultValues(d)
	for key, value := range settings {
		values.Settings[key] = value
	}
	plan, err := RenderInstall(d, values, InstallOptions{})
	if err != nil {
		t.Fatalf("render install: %v", err)
	}
	return plan
}

func ptr[T any](value T) *T { return &value }

// --- run, exec, shell -------------------------------------------------------------------

func TestPlainArgsKeepEmptyValues(t *testing.T) {
	d := mustParse(t, withRun(base, `  exec: [./server, -password, '{{ .Setting "password" }}', -savedir, data]
`))
	want := []string{"./server", "-password", "", "-savedir", "data"}
	if got := render(t, d, nil).Command; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestObjectArgsWithConditions(t *testing.T) {
	d := mustParse(t, withRun(base, `  exec:
    - ./server
    - arg: -crossplay
      when: { setting: crossplay, truthy: true }
    - arg: -password
      value: '{{ .Setting "password" }}'
    - arg: -name
      value: '{{ .Setting "name" }}'
`))
	if got, want := render(t, d, nil).Command, []string{"./server", "-name", "My Server"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("defaults: got %q, want %q", got, want)
	}
	got := render(t, d, map[string]string{"crossplay": "1", "password": "secret"}).Command
	want := []string{"./server", "-crossplay", "-password", "secret", "-name", "My Server"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("set: got %q, want %q", got, want)
	}
}

func TestShellStartAndEnv(t *testing.T) {
	d := mustParse(t, withRun(base, `  env:
    NAME: '{{ .Setting "name" }}'
  shell: exec ./server -name "$NAME"
`))
	plan := render(t, d, map[string]string{"name": `x"; rm -rf /`})
	if plan.Command[0] != "/bin/sh" || plan.Command[2] != `exec ./server -name "$NAME"` {
		t.Fatalf("unexpected command %q", plan.Command)
	}
	if plan.Env["NAME"] != `x"; rm -rf /` {
		t.Fatalf("setting must reach the shell via env only, got %q", plan.Env["NAME"])
	}
	if plan.Env["HOME"] != DefaultDataPath || plan.Env["IPS_PORT_GAME"] != "2456" {
		t.Fatalf("missing engine env: %v", plan.Env)
	}
}

func TestExecAndShellExclusive(t *testing.T) {
	expectError(t, withRun(base, "  exec: [./a]\n  shell: ./b\n"), "mutually exclusive")
}

func TestRenderedArgsAndSecretsStayVerbatim(t *testing.T) {
	d := mustParse(t, withRun(base, `  exec:
    - '{{ .Setting "password" }}'
    - arg: -name
      value: '{{ .Setting "name" }}'
`))
	got := render(t, d, map[string]string{"password": " secret ", "name": " "}).Command
	if want := []string{" secret ", "-name", " "}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestMachineIDAndTmpfs(t *testing.T) {
	d := mustParse(t, withRun(base, "  machineId: true\n  tmpfs: [/tmp, /var/run/wine]\n"))
	plan := render(t, d, nil)
	if !plan.MachineID || !reflect.DeepEqual(plan.Tmpfs, []string{"/tmp", "/var/run/wine"}) {
		t.Fatalf("unexpected plan: machineId=%v tmpfs=%v", plan.MachineID, plan.Tmpfs)
	}
	for tmpfs, fragment := range map[string]string{
		"[/]":                                  "must not be /",
		"[/ips]":                               "must not overlap /ips",
		"[/ips/cache]":                         "must not overlap /ips",
		"[/home/ips-hosting/cache]":            "data path",
		"[/home]":                              "data path",
		"[tmp]":                                "clean absolute path",
		"[/tmp/../etc]":                        "clean absolute path",
		"[/tmp, /tmp]":                         "duplicate",
		"[/dev/shm]":                           "/etc/machine-id",
		"[/etc]":                               "/etc/machine-id",
		"[/a, /b, /c, /d, /e, /f, /g, /h, /i]": "at most 8",
	} {
		expectError(t, withRun(base, "  tmpfs: "+tmpfs+"\n"), "/run/tmpfs", fragment)
	}
}

// --- strict parsing -----------------------------------------------------------------------

func TestStrictParseRejectsUnknownFields(t *testing.T) {
	cases := map[string]string{
		"top level":       withRun(base, "") + "futureField: true\n",
		"nested":          withRun(base, "  mounts: []\n"),
		"i18n object":     strings.Replace(withRun(base, ""), "name: Test", "name: { de: Test, fr: Essai }", 1),
		"arg object":      withRun(base, "  exec: [{ arg: -x, valeu: y }]\n"),
		"condition":       withRun(base, "  exec: [{ arg: -x, when: { setting: name, equalz: y } }]\n"),
		"removed field":   withRun(base, "") + "requires: { engine: 1 }\n",
		"http query url":  withRun(base, "") + "query: { protocol: http, port: game, path: /, url: 'http://x' }\n",
		"container stage": withRun(base, "") + "install: [{ kind: container, image: 'ipshosting/x:v1', args: [a] }]\n",
	}
	for name, doc := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse([]byte(doc)); err == nil {
				t.Fatalf("must be rejected:\n%s", doc)
			}
		})
	}
}

func TestStructuralChecksWithoutSchema(t *testing.T) {
	expectError(t, base, "/run", "image or imageVariants")
	expectError(t, withRun(strings.Replace(base, "short: testgame", "short: Test-Game", 1), ""), "/short")
	expectError(t, withRun(base, "  console: { via: telnet }\n"), "/run/console/via")
	expectError(t, withRun(base, "")+"install: [{ kind: steamcmd, appId: 1, url: 'https://x' }]\n", "another stage kind")
	expectError(t, withRun(base, "")+"hooks: { preStart: [{ ensureDir: { path: a }, symlink: { target: a, link: b } }] }\n", "exactly one operation")
}

// --- templates ------------------------------------------------------------------------------

func TestTemplateChecks(t *testing.T) {
	expectError(t, withRun(base, `  exec: ['{{ .Setting "nope" }}']`+"\n"), `undeclared setting "nope"`)
	expectError(t, withRun(base, `  exec: ['{{ (.Port "missing").Container }}']`+"\n"), `undeclared port "missing"`)
	expectError(t, withRun(base, `  exec: ['{{ call .Server }}']`+"\n"), "not allowed")
	for _, tmpl := range []string{`{{define "x"}}{{html "t"}}{{end}}{{template "x"}}`, `{{block "x" .}}{{html "t"}}{{end}}`} {
		expectError(t, withRun(base, "  exec: ['"+tmpl+"']\n"), "not allowed")
	}
	expectError(t, withRun(base, `  exec: ['{{ with .Port "game" }}{{ $.SettingValue "name" }}{{ end }}']`+"\n"), "not part of the template context")

	d := mustParse(t, withRun(base, `  exec: ['{{ with .Port "game" }}{{ .Container }}/{{ $.Setting "name" }}{{ end }}']`+"\n"))
	if got := render(t, d, nil).Command; !reflect.DeepEqual(got, []string{"2456/My Server"}) {
		t.Fatalf("got %q", got)
	}
}

func TestTemplatesOnlyInValueFields(t *testing.T) {
	expectError(t, withRun(base, "")+"config: [{ path: '{{ .Setting \"name\" }}.cfg', format: kv, values: { a: b } }]\n", "templates are not allowed")
	expectError(t, withRun(base, "  workingDir: '{{ .Setting \"name\" }}'\n"), "templates are not allowed")
	expectError(t, strings.Replace(withRun(base, ""), "ipshosting/runtime-test:v1", `'{{ .Setting "name" }}'`, 1), "/run/image")
}

// --- paths, config ------------------------------------------------------------------------

func TestPathEscape(t *testing.T) {
	expectError(t, withRun(base, "")+"config: [{ path: ../../etc/passwd, format: kv, values: { a: b } }]\n", "escapes the data directory")
	d := mustParse(t, withRun(base, "")+"hooks: { preStart: [{ ensureDir: { path: 'saves/{{ .Setting \"name\" }}' } }] }\n")
	values := DefaultValues(d)
	values.Settings["name"] = "../../.."
	if _, err := Render(d, values); err == nil || !strings.Contains(err.Error(), "escapes") {
		t.Fatalf("expected escape error, got %v", err)
	}
}

func TestConfigValuesRejectLineBreaks(t *testing.T) {
	d := mustParse(t, withRun(base, "")+"config: [{ path: server.properties, format: properties, values: { motd: '{{ .Setting \"name\" }}' } }]\n")
	values := DefaultValues(d)
	values.Settings["name"] = "A\nrcon.password=x"
	if _, err := Render(d, values); err == nil || !strings.Contains(err.Error(), "line breaks") {
		t.Fatalf("expected line break error, got %v", err)
	}
}

func TestConfigWhen(t *testing.T) {
	d := mustParse(t, withRun(base, "")+"config: [{ path: a.cfg, format: kv, values: { a: b }, when: { setting: crossplay, equals: '1' } }]\n")
	if len(render(t, d, nil).Config) != 0 || len(render(t, d, map[string]string{"crossplay": "1"}).Config) != 1 {
		t.Fatalf("config when not applied")
	}
}

func apply(t *testing.T, format string, values map[string]string, existing string) string {
	t.Helper()
	out, err := ApplyConfig(PlanConfigFile{Format: format, Values: values}, []byte(existing))
	if err != nil {
		t.Fatal(err)
	}
	// Every format must be idempotent.
	again, err := ApplyConfig(PlanConfigFile{Format: format, Values: values}, out)
	if err != nil || !bytes.Equal(again, out) {
		t.Fatalf("%s is not idempotent:\n%s\n---\n%s", format, out, again)
	}
	return string(out)
}

func TestApplyProperties(t *testing.T) {
	got := apply(t, FormatProperties, map[string]string{"motd": "new", "max-players": "10"}, "# comment\nmotd=old\nlevel-seed=42\nmotd=duplicate\n")
	if want := "# comment\nmotd=new\nlevel-seed=42\nmax-players=10\n"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestApplyKVDelimiter(t *testing.T) {
	out, _ := ApplyConfig(PlanConfigFile{Format: FormatKV, Delimiter: " ", Values: map[string]string{"hostname": "x"}}, []byte("hostname old\nsv_lan 0\n"))
	if string(out) != "hostname x\nsv_lan 0\n" {
		t.Fatalf("got %q", out)
	}
}

func TestApplyINI(t *testing.T) {
	got := apply(t, FormatINI, map[string]string{"/Script/Engine.GameSession.MaxPlayers": "16", "New.Section.Key": "v", "global": "2"},
		"global=1\n\n[/Script/Engine.GameSession]\nMaxPlayers=4\nOther=keep\nMaxPlayers=8\n\n[Other]\nx=1\n")
	if want := "global=2\n\n[/Script/Engine.GameSession]\nMaxPlayers=16\nOther=keep\n\n[Other]\nx=1\n[New.Section]\nKey=v\n"; got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestApplyJSONKeepsTypes(t *testing.T) {
	got := apply(t, FormatJSON, map[string]string{"name": "new", "port": "2", "version": "2", "nested.on": "true"}, `{"name":"old","port":1,"version":"1.0","nested":{"on":false}}`)
	if want := `{"name":"new","port":2,"version":"2","nested":{"on":true}}`; got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func TestApplyYAML(t *testing.T) {
	got := apply(t, FormatYAML, map[string]string{"server.name": "new", "server.port": "2", "extra.flag": "true"}, "# server config\nserver:\n  name: old # keep comment\n  port: 1\n")
	for _, want := range []string{"# server config", "name: new # keep comment", "port: 2", "extra:\n  flag: \"true\""} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in:\n%s", want, got)
		}
	}
	got = apply(t, FormatYAML, map[string]string{"port": "2", "version": "2", "server.name": "c"}, "port: 1\nversion: \"\"\nserver:\n  name: a\nport: 3\nserver:\n  name: b\n")
	if want := "port: 2\nversion: \"2\"\nserver:\n  name: c\n"; got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestApplyLines(t *testing.T) {
	existing := "# settings\nset maxplayers 4\nset maxplayers 8\nset motd old\nother line\n"
	got := apply(t, FormatLines, map[string]string{
		"set maxplayers ": "set maxplayers 16",
		"set motd":        "set motd Hello",
		"set pvp ":        "set pvp 1",
	}, existing)
	if want := "# settings\nset maxplayers 16\nset maxplayers 16\nset motd Hello\nother line\nset pvp 1\n"; got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
	// The longest matching prefix wins.
	got = apply(t, FormatLines, map[string]string{"a": "a=1", "ab": "ab=2"}, "ab=0\na=0\n")
	if got != "ab=2\na=1\n" {
		t.Fatalf("got %q", got)
	}
}

func TestLinesPrefixEnforced(t *testing.T) {
	// A rendered line must keep its prefix, otherwise it would be appended on every start.
	expectError(t, withRun(base, "")+"config: [{ path: a.cfg, format: lines, values: { 'name=': '{{ .Setting \"name\" }}' } }]\n", "must start with its prefix")
}

func TestApplyXML(t *testing.T) {
	existing := `<?xml version="1.0" encoding="utf-8"?>
<!-- server settings -->
<Config>
  <Server name="old">
    <Port>1</Port>
    <Empty/>
  </Server>
  <!-- keep me -->
</Config>
`
	got := apply(t, FormatXML, map[string]string{
		"Config.Server.Port":     "2",
		"Config.Server@name":     `A "quoted" & <name>`,
		"Config.Server.Motd":     "Hi & welcome",
		"Config.Network.Limit":   "10",
		"Config.Network@enabled": "true",
	}, existing)
	for _, want := range []string{
		`<?xml version="1.0" encoding="utf-8"?>`,
		"<!-- server settings -->",
		"<!-- keep me -->",
		`<Server name="A &quot;quoted&quot; &amp; &lt;name&gt;">`,
		"    <Port>2</Port>\n    <Empty/>\n    <Motd>Hi &amp; welcome</Motd>\n  </Server>",
		`<Network enabled="true"><Limit>10</Limit></Network>`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in:\n%s", want, got)
		}
	}

	// A missing file is created with the root element.
	if got := apply(t, FormatXML, map[string]string{"Root.A@x": "1", "Root.A.B": "v"}, ""); got != "<Root><A x=\"1\"><B>v</B></A></Root>\n" {
		t.Fatalf("got %q", got)
	}
	for values, fragment := range map[string]string{"Other.A": "root element", "Config.Server": "child elements"} {
		if _, err := ApplyConfig(PlanConfigFile{Format: FormatXML, Values: map[string]string{values: "x"}}, []byte(existing)); err == nil || !strings.Contains(err.Error(), fragment) {
			t.Fatalf("%s: expected %q, got %v", values, fragment, err)
		}
	}
	expectError(t, withRun(base, "")+"config: [{ path: a.xml, format: xml, values: { 'A..B': x } }]\n", "invalid element path")
}

func TestApplyXMLFixture(t *testing.T) {
	existing, err := os.ReadFile("testdata/serverconfig.xml")
	if err != nil {
		t.Fatal(err)
	}
	got := apply(t, FormatXML, map[string]string{"ServerSettings.property@value": "x"}, string(existing))
	if !strings.Contains(got, `<property name="ServerName" value="x" />`) || !strings.Contains(got, `<property name="ServerPort" value="26900"/>`) {
		t.Fatalf("unexpected result:\n%s", got)
	}
}

func TestApplyXMLPredicate(t *testing.T) {
	existing, err := os.ReadFile("testdata/serverconfig.xml")
	if err != nil {
		t.Fatal(err)
	}
	// apply also checks idempotence: the created element matches its predicate next time.
	got := apply(t, FormatXML, map[string]string{
		"ServerSettings.property[name=ServerPort]@value":       "27000",
		"ServerSettings.property[name=Server.Region@EU]@value": "eu",
		"ServerSettings.group[id=a].item[key=x]":               "text",
	}, string(existing))
	want := "<?xml version=\"1.0\"?>\n<ServerSettings>\n\t<!-- Server representation -->\n" +
		"\t<property name=\"ServerName\" value=\"My Game Host\" />\n" +
		"\t<property name=\"ServerPort\" value=\"27000\"/>\n" +
		"\t<group id=\"a\"><item key=\"x\">text</item></group>\n" +
		"\t<property name=\"Server.Region@EU\" value=\"eu\"/>\n</ServerSettings>\n"
	if got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}

	// A predicate never matches an element with a different attribute value (or without it).
	got = apply(t, FormatXML, map[string]string{"R.p[name=b]@v": "2"}, `<R><p name="a" v="1"/><p v="0"/></R>`)
	if want := `<R><p name="a" v="1"/><p v="0"/><p name="b" v="2"/></R>` + "\n"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	// The predicate also applies to the root element.
	if _, err := ApplyConfig(PlanConfigFile{Format: FormatXML, Values: map[string]string{"ServerSettings[v=2].x": "1"}}, existing); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("root predicate: got %v", err)
	}

	for _, key := range []string{"A.p[name=a]b]@v", "A.p[name=a", "A.p[=a]", "A.p[name]", "A.p[name=a][id=b]"} {
		expectError(t, withRun(base, "")+"config: [{ path: a.xml, format: xml, values: { '"+key+"': x } }]\n", "invalid element path")
	}
}

// --- conditions -----------------------------------------------------------------------------

func TestEvalCondition(t *testing.T) {
	settings := map[string]string{"a": "Hello", "b": "0", "c": "", "name": "My hello server", "list": "x"}
	cases := []struct {
		condition Condition
		want      bool
	}{
		{Condition{Setting: "a", Equals: ptr("Hello")}, true},
		{Condition{Setting: "a", Equals: ptr("hello")}, false},
		{Condition{Setting: "a", NotEquals: ptr("")}, true},
		{Condition{Setting: "c", NotEquals: ptr("")}, false},
		{Condition{Setting: "missing", Equals: ptr("")}, true},
		{Condition{Setting: "a", In: []string{"x", "Hello"}}, true},
		{Condition{Setting: "a", In: []string{"x"}}, false},
		{Condition{Setting: "a", Truthy: ptr(true)}, true},
		{Condition{Setting: "b", Truthy: ptr(true)}, false},
		{Condition{Setting: "b", Truthy: ptr(false)}, true},
		{Condition{Setting: "c", Truthy: ptr(false)}, true},
		{Condition{Setting: "a", MinLength: ptr(5)}, true},
		{Condition{Setting: "a", MinLength: ptr(6)}, false},
		{Condition{Setting: "a", MaxLength: ptr(5)}, true},
		{Condition{Setting: "a", MaxLength: ptr(4)}, false},
		{Condition{Setting: "a", Matches: ptr("^H.l+o$")}, true},
		{Condition{Setting: "a", Matches: ptr("^x")}, false},
		{Condition{Setting: "a", NotContains: "name"}, false}, // case-insensitive: "hello" is in the name
		{Condition{Setting: "list", NotContains: "name"}, true},
		{Condition{Setting: "c", NotContains: "name"}, true}, // empty never matches
		{Condition{All: []Condition{{Setting: "a", Truthy: ptr(true)}, {Setting: "b", Equals: ptr("0")}}}, true},
		{Condition{All: []Condition{{Setting: "a", Truthy: ptr(true)}, {Setting: "b", Truthy: ptr(true)}}}, false},
		{Condition{Any: []Condition{{Setting: "b", Truthy: ptr(true)}, {Setting: "a", Equals: ptr("Hello")}}}, true},
		{Condition{Any: []Condition{{Setting: "b", Truthy: ptr(true)}, {Setting: "c", Truthy: ptr(true)}}}, false},
		{Condition{Not: &Condition{Setting: "a", Truthy: ptr(true)}}, false},
	}
	for i, c := range cases {
		if got := EvalCondition(c.condition, settings); got != c.want {
			t.Errorf("case %d %+v: got %v, want %v", i, c.condition, got, c.want)
		}
	}
	for value, want := range map[string]bool{"": false, " 0 ": false, "FALSE": false, "false": false, "1": true, "no": true, "x": true} {
		if Truthy(value) != want {
			t.Errorf("Truthy(%q) != %v", value, want)
		}
	}
}

func TestConditionValidation(t *testing.T) {
	for when, fragment := range map[string]string{
		"{ setting: name, equals: a, notEquals: b }":                "exactly one operator",
		"{ setting: name }":                                         "exactly one operator",
		"{ equals: a }":                                             "needs a setting",
		"{ setting: nope, equals: a }":                              `undeclared setting "nope"`,
		"{ setting: name, notContains: nope }":                      `undeclared setting "nope"`,
		"{ setting: name, matches: '(' }":                           "invalid regular expression",
		"{ setting: name, in: [] }":                                 "must have 1 to 64 items",
		"{ setting: name, all: [{ setting: name, truthy: true }] }": "not allowed with all",
		"{ setting: name, not: { setting: name, truthy: true } }":   "not allowed with not",
	} {
		expectError(t, withRun(base, "  exec: [{ arg: -x, when: "+when+" }]\n"), "/run/exec/0/when", fragment)
	}
	expectError(t, withRun(base, "  exec: [{ arg: -x, when: { all: [] } }]\n"), "/run/exec/0/when/all", "must have 1 to 16 items")
	deep := strings.Repeat("{ not: ", 9) + "{ setting: name, truthy: true }" + strings.Repeat(" }", 9)
	expectError(t, withRun(base, "  exec: [{ arg: -x, when: "+deep+" }]\n"), "nested deeper")
}

func TestConditionsEverywhere(t *testing.T) {
	d := mustParse(t, withRun(base, "")+`
install:
  - kind: download
    url: https://example.com/a.zip
    checksum: sha256:0000000000000000000000000000000000000000000000000000000000000000
    when: { setting: crossplay, truthy: true }
hooks:
  preStart:
    - ensureDir: { path: mods }
      when: { any: [{ setting: crossplay, truthy: true }, { setting: name, equals: modded }] }
preconditions:
  - check: { setting: password, minLength: 5 }
    message: zu kurz
  - check: { setting: password, notContains: name }
    message: im Namen
    severity: warning
`)
	d.Settings[0].VisibleWhen = &Condition{Setting: "crossplay", Truthy: ptr(true)}
	if n := len(renderInstall(t, d, nil).Stages); n != 0 {
		t.Fatalf("stage when ignored: %d stages", n)
	}
	if n := len(renderInstall(t, d, map[string]string{"crossplay": "1"}).Stages); n != 1 {
		t.Fatalf("stage when ignored: %d stages", n)
	}
	if len(render(t, d, map[string]string{"name": "modded"}).PreStart) != 1 || len(render(t, d, nil).PreStart) != 0 {
		t.Fatalf("hook when ignored")
	}
	plan := render(t, d, map[string]string{"password": "Serv", "name": "My Server"})
	if len(plan.Blocking()) != 1 || plan.Blocking()[0].Message.De != "zu kurz" || plan.Preconditions[1].Passed {
		t.Fatalf("unexpected preconditions: %+v", plan.Preconditions)
	}
	expectError(t, withRun(base, "")+"preconditions: [{ check: '{{ true }}', message: x }]\n", "must be an object")
}

// --- settings ------------------------------------------------------------------------------

func TestValidateValue(t *testing.T) {
	number := Setting{Type: SettingNumber, Validation: &Validation{Min: ptr(1.0), Max: ptr(64.0), Integer: true}}
	text := Setting{Type: SettingString, Validation: &Validation{Required: true, MinLength: ptr(3), MaxLength: ptr(5), Pattern: "[a-zä]+"}}
	enum := Setting{Type: SettingStringEnum, Values: []SettingValue{{Value: "a"}, {Value: "b"}}}
	cases := []struct {
		setting Setting
		value   string
		ok      bool
	}{
		{number, "", true}, // not required
		{number, "1", true},
		{number, "64", true},
		{number, "0", false},
		{number, "65", false},
		{number, "1.5", false},
		{number, "abc", false},
		{text, "", false},
		{text, "äbc", true},
		{text, "ab", false},
		{text, "abcdef", false},
		{text, "abC", false}, // pattern must match the whole value
		{enum, "a", true},
		{enum, "c", false},
		{Setting{Type: SettingStringEnum, ValuesSource: "versions"}, "any", true},
		{Setting{Type: SettingBoolean}, "1", true},
		{Setting{Type: SettingBoolean}, "true", false},
		{Setting{Type: SettingStringList}, `["a"]`, true},
		{Setting{Type: SettingStringList}, `a`, false},
		{Setting{Type: SettingNumberEnum, Values: []SettingValue{{Value: "1"}}}, "1", true},
	}
	for i, c := range cases {
		if err := ValidateValue(c.setting, c.value); (err == nil) != c.ok {
			t.Errorf("case %d (%q): got %v, want ok=%v", i, c.value, err, c.ok)
		}
	}
}

func TestValidationRulesFitType(t *testing.T) {
	expectError(t, withRun(base+"  - { key: x, name: X, type: STRING, validation: { min: 1 } }\n", ""), "NUMBER and NUMBERENUM")
	expectError(t, withRun(base+"  - { key: x, name: X, type: NUMBER, validation: { maxLength: 1 } }\n", ""), "string settings only")
	expectError(t, withRun(base+"  - { key: x, name: X, type: NUMBER, default: '0', validation: { min: 1 } }\n", ""), "/settings/3/default", "at least 1")
	expectError(t, withRun(base+"  - { key: x, name: X, type: STRING, validation: { pattern: '(' } }\n", ""), "invalid regular expression")
	expectError(t, withRun(base+"  - { key: x, name: X, type: STRING, validation: 'string|max:5' }\n", ""), "must be an object")
}

func TestGenerate(t *testing.T) {
	d := mustParse(t, withRun(base+"  - { key: token, name: Token, type: HIDDENSTRING, generate: { length: 32 } }\n", ""))
	if g := d.Setting("token").Generate; g.Charset != CharsetAlnum || g.Length != 32 {
		t.Fatalf("unexpected generate %+v", g)
	}
	mustParse(t, withRun(base+"  - { key: token, name: Token, type: STRING, default: '', generate: { length: 8, charset: hex } }\n", ""))
	expectError(t, withRun(base+"  - { key: token, name: Token, type: HIDDENSTRING, default: x, generate: { length: 8 } }\n", ""), "must not have a default")
	expectError(t, withRun(base+"  - { key: token, name: Token, type: NUMBER, generate: { length: 8 } }\n", ""), "only STRING and HIDDENSTRING")
	expectError(t, withRun(base+"  - { key: token, name: Token, type: STRING, generate: { length: 8, charset: base64 } }\n", ""), "/generate/charset")
	expectError(t, withRun(base+"  - { key: token, name: Token, type: STRING, validation: { maxLength: 4 }, generate: { length: 8 } }\n", ""), "minLength/maxLength")
}

func TestCategories(t *testing.T) {
	d := mustParse(t, withRun(base+"  - { key: x, name: X, type: STRING, category: steamBeta }\n", ""))
	if d.Settings[0].Category != CategoryGeneral || d.Setting("x").Category != "steamBeta" {
		t.Fatalf("unexpected categories")
	}
	expectError(t, withRun(base+"  - { key: x, name: X, type: STRING, category: Allgemein }\n", ""), `unknown category "Allgemein"`)
	expectError(t, withRun(base+"  - { key: x, name: X, type: STRING, sortOrder: 1 }\n", ""), "invalid definition")
}

func TestSettingKeysCaseInsensitiveUnique(t *testing.T) {
	expectError(t, withRun(base+"  - { key: Name, name: Name2, type: STRING }\n", ""), "case-insensitively")
}

func TestI18nShorthand(t *testing.T) {
	d := mustParse(t, strings.Replace(withRun(base, ""), "name: Test", "name: { de: Test, en: Test EN }", 1))
	if d.Name.Text("en") != "Test EN" || d.Name.Text("fr") != "Test" || d.Settings[0].Name.De != "Name" {
		t.Fatalf("unexpected i18n %+v", d.Name)
	}
}

// --- images -------------------------------------------------------------------------------

const digest = "@sha256:c851b055fa784608244b9e494ada4af421f78001e42e71bf5aa73ded2b321ad1"

func TestImageAllowlist(t *testing.T) {
	cases := map[string]bool{
		"ipshosting/runtime-steam:v1":              true,
		"ghcr.io/ips-hosting/mirror/games:valheim": true,
		"ipshosting/runtime-steam":                 false, // own images need a tag
		"ghcr.io/parkervcp/games:valheim" + digest: true,
		"ghcr.io/parkervcp/games:valheim":          false, // external images need a digest
		"evil.example.com/miner:latest":            false,
		"ubuntu:24.04" + digest:                    true,
	}
	for image, ok := range cases {
		if err := CheckImage(image); (err == nil) != ok {
			t.Errorf("%s: got %v, want ok=%v", image, err, ok)
		}
	}
}

func TestImageVariants(t *testing.T) {
	enum := base + "  - { key: java, name: Java, type: STRINGENUM, default: '21', values: [{ value: '17' }, { value: '21' }] }\n"
	variants := "run:\n  imageVariants:\n    setting: java\n    images:\n      '17': ipshosting/java:17\n      '21': ghcr.io/parkervcp/yolks:java_21" + digest + "\n"
	d := mustParse(t, enum+variants)
	if got := render(t, d, nil).Image; got != "ghcr.io/parkervcp/yolks:java_21"+digest {
		t.Fatalf("default variant: got %s", got)
	}
	if got := render(t, d, map[string]string{"java": "17"}).Image; got != "ipshosting/java:17" {
		t.Fatalf("variant 17: got %s", got)
	}
	if got := d.Images(); len(got) != 2 {
		t.Fatalf("Images: %v", got)
	}
	expectError(t, enum+strings.Replace(variants, "'17'", "'11'", 1), "must equal the values")
	expectError(t, enum+strings.Replace(variants, "ipshosting/java:17", "ghcr.io/parkervcp/yolks:java_17", 1), "pinned by digest")
	expectError(t, enum+strings.Replace(variants, "setting: java", "setting: name", 1), "must be of type STRINGENUM")
	expectError(t, enum+strings.Replace(variants, "run:\n", "run:\n  image: ipshosting/x:v1\n", 1), "mutually exclusive")
	expectError(t, base+"  - { key: java, name: Java, type: STRINGENUM, valuesSource: javaVersions }\n"+variants, "no valuesSource")
}

// --- install ------------------------------------------------------------------------------

const betaSettings = "  - { key: betaBranch, name: Beta, type: STRING, default: public, category: steamBeta }\n" +
	"  - { key: betaPassword, name: Beta-Passwort, type: HIDDENSTRING, category: steamBeta }\n"

func TestSteamcmdRunscript(t *testing.T) {
	d := mustParse(t, withRun(base+"  - { key: branch, name: Branch, type: STRING, default: public }\n", "")+`
install:
  - kind: steamcmd
    appId: 896660
    login: host
    branch: '{{ .Setting "branch" }}'
`)
	values := DefaultValues(d)
	values.Settings["branch"] = " beta "
	plan, err := RenderInstall(d, values, InstallOptions{Validate: true})
	if err != nil {
		t.Fatal(err)
	}
	stage := plan.Stages[0]
	if strings.Join(stage.Command, " ") != "steamcmd +runscript "+SteamcmdRunscriptPath {
		t.Fatalf("credentials must not be in argv: %q", stage.Command)
	}
	if stage.Env["HOME"] != SteamHomePath {
		t.Fatalf("steamcmd HOME must be private, got %q", stage.Env["HOME"])
	}
	if _, err := stage.Steamcmd.Runscript("", ""); err == nil {
		t.Fatalf("host login without credentials must fail")
	}
	script, err := stage.Steamcmd.Runscript("user", "p a s s")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`force_install_dir "/home/ips-hosting"`, `login "user" "p a s s"`, `app_update 896660 -beta "beta" validate`, "quit"} {
		if !strings.Contains(script, want) {
			t.Fatalf("missing %q in runscript:\n%s", want, script)
		}
	}
	if _, err := stage.Steamcmd.Runscript("user", `pa"ss`); err == nil {
		t.Fatalf("quotes in credentials must be rejected")
	}
}

func TestSteamcmdBeta(t *testing.T) {
	d := mustParse(t, withRun(base+betaSettings, "")+`
install:
  - kind: steamcmd
    appId: 1
    beta: { branchSetting: betaBranch, passwordSetting: betaPassword }
`)
	for _, c := range []struct{ branch, password, wantBranch, wantPassword string }{
		{"public", "secret", "", ""},
		{"", "secret", "", ""},
		{"experimental", "", "experimental", ""},
		{"experimental", " secret ", "experimental", " secret "},
	} {
		stage := renderInstall(t, d, map[string]string{"betaBranch": c.branch, "betaPassword": c.password}).Stages[0].Steamcmd
		if stage.Branch != c.wantBranch || stage.BranchPassword != c.wantPassword {
			t.Errorf("%+v: got branch %q password %q", c, stage.Branch, stage.BranchPassword)
		}
	}
	expectError(t, withRun(base+betaSettings, "")+"install: [{ kind: steamcmd, appId: 1, branch: x, beta: { branchSetting: betaBranch } }]\n", "mutually exclusive")
	expectError(t, withRun(base+betaSettings, "")+"install: [{ kind: steamcmd, appId: 1, beta: { branchSetting: betaBranch, passwordSetting: name } }]\n", "HIDDENSTRING")
	expectError(t, withRun(base+betaSettings, "")+"install: [{ kind: steamcmd, appId: 1, beta: { branchSetting: crossplay } }]\n", "STRING or STRINGENUM")
}

func TestSteamSDKHooks(t *testing.T) {
	d := mustParse(t, withRun(base, "")+`
install:
  - kind: steamcmd
    appId: 1
    dir: server
    steamSdk: true
  - kind: steamcmd
    appId: 2
    steamSdk: true
    when: { setting: crossplay, truthy: true }
hooks:
  postInstall:
    - ensureDir: { path: saves }
`)
	want := []PlanHookOp{
		{EnsureDir: &EnsureDir{Path: ".steam/sdk64"}},
		{Symlink: &Symlink{Target: "server/linux64/steamclient.so", Link: ".steam/sdk64/steamclient.so"}},
		{EnsureDir: &EnsureDir{Path: ".steam/sdk32"}},
		{Symlink: &Symlink{Target: "server/steamclient.so", Link: ".steam/sdk32/steamclient.so"}},
		{EnsureDir: &EnsureDir{Path: "saves"}},
	}
	if got := renderInstall(t, d, nil).PostInstall; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v", got)
	}
	if got := renderInstall(t, d, map[string]string{"crossplay": "1"}).PostInstall; len(got) != 9 || got[5].Symlink.Target != "linux64/steamclient.so" {
		t.Fatalf("second stage: got %+v", got)
	}
}

func TestLoginHostExcludesScripts(t *testing.T) {
	script := "  - { kind: script, shell: 'echo hi' }\n"
	expectError(t, withRun(base, "")+"install:\n  - { kind: steamcmd, appId: 1, login: host }\n"+script, "login: host")
	expectError(t, withRun(base+"  - { key: mods, name: Mods, type: STRINGLIST }\n", "")+"workshop: { settings: [mods], login: host }\ninstall:\n"+script, "login: host")
	mustParse(t, withRun(base, "")+"install:\n  - { kind: steamcmd, appId: 1 }\n"+script)
}

func TestDownloadStage(t *testing.T) {
	stage := "install: [{ kind: download, url: %s, checksum: sha256:0000000000000000000000000000000000000000000000000000000000000000 }]\n"
	mustParse(t, withRun(base, "")+strings.Replace(stage, "%s", "https://example.com/a.zip", 1))
	expectError(t, withRun(base, "")+strings.Replace(stage, "%s", "http://example.com/a.zip", 1), "https URL")
	expectError(t, withRun(base, "")+strings.Replace(stage, "%s", `'https://example.com/{{ .Setting "name" }}.zip'`, 1), "templates are not allowed")
	expectError(t, withRun(base, "")+"install: [{ kind: download, url: 'https://example.com/a.zip' }]\n", "/install/0/checksum")
}

// --- query, flags ---------------------------------------------------------------------------

func TestHTTPQuery(t *testing.T) {
	d := mustParse(t, withRun(base, "")+"query: { protocol: http, port: game, path: /status, scheme: https, insecure: true, paths: { players: players.online, map: world } }\n")
	q := render(t, d, nil).Query
	if q.Port == nil || q.Port.Exposed != 2456 || q.Path != "/status" || q.Scheme != "https" || !q.Insecure || q.Paths.Players != "players.online" {
		t.Fatalf("unexpected query %+v", q)
	}
	if q := render(t, mustParse(t, withRun(base, "")+"query: { protocol: http, port: game, path: / }\n"), nil).Query; q.Scheme != "http" {
		t.Fatalf("default scheme: %+v", q)
	}
	expectError(t, withRun(base, "")+"query: { protocol: http, port: game, path: /, insecure: true }\n", "scheme https only")
	expectError(t, withRun(base, "")+"query: { protocol: http, port: game, path: status }\n", "/query/path")
	expectError(t, withRun(base, "")+"query: { protocol: http, path: / }\n", "/query/port")
	expectError(t, withRun(base, "")+"query: { protocol: a2s, port: game, path: / }\n", "not allowed for protocol a2s")
}

func TestEffectiveFlags(t *testing.T) {
	cases := []struct {
		extra string
		want  PanelFlags
	}{
		{"", PanelFlags{ConsoleCommands: true}},
		{"steamAppId: 1\nquery: { protocol: a2s, port: game }\n", PanelFlags{ShowMap: true, ShowPlayerCount: true, SteamConnect: true, ConsoleCommands: true}},
		{"query: { protocol: a2s, port: game }\nflags: { showMap: false }\n", PanelFlags{ShowPlayerCount: true, ConsoleCommands: true}},
		{"query: { protocol: minecraft, port: game }\n", PanelFlags{ShowPlayerCount: true, ConsoleCommands: true}},
		{"query: { protocol: http, port: game, path: /, paths: { map: m } }\n", PanelFlags{ShowMap: true, ShowPlayerCount: true, ConsoleCommands: true}},
		{"query: { protocol: rcon, port: game, passwordSetting: password, command: status, pattern: 'map (?P<map>\\S+)' }\n", PanelFlags{ShowMap: true, ShowPlayerCount: true, ConsoleCommands: true}},
		{"players: { joinPattern: joined }\n", PanelFlags{ShowPlayerCount: true, ConsoleCommands: true}},
		{"install: [{ kind: steamcmd, appId: 1 }]\nflags: { consoleCommands: false }\n", PanelFlags{Installable: true}},
	}
	for i, c := range cases {
		if got := EffectiveFlags(mustParse(t, withRun(base, "")+c.extra)); got != c.want {
			t.Errorf("case %d: got %+v, want %+v", i, got, c.want)
		}
	}
	expectError(t, withRun(base, "  console: { via: none }\n")+"flags: { consoleCommands: true }\n", "console channel")
	expectError(t, withRun(base, "")+"flags: { installable: true }\n", "invalid definition")
}

// --- engine features ------------------------------------------------------------------------

func TestMinEngine(t *testing.T) {
	d := mustParse(t, withRun(base, "  tmpfs: [/tmp]\n")+"install: [{ kind: steamcmd, appId: 1, steamSdk: true }]\n")
	if got := MinEngine(d); got != EngineBaseline {
		t.Fatalf("got %d", got)
	}
	saved := EngineFeatures
	t.Cleanup(func() { EngineFeatures = saved })
	EngineFeatures = append(slicesClone(saved), EngineFeature{Path: "/install/*/steamSdk", Engine: 2}, EngineFeature{Path: "/run/env/~1x", Engine: 3})
	if got := MinEngine(d); got != 2 {
		t.Fatalf("feature table not applied: got %d", got)
	}
	// Definitions not produced by Parse conservatively need this engine.
	d.raw = nil
	if got := MinEngine(d); got != EngineVersion {
		t.Fatalf("without source document: got %d", got)
	}
	if !matchPointer("/run/env/~1x", []string{"run", "env", "/x"}) || matchPointer("/install/*", []string{"install"}) {
		t.Fatalf("pointer matching")
	}
	for _, feature := range saved {
		if feature.Engine > EngineVersion || feature.Engine < EngineBaseline {
			t.Errorf("feature %s needs engine %d, this engine is %d", feature.Path, feature.Engine, EngineVersion)
		}
	}
}

func slicesClone[T any](s []T) []T { return append([]T(nil), s...) }

// TestGeneratedFiles fails when the generated JSON files are stale (run go generate).
func TestGeneratedFiles(t *testing.T) {
	for file, want := range map[string][]byte{
		"schema/engine-features.v1.json": EngineFeaturesJSON(),
		"schema/categories.v1.json":      CategoriesJSON(),
	} {
		got, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("%s is stale: run go generate", file)
		}
	}
}

// --- misc -----------------------------------------------------------------------------------

func TestProtectedFiles(t *testing.T) {
	mustParse(t, withRun(base, "")+"protectedFiles: [{ path: /server.bin, readOnly: true }, { path: 'secrets/**', hidden: true }]\n")
	expectError(t, withRun(base, "")+"protectedFiles: [{ path: /server.bin }]\n", "readOnly or hidden")
	expectError(t, withRun(base, "")+"protectedFiles: [{ path: ../x, readOnly: true }]\n", "inside the data directory")
	expectError(t, withRun(base, "")+"filePermissions: [{ match: /x, allowWrite: false }]\n", "invalid definition")
}

func TestSources(t *testing.T) {
	mustParse(t, withRun(base, "")+"sources: [{ kind: manual }, { kind: egg, ref: a@b:c, license: MIT, converter: egg2gamedef@0.1.0, importedSha256: "+strings.Repeat("a", 64)+" }]\n")
	expectError(t, withRun(base, "")+"sources: [{ kind: egg }]\n", "need ref and license")
	expectError(t, withRun(base, "")+"sources: [{ kind: manual, converter: x }]\n", "name@version")
}

func TestPortEnvNameCollision(t *testing.T) {
	doc := strings.Replace(base, "ports:\n", "ports:\n  - { identifier: game_exposed, protocol: UDP, default: 2457 }\n", 1)
	expectError(t, withRun(doc, ""), "IPS_PORT_GAME_EXPOSED")
}

// --- hardening ----------------------------------------------------------------------------

func TestRenderRejectsUndeclaredAndInvalidSettings(t *testing.T) {
	d := mustParse(t, withRun(base, "  exec: [./server]\n"))
	for name, settings := range map[string]map[string]string{
		"is not declared":    {"unknown": "x"},
		"must be '0' or '1'": {"crossplay": "yes"},
		"line breaks":        {"name": "a\nb"},
	} {
		values := DefaultValues(d)
		for key, value := range settings {
			values.Settings[key] = value
		}
		if _, err := Render(d, values); err == nil || !strings.Contains(err.Error(), name) {
			t.Errorf("render: want %q, got %v", name, err)
		}
		if _, err := RenderInstall(d, values, InstallOptions{}); err == nil || !strings.Contains(err.Error(), name) {
			t.Errorf("render install: want %q, got %v", name, err)
		}
	}
}

func TestValueSyntax(t *testing.T) {
	number := Setting{Type: SettingNumber}
	integer := Setting{Type: SettingNumber, Validation: &Validation{Integer: true}}
	for value, ok := range map[string]bool{"1": true, "-0.5": true, "1e5": true, "1E-3": true, "1.": false, ".5": false,
		"01": false, "+1": false, "0x10": false, "Inf": false, "NaN": false, " 1": false} {
		if err := ValidateValue(number, value); (err == nil) != ok {
			t.Errorf("NUMBER %q: %v", value, err)
		}
	}
	for value, ok := range map[string]bool{"-12": true, "1.0": false, "1e5": false} {
		if err := ValidateValue(integer, value); (err == nil) != ok {
			t.Errorf("integer %q: %v", value, err)
		}
	}
	for _, value := range []string{"a\nb", "a\rb", "a\x00b", "\xff"} {
		if ValidateValue(Setting{Type: SettingString}, value) == nil {
			t.Errorf("STRING %q accepted", value)
		}
	}
	if err := ValidateValue(Setting{Type: SettingMultilineString}, "a\r\nb"); err != nil {
		t.Errorf("MULTILINESTRING: %v", err)
	}
	if ValidateValue(Setting{Type: SettingMultilineString}, "a\x00") == nil {
		t.Errorf("MULTILINESTRING with NUL accepted")
	}
}

func TestMultilineStringNotInConfigValues(t *testing.T) {
	doc := base + "  - { key: motd, name: MOTD, type: MULTILINESTRING }\n"
	expectError(t, withRun(doc, "")+"config: [{ path: a.cfg, format: kv, values: { motd: '{{ .Setting \"motd\" }}' } }]\n", "MULTILINESTRING")
	mustParse(t, withRun(doc, "")+"config: [{ path: a.txt, format: template, template: '{{ .Setting \"motd\" }}' }]\n")
}

func TestRangeOnlyOverSettingList(t *testing.T) {
	doc := base + "  - { key: list, name: List, type: STRINGLIST }\n"
	mustParse(t, withRun(doc, "  exec: [./server]\n")+"config: [{ path: a.txt, format: template, template: '{{ range .SettingList \"list\" }}{{ . }}{{ end }}' }]\n")
	expectError(t, withRun(doc, "  exec: [./server]\n")+"config: [{ path: a.txt, format: template, template: '{{ range $i := 9 }}x{{ end }}' }]\n", "range is only allowed")
}

func TestRenderLimits(t *testing.T) {
	doc := base + "  - { key: list, name: List, type: STRINGLIST }\n"
	d := mustParse(t, withRun(doc, "  exec: [./server]\n")+"config: [{ path: a.txt, format: template, template: '{{ range .SettingList \"list\" }}{{ . }}{{ end }}' }]\n")
	values := DefaultValues(d)
	values.Settings["list"] = `["` + strings.Repeat("x", MaxRenderedField+1) + `"]`
	if _, err := Render(d, values); err == nil {
		t.Fatalf("oversized field accepted")
	}
}

func TestInstallEnvRules(t *testing.T) {
	expectError(t, withRun(base, "")+"install: [{ kind: steamcmd, appId: 1, env: { A: b } }]\n", "env")
	expectError(t, withRun(base, "  env: { LD_PRELOAD: '{{ .Setting \"name\" }}' }\n"), "loader variable")
	expectError(t, withRun(base, "")+"install: [{ kind: script, shell: 'true', env: { DYLD_LIBRARY_PATH: '{{ .Setting \"name\" }}' } }]\n", "loader variable")
	mustParse(t, withRun(base, "  env: { LD_LIBRARY_PATH: ./lib }\n  exec: [./server]\n"))
}

func TestSteamcmdBranchSyntax(t *testing.T) {
	d := mustParse(t, withRun(base+betaSettings, "")+"install: [{ kind: steamcmd, appId: 1, beta: { branchSetting: betaBranch } }]\n")
	for branch, ok := range map[string]bool{"experimental": true, "v1.2_rc-3": true, "a b": false, "x\"y": false, "+login": false, strings.Repeat("a", 65): false} {
		values := DefaultValues(d)
		values.Settings["betaBranch"] = branch
		if _, err := RenderInstall(d, values, InstallOptions{}); (err == nil) != ok {
			t.Errorf("branch %q: %v", branch, err)
		}
	}
}

func TestDownloadChecksumAlwaysSerialized(t *testing.T) {
	data, err := json.Marshal(DownloadStage{URL: "https://x"})
	if err != nil || !strings.Contains(string(data), `"checksum":""`) {
		t.Fatalf("got %s, %v", data, err)
	}
}

func TestWriteFileIfMissing(t *testing.T) {
	d := mustParse(t, withRun(base, "  exec: [./server]\n")+"hooks:\n  preStart:\n    - writeFile: { path: a.txt, content: x, ifMissing: true }\n")
	plan := render(t, d, nil)
	if len(plan.PreStart) != 1 || plan.PreStart[0].WriteFile == nil || !plan.PreStart[0].WriteFile.IfMissing {
		t.Fatalf("ifMissing not in plan: %+v", plan.PreStart)
	}
	if MinEngine(d) < 1 {
		t.Fatalf("ifMissing must count as an engine feature")
	}
}

func TestLinesLongerManagedPrefix(t *testing.T) {
	d := mustParse(t, withRun(base, "  exec: [./server]\n")+"config: [{ path: a.cfg, format: lines, values: { 'set a': 'set a{{ .Setting \"name\" }}', 'set ab': 'set ab 1' } }]\n")
	values := DefaultValues(d)
	values.Settings["name"] = "b"
	if _, err := Render(d, values); err == nil || !strings.Contains(err.Error(), "prefix") {
		t.Fatalf("want longer prefix error, got %v", err)
	}
}

func TestApplyXMLTextRules(t *testing.T) {
	if _, err := ApplyConfig(PlanConfigFile{Format: FormatXML, Values: map[string]string{"A": "x\x01"}}, []byte("<A/>")); err == nil {
		t.Errorf("control character accepted")
	}
	if _, err := ApplyConfig(PlanConfigFile{Format: FormatXML, Values: map[string]string{"A": "\xff"}}, []byte("<A/>")); err == nil {
		t.Errorf("invalid UTF-8 accepted")
	}
	got := apply(t, FormatXML, map[string]string{"A.B@v": "2"}, "<A>\r\n  <B v=\"1\" />\r\n  <C/>\r\n</A>\r\n")
	if want := "<A>\r\n  <B v=\"2\" />\r\n  <C/>\r\n</A>\r\n"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestApplyJSONStripsDuplicates(t *testing.T) {
	got := apply(t, FormatJSON, map[string]string{"a": "2", "o.b": "3"}, `{"a":1,"o":{"b":1,"b":2},"a":5,"c":1}`)
	if want := `{"a":2,"o":{"b":3},"c":1}`; got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func TestConfigTypeHints(t *testing.T) {
	types := map[string]string{"s": TypeString, "n": TypeNumber, "b": TypeBool}
	out, err := ApplyConfig(PlanConfigFile{Format: FormatJSON, Values: map[string]string{"s": "1", "n": "2", "b": "true", "x": "3"}, Types: types}, []byte(`{"s":0,"x":0}`))
	if err != nil || string(out) != `{"s":"1","x":3,"b":true,"n":2}` {
		t.Fatalf("json: %s, %v", out, err)
	}
	out, err = ApplyConfig(PlanConfigFile{Format: FormatYAML, Values: map[string]string{"s": "yes", "n": "2", "b": "true"}, Types: types}, []byte("s: 0\n"))
	if err != nil || string(out) != "s: \"yes\"\nb: true\nn: 2\n" {
		t.Fatalf("yaml: %q, %v", out, err)
	}

	doc := base + "  - { key: count, name: Count, type: NUMBER, default: '4' }\n"
	d := mustParse(t, withRun(doc, "  exec: [./server]\n")+`config:
  - path: a.json
    format: json
    values: { name: '{{ .Setting "name" }}', count: '{{ .Setting "count" }}', cross: '{{ .SettingBool "crossplay" }}', port: '{{ (.Port "game").Container }}', lit: 'false', mixed: 'x{{ .Setting "count" }}' }
`)
	want := map[string]string{"name": TypeString, "count": TypeNumber, "cross": TypeBool, "port": TypeNumber, "lit": TypeBool}
	if got := render(t, d, nil).Config[0].Types; !reflect.DeepEqual(got, want) {
		t.Fatalf("types %v, want %v", got, want)
	}
}

func TestApplyYAMLRefusesAnchors(t *testing.T) {
	for _, existing := range []string{"a: &x 1\nb: *x\n", "base: &b { k: 1 }\nother:\n  <<: *b\n"} {
		if _, err := ApplyConfig(PlanConfigFile{Format: FormatYAML, Values: map[string]string{"a": "2", "other.k": "2"}}, []byte(existing)); err == nil {
			t.Errorf("anchored document accepted:\n%s", existing)
		}
	}
}

func TestEscapedDotsInKeys(t *testing.T) {
	if got := apply(t, FormatJSON, map[string]string{`a.b\.c`: "1"}, `{"a":{"b.c":0}}`); got != `{"a":{"b.c":1}}` {
		t.Errorf("json: %s", got)
	}
	if got := apply(t, FormatYAML, map[string]string{`a.b\.c`: "1"}, "a:\n  b.c: 0\n"); got != "a:\n  b.c: 1\n" {
		t.Errorf("yaml: %q", got)
	}
}

func TestApplyINISectionHeaderComment(t *testing.T) {
	got := apply(t, FormatINI, map[string]string{"S.k": "2"}, "[S] ; main\nk=1\n")
	if got != "[S] ; main\nk=2\n" {
		t.Fatalf("got %q", got)
	}
}

// java.util.Properties reads backslashes as escapes and skips leading whitespace: values are
// escaped, so the game reads exactly the rendered value.
func TestPropertiesValuesAreEscaped(t *testing.T) {
	d := mustParse(t, withRun(base, "")+"config: [{ path: server.properties, format: properties, values: { motd: '{{ .Setting \"name\" }}' } }]\n")
	values := DefaultValues(d)
	values.Settings["name"] = `Welcome\`
	if _, err := Render(d, values); err != nil {
		t.Fatalf("a trailing backslash is escaped and must be accepted: %v", err)
	}
	got := apply(t, FormatProperties, map[string]string{"motd": `C:\Games\ ok\`, "pad": "  x"}, "motd=old\nnext=1\n")
	if want := "motd=C:\\\\Games\\\\ ok\\\\\nnext=1\npad=\\  x\n"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestXMLKeepsBOMBeforeDeclaration(t *testing.T) {
	got := apply(t, FormatXML, map[string]string{"Root.Name": "x"}, "\xEF\xBB\xBF\n<?xml version=\"1.0\"?>\n<Root><Name>old</Name></Root>\n")
	if want := "\xEF\xBB\xBF<?xml version=\"1.0\"?>\n<Root><Name>x</Name></Root>\n"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestReservedTmpfsTargetsIsACopy(t *testing.T) {
	targets := ReservedTmpfsTargets()
	targets[0] = "/changed"
	if ReservedTmpfsTargets()[0] == "/changed" {
		t.Fatal("ReservedTmpfsTargets must return a copy")
	}
}

func TestXMLDropsWhitespaceBeforeDeclaration(t *testing.T) {
	got := apply(t, FormatXML, map[string]string{"Root.Name": "x"}, "\n<?xml version=\"1.0\"?>\n<Root>\n  <Name>old</Name>\n</Root>")
	if want := "<?xml version=\"1.0\"?>\n<Root>\n  <Name>x</Name>\n</Root>\n"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
