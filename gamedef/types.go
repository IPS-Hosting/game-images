// Package gamedef parses, validates and renders IPS Hosting game definitions (apiVersion 1).
//
// The same package is used by the game-images CI (cmd/gamedef), by ipsd (engine) and by
// ips-init (in-container start). Specification: SPEC.md. The JSON schema check lives in the
// subpackage schema so that engines do not link the schema library.
package gamedef

import (
	"bytes"
	"encoding/json"
	"fmt"

	"go.yaml.in/yaml/v3"
)

// APIVersion is the definition format version handled by this package.
const APIVersion = 1

// EngineVersion is the engine feature level implemented by this package. The platform serves
// a definition only to engines with EngineVersion >= MinEngine(definition).
const EngineVersion = 1

// DefaultDataPath is the default mount point of the persistent data volume.
const DefaultDataPath = "/home/ips-hosting"

// Definition is one game definition (game.yaml).
type Definition struct {
	APIVersion     int             `yaml:"apiVersion" json:"apiVersion"`
	Short          string          `yaml:"short" json:"short"`
	Name           I18n            `yaml:"name" json:"name"`
	Description    *I18n           `yaml:"description,omitempty" json:"description,omitempty"`
	SteamAppID     int             `yaml:"steamAppId,omitempty" json:"steamAppId,omitempty"`
	Image          *string         `yaml:"image,omitempty" json:"image,omitempty"`
	Tags           []string        `yaml:"tags,omitempty" json:"tags,omitempty"`
	Sources        []Source        `yaml:"sources,omitempty" json:"sources,omitempty"`
	Run            Run             `yaml:"run" json:"run"`
	Install        []InstallStage  `yaml:"install,omitempty" json:"install,omitempty"`
	Players        *Players        `yaml:"players,omitempty" json:"players,omitempty"`
	Query          Query           `yaml:"query,omitempty" json:"query"`
	Ports          []Port          `yaml:"ports,omitempty" json:"ports,omitempty"`
	Settings       []Setting       `yaml:"settings,omitempty" json:"settings,omitempty"`
	Config         []ConfigFile    `yaml:"config,omitempty" json:"config,omitempty"`
	Hooks          Hooks           `yaml:"hooks,omitempty" json:"hooks"`
	ProtectedFiles []ProtectedFile `yaml:"protectedFiles,omitempty" json:"protectedFiles,omitempty"`
	BackupIgnore   []string        `yaml:"backupIgnore,omitempty" json:"backupIgnore,omitempty"`
	Preconditions  []Precondition  `yaml:"preconditions,omitempty" json:"preconditions,omitempty"`
	Workshop       *Workshop       `yaml:"workshop,omitempty" json:"workshop,omitempty"`
	Flags          Flags           `yaml:"flags,omitempty" json:"flags"`
	Requirements   Requirements    `yaml:"requirements,omitempty" json:"requirements"`

	// raw is the decoded source document, used by MinEngine.
	raw any
}

// I18n is a localized text. In YAML a plain string is shorthand for {de: <string>}.
type I18n struct {
	De string `yaml:"de" json:"de"`
	En string `yaml:"en,omitempty" json:"en,omitempty"`
}

// UnmarshalYAML accepts a plain string or a {de, en} mapping.
func (t *I18n) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.ScalarNode {
		*t = I18n{De: node.Value}
		return nil
	}
	type plain I18n
	var value plain
	if err := decodeStrict(node, &value); err != nil {
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

// Source kinds.
const (
	SourceManual = "manual"
	SourceEgg    = "egg"
	SourceAMP    = "amp"
	SourceLGSM   = "lgsm"
)

// Source records one upstream a definition was derived from.
type Source struct {
	Kind      string `yaml:"kind" json:"kind"`
	Ref       string `yaml:"ref,omitempty" json:"ref,omitempty"`
	License   string `yaml:"license,omitempty" json:"license,omitempty"`
	Converter string `yaml:"converter,omitempty" json:"converter,omitempty"`
	// ImportedSha256 is the sha256 of the unedited converter output (3-way re-imports).
	ImportedSha256 string `yaml:"importedSha256,omitempty" json:"importedSha256,omitempty"`
}

// Run describes the game container and its start.
type Run struct {
	Image         string            `yaml:"image,omitempty" json:"image,omitempty"`
	ImageVariants *ImageVariants    `yaml:"imageVariants,omitempty" json:"imageVariants,omitempty"`
	DataPath      string            `yaml:"dataPath,omitempty" json:"dataPath,omitempty"`
	WorkingDir    string            `yaml:"workingDir,omitempty" json:"workingDir,omitempty"`
	Env           map[string]string `yaml:"env,omitempty" json:"env,omitempty"`
	MachineID     bool              `yaml:"machineId,omitempty" json:"machineId,omitempty"`
	Tmpfs         []string          `yaml:"tmpfs,omitempty" json:"tmpfs,omitempty"`
	Exec          []ArgItem         `yaml:"exec,omitempty" json:"exec,omitempty"`
	Shell         string            `yaml:"shell,omitempty" json:"shell,omitempty"`
	Console       Console           `yaml:"console,omitempty" json:"console"`
	Stop          Stop              `yaml:"stop,omitempty" json:"stop"`
	Ready         *Ready            `yaml:"ready,omitempty" json:"ready,omitempty"`
}

// ImageVariants selects the game image by a STRINGENUM setting.
type ImageVariants struct {
	Setting string            `yaml:"setting" json:"setting"`
	Images  map[string]string `yaml:"images" json:"images"`
}

// Install stage kinds.
const (
	StageSteamcmd = "steamcmd"
	StageDownload = "download"
	StageScript   = "script"
)

// Steam login modes.
const (
	LoginAnonymous = "anonymous"
	LoginHost      = "host"
)

// InstallStage is one install/update stage. Only the fields of its Kind are set.
type InstallStage struct {
	Kind string     `yaml:"kind" json:"kind"`
	When *Condition `yaml:"when,omitempty" json:"when,omitempty"`

	// steamcmd
	AppID     int        `yaml:"appId,omitempty" json:"appId,omitempty"`
	Login     string     `yaml:"login,omitempty" json:"login,omitempty"`
	Platform  string     `yaml:"platform,omitempty" json:"platform,omitempty"`
	Branch    string     `yaml:"branch,omitempty" json:"branch,omitempty"`
	Beta      *SteamBeta `yaml:"beta,omitempty" json:"beta,omitempty"`
	SteamSDK  bool       `yaml:"steamSdk,omitempty" json:"steamSdk,omitempty"`
	Dir       string     `yaml:"dir,omitempty" json:"dir,omitempty"`
	ExtraArgs []string   `yaml:"extraArgs,omitempty" json:"extraArgs,omitempty"`

	// download
	URL      string   `yaml:"url,omitempty" json:"url,omitempty"`
	Checksum string   `yaml:"checksum,omitempty" json:"checksum,omitempty"`
	Dest     string   `yaml:"dest,omitempty" json:"dest,omitempty"`
	Extract  *Extract `yaml:"extract,omitempty" json:"extract,omitempty"`

	// script
	Image string            `yaml:"image,omitempty" json:"image,omitempty"`
	Shell string            `yaml:"shell,omitempty" json:"shell,omitempty"`
	Env   map[string]string `yaml:"env,omitempty" json:"env,omitempty"`
}

// SteamBeta takes the beta branch (and its password) from settings. The branch is used
// unless it is empty or "public".
type SteamBeta struct {
	BranchSetting   string `yaml:"branchSetting" json:"branchSetting"`
	PasswordSetting string `yaml:"passwordSetting,omitempty" json:"passwordSetting,omitempty"`
}

// Extract describes how a download is unpacked.
type Extract struct {
	Format          string `yaml:"format,omitempty" json:"format,omitempty"`
	StripComponents int    `yaml:"stripComponents,omitempty" json:"stripComponents,omitempty"`
}

// ArgItem is one exec argument: a plain template (one argv item, always emitted, also when
// it renders empty) or the object form {arg, value, when} (dropped as a whole when `when` is
// false or arg renders empty; value is appended as a second argv item when non-empty).
type ArgItem struct {
	Template *string
	Arg      string
	Value    *string
	When     *Condition
}

type argObject struct {
	Arg   string     `yaml:"arg" json:"arg"`
	Value *string    `yaml:"value,omitempty" json:"value,omitempty"`
	When  *Condition `yaml:"when,omitempty" json:"when,omitempty"`
}

// UnmarshalYAML accepts a scalar template or the object form.
func (a *ArgItem) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.ScalarNode {
		value := node.Value
		*a = ArgItem{Template: &value}
		return nil
	}
	var object argObject
	if err := decodeStrict(node, &object); err != nil {
		return err
	}
	*a = ArgItem{Arg: object.Arg, Value: object.Value, When: object.When}
	return nil
}

// MarshalJSON keeps the YAML shape.
func (a ArgItem) MarshalJSON() ([]byte, error) {
	if a.Template != nil {
		return json.Marshal(*a.Template)
	}
	return json.Marshal(argObject{Arg: a.Arg, Value: a.Value, When: a.When})
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
	Path              string      `yaml:"path,omitempty" json:"path,omitempty"`
	Scheme            string      `yaml:"scheme,omitempty" json:"scheme,omitempty"`
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
	Group      int    `yaml:"group,omitempty" json:"group,omitempty"`
	Static     bool   `yaml:"static,omitempty" json:"static,omitempty"`
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
	Validation   *Validation    `yaml:"validation,omitempty" json:"validation,omitempty"`
	Generate     *Generate      `yaml:"generate,omitempty" json:"generate,omitempty"`
	Category     string         `yaml:"category,omitempty" json:"category,omitempty"`
	Values       []SettingValue `yaml:"values,omitempty" json:"values,omitempty"`
	ValuesSource string         `yaml:"valuesSource,omitempty" json:"valuesSource,omitempty"`
	VisibleWhen  *Condition     `yaml:"visibleWhen,omitempty" json:"visibleWhen,omitempty"`
	UserEditable *bool          `yaml:"userEditable,omitempty" json:"userEditable,omitempty"`
}

// IsEnum reports whether the setting is an enum type.
func (s Setting) IsEnum() bool {
	return s.Type == SettingStringEnum || s.Type == SettingNumberEnum
}

// IsNumeric reports whether the setting holds a number.
func (s Setting) IsNumeric() bool {
	return s.Type == SettingNumber || s.Type == SettingNumberEnum
}

// SettingValue is one allowed setting value.
type SettingValue struct {
	Value string `yaml:"value" json:"value"`
	Label *I18n  `yaml:"label,omitempty" json:"label,omitempty"`
}

// Validation constrains setting values (see ValidateValue).
type Validation struct {
	Required  bool     `yaml:"required,omitempty" json:"required,omitempty"`
	Min       *float64 `yaml:"min,omitempty" json:"min,omitempty"`
	Max       *float64 `yaml:"max,omitempty" json:"max,omitempty"`
	Integer   bool     `yaml:"integer,omitempty" json:"integer,omitempty"`
	MinLength *int     `yaml:"minLength,omitempty" json:"minLength,omitempty"`
	MaxLength *int     `yaml:"maxLength,omitempty" json:"maxLength,omitempty"`
	Pattern   string   `yaml:"pattern,omitempty" json:"pattern,omitempty"`
}

// Generate charsets.
const (
	CharsetAlnum = "alnum"
	CharsetHex   = "hex"
)

// Generate makes the platform fill a random value at server creation when the value is empty.
type Generate struct {
	Length  int    `yaml:"length" json:"length"`
	Charset string `yaml:"charset,omitempty" json:"charset,omitempty"`
}

// Config file formats.
const (
	FormatProperties = "properties"
	FormatKV         = "kv"
	FormatINI        = "ini"
	FormatJSON       = "json"
	FormatYAML       = "yaml"
	FormatLines      = "lines"
	FormatXML        = "xml"
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
	When         *Condition        `yaml:"when,omitempty" json:"when,omitempty"`
}

// Hooks are declarative file operations.
type Hooks struct {
	PostInstall []HookOp `yaml:"postInstall,omitempty" json:"postInstall,omitempty"`
	PreStart    []HookOp `yaml:"preStart,omitempty" json:"preStart,omitempty"`
}

// HookOp is one file operation. Exactly one operation field is set.
type HookOp struct {
	When          *Condition     `yaml:"when,omitempty" json:"when,omitempty"`
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
	// IfMissing writes the file only when it does not exist yet.
	IfMissing bool `yaml:"ifMissing,omitempty" json:"ifMissing,omitempty"`
}

// ProtectedFile marks files in the data directory as read-only or hidden in the panel file
// manager. Earlier entries take precedence.
type ProtectedFile struct {
	Path     string `yaml:"path" json:"path"`
	ReadOnly bool   `yaml:"readOnly,omitempty" json:"readOnly,omitempty"`
	Hidden   bool   `yaml:"hidden,omitempty" json:"hidden,omitempty"`
}

// Precondition severities.
const (
	SeverityError   = "error"
	SeverityWarning = "warning"
)

// Precondition blocks the start (error) or shows a warning when Check is false.
type Precondition struct {
	Check    Condition `yaml:"check" json:"check"`
	Message  I18n      `yaml:"message" json:"message"`
	Severity string    `yaml:"severity,omitempty" json:"severity"`
}

// Workshop enables Steam Workshop items.
type Workshop struct {
	Settings    []string `yaml:"settings" json:"settings"`
	Collections bool     `yaml:"collections,omitempty" json:"collections,omitempty"`
	AppID       int      `yaml:"appId,omitempty" json:"appId,omitempty"`
	Login       string   `yaml:"login,omitempty" json:"login,omitempty"`
	Dir         string   `yaml:"dir,omitempty" json:"dir,omitempty"`
}

// Flags override the derived panel flags (see EffectiveFlags).
type Flags struct {
	ShowMap         *bool `yaml:"showMap,omitempty" json:"showMap,omitempty"`
	ShowPlayerCount *bool `yaml:"showPlayerCount,omitempty" json:"showPlayerCount,omitempty"`
	SteamConnect    *bool `yaml:"steamConnect,omitempty" json:"steamConnect,omitempty"`
	ConsoleCommands *bool `yaml:"consoleCommands,omitempty" json:"consoleCommands,omitempty"`
}

// Requirements are the minimum resources of a game server.
type Requirements struct {
	MinCPUThreads float64 `yaml:"minCpuThreads,omitempty" json:"minCpuThreads,omitempty"`
	MinRAM        int     `yaml:"minRam,omitempty" json:"minRam,omitempty"`
	MinDiskspace  int     `yaml:"minDiskspace,omitempty" json:"minDiskspace,omitempty"`
}

// applyDefaults fills the documented defaults after decoding.
func (d *Definition) applyDefaults() {
	if d.Run.DataPath == "" {
		d.Run.DataPath = DefaultDataPath
	}
	if d.Run.Console.Via == "" {
		d.Run.Console.Via = ConsoleStdin
	}
	if d.Run.Stop.Via == "" {
		d.Run.Stop.Via = StopSignal
	}
	if d.Run.Stop.Signal == "" {
		d.Run.Stop.Signal = "SIGTERM"
	}
	if d.Run.Stop.TimeoutSeconds == 0 {
		d.Run.Stop.TimeoutSeconds = 10
	}
	if d.Query.Protocol == "" {
		d.Query.Protocol = QueryNone
	}
	if d.Query.Protocol == QueryHTTP && d.Query.Scheme == "" {
		d.Query.Scheme = "http"
	}
	for i := range d.Install {
		if d.Install[i].Kind == StageSteamcmd && d.Install[i].Login == "" {
			d.Install[i].Login = LoginAnonymous
		}
	}
	if d.Workshop != nil && d.Workshop.Login == "" {
		d.Workshop.Login = LoginAnonymous
	}
	for i := range d.Settings {
		if d.Settings[i].Category == "" {
			d.Settings[i].Category = CategoryGeneral
		}
		if g := d.Settings[i].Generate; g != nil && g.Charset == "" {
			g.Charset = CharsetAlnum
		}
	}
	for i := range d.Preconditions {
		if d.Preconditions[i].Severity == "" {
			d.Preconditions[i].Severity = SeverityError
		}
	}
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

// HasSetting reports whether the definition declares the setting key.
func (d *Definition) HasSetting(key string) bool {
	return d.Setting(key) != nil
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

// Images returns every image reference of the definition (run image, image variants and
// script stage images; the default installer image is not included).
func (d *Definition) Images() []string {
	var images []string
	if d.Run.Image != "" {
		images = append(images, d.Run.Image)
	}
	if d.Run.ImageVariants != nil {
		for _, key := range sortedKeys(d.Run.ImageVariants.Images) {
			images = append(images, d.Run.ImageVariants.Images[key])
		}
	}
	for _, stage := range d.Install {
		if stage.Image != "" {
			images = append(images, stage.Image)
		}
	}
	return images
}

// decodeStrict decodes a node with unknown fields rejected. yaml.Node.Decode does not
// inherit KnownFields from the outer decoder, so custom unmarshalers use this instead.
func decodeStrict(node *yaml.Node, out any) error {
	data, err := yaml.Marshal(node)
	if err != nil {
		return err
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	return decoder.Decode(out)
}

// Error is one validation problem with a JSON pointer location.
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
