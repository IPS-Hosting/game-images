package gamedef

import (
	"encoding/json"
	"fmt"
	"path"
	"regexp"
	"strconv"
	"strings"
)

// imageReferencePattern matches docker image references with optional registry, tag and
// sha256 digest.
var imageReferencePattern = regexp.MustCompile(`^(?:[a-z0-9.-]+(?::[0-9]+)?/)?[a-z0-9]+(?:[._-][a-z0-9]+)*(?:/[a-z0-9]+(?:[._-][a-z0-9]+)*)*(?::[A-Za-z0-9_][A-Za-z0-9._-]{0,127})?(?:@sha256:[a-f0-9]{64})?$`)

// OwnImagePrefixes are the platform's own repositories. Their images may be referenced by
// tag; the repository's release process guarantees immutable tags.
var OwnImagePrefixes = []string{"docker.io/ipshosting/", "ghcr.io/ips-hosting/"}

// AllowedImagePrefixes are the external repositories game definitions may use. External
// images must be pinned by digest (renovate keeps the digests current).
var AllowedImagePrefixes = []string{
	"ghcr.io/pelican-eggs/",
	"ghcr.io/parkervcp/",
	"ghcr.io/ptero-eggs/",
	"docker.io/itzg/",
	"docker.io/library/",
}

// reservedEnv are always set by the engine and must not be overridden.
var reservedEnv = map[string]struct{}{"TZ": {}, "HOME": {}}

// NormalizeImage expands a docker image reference to registry/repository form
// (valheim -> docker.io/library/valheim, itzg/minecraft-server -> docker.io/itzg/...).
func NormalizeImage(reference string) string {
	first, rest, found := strings.Cut(reference, "/")
	if !found {
		return "docker.io/library/" + reference
	}
	if strings.ContainsAny(first, ".:") || first == "localhost" {
		return reference
	}
	return "docker.io/" + first + "/" + rest
}

// CheckImage validates a rendered image reference against the pattern and the registry
// allowlist.
func CheckImage(reference string) error {
	if !imageReferencePattern.MatchString(reference) {
		return fmt.Errorf("%q is not a valid image reference", reference)
	}
	normalized := NormalizeImage(reference)
	for _, prefix := range OwnImagePrefixes {
		if strings.HasPrefix(normalized, prefix) {
			if !strings.Contains(reference, ":") {
				return fmt.Errorf("image %q must specify a tag", reference)
			}
			return nil
		}
	}
	for _, prefix := range AllowedImagePrefixes {
		if strings.HasPrefix(normalized, prefix) {
			if !strings.Contains(reference, "@sha256:") {
				return fmt.Errorf("external image %q must be pinned by digest (image:tag@sha256:...)", reference)
			}
			return nil
		}
	}
	return fmt.Errorf("image %q is not from an allowed registry (%s)", reference, strings.Join(append(append([]string{}, OwnImagePrefixes...), AllowedImagePrefixes...), ", "))
}

// CheckRelativePath rejects absolute paths and paths that escape the data directory. It
// returns the cleaned path.
func CheckRelativePath(value string) (string, error) {
	if strings.ContainsRune(value, 0) {
		return "", fmt.Errorf("path %q contains a NUL byte", value)
	}
	if path.IsAbs(value) {
		return "", fmt.Errorf("path %q must be relative to the data directory", value)
	}
	cleaned := path.Clean(value)
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", fmt.Errorf("path %q escapes the data directory", value)
	}
	return cleaned, nil
}

type validator struct {
	definition *Definition
	errors     []Error
}

func (v *validator) add(at string, format string, args ...any) {
	v.errors = append(v.errors, Error{Path: at, Message: fmt.Sprintf(format, args...)})
}

// validate runs the semantic checks (SPEC §9) that the JSON schema cannot express.
func (d *Definition) validate() []Error {
	v := &validator{definition: d}

	v.checkPorts()
	v.checkSettings()
	v.checkReferences()
	v.checkStart()
	v.checkInstall()
	v.checkPatterns()
	v.checkPaths()
	v.checkEnv()
	v.checkTemplates()

	// A definition that passes all static checks must also render with the declared
	// defaults: this catches runtime-only errors (image pattern, type conversions,
	// path escapes produced by templates).
	if len(v.errors) == 0 {
		if _, err := Render(d, DefaultValues(d)); err != nil {
			v.add("", "does not render with default values: %v", err)
		}
		if len(d.Install) > 0 {
			if _, err := RenderInstall(d, DefaultValues(d), InstallOptions{}); err != nil {
				v.add("/install", "does not render with default values: %v", err)
			}
		}
	}
	return v.errors
}

func (v *validator) checkPorts() {
	identifiers := map[string]struct{}{}
	defaults := map[string]string{}
	// baseEnv exports IPS_PORT_<ID> and IPS_PORT_<ID>_EXPOSED; identifiers like game and
	// game_exposed would generate the same name.
	envNames := map[string]string{}
	for i, port := range v.definition.Ports {
		at := fmt.Sprintf("/ports/%d", i)
		if _, ok := identifiers[port.Identifier]; ok {
			v.add(at, "duplicate port identifier %q", port.Identifier)
		}
		identifiers[port.Identifier] = struct{}{}
		for _, name := range portEnvNames(port.Identifier) {
			if other, ok := envNames[name]; ok && other != port.Identifier {
				v.add(at, "port %q and %q both generate the environment variable %s", port.Identifier, other, name)
			}
			envNames[name] = port.Identifier
		}
		key := fmt.Sprintf("%d/%s", port.Default, port.Protocol)
		if other, ok := defaults[key]; ok {
			v.add(at, "port %q has the same default and protocol as %q", port.Identifier, other)
		}
		defaults[key] = port.Identifier
	}
}

func (v *validator) checkSettings() {
	keys := map[string]string{}
	for i, setting := range v.definition.Settings {
		at := fmt.Sprintf("/settings/%d", i)
		// Setting keys are compared case-insensitively by the platform database
		// (utf8mb4_unicode_ci), so keys that differ only in case would collide.
		lower := strings.ToLower(setting.Key)
		if other, ok := keys[lower]; ok {
			v.add(at, "setting key %q collides with %q (keys must be unique case-insensitively)", setting.Key, other)
		}
		keys[lower] = setting.Key

		if setting.IsEnum() && len(setting.Values) == 0 && setting.ValuesSource == "" {
			v.add(at, "%s setting %q requires values or valuesSource", setting.Type, setting.Key)
		}
		if setting.Default == nil {
			continue
		}
		value := *setting.Default
		switch setting.Type {
		case SettingBoolean:
			if value != "0" && value != "1" {
				v.add(at+"/default", "BOOLEAN default must be '0' or '1'")
			}
		case SettingNumber, SettingNumberEnum:
			if _, err := strconv.ParseFloat(value, 64); err != nil {
				v.add(at+"/default", "%s default %q is not a number", setting.Type, value)
			}
		case SettingStringList:
			var list []string
			if err := json.Unmarshal([]byte(value), &list); err != nil {
				v.add(at+"/default", "STRINGLIST default must be a JSON array of strings")
			}
		}
		if setting.IsEnum() && len(setting.Values) > 0 && setting.ValuesSource == "" {
			found := false
			for _, candidate := range setting.Values {
				if candidate.Value == value {
					found = true
					break
				}
			}
			if !found {
				v.add(at+"/default", "default %q is not one of the declared values", value)
			}
		}
	}
}

func (v *validator) requirePort(at string, identifier string) {
	if identifier != "" && !v.definition.HasPort(identifier) {
		v.add(at, "references undeclared port %q", identifier)
	}
}

func (v *validator) requireSetting(at string, key string, types ...string) {
	if key == "" {
		return
	}
	setting := v.definition.Setting(key)
	if setting == nil {
		v.add(at, "references undeclared setting %q", key)
		return
	}
	if len(types) == 0 {
		return
	}
	for _, allowed := range types {
		if setting.Type == allowed {
			return
		}
	}
	v.add(at, "setting %q must be of type %s, got %s", key, strings.Join(types, " or "), setting.Type)
}

func (v *validator) checkReferences() {
	d := v.definition
	v.requirePort("/console/port", d.Console.Port)
	v.requireSetting("/console/passwordSetting", d.Console.PasswordSetting, SettingHiddenString)
	if d.Stop.Via == StopCommand && d.Console.Via == ConsoleNone {
		v.add("/stop", "stop via command needs a console channel (console.via stdin or rcon)")
	}

	v.requirePort("/query/port", d.Query.Port)
	v.requirePort("/query/fallbackPort", d.Query.FallbackPort)
	v.requireSetting("/query/passwordSetting", d.Query.PasswordSetting, SettingHiddenString)
	if d.Query.FallbackPort != "" && d.Query.Protocol != QueryMinecraft {
		v.add("/query/fallbackPort", "only allowed with protocol minecraft")
	}
	if d.Query.Protocol == QueryRcon && d.Query.Command == "" && d.Query.CountLinesCommand == "" {
		v.add("/query", "rcon query needs command or countLinesCommand")
	}

	if d.Workshop != nil {
		for i, key := range d.Workshop.Settings {
			v.requireSetting(fmt.Sprintf("/workshop/settings/%d", i), key, SettingStringList, SettingString)
		}
	}
	if *d.Flags.Installable && len(d.Install) == 0 {
		v.add("/flags/installable", "installable requires at least one install stage")
	}
	if !*d.Flags.Installable && len(d.Install) > 0 {
		v.add("/flags/installable", "install stages are declared, so the game is installable")
	}
	if *d.Flags.ConsoleCommands && d.Console.Via == ConsoleNone {
		v.add("/flags/consoleCommands", "console commands need a console channel")
	}
}

func (v *validator) checkStart() {
	start := v.definition.Start
	if len(start.Args) > 0 && start.Shell != "" {
		// Also enforced here because Lenient parsing skips the schema.
		v.add("/start", "args and shell are mutually exclusive")
	}
	if len(start.Args) == 0 && start.Shell == "" {
		// An empty start uses the image entrypoint (external images that own their start).
		return
	}
	for i, item := range start.Args {
		if item.Template == nil && item.Arg == "" {
			v.add(fmt.Sprintf("/start/args/%d", i), "arg must not be empty")
		}
	}
}

func (v *validator) checkInstall() {
	for i, stage := range v.definition.Install {
		at := fmt.Sprintf("/install/%d", i)
		// Templated URLs (versions chosen by settings) cannot be pinned statically; ips
		// verifies them when the definition provides a checksum.
		if stage.Kind == StageDownload && stage.Checksum == "" && !strings.Contains(stage.URL, "{{") {
			v.add(at+"/checksum", "static download URLs must be pinned by checksum")
		}
	}
}

func (v *validator) checkPatterns() {
	d := v.definition
	compile := func(at string, pattern string) {
		if pattern == "" {
			return
		}
		if _, err := regexp.Compile(pattern); err != nil {
			v.add(at, "invalid regular expression: %v", err)
		}
	}
	if d.Ready != nil {
		compile("/ready/pattern", d.Ready.Pattern)
	}
	if d.Players != nil {
		compile("/players/joinPattern", d.Players.JoinPattern)
		compile("/players/leavePattern", d.Players.LeavePattern)
	}
	compile("/query/pattern", d.Query.Pattern)
	for name, ops := range map[string][]HookOp{"postInstall": d.Hooks.PostInstall, "preStart": d.Hooks.PreStart} {
		for i, op := range ops {
			if op.Replace != nil {
				compile(fmt.Sprintf("/hooks/%s/%d/replace/pattern", name, i), op.Replace.Pattern)
			}
		}
	}
}

// staticPath checks paths that contain no template actions; templated paths are checked
// after rendering.
func (v *validator) staticPath(at string, value string) {
	if value == "" || strings.Contains(value, "{{") {
		return
	}
	if _, err := CheckRelativePath(value); err != nil {
		v.add(at, "%v", err)
	}
}

func (v *validator) checkPaths() {
	d := v.definition
	v.staticPath("/start/workingDir", d.Start.WorkingDir)
	for i, mount := range d.Runtime.Mounts {
		v.staticPath(fmt.Sprintf("/runtime/mounts/%d/source", i), mount.Source)
	}
	for i, file := range d.Config {
		v.staticPath(fmt.Sprintf("/config/%d/path", i), file.Path)
		v.staticPath(fmt.Sprintf("/config/%d/createFrom", i), file.CreateFrom)
	}
	for i, stage := range d.Install {
		v.staticPath(fmt.Sprintf("/install/%d/dir", i), stage.Dir)
		v.staticPath(fmt.Sprintf("/install/%d/dest", i), stage.Dest)
	}
	for name, ops := range map[string][]HookOp{"postInstall": d.Hooks.PostInstall, "preStart": d.Hooks.PreStart} {
		for i, op := range ops {
			at := fmt.Sprintf("/hooks/%s/%d", name, i)
			for _, p := range op.paths() {
				v.staticPath(at, p)
			}
		}
	}
	for i, glob := range d.BackupIgnore {
		if strings.Contains(glob, "..") {
			v.add(fmt.Sprintf("/backupIgnore/%d", i), "must not contain '..'")
		}
	}
}

func (v *validator) checkEnv() {
	check := func(at string, env map[string]string) {
		for name := range env {
			if _, ok := reservedEnv[name]; ok {
				v.add(at, "must not set reserved variable %q", name)
			}
			if strings.HasPrefix(name, "IPS_") {
				v.add(at, "must not set %q: the IPS_ prefix is reserved for the engine", name)
			}
		}
	}
	check("/runtime/env", v.definition.Runtime.Env)
	check("/start/env", v.definition.Start.Env)
	for i, stage := range v.definition.Install {
		check(fmt.Sprintf("/install/%d/env", i), stage.Env)
	}
}

func (v *validator) checkTemplates() {
	for at, text := range v.definition.templates() {
		if !strings.Contains(text, "{{") {
			continue
		}
		if _, err := compileTemplate(at, text, v.definition); err != nil {
			v.add(at, "%v", err)
		}
	}
}

// templates returns every template string of the definition keyed by its location.
func (d *Definition) templates() map[string]string {
	t := map[string]string{"/runtime/image": d.Runtime.Image}
	add := func(at string, value string) {
		if value != "" {
			t[at] = value
		}
	}
	for name, value := range d.Runtime.Env {
		add("/runtime/env/"+name, value)
	}
	for i, mount := range d.Runtime.Mounts {
		add(fmt.Sprintf("/runtime/mounts/%d/source", i), mount.Source)
	}
	for i, stage := range d.Install {
		at := fmt.Sprintf("/install/%d", i)
		add(at+"/when", stage.When)
		add(at+"/branch", stage.Branch)
		add(at+"/branchPassword", stage.BranchPassword)
		add(at+"/dir", stage.Dir)
		add(at+"/url", stage.URL)
		add(at+"/dest", stage.Dest)
		add(at+"/image", stage.Image)
		for j, arg := range stage.Args {
			add(fmt.Sprintf("%s/args/%d", at, j), arg)
		}
		for name, value := range stage.Env {
			add(at+"/env/"+name, value)
		}
	}
	for i, item := range d.Start.Args {
		at := fmt.Sprintf("/start/args/%d", i)
		if item.Template != nil {
			add(at, *item.Template)
			continue
		}
		add(at+"/arg", item.Arg)
		if item.Value != nil {
			add(at+"/value", *item.Value)
		}
		add(at+"/when", item.When)
	}
	for name, value := range d.Start.Env {
		add("/start/env/"+name, value)
	}
	add("/start/workingDir", d.Start.WorkingDir)
	add("/query/url", d.Query.URL)
	for i, setting := range d.Settings {
		add(fmt.Sprintf("/settings/%d/visibleWhen", i), setting.VisibleWhen)
	}
	for i, file := range d.Config {
		at := fmt.Sprintf("/config/%d", i)
		add(at+"/path", file.Path)
		add(at+"/template", file.Template)
		add(at+"/when", file.When)
		for key, value := range file.Values {
			add(at+"/values/"+key, value)
		}
	}
	for name, ops := range map[string][]HookOp{"postInstall": d.Hooks.PostInstall, "preStart": d.Hooks.PreStart} {
		for i, op := range ops {
			at := fmt.Sprintf("/hooks/%s/%d", name, i)
			add(at+"/when", op.When)
			for j, p := range op.paths() {
				add(fmt.Sprintf("%s/path%d", at, j), p)
			}
			if op.Replace != nil {
				add(at+"/replace/replacement", op.Replace.Replacement)
			}
			if op.WriteFile != nil {
				add(at+"/writeFile/content", op.WriteFile.Content)
			}
		}
	}
	for i, precondition := range d.Preconditions {
		add(fmt.Sprintf("/preconditions/%d/check", i), precondition.Check)
	}
	return t
}

// paths returns the data-directory paths an operation touches.
func (op HookOp) paths() []string {
	switch {
	case op.CopyIfMissing != nil:
		return []string{op.CopyIfMissing.From, op.CopyIfMissing.To}
	case op.Symlink != nil:
		return []string{op.Symlink.Target, op.Symlink.Link}
	case op.Replace != nil:
		return []string{op.Replace.Path}
	case op.EnsureDir != nil:
		return []string{op.EnsureDir.Path}
	case op.WriteFile != nil:
		return []string{op.WriteFile.Path}
	}
	return nil
}
