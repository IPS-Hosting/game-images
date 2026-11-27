package gamedef

import (
	"fmt"
	"path"
	"sort"
	"strconv"
	"strings"
)

// StartPlan is the fully rendered start of one game server. ipsd creates the container from
// it and hands it to ips-init, which applies the file operations inside the container and
// execs the command. Nothing in a plan is a template anymore.
type StartPlan struct {
	Short      string            `json:"short"`
	Image      string            `json:"image"`
	DataPath   string            `json:"dataPath"`
	WorkingDir string            `json:"workingDir"`
	Command    []string          `json:"command,omitempty"`
	Env        map[string]string `json:"env"`
	// MachineID requests a stable random /etc/machine-id (read-only bind from the private
	// directory, created once by the engine).
	MachineID bool `json:"machineId,omitempty"`
	// Tmpfs lists absolute container paths that get a tmpfs (the root filesystem is read-only).
	Tmpfs         []string           `json:"tmpfs,omitempty"`
	Config        []PlanConfigFile   `json:"config,omitempty"`
	PreStart      []PlanHookOp       `json:"preStart,omitempty"`
	Console       PlanConsole        `json:"console"`
	Stop          Stop               `json:"stop"`
	Ready         *Ready             `json:"ready,omitempty"`
	Players       *Players           `json:"players,omitempty"`
	Query         PlanQuery          `json:"query"`
	Preconditions []PlanPrecondition `json:"preconditions,omitempty"`
	BackupIgnore  []string           `json:"backupIgnore,omitempty"`
}

// PlanConfigFile is a rendered config file operation.
type PlanConfigFile struct {
	Path         string            `json:"path"`
	Format       string            `json:"format"`
	Values       map[string]string `json:"values,omitempty"`
	Content      string            `json:"content,omitempty"`
	Delimiter    string            `json:"delimiter,omitempty"`
	OnlyIfExists bool              `json:"onlyIfExists,omitempty"`
	CreateFrom   string            `json:"createFrom,omitempty"`
	// Types holds the value type (TypeString, TypeNumber, TypeBool) of json and yaml keys whose
	// type is known from the definition; other keys keep the type of the existing value.
	Types map[string]string `json:"types,omitempty"`
}

// PlanHookOp is a rendered hook operation.
type PlanHookOp struct {
	CopyIfMissing *CopyIfMissing `json:"copyIfMissing,omitempty"`
	Symlink       *Symlink       `json:"symlink,omitempty"`
	Replace       *Replace       `json:"replace,omitempty"`
	EnsureDir     *EnsureDir     `json:"ensureDir,omitempty"`
	WriteFile     *WriteFile     `json:"writeFile,omitempty"`
}

// PlanConsole is the rendered console channel.
type PlanConsole struct {
	Via      string    `json:"via"`
	Port     *PortInfo `json:"port,omitempty"`
	Password string    `json:"password,omitempty"`
}

// PlanQuery is the rendered query configuration. For http the engine builds the URL
// scheme://<server IPv4>:<Port.Exposed><Path>; the host is never configurable.
type PlanQuery struct {
	Protocol          string      `json:"protocol"`
	Port              *PortInfo   `json:"port,omitempty"`
	FallbackPort      *PortInfo   `json:"fallbackPort,omitempty"`
	Password          string      `json:"password,omitempty"`
	Command           string      `json:"command,omitempty"`
	Pattern           string      `json:"pattern,omitempty"`
	CountLinesCommand string      `json:"countLinesCommand,omitempty"`
	CountLinesSkip    int         `json:"countLinesSkip,omitempty"`
	Path              string      `json:"path,omitempty"`
	Scheme            string      `json:"scheme,omitempty"`
	Insecure          bool        `json:"insecure,omitempty"`
	Paths             *QueryPaths `json:"paths,omitempty"`
}

// PlanPrecondition is an evaluated precondition.
type PlanPrecondition struct {
	Passed   bool   `json:"passed"`
	Message  I18n   `json:"message"`
	Severity string `json:"severity"`
}

// Blocking returns the failed preconditions with severity error.
func (p *StartPlan) Blocking() []PlanPrecondition {
	var failed []PlanPrecondition
	for _, precondition := range p.Preconditions {
		if !precondition.Passed && precondition.Severity == "error" {
			failed = append(failed, precondition)
		}
	}
	return failed
}

// DefaultValues returns render inputs that use the declared defaults: every setting at its
// default, every port at its default port, and a small reference server. The CI uses it to
// prove that a definition renders.
func DefaultValues(d *Definition) Values {
	ports := make(map[string]PortInfo, len(d.Ports))
	for _, port := range d.Ports {
		ports[port.Identifier] = PortInfo{Container: port.Default, Exposed: port.Default, Protocol: port.Protocol}
	}
	return Values{
		Settings: map[string]string{},
		Ports:    ports,
		Server:   ServerInfo{ID: 1, IPv4: "192.0.2.10", CpuThreads: 2, RamMB: 4096, DiskGB: 20},
		Timezone: "Europe/Berlin",
	}
}

// ResolveSettings returns the value of every declared setting: the current value, else the
// declared default, else "".
func ResolveSettings(d *Definition, values Values) map[string]string {
	resolved := make(map[string]string, len(d.Settings))
	for _, setting := range d.Settings {
		if value, ok := values.Settings[setting.Key]; ok {
			resolved[setting.Key] = value
		} else if setting.Default != nil {
			resolved[setting.Key] = *setting.Default
		} else {
			resolved[setting.Key] = ""
		}
	}
	return resolved
}

// Limits of rendered template output (SPEC §5.2).
const (
	MaxRenderedField = 64 << 10
	MaxRenderedPlan  = 1 << 20
)

type renderer struct {
	definition *Definition
	ctx        *TemplateContext
	settings   map[string]string
	cache      templateCache
	rendered   int
}

// newRenderer checks the supplied setting values (declared keys only, ValidateValue) and
// prepares the render context.
func newRenderer(d *Definition, values Values) (*renderer, error) {
	for _, key := range sortedKeys(values.Settings) {
		setting := d.Setting(key)
		if setting == nil {
			return nil, fmt.Errorf("setting %q is not declared", key)
		}
		if err := ValidateValue(*setting, values.Settings[key]); err != nil {
			return nil, fmt.Errorf("setting %q: %w", key, err)
		}
	}
	return &renderer{definition: d, ctx: NewTemplateContext(d, values), settings: ResolveSettings(d, values), cache: templateCache{}}, nil
}

func (r *renderer) text(at string, value string) (string, error) {
	rendered, err := r.ctx.render(at, value, r.cache)
	if err != nil {
		return "", fmt.Errorf("%s: %w", at, err)
	}
	r.rendered += len(rendered)
	switch {
	case len(rendered) > MaxRenderedField:
		return "", fmt.Errorf("%s: rendered value exceeds %d bytes", at, MaxRenderedField)
	case r.rendered > MaxRenderedPlan:
		return "", fmt.Errorf("%s: rendered plan exceeds %d bytes", at, MaxRenderedPlan)
	}
	return rendered, nil
}

// when evaluates an optional condition (nil is true).
func (r *renderer) when(c *Condition) bool {
	return c == nil || EvalCondition(*c, r.settings)
}

// relPath renders a templated path, trims it and checks it.
func (r *renderer) relPath(at string, value string) (string, error) {
	rendered, err := r.text(at, value)
	if err != nil {
		return "", err
	}
	cleaned, err := CheckRelativePath(strings.TrimSpace(rendered))
	if err != nil {
		return "", fmt.Errorf("%s: %w", at, err)
	}
	return cleaned, nil
}

func (r *renderer) env(at string, env map[string]string, into map[string]string) error {
	for _, name := range sortedKeys(env) {
		value, err := r.text(at+"/"+name, env[name])
		if err != nil {
			return err
		}
		if strings.ContainsRune(value, 0) {
			return fmt.Errorf("%s/%s: value contains a NUL byte", at, name)
		}
		into[name] = value
	}
	return nil
}

func (r *renderer) port(identifier string) (*PortInfo, error) {
	if identifier == "" {
		return nil, nil
	}
	port, err := r.ctx.Port(identifier)
	if err != nil {
		return nil, err
	}
	return &port, nil
}

// portEnvNames returns the environment variables baseEnv generates for a port identifier.
func portEnvNames(identifier string) [2]string {
	name := "IPS_PORT_" + strings.ToUpper(identifier)
	return [2]string{name, name + "_EXPOSED"}
}

// baseEnv returns the engine-provided environment shared by the game and install
// containers.
func baseEnv(d *Definition, values Values) map[string]string {
	env := map[string]string{
		"TZ":              values.Timezone,
		"HOME":            d.Run.DataPath,
		"IPS_DATA_PATH":   d.Run.DataPath,
		"IPS_SERVER_ID":   strconv.Itoa(values.Server.ID),
		"IPS_SERVER_IP":   values.Server.IPv4,
		"IPS_RAM_MB":      strconv.Itoa(values.Server.RamMB),
		"IPS_CPU_THREADS": strconv.FormatFloat(values.Server.CpuThreads, 'f', -1, 64),
	}
	for identifier, port := range values.Ports {
		names := portEnvNames(identifier)
		env[names[0]] = strconv.Itoa(port.Container)
		env[names[1]] = strconv.Itoa(port.Exposed)
	}
	if env["TZ"] == "" {
		env["TZ"] = "UTC"
	}
	return env
}

// Render renders the start plan of one game server.
func Render(d *Definition, values Values) (*StartPlan, error) {
	r, err := newRenderer(d, values)
	if err != nil {
		return nil, err
	}
	run := d.Run
	plan := &StartPlan{
		Short:        d.Short,
		Image:        run.Image,
		DataPath:     run.DataPath,
		WorkingDir:   run.DataPath,
		Env:          baseEnv(d, values),
		MachineID:    run.MachineID,
		Tmpfs:        run.Tmpfs,
		Stop:         run.Stop,
		Ready:        run.Ready,
		Players:      d.Players,
		BackupIgnore: d.BackupIgnore,
	}

	if variants := run.ImageVariants; variants != nil {
		choice := r.settings[variants.Setting]
		image, ok := variants.Images[choice]
		if !ok {
			return nil, fmt.Errorf("/run/imageVariants: no image for %s=%q", variants.Setting, choice)
		}
		plan.Image = image
	}
	if err := r.env("/run/env", run.Env, plan.Env); err != nil {
		return nil, err
	}
	if run.WorkingDir != "" {
		plan.WorkingDir = path.Join(run.DataPath, path.Clean(run.WorkingDir))
	}

	switch {
	case run.Shell != "":
		plan.Command = []string{"/bin/sh", "-c", run.Shell}
	case len(run.Exec) > 0:
		argv, err := r.args(run.Exec)
		if err != nil {
			return nil, err
		}
		plan.Command = argv
	}

	for i, file := range d.Config {
		if !r.when(file.When) {
			continue
		}
		rendered, err := r.configFile(i, file)
		if err != nil {
			return nil, err
		}
		plan.Config = append(plan.Config, rendered)
	}

	if plan.PreStart, err = r.hooks("/hooks/preStart", d.Hooks.PreStart); err != nil {
		return nil, err
	}

	plan.Console.Via = run.Console.Via
	if run.Console.Via == ConsoleRcon {
		if plan.Console.Port, err = r.port(run.Console.Port); err != nil {
			return nil, fmt.Errorf("/run/console/port: %w", err)
		}
		plan.Console.Password = r.settings[run.Console.PasswordSetting]
	}

	q := d.Query
	plan.Query = PlanQuery{
		Protocol: q.Protocol, Password: r.settings[q.PasswordSetting],
		Command: q.Command, Pattern: q.Pattern, CountLinesCommand: q.CountLinesCommand, CountLinesSkip: q.CountLinesSkip,
		Path: q.Path, Scheme: q.Scheme, Insecure: q.Insecure, Paths: q.Paths,
	}
	if plan.Query.Port, err = r.port(q.Port); err != nil {
		return nil, fmt.Errorf("/query/port: %w", err)
	}
	if plan.Query.FallbackPort, err = r.port(q.FallbackPort); err != nil {
		return nil, fmt.Errorf("/query/fallbackPort: %w", err)
	}

	for _, precondition := range d.Preconditions {
		plan.Preconditions = append(plan.Preconditions, PlanPrecondition{
			Passed: EvalCondition(precondition.Check, r.settings), Message: precondition.Message, Severity: precondition.Severity,
		})
	}
	return plan, nil
}

// args renders the exec argv (SPEC §4):
//   - plain item: always exactly one argv item, also when it renders empty (an empty
//     password stays an empty argument instead of shifting the following flags);
//   - object item: dropped when `when` is false or arg renders empty; with a value, the
//     pair is dropped when the value renders empty, otherwise both items are appended.
func (r *renderer) args(items []ArgItem) ([]string, error) {
	argv := make([]string, 0, len(items))
	for i, item := range items {
		at := fmt.Sprintf("/run/exec/%d", i)
		if item.Template != nil {
			rendered, err := r.text(at, *item.Template)
			if err != nil {
				return nil, err
			}
			argv = append(argv, rendered)
			continue
		}
		if !r.when(item.When) {
			continue
		}
		arg, err := r.text(at+"/arg", item.Arg)
		if err != nil {
			return nil, err
		}
		if arg == "" {
			continue
		}
		if item.Value == nil {
			argv = append(argv, arg)
			continue
		}
		value, err := r.text(at+"/value", *item.Value)
		if err != nil {
			return nil, err
		}
		if value != "" {
			argv = append(argv, arg, value)
		}
	}
	for i, arg := range argv {
		if strings.ContainsRune(arg, 0) {
			return nil, fmt.Errorf("/run/exec: argument %d contains a NUL byte", i)
		}
	}
	return argv, nil
}

func (r *renderer) configFile(index int, file ConfigFile) (PlanConfigFile, error) {
	at := fmt.Sprintf("/config/%d", index)
	rendered := PlanConfigFile{Path: path.Clean(file.Path), Format: file.Format, Delimiter: file.Delimiter, OnlyIfExists: file.OnlyIfExists}
	if file.CreateFrom != "" {
		rendered.CreateFrom = path.Clean(file.CreateFrom)
	}
	if file.Format == FormatTemplate {
		content, err := r.text(at+"/template", file.Template)
		rendered.Content = content
		return rendered, err
	}
	rendered.Values = make(map[string]string, len(file.Values))
	for _, key := range sortedKeys(file.Values) {
		value, err := r.text(at+"/values/"+key, file.Values[key])
		if err != nil {
			return PlanConfigFile{}, err
		}
		// Line-based formats would let a value inject additional keys.
		if strings.ContainsAny(value, "\r\n\x00") {
			return PlanConfigFile{}, fmt.Errorf("%s/values/%s: value must not contain line breaks", at, key)
		}
		if err := checkConfigValue(file, key, value); err != nil {
			return PlanConfigFile{}, fmt.Errorf("%s/values/%s: %w", at, key, err)
		}
		rendered.Values[key] = value
		if file.Format == FormatJSON || file.Format == FormatYAML {
			if hint := r.valueType(file.Values[key]); hint != "" {
				if rendered.Types == nil {
					rendered.Types = map[string]string{}
				}
				rendered.Types[key] = hint
			}
		}
	}
	return rendered, nil
}

// checkConfigValue applies the format rules to a rendered value.
func checkConfigValue(file ConfigFile, key string, value string) error {
	switch file.Format {
	case FormatLines:
		// A line must keep its prefix and must not start with a longer managed prefix,
		// otherwise the next start would treat it as another line.
		if !strings.HasPrefix(value, key) {
			return fmt.Errorf("the line must start with its prefix")
		}
		for other := range file.Values {
			if len(other) > len(key) && strings.HasPrefix(value, other) {
				return fmt.Errorf("the line starts with the longer managed prefix %q", other)
			}
		}
	case FormatXML:
		return checkXMLText(value)
	}
	return nil
}

// valueType derives the type hint of a json/yaml value: a literal uses its own syntax, a
// single reference to a setting, port or server fact uses that type, everything else has no
// hint (SPEC §5.4).
func (r *renderer) valueType(text string) string {
	if !strings.Contains(text, "{{") {
		if kind := literalKind(text); kind != "" {
			return kind
		}
		return TypeString
	}
	method, arg := singleReference(text)
	switch method {
	case "Setting":
		setting := r.definition.Setting(arg)
		switch {
		case setting == nil || setting.Type == SettingBoolean:
			return ""
		case setting.IsNumeric():
			return TypeNumber
		}
		return TypeString
	case "SettingInt", "Port":
		return TypeNumber
	case "SettingBool":
		return TypeBool
	case "Server":
		if arg == "IPv4" {
			return TypeString
		}
		return TypeNumber
	}
	return ""
}

func (r *renderer) hooks(at string, ops []HookOp) ([]PlanHookOp, error) {
	var rendered []PlanHookOp
	for i, op := range ops {
		if !r.when(op.When) {
			continue
		}
		out, err := r.hook(fmt.Sprintf("%s/%d", at, i), op)
		if err != nil {
			return nil, err
		}
		rendered = append(rendered, out)
	}
	return rendered, nil
}

func (r *renderer) hook(at string, op HookOp) (PlanHookOp, error) {
	var out PlanHookOp
	var err error
	paths := op.paths()
	rendered := make([]string, len(paths))
	for i, p := range paths {
		if rendered[i], err = r.relPath(fmt.Sprintf("%s/path%d", at, i), p); err != nil {
			return out, err
		}
	}
	switch {
	case op.CopyIfMissing != nil:
		out.CopyIfMissing = &CopyIfMissing{From: rendered[0], To: rendered[1]}
	case op.Symlink != nil:
		out.Symlink = &Symlink{Target: rendered[0], Link: rendered[1]}
	case op.Replace != nil:
		replacement, err := r.text(at+"/replace/replacement", op.Replace.Replacement)
		if err != nil {
			return out, err
		}
		out.Replace = &Replace{Path: rendered[0], Pattern: op.Replace.Pattern, Replacement: replacement}
	case op.EnsureDir != nil:
		out.EnsureDir = &EnsureDir{Path: rendered[0]}
	case op.WriteFile != nil:
		content, err := r.text(at+"/writeFile/content", op.WriteFile.Content)
		if err != nil {
			return out, err
		}
		mode := op.WriteFile.Mode
		if mode == "" {
			mode = "0644"
		}
		out.WriteFile = &WriteFile{Path: rendered[0], Content: content, Mode: mode, IfMissing: op.WriteFile.IfMissing}
	default:
		return out, fmt.Errorf("%s: no operation", at)
	}
	return out, nil
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
