package gamedef

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	texttemplate "text/template"
	"text/template/parse"
)

// PortInfo is the allocation of one port identifier, as referenced by templates via
// {{ (.Port "game").Container }}.
type PortInfo struct {
	Container int    `json:"container"`
	Exposed   int    `json:"exposed"`
	Protocol  string `json:"protocol"`
}

// ServerInfo holds the game server facts referenced by templates via {{ .Server.RamMB }}.
type ServerInfo struct {
	ID         int     `json:"id"`
	IPv4       string  `json:"ipv4"`
	CpuThreads float64 `json:"cpuThreads"`
	RamMB      int     `json:"ramMB"`
	DiskGB     int     `json:"diskGB"`
}

// Values are the live inputs of one game server: resolved setting values and ports.
type Values struct {
	// Settings maps setting keys to their current value. Missing keys fall back to the
	// declared default.
	Settings map[string]string `json:"settings"`
	// Ports maps port identifiers to their allocation.
	Ports map[string]PortInfo `json:"ports"`
	// Server holds the game server facts.
	Server ServerInfo `json:"server"`
	// Timezone is the platform timezone (also set as TZ).
	Timezone string `json:"timezone"`
}

// TemplateContext is the restricted render context for definition templates (SPEC §4.1).
// Only the methods and fields defined on it are reachable from templates.
type TemplateContext struct {
	definition *Definition
	values     Values
	Server     ServerInfo
	Timezone   string
}

// NewTemplateContext creates the render context for one game server.
func NewTemplateContext(definition *Definition, values Values) *TemplateContext {
	return &TemplateContext{definition: definition, values: values, Server: values.Server, Timezone: values.Timezone}
}

// SettingValue resolves a setting: current value, then declared default, then "".
func (c *TemplateContext) SettingValue(key string) (string, error) {
	setting := c.definition.Setting(key)
	if setting == nil {
		return "", fmt.Errorf("template references undeclared setting %q", key)
	}
	if value, ok := c.values.Settings[key]; ok {
		return value, nil
	}
	if setting.Default != nil {
		return *setting.Default, nil
	}
	return "", nil
}

// Setting resolves the value of the given setting as string.
func (c *TemplateContext) Setting(key string) (string, error) {
	return c.SettingValue(key)
}

// SettingBool resolves the boolean value of the given setting ("1" and "true" are true).
func (c *TemplateContext) SettingBool(key string) (bool, error) {
	value, err := c.SettingValue(key)
	if err != nil {
		return false, err
	}
	value = strings.TrimSpace(value)
	return value == "1" || strings.EqualFold(value, "true"), nil
}

// SettingInt resolves the integer value of the given setting.
func (c *TemplateContext) SettingInt(key string) (int, error) {
	value, err := c.SettingValue(key)
	if err != nil {
		return 0, err
	}
	intValue, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0, fmt.Errorf("setting %q is not an integer: %q", key, value)
	}
	return intValue, nil
}

// SettingList resolves the given STRINGLIST setting (JSON array) as a string slice.
func (c *TemplateContext) SettingList(key string) ([]string, error) {
	value, err := c.SettingValue(key)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(value) == "" {
		return []string{}, nil
	}
	var list []string
	if err := json.Unmarshal([]byte(value), &list); err != nil {
		return nil, fmt.Errorf("setting %q is not a valid string list: %w", key, err)
	}
	return list, nil
}

// Port resolves the allocation of the given port identifier.
func (c *TemplateContext) Port(identifier string) (PortInfo, error) {
	if !c.definition.HasPort(identifier) {
		return PortInfo{}, fmt.Errorf("template references undeclared port %q", identifier)
	}
	port, ok := c.values.Ports[identifier]
	if !ok {
		return PortInfo{}, fmt.Errorf("port %q is not allocated", identifier)
	}
	return port, nil
}

// contextRoots are the root fields and methods reachable on the template context.
var contextRoots = map[string]struct{}{
	"Setting":     {},
	"SettingBool": {},
	"SettingInt":  {},
	"SettingList": {},
	"Port":        {},
	"Server":      {},
	"Timezone":    {},
}

// compileTemplate parses a template with the function whitelist and runs the static
// checks (only context roots, declared settings and declared ports).
func compileTemplate(name string, text string, definition *Definition) (*texttemplate.Template, error) {
	tmpl, err := texttemplate.New(name).Option("missingkey=error").Funcs(funcMap).Parse(text)
	if err != nil {
		return nil, fmt.Errorf("invalid template: %w", err)
	}
	// define and block create associated templates that the checks below would not see.
	if len(tmpl.Templates()) > 1 || tmpl.Tree == nil {
		return nil, fmt.Errorf("invalid template: define and block are not allowed")
	}

	refs := templateRefs{settings: map[string]struct{}{}, ports: map[string]struct{}{}}
	if err := walkNode(tmpl.Tree.Root, true, &refs); err != nil {
		return nil, fmt.Errorf("invalid template: %w", err)
	}
	for key := range refs.settings {
		if !definition.HasSetting(key) {
			return nil, fmt.Errorf("template references undeclared setting %q", key)
		}
	}
	for identifier := range refs.ports {
		if !definition.HasPort(identifier) {
			return nil, fmt.Errorf("template references undeclared port %q", identifier)
		}
	}
	return tmpl, nil
}

// templateCache avoids re-parsing the same template text during one render pass.
type templateCache map[string]*texttemplate.Template

// render executes one template. Plain strings without actions are returned unchanged.
func (c *TemplateContext) render(name string, text string, cache templateCache) (string, error) {
	if !strings.Contains(text, "{{") {
		return text, nil
	}
	tmpl, ok := cache[text]
	if !ok {
		var err error
		tmpl, err = compileTemplate(name, text, c.definition)
		if err != nil {
			return "", err
		}
		cache[text] = tmpl
	}
	var out strings.Builder
	if err := tmpl.Execute(&out, c); err != nil {
		return "", fmt.Errorf("render: %w", err)
	}
	return out.String(), nil
}

// templateRefs holds the settings and ports referenced by one template.
type templateRefs struct {
	settings map[string]struct{}
	ports    map[string]struct{}
}

// walkNode walks the template tree and enforces the function whitelist and the restricted
// context. It collects the referenced settings and ports for the declaration checks.
// rootDot reports whether dot is the template context. Inside with and range bodies dot is
// a sub-value (for example a PortInfo) whose fields are not restricted; the context stays
// reachable there only through $, which is checked like dot at the root.
func walkNode(node parse.Node, rootDot bool, refs *templateRefs) error {
	switch n := node.(type) {
	case *parse.ListNode:
		if n == nil {
			return nil
		}
		for _, child := range n.Nodes {
			if err := walkNode(child, rootDot, refs); err != nil {
				return err
			}
		}
	case *parse.ActionNode:
		return walkPipe(n.Pipe, rootDot, refs)
	case *parse.IfNode:
		if err := walkPipe(n.Pipe, rootDot, refs); err != nil {
			return err
		}
		if err := walkNode(n.List, rootDot, refs); err != nil {
			return err
		}
		return walkNode(n.ElseList, rootDot, refs)
	case *parse.RangeNode:
		if !rangesOverSettingList(n.Pipe, rootDot) {
			return fmt.Errorf("range is only allowed over .SettingList")
		}
		return walkScopeNode(&n.BranchNode, rootDot, refs)
	case *parse.WithNode:
		return walkScopeNode(&n.BranchNode, rootDot, refs)
	case *parse.BreakNode, *parse.ContinueNode, *parse.TextNode, *parse.StringNode,
		*parse.NumberNode, *parse.BoolNode, *parse.DotNode, *parse.NilNode, *parse.CommentNode:
	case *parse.VariableNode:
		return checkVariableNode(n)
	case *parse.TemplateNode:
		return fmt.Errorf("template invocations are not allowed")
	case *parse.PipeNode:
		return walkPipe(n, rootDot, refs)
	case *parse.FieldNode:
		return checkFieldNode(n, rootDot)
	case *parse.ChainNode:
		if err := checkChainNode(n, rootDot); err != nil {
			return err
		}
		return walkNode(n.Node, rootDot, refs)
	case *parse.CommandNode:
		return walkCommand(n, rootDot, refs)
	default:
		return fmt.Errorf("unexpected template node %T", node)
	}
	return nil
}

// rangesOverSettingList reports whether a range pipeline is exactly a .SettingList "key" call
// on the template context (bounded by the setting value), optionally with declarations.
func rangesOverSettingList(pipe *parse.PipeNode, rootDot bool) bool {
	if pipe == nil || len(pipe.Cmds) != 1 || len(pipe.Cmds[0].Args) != 2 {
		return false
	}
	if _, ok := pipe.Cmds[0].Args[1].(*parse.StringNode); !ok {
		return false
	}
	switch first := pipe.Cmds[0].Args[0].(type) {
	case *parse.FieldNode:
		return rootDot && len(first.Ident) == 1 && first.Ident[0] == "SettingList"
	case *parse.VariableNode:
		return len(first.Ident) == 2 && first.Ident[0] == "$" && first.Ident[1] == "SettingList"
	}
	return false
}

// walkScopeNode walks with and range. The pipeline and the else branch see the outer dot,
// the body sees the pipeline value.
func walkScopeNode(node *parse.BranchNode, rootDot bool, refs *templateRefs) error {
	if err := walkPipe(node.Pipe, rootDot, refs); err != nil {
		return err
	}
	if err := walkNode(node.List, false, refs); err != nil {
		return err
	}
	return walkNode(node.ElseList, rootDot, refs)
}

func walkPipe(pipe *parse.PipeNode, rootDot bool, refs *templateRefs) error {
	if pipe == nil {
		return nil
	}
	for _, command := range pipe.Cmds {
		if err := walkCommand(command, rootDot, refs); err != nil {
			return err
		}
	}
	return nil
}

func walkCommand(command *parse.CommandNode, rootDot bool, refs *templateRefs) error {
	if command == nil || len(command.Args) == 0 {
		return nil
	}

	// The first argument of a command is a function call, a method call on the context,
	// or a plain value.
	switch first := command.Args[0].(type) {
	case *parse.IdentifierNode:
		if _, allowed := functionWhitelist[first.Ident]; !allowed {
			return fmt.Errorf("template function %q is not allowed", first.Ident)
		}
	case *parse.FieldNode:
		if err := checkFieldNode(first, rootDot); err != nil {
			return err
		}
		if rootDot {
			collectMethodRefs(first.Ident, command.Args, refs)
		}
	case *parse.VariableNode:
		if err := checkVariableNode(first); err != nil {
			return err
		}
		if len(first.Ident) > 1 && first.Ident[0] == "$" {
			collectMethodRefs(first.Ident[1:], command.Args, refs)
		}
	case *parse.PipeNode:
		if err := walkPipe(first, rootDot, refs); err != nil {
			return err
		}
	case *parse.ChainNode:
		if err := checkChainNode(first, rootDot); err != nil {
			return err
		}
		if err := walkNode(first.Node, rootDot, refs); err != nil {
			return err
		}
	}

	for _, arg := range command.Args[1:] {
		if err := walkNode(arg, rootDot, refs); err != nil {
			return err
		}
	}
	return nil
}

// collectMethodRefs extracts the setting and port references of context method calls like
// {{ .Setting "key" }} or {{ (.Port "game").Container }}.
func collectMethodRefs(ident []string, args []parse.Node, refs *templateRefs) {
	if len(ident) == 0 || len(args) < 2 {
		return
	}
	stringNode, ok := args[1].(*parse.StringNode)
	if !ok {
		return
	}
	switch ident[0] {
	case "Setting", "SettingBool", "SettingInt", "SettingList":
		refs.settings[stringNode.Text] = struct{}{}
	case "Port":
		refs.ports[stringNode.Text] = struct{}{}
	}
}

// checkFieldNode enforces that field accesses on the template context use its roots.
// Fields of sub-values (dot inside with and range) are not restricted.
func checkFieldNode(node *parse.FieldNode, rootDot bool) error {
	if !rootDot || len(node.Ident) == 0 {
		return nil
	}
	return checkRoot(node.Ident)
}

// checkVariableNode enforces the context roots for $ (the template context).
func checkVariableNode(node *parse.VariableNode) error {
	if len(node.Ident) < 2 || node.Ident[0] != "$" {
		return nil
	}
	return checkRoot(node.Ident[1:])
}

// checkChainNode enforces the context roots for chains like .Server.ID.
func checkChainNode(node *parse.ChainNode, rootDot bool) error {
	if len(node.Field) == 0 {
		return nil
	}
	if _, isDot := node.Node.(*parse.DotNode); !isDot || !rootDot {
		// Chains on sub-values like (.Port "game").Container are field accesses on the
		// result and cannot be restricted further.
		return nil
	}
	return checkRoot(node.Field)
}

func checkRoot(ident []string) error {
	if _, ok := contextRoots[ident[0]]; !ok {
		return fmt.Errorf("template field %q is not part of the template context", "."+strings.Join(ident, "."))
	}
	return nil
}

// singleReference recognizes a template that consists of exactly one context reference:
// {{ .Setting "k" }}, {{ .SettingBool "k" }}, {{ .SettingInt "k" }}, {{ (.Port "id").Container }}
// (or .Exposed) and {{ .Server.Field }}. It returns the method (Setting, SettingBool,
// SettingInt, Port, Server) and its argument, or "" for anything else.
func singleReference(text string) (string, string) {
	tmpl, err := texttemplate.New("").Funcs(funcMap).Parse(text)
	if err != nil || tmpl.Tree == nil || len(tmpl.Tree.Root.Nodes) != 1 {
		return "", ""
	}
	action, ok := tmpl.Tree.Root.Nodes[0].(*parse.ActionNode)
	if !ok || len(action.Pipe.Decl) > 0 || len(action.Pipe.Cmds) != 1 {
		return "", ""
	}
	args := action.Pipe.Cmds[0].Args
	switch first := args[0].(type) {
	case *parse.FieldNode:
		if len(first.Ident) == 2 && first.Ident[0] == "Server" && len(args) == 1 {
			return "Server", first.Ident[1]
		}
		if len(first.Ident) == 1 && len(args) == 2 {
			if key, ok := args[1].(*parse.StringNode); ok {
				switch first.Ident[0] {
				case "Setting", "SettingBool", "SettingInt":
					return first.Ident[0], key.Text
				}
			}
		}
	case *parse.ChainNode:
		inner, ok := first.Node.(*parse.PipeNode)
		if !ok || len(args) != 1 || len(first.Field) != 1 || (first.Field[0] != "Container" && first.Field[0] != "Exposed") ||
			len(inner.Cmds) != 1 || len(inner.Cmds[0].Args) != 2 {
			return "", ""
		}
		field, isField := inner.Cmds[0].Args[0].(*parse.FieldNode)
		key, isString := inner.Cmds[0].Args[1].(*parse.StringNode)
		if isField && isString && len(field.Ident) == 1 && field.Ident[0] == "Port" {
			return "Port", key.Text
		}
	}
	return "", ""
}

// templateSettings returns the settings a template references (nil if it does not parse).
func templateSettings(text string) []string {
	tmpl, err := texttemplate.New("").Funcs(funcMap).Parse(text)
	if err != nil || tmpl.Tree == nil {
		return nil
	}
	refs := templateRefs{settings: map[string]struct{}{}, ports: map[string]struct{}{}}
	if walkNode(tmpl.Tree.Root, true, &refs) != nil {
		return nil
	}
	return sortedKeys(refs.settings)
}
