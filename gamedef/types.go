// Package gamedef parses, validates and renders IPS Hosting game definitions (apiVersion 1).
//
// The same package is used by the game-images CI (cmd/gamedef), by ipsd (engine) and by
// ips-init (in-container start). Specification: SPEC.md.
package gamedef

import (
	"fmt"

	"go.yaml.in/yaml/v3"
)

// APIVersion is the definition format version handled by this package.
const APIVersion = 1

// EngineVersion is the engine feature level implemented by this package. A definition with
// requires.engine greater than EngineVersion cannot be run and must be rejected.
const EngineVersion = 1

// DefaultDataPath is the default mount point of the persistent data volume.
const DefaultDataPath = "/home/ips-hosting"

// Definition is one game definition (game.yaml).
type Definition struct {
	APIVersion      int              `yaml:"apiVersion" json:"apiVersion"`
	Short           string           `yaml:"short" json:"short"`
	Name            I18n             `yaml:"name" json:"name"`
	Description     *I18n            `yaml:"description,omitempty" json:"description,omitempty"`
	SteamAppID      int              `yaml:"steamAppId,omitempty" json:"steamAppId,omitempty"`
	Image           *string          `yaml:"image,omitempty" json:"image,omitempty"`
	Tags            []string         `yaml:"tags,omitempty" json:"tags,omitempty"`
	Source          *Source          `yaml:"source,omitempty" json:"source,omitempty"`
	Requires        Requires         `yaml:"requires" json:"requires"`
	Runtime         Runtime          `yaml:"runtime" json:"runtime"`
	Install         []InstallStage   `yaml:"install,omitempty" json:"install,omitempty"`
	Start           Start            `yaml:"start" json:"start"`
	Console         Console          `yaml:"console,omitempty" json:"console"`
	Stop            Stop             `yaml:"stop,omitempty" json:"stop"`
	Ready           *Ready           `yaml:"ready,omitempty" json:"ready,omitempty"`
	Players         *Players         `yaml:"players,omitempty" json:"players,omitempty"`
	Query           Query            `yaml:"query,omitempty" json:"query"`
	Ports           []Port           `yaml:"ports,omitempty" json:"ports,omitempty"`
	Settings        []Setting        `yaml:"settings,omitempty" json:"settings,omitempty"`
	Config          []ConfigFile     `yaml:"config,omitempty" json:"config,omitempty"`
	Hooks           Hooks            `yaml:"hooks,omitempty" json:"hooks"`
	FilePermissions []FilePermission `yaml:"filePermissions,omitempty" json:"filePermissions,omitempty"`
	BackupIgnore    []string         `yaml:"backupIgnore,omitempty" json:"backupIgnore,omitempty"`
	Preconditions   []Precondition   `yaml:"preconditions,omitempty" json:"preconditions,omitempty"`
	Workshop        *Workshop        `yaml:"workshop,omitempty" json:"workshop,omitempty"`
	CurseForge      *CurseForge      `yaml:"curseforge,omitempty" json:"curseforge,omitempty"`
	Flags           Flags            `yaml:"flags,omitempty" json:"flags"`
	Requirements    Requirements     `yaml:"requirements,omitempty" json:"requirements"`
}

// I18n is a localized text. In YAML a plain string is shorthand for {de: <string>}.
type I18n struct {
	De string `yaml:"de" json:"de"`
	En string `yaml:"en,omitempty" json:"en,omitempty"`
}

// UnmarshalYAML accepts a plain string or a {de, en} mapping.
func (t *I18n) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.ScalarNode {
		t.De = node.Value
		t.En = ""
		return nil
	}
	type plain I18n
	var value plain
	if err := node.Decode(&value); err != nil {
		return err
	}
	*t = I18n(value)
	return nil
}

// Text returns the text for the given language, falling back to German.
func (t I18n) Text(lang string) string {
	if lang == "en" && t.En != "" {
		return t.En
	}
	return t.De
}

// Source records where a definition was imported from.
type Source struct {
	From    string `yaml:"from" json:"from"`
	Ref     string `yaml:"ref,omitempty" json:"ref,omitempty"`
	License string `yaml:"license,omitempty" json:"license,omitempty"`
}

// Requires declares the minimum engine version.
type Requires struct {
	Engine int `yaml:"engine" json:"engine"`
}

// Runtime describes the game container.
type Runtime struct {
	Image    string            `yaml:"image" json:"image"`
	DataPath string            `yaml:"dataPath,omitempty" json:"dataPath"`
	Env      map[string]string `yaml:"env,omitempty" json:"env,omitempty"`
	Mounts   []Mount           `yaml:"mounts,omitempty" json:"mounts,omitempty"`
}

// Mount binds a file or directory of the data directory to an absolute container path.
type Mount struct {
	Source          string `yaml:"source" json:"source"`
	Target          string `yaml:"target" json:"target"`
	ReadOnly        *bool  `yaml:"readOnly,omitempty" json:"readOnly,omitempty"`
	CreateIfMissing string `yaml:"createIfMissing,omitempty" json:"createIfMissing,omitempty"`
}

// IsReadOnly reports whether the mount is read-only (default true).
func (m Mount) IsReadOnly() bool {
	return m.ReadOnly == nil || *m.ReadOnly
}

// Install stage kinds.
const (
	StageSteamcmd  = "steamcmd"
	StageDownload  = "download"
	StageScript    = "script"
	StageContainer = "container"
)

// InstallStage is one install/update stage. Only the fields of its Kind are set.
type InstallStage struct {
	Kind string `yaml:"kind" json:"kind"`
	When string `yaml:"when,omitempty" json:"when,omitempty"`

	// steamcmd
	AppID          int      `yaml:"appId,omitempty" json:"appId,omitempty"`
	Login          string   `yaml:"login,omitempty" json:"login,omitempty"`
	Platform       string   `yaml:"platform,omitempty" json:"platform,omitempty"`
	Branch         string   `yaml:"branch,omitempty" json:"branch,omitempty"`
	BranchPassword string   `yaml:"branchPassword,omitempty" json:"branchPassword,omitempty"`
	Dir            string   `yaml:"dir,omitempty" json:"dir,omitempty"`
	ExtraArgs      []string `yaml:"extraArgs,omitempty" json:"extraArgs,omitempty"`

	// download
	URL      string   `yaml:"url,omitempty" json:"url,omitempty"`
	Checksum string   `yaml:"checksum,omitempty" json:"checksum,omitempty"`
	Dest     string   `yaml:"dest,omitempty" json:"dest,omitempty"`
	Extract  *Extract `yaml:"extract,omitempty" json:"extract,omitempty"`

	// script and container
	Image string            `yaml:"image,omitempty" json:"image,omitempty"`
	Shell string            `yaml:"shell,omitempty" json:"shell,omitempty"`
	Args  []string          `yaml:"args,omitempty" json:"args,omitempty"`
	Env   map[string]string `yaml:"env,omitempty" json:"env,omitempty"`
}

// Extract describes how a download is unpacked.
type Extract struct {
	Format          string `yaml:"format,omitempty" json:"format,omitempty"`
	StripComponents int    `yaml:"stripComponents,omitempty" json:"stripComponents,omitempty"`
}

// ArgItem is one start argument: a plain template (one argv item, always emitted, also when
// it renders empty) or the object form {arg, value, when} (dropped as a whole when `when` is
// falsy or arg renders empty; value is appended as a second argv item when non-empty).
type ArgItem struct {
	Template *string
	Arg      string
	Value    *string
	When     string
}

// UnmarshalYAML accepts a scalar template or the object form.
func (a *ArgItem) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.ScalarNode {
		value := node.Value
		*a = ArgItem{Template: &value}
		return nil
	}
	var object struct {
		Arg   string  `yaml:"arg"`
		Value *string `yaml:"value"`
		When  string  `yaml:"when"`
	}
	if err := node.Decode(&object); err != nil {
		return err
	}
	*a = ArgItem{Arg: object.Arg, Value: object.Value, When: object.When}
	return nil
}

// MarshalJSON keeps the YAML shape.
func (a ArgItem) MarshalJSON() ([]byte, error) {
	return marshalArgItem(a)
}

// Start is the start command: args (exec argv, no shell) or shell (sh -c).
type Start struct {
	Args       []ArgItem         `yaml:"args,omitempty" json:"args,omitempty"`
	Shell      string            `yaml:"shell,omitempty" json:"shell,omitempty"`
	Env        map[string]string `yaml:"env,omitempty" json:"env,omitempty"`
	WorkingDir string            `yaml:"workingDir,omitempty" json:"workingDir,omitempty"`
}

// Console channels.
const (
	ConsoleStdin = "stdin"
	ConsoleRcon  = "rcon"
	ConsoleNone  = "none"
)

// Console is the channel for console commands (and command-based stops).
type Console struct {
	Via             string `yaml:"via,omitempty" json:"via"`
	Port            string `yaml:"port,omitempty" json:"port,omitempty"`
	PasswordSetting string `yaml:"passwordSetting,omitempty" json:"passwordSetting,omitempty"`
}

// Stop methods.
const (
	StopSignal  = "signal"
	StopCommand = "command"
)

// Stop describes the graceful stop.
type Stop struct {
	Via            string `yaml:"via,omitempty" json:"via"`
	Signal         string `yaml:"signal,omitempty" json:"signal"`
	Command        string `yaml:"command,omitempty" json:"command,omitempty"`
	TimeoutSeconds int    `yaml:"timeoutSeconds,omitempty" json:"timeoutSeconds"`
}

// Ready marks the server as running once the pattern matches a console line.
type Ready struct {
	Pattern string `yaml:"pattern" json:"pattern"`
}

// Players counts players from console join/leave lines.
type Players struct {
	JoinPattern  string `yaml:"joinPattern,omitempty" json:"joinPattern,omitempty"`
	LeavePattern string `yaml:"leavePattern,omitempty" json:"leavePattern,omitempty"`
}

// Query protocols.
const (
	QueryA2S       = "a2s"
	QueryMinecraft = "minecraft"
	QueryRcon      = "rcon"
	QueryHTTP      = "http"
	QueryNone      = "none"
)

// Query describes how player counts and server names are queried.
type Query struct {
	Protocol          string      `yaml:"protocol,omitempty" json:"protocol"`
	Port              string      `yaml:"port,omitempty" json:"port,omitempty"`
	FallbackPort      string      `yaml:"fallbackPort,omitempty" json:"fallbackPort,omitempty"`
	PasswordSetting   string      `yaml:"passwordSetting,omitempty" json:"passwordSetting,omitempty"`
	Command           string      `yaml:"command,omitempty" json:"command,omitempty"`
	Pattern           string      `yaml:"pattern,omitempty" json:"pattern,omitempty"`
	CountLinesCommand string      `yaml:"countLinesCommand,omitempty" json:"countLinesCommand,omitempty"`
	CountLinesSkip    int         `yaml:"countLinesSkip,omitempty" json:"countLinesSkip,omitempty"`
	URL               string      `yaml:"url,omitempty" json:"url,omitempty"`
	Insecure          bool        `yaml:"insecure,omitempty" json:"insecure,omitempty"`
	Paths             *QueryPaths `yaml:"paths,omitempty" json:"paths,omitempty"`
}

// QueryPaths are dot-paths into a JSON query response.
type QueryPaths struct {
	Name       string `yaml:"name,omitempty" json:"name,omitempty"`
	Map        string `yaml:"map,omitempty" json:"map,omitempty"`
	Players    string `yaml:"players,omitempty" json:"players,omitempty"`
	MaxPlayers string `yaml:"maxPlayers,omitempty" json:"maxPlayers,omitempty"`
}

// Port is one allocated port (maps to GamePort).
type Port struct {
	Identifier string `yaml:"identifier" json:"identifier"`
	Name       *I18n  `yaml:"name,omitempty" json:"name,omitempty"`
	Protocol   string `yaml:"protocol" json:"protocol"`
	Default    int    `yaml:"default" json:"default"`
	Group      int    `yaml:"group,omitempty" json:"group"`
	Static     bool   `yaml:"static,omitempty" json:"static"`
}

// Setting types (identical to the platform GameSettingType enum).
const (
	SettingString          = "STRING"
	SettingHiddenString    = "HIDDENSTRING"
	SettingMultilineString = "MULTILINESTRING"
	SettingNumber          = "NUMBER"
	SettingBoolean         = "BOOLEAN"
	SettingStringEnum      = "STRINGENUM"
	SettingNumberEnum      = "NUMBERENUM"
	SettingStringList      = "STRINGLIST"
)

// Setting is one user-facing setting (maps to GameSetting).
type Setting struct {
	Key          string         `yaml:"key" json:"key"`
	Name         I18n           `yaml:"name" json:"name"`
	Description  *I18n          `yaml:"description,omitempty" json:"description,omitempty"`
	Type         string         `yaml:"type" json:"type"`
	Default      *string        `yaml:"default,omitempty" json:"default,omitempty"`
	Validation   string         `yaml:"validation,omitempty" json:"validation,omitempty"`
	Category     string         `yaml:"category,omitempty" json:"category,omitempty"`
	SortOrder    int            `yaml:"sortOrder,omitempty" json:"sortOrder"`
	Values       []SettingValue `yaml:"values,omitempty" json:"values,omitempty"`
	ValuesSource string         `yaml:"valuesSource,omitempty" json:"valuesSource,omitempty"`
	VisibleWhen  string         `yaml:"visibleWhen,omitempty" json:"visibleWhen,omitempty"`
	UserEditable *bool          `yaml:"userEditable,omitempty" json:"userEditable,omitempty"`
}

// IsEnum reports whether the setting is an enum type.
func (s Setting) IsEnum() bool {
	return s.Type == SettingStringEnum || s.Type == SettingNumberEnum
}

// SettingValue is one allowed or recommended setting value.
type SettingValue struct {
	Value string `yaml:"value" json:"value"`
	Label *I18n  `yaml:"label,omitempty" json:"label,omitempty"`
}

// Config file formats.
const (
	FormatProperties = "properties"
	FormatKV         = "kv"
	FormatINI        = "ini"
	FormatJSON       = "json"
	FormatYAML       = "yaml"
	FormatTemplate   = "template"
)

// ConfigFile is one managed config file.
type ConfigFile struct {
	Path         string            `yaml:"path" json:"path"`
	Format       string            `yaml:"format" json:"format"`
	Values       map[string]string `yaml:"values,omitempty" json:"values,omitempty"`
	Template     string            `yaml:"template,omitempty" json:"template,omitempty"`
	Delimiter    string            `yaml:"delimiter,omitempty" json:"delimiter,omitempty"`
	OnlyIfExists bool              `yaml:"onlyIfExists,omitempty" json:"onlyIfExists,omitempty"`
	CreateFrom   string            `yaml:"createFrom,omitempty" json:"createFrom,omitempty"`
	When         string            `yaml:"when,omitempty" json:"when,omitempty"`
}

// Hooks are declarative file operations.
type Hooks struct {
	PostInstall []HookOp `yaml:"postInstall,omitempty" json:"postInstall,omitempty"`
	PreStart    []HookOp `yaml:"preStart,omitempty" json:"preStart,omitempty"`
}

// HookOp is one file operation. Exactly one operation field is set.
type HookOp struct {
	When          string         `yaml:"when,omitempty" json:"when,omitempty"`
	CopyIfMissing *CopyIfMissing `yaml:"copyIfMissing,omitempty" json:"copyIfMissing,omitempty"`
	Symlink       *Symlink       `yaml:"symlink,omitempty" json:"symlink,omitempty"`
	Replace       *Replace       `yaml:"replace,omitempty" json:"replace,omitempty"`
	EnsureDir     *EnsureDir     `yaml:"ensureDir,omitempty" json:"ensureDir,omitempty"`
	WriteFile     *WriteFile     `yaml:"writeFile,omitempty" json:"writeFile,omitempty"`
}

// CopyIfMissing copies From to To when To does not exist.
type CopyIfMissing struct {
	From string `yaml:"from" json:"from"`
	To   string `yaml:"to" json:"to"`
}

// Symlink creates Link pointing to Target (both relative to the data directory).
type Symlink struct {
	Target string `yaml:"target" json:"target"`
	Link   string `yaml:"link" json:"link"`
}

// Replace applies a regex replacement to a file.
type Replace struct {
	Path        string `yaml:"path" json:"path"`
	Pattern     string `yaml:"pattern" json:"pattern"`
	Replacement string `yaml:"replacement" json:"replacement"`
}

// EnsureDir creates a directory.
type EnsureDir struct {
	Path string `yaml:"path" json:"path"`
}

// WriteFile writes a rendered file (overwrite).
type WriteFile struct {
	Path    string `yaml:"path" json:"path"`
	Content string `yaml:"content" json:"content"`
	Mode    string `yaml:"mode,omitempty" json:"mode,omitempty"`
}

// FilePermission maps to GameFilePermission.
type FilePermission struct {
	Match      string `yaml:"match" json:"match"`
	AllowRead  *bool  `yaml:"allowRead,omitempty" json:"allowRead"`
	AllowWrite *bool  `yaml:"allowWrite,omitempty" json:"allowWrite"`
	Priority   int    `yaml:"priority,omitempty" json:"priority"`
}

// Precondition blocks the start (error) or shows a warning when Check renders falsy.
type Precondition struct {
	Check    string `yaml:"check" json:"check"`
	Message  I18n   `yaml:"message" json:"message"`
	Severity string `yaml:"severity,omitempty" json:"severity"`
}

// Workshop enables Steam Workshop items.
type Workshop struct {
	Settings    []string `yaml:"settings" json:"settings"`
	Collections bool     `yaml:"collections,omitempty" json:"collections,omitempty"`
	AppID       int      `yaml:"appId,omitempty" json:"appId,omitempty"`
	Login       string   `yaml:"login,omitempty" json:"login,omitempty"`
	Dir         string   `yaml:"dir,omitempty" json:"dir,omitempty"`
}

// CurseForge enables CurseForge mod browsing.
type CurseForge struct {
	GameID int `yaml:"gameId" json:"gameId"`
}

// Flags are panel flags.
type Flags struct {
	RenderLogo      bool  `yaml:"renderLogo,omitempty" json:"renderLogo"`
	SteamConnect    bool  `yaml:"steamConnect,omitempty" json:"steamConnect"`
	ShowMap         bool  `yaml:"showMap,omitempty" json:"showMap"`
	ShowPlayerCount *bool `yaml:"showPlayerCount,omitempty" json:"showPlayerCount"`
	Installable     *bool `yaml:"installable,omitempty" json:"installable"`
	ConsoleCommands *bool `yaml:"consoleCommands,omitempty" json:"consoleCommands"`
}

// Requirements are the minimum resources of a game server.
type Requirements struct {
	MinCPUThreads float64 `yaml:"minCpuThreads,omitempty" json:"minCpuThreads,omitempty"`
	MinRAM        int     `yaml:"minRam,omitempty" json:"minRam,omitempty"`
	MinDiskspace  int     `yaml:"minDiskspace,omitempty" json:"minDiskspace,omitempty"`
}

// applyDefaults fills the documented schema defaults after decoding.
func (d *Definition) applyDefaults() {
	if d.Runtime.DataPath == "" {
		d.Runtime.DataPath = DefaultDataPath
	}
	if d.Console.Via == "" {
		d.Console.Via = ConsoleStdin
	}
	if d.Stop.Via == "" {
		d.Stop.Via = StopSignal
	}
	if d.Stop.Signal == "" {
		d.Stop.Signal = "SIGTERM"
	}
	if d.Stop.TimeoutSeconds == 0 {
		d.Stop.TimeoutSeconds = 10
	}
	if d.Query.Protocol == "" {
		d.Query.Protocol = QueryNone
	}
	for i := range d.Install {
		if d.Install[i].Kind == StageSteamcmd && d.Install[i].Login == "" {
			d.Install[i].Login = "anonymous"
		}
	}
	for i := range d.Preconditions {
		if d.Preconditions[i].Severity == "" {
			d.Preconditions[i].Severity = "error"
		}
	}
	if d.Flags.ShowPlayerCount == nil {
		d.Flags.ShowPlayerCount = boolPtr(true)
	}
	if d.Flags.Installable == nil {
		d.Flags.Installable = boolPtr(len(d.Install) > 0)
	}
	if d.Flags.ConsoleCommands == nil {
		d.Flags.ConsoleCommands = boolPtr(d.Console.Via != ConsoleNone)
	}
}

// HasSetting reports whether the definition declares the setting key.
func (d *Definition) HasSetting(key string) bool {
	return d.Setting(key) != nil
}

// Setting returns the declared setting or nil.
func (d *Definition) Setting(key string) *Setting {
	for i := range d.Settings {
		if d.Settings[i].Key == key {
			return &d.Settings[i]
		}
	}
	return nil
}

// HasPort reports whether the definition declares the port identifier.
func (d *Definition) HasPort(identifier string) bool {
	for _, port := range d.Ports {
		if port.Identifier == identifier {
			return true
		}
	}
	return false
}

func boolPtr(value bool) *bool { return &value }

func marshalArgItem(a ArgItem) ([]byte, error) {
	if a.Template != nil {
		return jsonMarshal(*a.Template)
	}
	object := map[string]any{"arg": a.Arg}
	if a.Value != nil {
		object["value"] = *a.Value
	}
	if a.When != "" {
		object["when"] = a.When
	}
	return jsonMarshal(object)
}

// Error is one validation problem with a JSON-pointer-like location.
type Error struct {
	Path    string
	Message string
}

func (e Error) Error() string {
	if e.Path == "" {
		return e.Message
	}
	return fmt.Sprintf("%s: %s", e.Path, e.Message)
}
