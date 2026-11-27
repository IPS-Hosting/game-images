package gamedef

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

// Condition is a structured boolean expression over setting values (SPEC §5.3). Exactly
// one operator is set. Leaf operators compare the value of Setting; All, Any and Not
// combine conditions and take no Setting.
type Condition struct {
	Setting     string      `yaml:"setting,omitempty" json:"setting,omitempty"`
	Equals      *string     `yaml:"equals,omitempty" json:"equals,omitempty"`
	NotEquals   *string     `yaml:"notEquals,omitempty" json:"notEquals,omitempty"`
	In          []string    `yaml:"in,omitempty" json:"in,omitempty"`
	Truthy      *bool       `yaml:"truthy,omitempty" json:"truthy,omitempty"`
	MinLength   *int        `yaml:"minLength,omitempty" json:"minLength,omitempty"`
	MaxLength   *int        `yaml:"maxLength,omitempty" json:"maxLength,omitempty"`
	Matches     *string     `yaml:"matches,omitempty" json:"matches,omitempty"`
	NotContains string      `yaml:"notContains,omitempty" json:"notContains,omitempty"`
	All         []Condition `yaml:"all,omitempty" json:"all,omitempty"`
	Any         []Condition `yaml:"any,omitempty" json:"any,omitempty"`
	Not         *Condition  `yaml:"not,omitempty" json:"not,omitempty"`
}

// maxConditionDepth bounds nesting so that evaluation stays cheap in every implementation.
const maxConditionDepth = 8

// Truthy reports the truthiness of a setting value: trimmed, empty, "0" and "false"
// (case-insensitive) are false, everything else is true.
func Truthy(value string) bool {
	trimmed := strings.TrimSpace(value)
	return trimmed != "" && trimmed != "0" && !strings.EqualFold(trimmed, "false")
}

// EvalCondition evaluates a condition. settings holds the resolved setting values (current
// value, else default); missing keys are "". An invalid matches pattern evaluates to false.
func EvalCondition(c Condition, settings map[string]string) bool {
	switch {
	case c.All != nil:
		for _, child := range c.All {
			if !EvalCondition(child, settings) {
				return false
			}
		}
		return true
	case c.Any != nil:
		for _, child := range c.Any {
			if EvalCondition(child, settings) {
				return true
			}
		}
		return false
	case c.Not != nil:
		return !EvalCondition(*c.Not, settings)
	}

	value := settings[c.Setting]
	switch {
	case c.Equals != nil:
		return value == *c.Equals
	case c.NotEquals != nil:
		return value != *c.NotEquals
	case c.In != nil:
		for _, candidate := range c.In {
			if value == candidate {
				return true
			}
		}
		return false
	case c.Truthy != nil:
		return Truthy(value) == *c.Truthy
	case c.MinLength != nil:
		return utf8.RuneCountInString(value) >= *c.MinLength
	case c.MaxLength != nil:
		return utf8.RuneCountInString(value) <= *c.MaxLength
	case c.Matches != nil:
		re, err := regexp.Compile(*c.Matches)
		return err == nil && re.MatchString(value)
	case c.NotContains != "":
		other := strings.ToLower(settings[c.NotContains])
		return value == "" || !strings.Contains(other, strings.ToLower(value))
	}
	return false
}

// operators returns the names of the operators set on c.
func (c Condition) operators() []string {
	var ops []string
	add := func(set bool, name string) {
		if set {
			ops = append(ops, name)
		}
	}
	add(c.Equals != nil, "equals")
	add(c.NotEquals != nil, "notEquals")
	add(c.In != nil, "in")
	add(c.Truthy != nil, "truthy")
	add(c.MinLength != nil, "minLength")
	add(c.MaxLength != nil, "maxLength")
	add(c.Matches != nil, "matches")
	add(c.NotContains != "", "notContains")
	add(c.All != nil, "all")
	add(c.Any != nil, "any")
	add(c.Not != nil, "not")
	return ops
}

// checkCondition validates the structure and the setting references of a condition.
func (v *validator) checkCondition(at string, c Condition, depth int) {
	if depth > maxConditionDepth {
		v.add(at, "conditions must not be nested deeper than %d levels", maxConditionDepth)
		return
	}
	ops := c.operators()
	if len(ops) != 1 {
		v.add(at, "a condition needs exactly one operator, got %d (%s)", len(ops), strings.Join(ops, ", "))
		return
	}
	switch ops[0] {
	case "all", "any":
		children := c.All
		if ops[0] == "any" {
			children = c.Any
		}
		if c.Setting != "" {
			v.add(at+"/setting", "not allowed with %s", ops[0])
		}
		if len(children) == 0 || len(children) > 16 {
			v.add(at+"/"+ops[0], "must have 1 to 16 items")
		}
		for i, child := range children {
			v.checkCondition(fmt.Sprintf("%s/%s/%d", at, ops[0], i), child, depth+1)
		}
		return
	case "not":
		if c.Setting != "" {
			v.add(at+"/setting", "not allowed with not")
		}
		v.checkCondition(at+"/not", *c.Not, depth+1)
		return
	}

	if c.Setting == "" {
		v.add(at, "%s needs a setting", ops[0])
		return
	}
	v.requireSetting(at+"/setting", c.Setting)
	switch {
	case c.MinLength != nil && *c.MinLength < 0, c.MaxLength != nil && *c.MaxLength < 0:
		v.add(at, "lengths must not be negative")
	case c.In != nil && (len(c.In) == 0 || len(c.In) > 64):
		v.add(at+"/in", "must have 1 to 64 items")
	case c.Matches != nil && utf8.RuneCountInString(*c.Matches) > 512:
		v.add(at+"/matches", "must have at most 512 characters")
	case c.Matches != nil:
		if _, err := regexp.Compile(*c.Matches); err != nil {
			v.add(at+"/matches", "invalid regular expression: %v", err)
		}
	case c.NotContains != "":
		v.requireSetting(at+"/notContains", c.NotContains)
	}
}
