package gamedef

import (
	"fmt"
	"path"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"
)

// imageReferencePattern matches docker image references with optional registry, tag and
// sha256 digest.
var imageReferencePattern = regexp.MustCompile(`^(?:[a-z0-9.-]+(?::[0-9]+)?/)?[a-z0-9]+(?:[._-][a-z0-9]+)*(?:/[a-z0-9]+(?:[._-][a-z0-9]+)*)*(?::[A-Za-z0-9_][A-Za-z0-9._-]{0,127})?(?:@sha256:[a-f0-9]{64})?$`)

// OwnImagePrefixes are the platform's own repositories (including mirrors of external
// images). Their images may be referenced by tag; the release process keeps tags immutable.
var OwnImagePrefixes = []string{"docker.io/ipshosting/", "ghcr.io/ips-hosting/"}

// AllowedImagePrefixes are the external repositories definitions may use. External images
// must be pinned by digest (renovate keeps the digests current).
var AllowedImagePrefixes = []string{
	"ghcr.io/pelican-eggs/",
	"ghcr.io/parkervcp/",
	"ghcr.io/ptero-eggs/",
	"docker.io/itzg/",
	"docker.io/library/",
}

// Tags is the controlled tag vocabulary.
var Tags = []string{
	"action", "adventure", "battle-royale", "building", "co-op", "crafting", "fps", "horror",
	"mmo", "moddable", "multiplayer", "open-world", "pvp", "pve", "racing", "roleplay",
	"rts", "sandbox", "simulation", "space", "sports", "strategy", "survival", "tactical",
	"voice", "zombies",
}

// reservedEnv are always set by the engine and must not be overridden.
var reservedEnv = map[string]struct{}{"TZ": {}, "HOME": {}}

// EnginePath is the container directory reserved for the engine (ips-init, private files).
const EnginePath = "/ips"

const maxTmpfs = 8

var (
	shortPattern        = regexp.MustCompile(`^[a-z0-9]{1,32}$`)
	portIDPattern       = regexp.MustCompile(`^[a-z][a-z0-9_]{0,31}$`)
	envNamePattern      = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{0,127}$`)
	checksumPattern     = regexp.MustCompile(`^sha256:[a-f0-9]{64}$`)
	sha256Pattern       = regexp.MustCompile(`^[a-f0-9]{64}$`)
	converterPattern    = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*@[0-9A-Za-z][0-9A-Za-z.+-]*$`)
	modePattern         = regexp.MustCompile(`^0[0-7]{3}$`)
	xmlNamePattern      = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_:-]*$`)
	valuesSourcePattern = regexp.MustCompile(`^[a-z][a-zA-Z0-9]{0,63}$`)
)

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

// CheckImage validates an image reference against the pattern and the registry allowlist.
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
	return fmt.Errorf("image %q is not from an allowed registry (%s)", reference, strings.Join(append(slices.Clone(OwnImagePrefixes), AllowedImagePrefixes...), ", "))
}

// CheckRelativePath rejects absolute paths and paths that escape the data directory. It
// returns the cleaned path. Symlinks are resolved by ips-init inside the container.
func CheckRelativePath(value string) (string, error) {
	if value == "" {
		return "", fmt.Errorf("path must not be empty")
	}
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

func (v *validator) oneOf(at string, value string, allowed ...string) {
	if !slices.Contains(allowed, value) {
		v.add(at, "must be one of %s, got %q", strings.Join(allowed, ", "), value)
	}
}

func (v *validator) requireText(at string, text I18n) {
	if strings.TrimSpace(text.De) == "" {
		v.add(at, "German text (de) is required")
	}
}

// literal rejects template syntax in fields that are not templates.
func (v *validator) literal(at string, value string) bool {
	if strings.Contains(value, "{{") {
		v.add(at, "templates are not allowed here")
		return false
	}
	return true
}

// relPath checks a literal path relative to the data directory.
func (v *validator) relPath(at string, value string) {
	if value == "" || !v.literal(at, value) {
		return
	}
	if _, err := CheckRelativePath(value); err != nil {
		v.add(at, "%v", err)
	}
}

// templatePath checks a templated path statically when it contains no template actions;
// rendered paths are checked again at render time.
func (v *validator) templatePath(at string, value string) {
	if strings.Contains(value, "{{") {
		return
	}
	if _, err := CheckRelativePath(value); err != nil {
		v.add(at, "%v", err)
	}
}

func (v *validator) requirePort(at string, identifier string) {
	if identifier != "" && !v.definition.HasPort(identifier) {
		v.add(at, "references undeclared port %q", identifier)
	}
}

func (v *validator) requireSetting(at string, key string, types ...string) *Setting {
	setting := v.definition.Setting(key)
	if setting == nil {
		v.add(at, "references undeclared setting %q", key)
		return nil
	}
	if len(types) > 0 && !slices.Contains(types, setting.Type) {
		v.add(at, "setting %q must be of type %s, got %s", key, strings.Join(types, " or "), setting.Type)
	}
	return setting
}

// validate runs the structural and semantic checks (SPEC §9).
func (d *Definition) validate() []Error {
	v := &validator{definition: d}

	v.checkIdentity()
	v.checkPorts()
	v.checkSettings()
	v.checkRun()
	v.checkInstall()
	v.checkQuery()
	v.checkConfig()
	v.checkHooks()
	v.checkMisc()
	v.checkTemplates()

	// A definition that passes all static checks must also render with the declared
	// defaults: this catches runtime-only errors (type conversions, rendered path escapes).
	if len(v.errors) == 0 {
		values := DefaultValues(d)
		if _, err := Render(d, values); err != nil {
			v.add("", "does not render with default values: %v", err)
		}
		if _, err := RenderInstall(d, values, InstallOptions{}); err != nil {
			v.add("/install", "does not render with default values: %v", err)
		}
	}
	return v.errors
}

func (v *validator) checkIdentity() {
	d := v.definition
	if !shortPattern.MatchString(d.Short) {
		v.add("/short", "must match %s", shortPattern)
	}
	v.requireText("/name", d.Name)
	if len(d.Tags) > 6 {
		v.add("/tags", "at most 6 tags")
	}
	seen := map[string]bool{}
	for i, tag := range d.Tags {
		v.oneOf(fmt.Sprintf("/tags/%d", i), tag, Tags...)
		if seen[tag] {
			v.add(fmt.Sprintf("/tags/%d", i), "duplicate tag %q", tag)
		}
		seen[tag] = true
	}
	for i, source := range d.Sources {
		at := fmt.Sprintf("/sources/%d", i)
		v.oneOf(at+"/kind", source.Kind, SourceManual, SourceEgg, SourceAMP, SourceLGSM)
		if source.Kind != SourceManual && (source.Ref == "" || source.License == "") {
			v.add(at, "imported sources need ref and license")
		}
		if source.Converter != "" && !converterPattern.MatchString(source.Converter) {
			v.add(at+"/converter", "must be name@version")
		}
		if source.ImportedSha256 != "" && !sha256Pattern.MatchString(source.ImportedSha256) {
			v.add(at+"/importedSha256", "must be a lowercase hex sha256")
		}
	}
}

func (v *validator) checkPorts() {
	identifiers := map[string]struct{}{}
	defaults := map[string]string{}
	// baseEnv exports IPS_PORT_<ID> and IPS_PORT_<ID>_EXPOSED; identifiers like game and
	// game_exposed would generate the same name.
	envNames := map[string]string{}
	for i, port := range v.definition.Ports {
		at := fmt.Sprintf("/ports/%d", i)
		if !portIDPattern.MatchString(port.Identifier) {
			v.add(at+"/identifier", "must match %s", portIDPattern)
		}
		v.oneOf(at+"/protocol", port.Protocol, "TCP", "UDP")
		if port.Default < 1 || port.Default > 65535 {
			v.add(at+"/default", "must be a port number")
		}
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

func (v *validator) checkEnv(at string, env map[string]string) {
	for _, name := range sortedKeys(env) {
		switch _, reserved := reservedEnv[name]; {
		case !envNamePattern.MatchString(name):
			v.add(at, "invalid environment variable name %q", name)
		case reserved:
			v.add(at, "must not set reserved variable %q", name)
		case strings.HasPrefix(name, "IPS_"):
			v.add(at, "must not set %q: the IPS_ prefix is reserved for the engine", name)
		case (strings.HasPrefix(name, "LD_") || strings.HasPrefix(name, "DYLD_")) && strings.Contains(env[name], "{{"):
			// Loader variables load code: a setting value must never reach them.
			v.add(at, "loader variable %q must be a literal", name)
		}
	}
}

func (v *validator) checkRun() {
	d := v.definition
	run := d.Run
	switch {
	case run.Image == "" && run.ImageVariants == nil:
		v.add("/run", "image or imageVariants is required")
	case run.Image != "" && run.ImageVariants != nil:
		v.add("/run", "image and imageVariants are mutually exclusive")
	case run.Image != "":
		if err := CheckImage(run.Image); err != nil {
			v.add("/run/image", "%v", err)
		}
	default:
		v.checkImageVariants(*run.ImageVariants)
	}

	if !path.IsAbs(run.DataPath) || path.Clean(run.DataPath) != run.DataPath || run.DataPath == "/" || overlaps(run.DataPath, EnginePath) {
		v.add("/run/dataPath", "must be a clean absolute path other than / and outside %s", EnginePath)
	}
	v.relPath("/run/workingDir", run.WorkingDir)
	v.checkEnv("/run/env", run.Env)

	if len(run.Tmpfs) > maxTmpfs {
		v.add("/run/tmpfs", "at most %d entries", maxTmpfs)
	}
	seen := map[string]bool{}
	for i, dir := range run.Tmpfs {
		at := fmt.Sprintf("/run/tmpfs/%d", i)
		switch {
		case !path.IsAbs(dir) || path.Clean(dir) != dir || strings.ContainsRune(dir, 0):
			v.add(at, "must be a clean absolute path")
		case dir == "/":
			v.add(at, "must not be /")
		case overlaps(dir, EnginePath):
			v.add(at, "must not overlap %s", EnginePath)
		case overlaps(dir, run.DataPath):
			v.add(at, "must not overlap the data path")
		case seen[dir]:
			v.add(at, "duplicate entry")
		}
		seen[dir] = true
	}

	if len(run.Exec) > 0 && run.Shell != "" {
		v.add("/run", "exec and shell are mutually exclusive")
	}
	for i, item := range run.Exec {
		at := fmt.Sprintf("/run/exec/%d", i)
		if item.Template == nil && item.Arg == "" {
			v.add(at+"/arg", "must not be empty")
		}
		if item.When != nil {
			v.checkCondition(at+"/when", *item.When, 0)
		}
	}

	console := run.Console
	v.oneOf("/run/console/via", console.Via, ConsoleStdin, ConsoleRcon, ConsoleNone)
	if console.Via == ConsoleRcon {
		if console.Port == "" || console.PasswordSetting == "" {
			v.add("/run/console", "rcon needs port and passwordSetting")
		}
		v.requirePort("/run/console/port", console.Port)
		if console.PasswordSetting != "" {
			v.requireSetting("/run/console/passwordSetting", console.PasswordSetting, SettingHiddenString)
		}
	} else if console.Port != "" || console.PasswordSetting != "" {
		v.add("/run/console", "port and passwordSetting apply to rcon only")
	}

	stop := run.Stop
	v.oneOf("/run/stop/via", stop.Via, StopSignal, StopCommand)
	v.oneOf("/run/stop/signal", stop.Signal, "SIGTERM", "SIGINT", "SIGQUIT", "SIGHUP", "SIGKILL")
	if stop.Via == StopCommand {
		if stop.Command == "" {
			v.add("/run/stop/command", "required for stop via command")
		}
		if console.Via == ConsoleNone {
			v.add("/run/stop", "stop via command needs a console channel (console.via stdin or rcon)")
		}
	}
	if stop.TimeoutSeconds < 1 || stop.TimeoutSeconds > 600 {
		v.add("/run/stop/timeoutSeconds", "must be between 1 and 600")
	}
	if run.Ready != nil {
		v.pattern("/run/ready/pattern", run.Ready.Pattern, true)
	}
}

// overlaps reports whether one clean absolute path contains the other.
func overlaps(a string, b string) bool {
	return a == b || strings.HasPrefix(a, b+"/") || strings.HasPrefix(b, a+"/")
}

func (v *validator) checkImageVariants(variants ImageVariants) {
	at := "/run/imageVariants"
	setting := v.requireSetting(at+"/setting", variants.Setting, SettingStringEnum)
	if setting == nil {
		return
	}
	if setting.ValuesSource != "" {
		v.add(at+"/setting", "the setting must declare its values (no valuesSource)")
	}
	var values []string
	for _, value := range setting.Values {
		values = append(values, value.Value)
	}
	slices.Sort(values)
	if keys := sortedKeys(variants.Images); !slices.Equal(keys, values) {
		v.add(at+"/images", "keys %v must equal the values of setting %q %v", keys, variants.Setting, values)
	}
	for _, key := range sortedKeys(variants.Images) {
		if err := CheckImage(variants.Images[key]); err != nil {
			v.add(at+"/images/"+key, "%v", err)
		}
	}
}

func (v *validator) pattern(at string, pattern string, required bool) {
	if pattern == "" {
		if required {
			v.add(at, "must not be empty")
		}
		return
	}
	if _, err := regexp.Compile(pattern); err != nil {
		v.add(at, "invalid regular expression: %v", err)
	}
}

func (v *validator) checkInstall() {
	d := v.definition
	hostLogin := d.Workshop != nil && d.Workshop.Login == LoginHost
	hasScript := false
	for i, stage := range d.Install {
		at := fmt.Sprintf("/install/%d", i)
		if stage.When != nil {
			v.checkCondition(at+"/when", *stage.When, 0)
		}
		v.checkEnv(at+"/env", stage.Env)
		steamcmd := stage.AppID != 0 || stage.Login != "" || stage.Platform != "" || stage.Branch != "" || stage.Beta != nil || stage.SteamSDK || stage.Dir != "" || len(stage.ExtraArgs) > 0
		download := stage.URL != "" || stage.Checksum != "" || stage.Dest != "" || stage.Extract != nil
		script := stage.Image != "" || stage.Shell != ""
		foreign := map[string]bool{StageSteamcmd: download || script, StageDownload: steamcmd || script, StageScript: steamcmd || download}

		switch stage.Kind {
		case StageSteamcmd:
			hostLogin = hostLogin || stage.Login == LoginHost
			v.checkSteamcmd(at, stage)
		case StageDownload:
			if !strings.HasPrefix(stage.URL, "https://") || strings.ContainsAny(stage.URL, " \t\r\n{}") {
				v.add(at+"/url", "must be a literal https URL")
			}
			v.literal(at+"/url", stage.URL)
			if !checksumPattern.MatchString(stage.Checksum) {
				v.add(at+"/checksum", "required, sha256:<hex>")
			}
			v.relPath(at+"/dest", stage.Dest)
			if stage.Extract != nil {
				if stage.Extract.Format != "" {
					v.oneOf(at+"/extract/format", stage.Extract.Format, "none", "zip", "tar", "tar.gz", "tar.xz")
				}
				if stage.Extract.StripComponents < 0 || stage.Extract.StripComponents > 8 {
					v.add(at+"/extract/stripComponents", "must be between 0 and 8")
				}
			}
		case StageScript:
			hasScript = true
			if strings.TrimSpace(stage.Shell) == "" {
				v.add(at+"/shell", "required")
			}
			if stage.Image != "" {
				if err := CheckImage(stage.Image); err != nil {
					v.add(at+"/image", "%v", err)
				}
			}
		default:
			v.add(at+"/kind", "must be one of steamcmd, download, script, got %q", stage.Kind)
			continue
		}
		if foreign[stage.Kind] {
			v.add(at, "contains fields of another stage kind")
		}
		if stage.Kind != StageScript && len(stage.Env) > 0 {
			v.add(at+"/env", "only script stages take env")
		}
	}
	// Host Steam credentials must never share an install with customer-controlled scripts.
	if hostLogin && hasScript {
		v.add("/install", "script stages are not allowed when a steamcmd stage or the workshop uses login: host")
	}
}

func (v *validator) checkSteamcmd(at string, stage InstallStage) {
	if stage.AppID < 1 {
		v.add(at+"/appId", "required")
	}
	v.oneOf(at+"/login", stage.Login, LoginAnonymous, LoginHost)
	if stage.Platform != "" {
		v.oneOf(at+"/platform", stage.Platform, "linux", "windows")
	}
	if stage.Branch != "" && stage.Beta != nil {
		v.add(at, "branch and beta are mutually exclusive")
	}
	if stage.Beta != nil {
		v.requireSetting(at+"/beta/branchSetting", stage.Beta.BranchSetting, SettingString, SettingStringEnum)
		if stage.Beta.PasswordSetting != "" {
			v.requireSetting(at+"/beta/passwordSetting", stage.Beta.PasswordSetting, SettingHiddenString)
		}
	}
	v.relPath(at+"/dir", stage.Dir)
	for j, arg := range stage.ExtraArgs {
		if arg == "" || strings.ContainsAny(arg, "{}\"\r\n\x00") {
			v.add(fmt.Sprintf("%s/extraArgs/%d", at, j), "must be a non-empty literal without quotes, braces or line breaks")
		}
	}
}

func (v *validator) checkQuery() {
	d := v.definition
	q := d.Query
	if query, ok := lookupPointer(d.raw, []string{"query"}).(map[string]any); ok && query["protocol"] == nil {
		v.add("/query/protocol", "required")
	}
	v.oneOf("/query/protocol", q.Protocol, QueryA2S, QueryMinecraft, QueryRcon, QueryHTTP, QueryNone)
	allowed := map[string]bool{}
	switch q.Protocol {
	case QueryA2S:
		allowed = map[string]bool{"port": true}
	case QueryMinecraft:
		allowed = map[string]bool{"port": true, "fallbackPort": true}
	case QueryRcon:
		allowed = map[string]bool{"port": true, "passwordSetting": true, "command": true, "pattern": true, "countLinesCommand": true, "countLinesSkip": true}
		if q.PasswordSetting == "" {
			v.add("/query/passwordSetting", "required for rcon")
		}
		if q.Command == "" && q.CountLinesCommand == "" {
			v.add("/query", "rcon query needs command or countLinesCommand")
		}
	case QueryHTTP:
		allowed = map[string]bool{"port": true, "path": true, "scheme": true, "insecure": true, "paths": true}
		if !strings.HasPrefix(q.Path, "/") || strings.ContainsAny(q.Path, " \t\r\n#{}") {
			v.add("/query/path", "must be a literal absolute URL path")
		}
		v.literal("/query/path", q.Path)
		v.oneOf("/query/scheme", q.Scheme, "http", "https")
		if q.Insecure && q.Scheme != "https" {
			v.add("/query/insecure", "applies to scheme https only")
		}
	}
	if q.Protocol != QueryNone && q.Port == "" {
		v.add("/query/port", "required for protocol %s", q.Protocol)
	}
	set := map[string]bool{
		"port": q.Port != "", "fallbackPort": q.FallbackPort != "", "passwordSetting": q.PasswordSetting != "",
		"command": q.Command != "", "pattern": q.Pattern != "", "countLinesCommand": q.CountLinesCommand != "",
		"countLinesSkip": q.CountLinesSkip != 0, "path": q.Path != "", "insecure": q.Insecure, "paths": q.Paths != nil,
		"scheme": q.Scheme != "",
	}
	for _, field := range sortedKeys(set) {
		if set[field] && !allowed[field] {
			v.add("/query/"+field, "not allowed for protocol %s", q.Protocol)
		}
	}
	v.requirePort("/query/port", q.Port)
	v.requirePort("/query/fallbackPort", q.FallbackPort)
	if q.PasswordSetting != "" {
		v.requireSetting("/query/passwordSetting", q.PasswordSetting, SettingHiddenString)
	}
	v.pattern("/query/pattern", q.Pattern, false)
	if q.CountLinesSkip < 0 || q.CountLinesSkip > 8 {
		v.add("/query/countLinesSkip", "must be between 0 and 8")
	}
	if d.Players != nil {
		v.pattern("/players/joinPattern", d.Players.JoinPattern, false)
		v.pattern("/players/leavePattern", d.Players.LeavePattern, false)
	}
}

func (v *validator) checkConfig() {
	for i, file := range v.definition.Config {
		at := fmt.Sprintf("/config/%d", i)
		v.relPath(at+"/path", file.Path)
		if file.Path == "" {
			v.add(at+"/path", "required")
		}
		v.relPath(at+"/createFrom", file.CreateFrom)
		v.oneOf(at+"/format", file.Format, FormatProperties, FormatKV, FormatINI, FormatJSON, FormatYAML, FormatLines, FormatXML, FormatTemplate)
		if file.When != nil {
			v.checkCondition(at+"/when", *file.When, 0)
		}
		if file.Delimiter != "" && file.Format != FormatKV {
			v.add(at+"/delimiter", "applies to format kv only")
		}
		if file.Format == FormatTemplate {
			if file.Template == "" || len(file.Values) > 0 {
				v.add(at, "format template needs template and no values")
			}
			continue
		}
		if len(file.Values) == 0 || file.Template != "" {
			v.add(at, "format %s needs values and no template", file.Format)
		}
		for _, key := range sortedKeys(file.Values) {
			if err := checkConfigKey(file.Format, key); err != nil {
				v.add(at+"/values/"+key, "%v", err)
			}
		}
	}
}

// checkConfigKey validates a managed key for its format.
func checkConfigKey(format string, key string) error {
	if strings.TrimSpace(key) == "" || strings.ContainsAny(key, "\r\n\x00") || utf8.RuneCountInString(key) > 256 {
		return fmt.Errorf("invalid key %q", key)
	}
	switch format {
	case FormatXML:
		_, _, err := parseXMLKey(key)
		return err
	case FormatJSON, FormatYAML:
		// Path syntax: dots separate keys, "\." is a literal dot; query characters are reserved.
		if strings.ContainsAny(key, "*?#|") || slices.Contains(splitKeyPath(key), "") {
			return fmt.Errorf("invalid key path %q", key)
		}
	}
	return nil
}

func (v *validator) checkHooks() {
	for name, ops := range map[string][]HookOp{"postInstall": v.definition.Hooks.PostInstall, "preStart": v.definition.Hooks.PreStart} {
		for i, op := range ops {
			at := fmt.Sprintf("/hooks/%s/%d", name, i)
			if op.When != nil {
				v.checkCondition(at+"/when", *op.When, 0)
			}
			count := 0
			for _, set := range []bool{op.CopyIfMissing != nil, op.Symlink != nil, op.Replace != nil, op.EnsureDir != nil, op.WriteFile != nil} {
				if set {
					count++
				}
			}
			if count != 1 {
				v.add(at, "exactly one operation is required")
				continue
			}
			for _, p := range op.paths() {
				v.templatePath(at, p)
			}
			if op.Replace != nil {
				v.pattern(at+"/replace/pattern", op.Replace.Pattern, true)
			}
			if op.WriteFile != nil && op.WriteFile.Mode != "" && !modePattern.MatchString(op.WriteFile.Mode) {
				v.add(at+"/writeFile/mode", "must be an octal mode like 0644")
			}
		}
	}
}

func (v *validator) checkMisc() {
	d := v.definition
	for i, file := range d.ProtectedFiles {
		at := fmt.Sprintf("/protectedFiles/%d", i)
		if file.Path == "" || strings.Contains(file.Path, "..") || strings.ContainsRune(file.Path, 0) {
			v.add(at+"/path", "must be a glob inside the data directory")
		}
		v.literal(at+"/path", file.Path)
		if !file.ReadOnly && !file.Hidden {
			v.add(at, "readOnly or hidden must be true")
		}
	}
	for i, glob := range d.BackupIgnore {
		if glob == "" || strings.Contains(glob, "..") {
			v.add(fmt.Sprintf("/backupIgnore/%d", i), "must be a glob inside the data directory")
		}
		v.literal(fmt.Sprintf("/backupIgnore/%d", i), glob)
	}
	for i, precondition := range d.Preconditions {
		at := fmt.Sprintf("/preconditions/%d", i)
		v.checkCondition(at+"/check", precondition.Check, 0)
		v.requireText(at+"/message", precondition.Message)
		v.oneOf(at+"/severity", precondition.Severity, SeverityError, SeverityWarning)
	}
	for i, setting := range d.Settings {
		if setting.ValuesSource != "" && !valuesSourcePattern.MatchString(setting.ValuesSource) {
			v.add(fmt.Sprintf("/settings/%d/valuesSource", i), "invalid value source name")
		}
	}
	if w := d.Workshop; w != nil {
		if len(w.Settings) == 0 {
			v.add("/workshop/settings", "required")
		}
		for i, key := range w.Settings {
			v.requireSetting(fmt.Sprintf("/workshop/settings/%d", i), key, SettingStringList, SettingString)
		}
		v.oneOf("/workshop/login", w.Login, LoginAnonymous, LoginHost)
		v.relPath("/workshop/dir", w.Dir)
	}
	if d.Flags.ConsoleCommands != nil && *d.Flags.ConsoleCommands && d.Run.Console.Via == ConsoleNone {
		v.add("/flags/consoleCommands", "console commands need a console channel")
	}
}

// checkTemplates compiles every template field with the static whitelist checks.
func (v *validator) checkTemplates() {
	templates := v.definition.templates()
	for _, at := range sortedKeys(templates) {
		text := templates[at]
		if !strings.Contains(text, "{{") {
			continue
		}
		if _, err := compileTemplate(at, text, v.definition); err != nil {
			v.add(at, "%v", err)
			continue
		}
		// Config values must not contain line breaks, so multi-line settings cannot go there.
		if strings.HasPrefix(at, "/config/") && strings.Contains(at, "/values/") {
			for _, key := range templateSettings(text) {
				if v.definition.Setting(key).Type == SettingMultilineString {
					v.add(at, "MULTILINESTRING setting %q cannot be used in config values", key)
				}
			}
		}
	}
}

// templates returns every template field of the definition keyed by its location. All other
// string fields are literals.
func (d *Definition) templates() map[string]string {
	t := map[string]string{}
	add := func(at string, value string) {
		if value != "" {
			t[at] = value
		}
	}
	for name, value := range d.Run.Env {
		add("/run/env/"+name, value)
	}
	for i, item := range d.Run.Exec {
		at := fmt.Sprintf("/run/exec/%d", i)
		if item.Template != nil {
			add(at, *item.Template)
			continue
		}
		add(at+"/arg", item.Arg)
		if item.Value != nil {
			add(at+"/value", *item.Value)
		}
	}
	for i, stage := range d.Install {
		at := fmt.Sprintf("/install/%d", i)
		add(at+"/branch", stage.Branch)
		for name, value := range stage.Env {
			add(at+"/env/"+name, value)
		}
	}
	for i, file := range d.Config {
		at := fmt.Sprintf("/config/%d", i)
		add(at+"/template", file.Template)
		for key, value := range file.Values {
			add(at+"/values/"+key, value)
		}
	}
	for name, ops := range map[string][]HookOp{"postInstall": d.Hooks.PostInstall, "preStart": d.Hooks.PreStart} {
		for i, op := range ops {
			at := fmt.Sprintf("/hooks/%s/%d", name, i)
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
