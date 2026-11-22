package gamedef

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const base = `
apiVersion: 1
short: testgame
name: Test
requires: { engine: 1 }
runtime:
  image: ipshosting/runtime-test:v1
ports:
  - { identifier: game, protocol: UDP, default: 2456 }
settings:
  - { key: name, name: Name, type: STRING, default: My Server }
  - { key: password, name: Passwort, type: HIDDENSTRING }
  - { key: crossplay, name: Crossplay, type: BOOLEAN, default: '0' }
`

func mustParse(t *testing.T, doc string) *Definition {
	t.Helper()
	definition, err := Parse([]byte(doc), Strict)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return definition
}

func parseError(t *testing.T, doc string) string {
	t.Helper()
	_, err := Parse([]byte(doc), Strict)
	if err == nil {
		t.Fatalf("expected an error")
	}
	return err.Error()
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

func TestPlainArgsKeepEmptyValues(t *testing.T) {
	d := mustParse(t, base+`
start:
  args:
    - ./server
    - -password
    - '{{ .Setting "password" }}'
    - -savedir
    - data
`)
	plan := render(t, d, nil)
	want := []string{"./server", "-password", "", "-savedir", "data"}
	if !reflect.DeepEqual(plan.Command, want) {
		t.Fatalf("got %q, want %q", plan.Command, want)
	}
}

func TestObjectArgs(t *testing.T) {
	d := mustParse(t, base+`
start:
  args:
    - ./server
    - arg: -crossplay
      when: '{{ .Setting "crossplay" }}'
    - arg: -password
      value: '{{ .Setting "password" }}'
    - arg: -name
      value: '{{ .Setting "name" }}'
`)
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
	d := mustParse(t, base+`
start:
  env:
    NAME: '{{ .Setting "name" }}'
  shell: exec ./server -name "$NAME"
`)
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

func TestStartRequiresExactlyOneForm(t *testing.T) {
	message := parseError(t, base+`
start:
  args: [./a]
  shell: ./b
`)
	if !strings.Contains(message, "/start") {
		t.Fatalf("unexpected error: %s", message)
	}
}

func TestUnknownFieldStrictVsLenient(t *testing.T) {
	doc := base + "start: {}\nfutureField: true\n"
	if _, err := Parse([]byte(doc), Strict); err == nil {
		t.Fatalf("strict mode must reject unknown fields")
	}
	if _, err := Parse([]byte(doc), Lenient); err != nil {
		t.Fatalf("lenient mode must tolerate unknown fields: %v", err)
	}
}

func TestEngineTooNew(t *testing.T) {
	doc := strings.Replace(base, "engine: 1", "engine: 99", 1) + "start: {}\n"
	_, err := Parse([]byte(doc), Lenient)
	if !errors.Is(err, ErrUnsupportedEngine) {
		t.Fatalf("expected ErrUnsupportedEngine, got %v", err)
	}
}

func TestUndeclaredReferences(t *testing.T) {
	message := parseError(t, base+`
start:
  args: ['{{ .Setting "nope" }}', '{{ (.Port "missing").Container }}']
`)
	if !strings.Contains(message, `undeclared setting "nope"`) && !strings.Contains(message, `undeclared port "missing"`) {
		t.Fatalf("unexpected error: %s", message)
	}
}

func TestForbiddenTemplateFunction(t *testing.T) {
	message := parseError(t, base+`
start:
  args: ['{{ call .Server }}']
`)
	if !strings.Contains(message, "not allowed") {
		t.Fatalf("unexpected error: %s", message)
	}
}

func TestSettingKeysCaseInsensitiveUnique(t *testing.T) {
	message := parseError(t, base+`
  - { key: Name, name: Name2, type: STRING }
start: {}
`)
	if !strings.Contains(message, "case-insensitively") {
		t.Fatalf("unexpected error: %s", message)
	}
}

func TestImageAllowlist(t *testing.T) {
	cases := map[string]bool{
		"ipshosting/runtime-steam:v1": true,
		"ipshosting/runtime-steam":    false, // own images need a tag
		"ghcr.io/parkervcp/games:valheim@sha256:c851b055fa784608244b9e494ada4af421f78001e42e71bf5aa73ded2b321ad1": true,
		"ghcr.io/parkervcp/games:valheim": false, // external images need a digest
		"evil.example.com/miner:latest":   false,
		"ubuntu:24.04@sha256:9c7f200c07e3d4a03e7249d2c16bf66a51120fb876aa2242cf31451412543289": true,
	}
	for image, ok := range cases {
		if err := CheckImage(image); (err == nil) != ok {
			t.Errorf("%s: got %v, want ok=%v", image, err, ok)
		}
	}
}

func TestPathEscape(t *testing.T) {
	message := parseError(t, base+`
start: {}
config:
  - { path: ../../etc/passwd, format: kv, values: { a: b } }
`)
	if !strings.Contains(message, "escapes the data directory") {
		t.Fatalf("unexpected error: %s", message)
	}
	// Templated paths are checked after rendering.
	d := mustParse(t, base+`
start: {}
config:
  - { path: '{{ .Setting "name" }}/server.cfg', format: kv, values: { a: b } }
`)
	values := DefaultValues(d)
	values.Settings["name"] = "../../.."
	if _, err := Render(d, values); err == nil || !strings.Contains(err.Error(), "escapes") {
		t.Fatalf("expected escape error, got %v", err)
	}
}

func TestConfigValuesRejectLineBreaks(t *testing.T) {
	d := mustParse(t, base+`
start: {}
config:
  - { path: server.properties, format: properties, values: { motd: '{{ .Setting "name" }}' } }
`)
	values := DefaultValues(d)
	values.Settings["name"] = "A\nrcon.password=x"
	if _, err := Render(d, values); err == nil || !strings.Contains(err.Error(), "line breaks") {
		t.Fatalf("expected line break error, got %v", err)
	}
}

func TestApplyProperties(t *testing.T) {
	existing := "# comment\nmotd=old\nlevel-seed=42\nmotd=duplicate\n"
	out, err := ApplyConfig(PlanConfigFile{Format: FormatProperties, Values: map[string]string{"motd": "new", "max-players": "10"}}, []byte(existing))
	if err != nil {
		t.Fatal(err)
	}
	want := "# comment\nmotd=new\nlevel-seed=42\nmax-players=10\n"
	if string(out) != want {
		t.Fatalf("got %q, want %q", out, want)
	}
}

func TestApplyKVDelimiter(t *testing.T) {
	out, _ := ApplyConfig(PlanConfigFile{Format: FormatKV, Delimiter: " ", Values: map[string]string{"hostname": "x"}}, []byte("hostname old\nsv_lan 0\n"))
	if string(out) != "hostname x\nsv_lan 0\n" {
		t.Fatalf("got %q", out)
	}
}

func TestApplyINI(t *testing.T) {
	existing := "global=1\n\n[/Script/Engine.GameSession]\nMaxPlayers=4\nOther=keep\nMaxPlayers=8\n\n[Other]\nx=1\n"
	out, err := ApplyConfig(PlanConfigFile{Format: FormatINI, Values: map[string]string{
		"/Script/Engine.GameSession.MaxPlayers": "16",
		"New.Section.Key":                       "v",
		"global":                                "2",
	}}, []byte(existing))
	if err != nil {
		t.Fatal(err)
	}
	want := "global=2\n\n[/Script/Engine.GameSession]\nMaxPlayers=16\nOther=keep\n\n[Other]\nx=1\n[New.Section]\nKey=v\n"
	if string(out) != want {
		t.Fatalf("got:\n%s\nwant:\n%s", out, want)
	}
}

func TestApplyJSONKeepsTypes(t *testing.T) {
	existing := `{"name":"old","port":1,"version":"1.0","nested":{"on":false}}`
	out, err := ApplyConfig(PlanConfigFile{Format: FormatJSON, Values: map[string]string{"name": "new", "port": "2", "version": "2", "nested.on": "true"}}, []byte(existing))
	if err != nil {
		t.Fatal(err)
	}
	want := `{"name":"new","port":2,"version":"2","nested":{"on":true}}`
	if string(out) != want {
		t.Fatalf("got %s, want %s", out, want)
	}
}

func TestApplyYAML(t *testing.T) {
	existing := "# server config\nserver:\n  name: old # keep comment\n  port: 1\n"
	out, err := ApplyConfig(PlanConfigFile{Format: FormatYAML, Values: map[string]string{"server.name": "new", "server.port": "2", "extra.flag": "true"}}, []byte(existing))
	if err != nil {
		t.Fatal(err)
	}
	text := string(out)
	for _, want := range []string{"# server config", "name: new # keep comment", "port: 2", "extra:\n  flag: true"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in:\n%s", want, text)
		}
	}
}

func TestSteamcmdRunscript(t *testing.T) {
	d := mustParse(t, base+`
install:
  - kind: steamcmd
    appId: 896660
    login: host
    branch: '{{ .Setting "name" }}'
start: {}
`)
	values := DefaultValues(d)
	values.Settings["name"] = "beta"
	plan, err := RenderInstall(d, values, InstallOptions{Validate: true})
	if err != nil {
		t.Fatal(err)
	}
	stage := plan.Stages[0]
	if strings.Join(stage.Command, " ") != "steamcmd +runscript "+SteamcmdRunscriptPath {
		t.Fatalf("credentials must not be in argv: %q", stage.Command)
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

func TestPreconditions(t *testing.T) {
	d := mustParse(t, base+`
start: {}
preconditions:
  - check: '{{ ge (len (.Setting "password")) 5 }}'
    message: zu kurz
  - check: '{{ not (.Setting "name" | contains (.Setting "password")) }}'
    message: im Namen
    severity: warning
`)
	plan := render(t, d, map[string]string{"password": "Serv", "name": "My Server"})
	if len(plan.Blocking()) != 1 || plan.Blocking()[0].Message.De != "zu kurz" {
		t.Fatalf("unexpected blocking preconditions: %+v", plan.Preconditions)
	}
	if plan.Preconditions[1].Passed {
		t.Fatalf("password contained in name must fail the warning")
	}
}

func TestI18nShorthand(t *testing.T) {
	d := mustParse(t, strings.Replace(base, "name: Test", "name: { de: Test, en: Test EN }", 1)+"start: {}\n")
	if d.Name.Text("en") != "Test EN" || d.Name.Text("fr") != "Test" {
		t.Fatalf("unexpected i18n %+v", d.Name)
	}
	if d.Settings[0].Name.De != "Name" {
		t.Fatalf("plain string must map to de")
	}
}

// TestCatalog validates every definition in the repository, like the CI does.
func TestCatalog(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("..", "games", "*", "game.yaml"))
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
		definition, err := Parse(data, Strict)
		if err != nil {
			t.Errorf("%s: %v", file, err)
			continue
		}
		if dir := filepath.Base(filepath.Dir(file)); definition.Short != dir {
			t.Errorf("%s: short %q != directory %q", file, definition.Short, dir)
		}
	}
}

func TestAssociatedTemplatesRejected(t *testing.T) {
	for _, tmpl := range []string{
		`{{define "x"}}{{html "text"}}{{end}}{{template "x"}}`,
		`{{block "x" .}}{{html "text"}}{{end}}`,
	} {
		message := parseError(t, base+"start:\n  args: ['"+tmpl+"']\n")
		if !strings.Contains(message, "not allowed") {
			t.Fatalf("%s: unexpected error: %s", tmpl, message)
		}
	}
}

func TestWithChangesDot(t *testing.T) {
	d := mustParse(t, base+`
start:
  args:
    - '{{ with .Port "game" }}{{ .Container }}/{{ $.Setting "name" }}{{ end }}'
`)
	if got := render(t, d, nil).Command; !reflect.DeepEqual(got, []string{"2456/My Server"}) {
		t.Fatalf("got %q", got)
	}
	// $ is the template context and stays restricted to its roots and declared settings.
	for _, tmpl := range []string{
		`{{ with .Port "game" }}{{ $.SettingValue "name" }}{{ end }}`,
		`{{ with .Port "game" }}{{ $.Setting "nope" }}{{ end }}`,
	} {
		message := parseError(t, base+"start:\n  args: ['"+tmpl+"']\n")
		if !strings.Contains(message, "not part of the template context") && !strings.Contains(message, `undeclared setting "nope"`) {
			t.Fatalf("%s: unexpected error: %s", tmpl, message)
		}
	}
}

func TestRenderedArgsAndSecretsStayVerbatim(t *testing.T) {
	d := mustParse(t, base+`
install:
  - kind: steamcmd
    appId: 896660
    branch: beta
    branchPassword: '{{ .Setting "password" }}'
start:
  args:
    - '{{ .Setting "password" }}'
    - arg: -name
      value: '{{ .Setting "name" }}'
`)
	values := DefaultValues(d)
	values.Settings["password"] = " secret "
	values.Settings["name"] = " "
	plan, err := Render(d, values)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{" secret ", "-name", " "}; !reflect.DeepEqual(plan.Command, want) {
		t.Fatalf("got %q, want %q", plan.Command, want)
	}
	install, err := RenderInstall(d, values, InstallOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if got := install.Stages[0].Steamcmd.BranchPassword; got != " secret " {
		t.Fatalf("branch password changed: %q", got)
	}
}

func TestPortEnvNameCollision(t *testing.T) {
	doc := strings.Replace(base, "ports:\n", "ports:\n  - { identifier: game_exposed, protocol: UDP, default: 2457 }\n", 1)
	message := parseError(t, doc+"start: {}\n")
	if !strings.Contains(message, "IPS_PORT_GAME_EXPOSED") {
		t.Fatalf("unexpected error: %s", message)
	}
}

func TestStartExclusivityLenient(t *testing.T) {
	_, err := Parse([]byte(base+"start:\n  args: [./a]\n  shell: ./b\n"), Lenient)
	if err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
		t.Fatalf("lenient mode must reject args and shell together, got %v", err)
	}
}

func TestApplyYAMLDuplicatesAndEmptyStrings(t *testing.T) {
	existing := "port: 1\nversion: \"\"\nserver:\n  name: a\nport: 3\nserver:\n  name: b\n"
	out, err := ApplyConfig(PlanConfigFile{Format: FormatYAML, Values: map[string]string{"port": "2", "version": "2", "server.name": "c"}}, []byte(existing))
	if err != nil {
		t.Fatal(err)
	}
	want := "port: 2\nversion: \"2\"\nserver:\n  name: c\n"
	if string(out) != want {
		t.Fatalf("got:\n%s\nwant:\n%s", out, want)
	}
}
