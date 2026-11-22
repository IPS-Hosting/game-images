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
	Short         string             `json:"short"`
	Image         string             `json:"image"`
	DataPath      string             `json:"dataPath"`
	WorkingDir    string             `json:"workingDir"`
	Command       []string           `json:"command,omitempty"`
	Env           map[string]string  `json:"env"`
	Mounts        []PlanMount        `json:"mounts,omitempty"`
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

// PlanMount is a rendered mount; Source is relative to the data directory.
type PlanMount struct {
	Source          string `json:"source"`
	Target          string `json:"target"`
	ReadOnly        bool   `json:"readOnly"`
	CreateIfMissing string `json:"createIfMissing,omitempty"`
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

// PlanQuery is the rendered query configuration.
type PlanQuery struct {
	Query
	PortInfo         *PortInfo `json:"portInfo,omitempty"`
	FallbackPortInfo *PortInfo `json:"fallbackPortInfo,omitempty"`
	Password         string    `json:"password,omitempty"`
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

type renderer struct {
	ctx   *TemplateContext
	cache templateCache
}

func newRenderer(d *Definition, values Values) *renderer {
	return &renderer{ctx: NewTemplateContext(d, values), cache: templateCache{}}
}

func (r *renderer) text(at string, value string) (string, error) {
	rendered, err := r.ctx.render(at, value, r.cache)
	if err != nil {
		return "", fmt.Errorf("%s: %w", at, err)
	}
	return rendered, nil
}

// scalar renders a value whose surrounding whitespace has no meaning (images, URLs,
// paths, branch names) and trims it. Arguments, secrets and config values use text and
// stay verbatim.
func (r *renderer) scalar(at string, value string) (string, error) {
	rendered, err := r.text(at, value)
	return strings.TrimSpace(rendered), err
}

func (r *renderer) when(at string, value string) (bool, error) {
	if value == "" {
		return true, nil
	}
	rendered, err := r.text(at, value)
	if err != nil {
		return false, err
	}
	return truthyString(rendered), nil
}

func (r *renderer) relPath(at string, value string) (string, error) {
	rendered, err := r.scalar(at, value)
	if err != nil {
		return "", err
	}
	cleaned, err := CheckRelativePath(rendered)
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

func (r *renderer) password(key string) (string, error) {
	if key == "" {
		return "", nil
	}
	return r.ctx.SettingValue(key)
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
		"HOME":            d.Runtime.DataPath,
		"IPS_DATA_PATH":   d.Runtime.DataPath,
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
	r := newRenderer(d, values)
	plan := &StartPlan{
		Short:        d.Short,
		DataPath:     d.Runtime.DataPath,
		Env:          baseEnv(d, values),
		Stop:         d.Stop,
		Ready:        d.Ready,
		Players:      d.Players,
		BackupIgnore: d.BackupIgnore,
	}

	image, err := r.scalar("/runtime/image", d.Runtime.Image)
	if err != nil {
		return nil, err
	}
	if err := CheckImage(image); err != nil {
		return nil, fmt.Errorf("/runtime/image: %w", err)
	}
	plan.Image = image

	if err := r.env("/runtime/env", d.Runtime.Env, plan.Env); err != nil {
		return nil, err
	}
	if err := r.env("/start/env", d.Start.Env, plan.Env); err != nil {
		return nil, err
	}

	plan.WorkingDir = d.Runtime.DataPath
	if d.Start.WorkingDir != "" {
		dir, err := r.relPath("/start/workingDir", d.Start.WorkingDir)
		if err != nil {
			return nil, err
		}
		plan.WorkingDir = path.Join(d.Runtime.DataPath, dir)
	}

	switch {
	case d.Start.Shell != "":
		plan.Command = []string{"/bin/sh", "-c", d.Start.Shell}
	case len(d.Start.Args) > 0:
		argv, err := r.args(d.Start.Args)
		if err != nil {
			return nil, err
		}
		plan.Command = argv
	}

	for i, mount := range d.Runtime.Mounts {
		source, err := r.relPath(fmt.Sprintf("/runtime/mounts/%d/source", i), mount.Source)
		if err != nil {
			return nil, err
		}
		plan.Mounts = append(plan.Mounts, PlanMount{Source: source, Target: mount.Target, ReadOnly: mount.IsReadOnly(), CreateIfMissing: mount.CreateIfMissing})
	}

	for i, file := range d.Config {
		rendered, include, err := r.configFile(i, file)
		if err != nil {
			return nil, err
		}
		if include {
			plan.Config = append(plan.Config, rendered)
		}
	}

	plan.PreStart, err = r.hooks("/hooks/preStart", d.Hooks.PreStart)
	if err != nil {
		return nil, err
	}

	plan.Console.Via = d.Console.Via
	if d.Console.Via == ConsoleRcon {
		if plan.Console.Port, err = r.port(d.Console.Port); err != nil {
			return nil, fmt.Errorf("/console/port: %w", err)
		}
		if plan.Console.Password, err = r.password(d.Console.PasswordSetting); err != nil {
			return nil, fmt.Errorf("/console/passwordSetting: %w", err)
		}
	}

	plan.Query.Query = d.Query
	if plan.Query.PortInfo, err = r.port(d.Query.Port); err != nil {
		return nil, fmt.Errorf("/query/port: %w", err)
	}
	if plan.Query.FallbackPortInfo, err = r.port(d.Query.FallbackPort); err != nil {
		return nil, fmt.Errorf("/query/fallbackPort: %w", err)
	}
	if plan.Query.Password, err = r.password(d.Query.PasswordSetting); err != nil {
		return nil, fmt.Errorf("/query/passwordSetting: %w", err)
	}
	if d.Query.URL != "" {
		if plan.Query.URL, err = r.scalar("/query/url", d.Query.URL); err != nil {
			return nil, err
		}
	}

	for i, precondition := range d.Preconditions {
		passed, err := r.when(fmt.Sprintf("/preconditions/%d/check", i), precondition.Check)
		if err != nil {
			return nil, err
		}
		plan.Preconditions = append(plan.Preconditions, PlanPrecondition{Passed: passed, Message: precondition.Message, Severity: precondition.Severity})
	}

	return plan, nil
}

// args renders the start argv (SPEC §5.1):
//   - plain item: always exactly one argv item, also when it renders empty (an empty
//     password stays an empty argument instead of shifting the following flags);
//   - object item: dropped when `when` is falsy or arg renders empty; with a value, the
//     pair is dropped when the value renders empty, otherwise both items are appended.
func (r *renderer) args(items []ArgItem) ([]string, error) {
	argv := make([]string, 0, len(items))
	for i, item := range items {
		at := fmt.Sprintf("/start/args/%d", i)
		if item.Template != nil {
			rendered, err := r.text(at, *item.Template)
			if err != nil {
				return nil, err
			}
			argv = append(argv, rendered)
			continue
		}
		include, err := r.when(at+"/when", item.When)
		if err != nil {
			return nil, err
		}
		if !include {
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
		if value == "" {
			continue
		}
		argv = append(argv, arg, value)
	}
	for i, arg := range argv {
		if strings.ContainsRune(arg, 0) {
			return nil, fmt.Errorf("/start/args: argument %d contains a NUL byte", i)
		}
	}
	return argv, nil
}

func (r *renderer) configFile(index int, file ConfigFile) (PlanConfigFile, bool, error) {
	at := fmt.Sprintf("/config/%d", index)
	include, err := r.when(at+"/when", file.When)
	if err != nil || !include {
		return PlanConfigFile{}, false, err
	}
	filePath, err := r.relPath(at+"/path", file.Path)
	if err != nil {
		return PlanConfigFile{}, false, err
	}
	rendered := PlanConfigFile{Path: filePath, Format: file.Format, Delimiter: file.Delimiter, OnlyIfExists: file.OnlyIfExists}
	if file.CreateFrom != "" {
		if rendered.CreateFrom, err = r.relPath(at+"/createFrom", file.CreateFrom); err != nil {
			return PlanConfigFile{}, false, err
		}
	}
	if file.Format == FormatTemplate {
		if rendered.Content, err = r.text(at+"/template", file.Template); err != nil {
			return PlanConfigFile{}, false, err
		}
		return rendered, true, nil
	}
	rendered.Values = make(map[string]string, len(file.Values))
	for _, key := range sortedKeys(file.Values) {
		value, err := r.text(at+"/values/"+key, file.Values[key])
		if err != nil {
			return PlanConfigFile{}, false, err
		}
		// Line-based formats would let a value inject additional keys.
		if strings.ContainsAny(value, "\r\n\x00") {
			return PlanConfigFile{}, false, fmt.Errorf("%s/values/%s: value must not contain line breaks", at, key)
		}
		rendered.Values[key] = value
	}
	return rendered, true, nil
}

func (r *renderer) hooks(at string, ops []HookOp) ([]PlanHookOp, error) {
	var rendered []PlanHookOp
	for i, op := range ops {
		opAt := fmt.Sprintf("%s/%d", at, i)
		include, err := r.when(opAt+"/when", op.When)
		if err != nil {
			return nil, err
		}
		if !include {
			continue
		}
		var out PlanHookOp
		switch {
		case op.CopyIfMissing != nil:
			from, err := r.relPath(opAt+"/copyIfMissing/from", op.CopyIfMissing.From)
			if err != nil {
				return nil, err
			}
			to, err := r.relPath(opAt+"/copyIfMissing/to", op.CopyIfMissing.To)
			if err != nil {
				return nil, err
			}
			out.CopyIfMissing = &CopyIfMissing{From: from, To: to}
		case op.Symlink != nil:
			target, err := r.relPath(opAt+"/symlink/target", op.Symlink.Target)
			if err != nil {
				return nil, err
			}
			link, err := r.relPath(opAt+"/symlink/link", op.Symlink.Link)
			if err != nil {
				return nil, err
			}
			out.Symlink = &Symlink{Target: target, Link: link}
		case op.Replace != nil:
			filePath, err := r.relPath(opAt+"/replace/path", op.Replace.Path)
			if err != nil {
				return nil, err
			}
			replacement, err := r.text(opAt+"/replace/replacement", op.Replace.Replacement)
			if err != nil {
				return nil, err
			}
			out.Replace = &Replace{Path: filePath, Pattern: op.Replace.Pattern, Replacement: replacement}
		case op.EnsureDir != nil:
			dir, err := r.relPath(opAt+"/ensureDir/path", op.EnsureDir.Path)
			if err != nil {
				return nil, err
			}
			out.EnsureDir = &EnsureDir{Path: dir}
		case op.WriteFile != nil:
			filePath, err := r.relPath(opAt+"/writeFile/path", op.WriteFile.Path)
			if err != nil {
				return nil, err
			}
			content, err := r.text(opAt+"/writeFile/content", op.WriteFile.Content)
			if err != nil {
				return nil, err
			}
			mode := op.WriteFile.Mode
			if mode == "" {
				mode = "0644"
			}
			out.WriteFile = &WriteFile{Path: filePath, Content: content, Mode: mode}
		default:
			return nil, fmt.Errorf("%s: no operation", opAt)
		}
		rendered = append(rendered, out)
	}
	return rendered, nil
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
