package gamedef

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Category is one setting category of the controlled vocabulary.
type Category struct {
	ID    string `json:"id"`
	Label I18n   `json:"label"`
}

// CategoryGeneral is the default category.
const CategoryGeneral = "general"

// Categories is the setting category vocabulary in panel order. Exported as
// schema/categories.v1.json.
var Categories = []Category{
	{ID: CategoryGeneral, Label: I18n{De: "Allgemein", En: "General"}},
	{ID: "network", Label: I18n{De: "Netzwerk", En: "Network"}},
	{ID: "gameplay", Label: I18n{De: "Spielablauf", En: "Gameplay"}},
	{ID: "world", Label: I18n{De: "Welt", En: "World"}},
	{ID: "admin", Label: I18n{De: "Administration", En: "Administration"}},
	{ID: "mods", Label: I18n{De: "Mods", En: "Mods"}},
	{ID: "steamBeta", Label: I18n{De: "Steam Beta Branch", En: "Steam beta branch"}},
	{ID: "advanced", Label: I18n{De: "Erweitert", En: "Advanced"}},
	{ID: "performance", Label: I18n{De: "Leistung", En: "Performance"}},
}

// CategoriesJSON returns the content of schema/categories.v1.json.
func CategoriesJSON() []byte {
	return marshalGenerated(map[string]any{"categories": Categories})
}

func isCategory(id string) bool {
	for _, category := range Categories {
		if category.ID == id {
			return true
		}
	}
	return false
}

// ValidateValue checks a setting value against the setting type, its enum values and its
// validation rules (SPEC §7.2). An empty value is valid unless validation.required is set;
// the other rules apply to non-empty values only.
func ValidateValue(setting Setting, value string) error {
	if !utf8.ValidString(value) || strings.ContainsRune(value, 0) {
		return errors.New("must be valid UTF-8 without NUL bytes")
	}
	if setting.Type != SettingMultilineString && strings.ContainsAny(value, "\r\n") {
		return errors.New("must not contain line breaks")
	}
	if value == "" {
		if setting.Validation != nil && setting.Validation.Required {
			return errors.New("a value is required")
		}
		return nil
	}

	var number float64
	switch setting.Type {
	case SettingBoolean:
		if value != "0" && value != "1" {
			return errors.New("must be '0' or '1'")
		}
	case SettingNumber, SettingNumberEnum:
		parsed, err := strconv.ParseFloat(value, 64)
		if !numberPattern.MatchString(value) || err != nil {
			return errors.New("must be a decimal number")
		}
		number = parsed
	case SettingStringList:
		var list []string
		if err := json.Unmarshal([]byte(value), &list); err != nil {
			return errors.New("must be a JSON array of strings")
		}
	}
	if setting.IsEnum() && setting.ValuesSource == "" {
		found := false
		for _, candidate := range setting.Values {
			found = found || candidate.Value == value
		}
		if !found {
			return fmt.Errorf("%q is not one of the allowed values", value)
		}
	}

	rules := setting.Validation
	if rules == nil {
		return nil
	}
	if rules.Integer && !integerPattern.MatchString(value) {
		return errors.New("must be an integer")
	}
	if rules.Min != nil && number < *rules.Min {
		return fmt.Errorf("must be at least %s", formatNumber(*rules.Min))
	}
	if rules.Max != nil && number > *rules.Max {
		return fmt.Errorf("must be at most %s", formatNumber(*rules.Max))
	}
	length := utf8.RuneCountInString(value)
	if rules.MinLength != nil && length < *rules.MinLength {
		return fmt.Errorf("must be at least %d characters long", *rules.MinLength)
	}
	if rules.MaxLength != nil && length > *rules.MaxLength {
		return fmt.Errorf("must be at most %d characters long", *rules.MaxLength)
	}
	if rules.Pattern != "" {
		re, err := regexp.Compile(anchored(rules.Pattern))
		if err != nil {
			return fmt.Errorf("invalid pattern: %w", err)
		}
		if !re.MatchString(value) {
			return fmt.Errorf("must match %s", rules.Pattern)
		}
	}
	return nil
}

// Number syntax of NUMBER and NUMBERENUM values (SPEC §7.2).
var (
	numberPattern  = regexp.MustCompile(`^-?(0|[1-9][0-9]*)(\.[0-9]+)?([eE][+-]?[0-9]+)?$`)
	integerPattern = regexp.MustCompile(`^-?(0|[1-9][0-9]*)$`)
)

// anchored makes a validation pattern match the whole value.
func anchored(pattern string) string {
	return "^(?:" + pattern + ")$"
}

func formatNumber(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

// checkValidation checks that the rules fit the setting type.
func (v *validator) checkValidation(at string, setting Setting) {
	rules := setting.Validation
	if rules == nil {
		return
	}
	numeric := setting.IsNumeric()
	if !numeric && (rules.Min != nil || rules.Max != nil || rules.Integer) {
		v.add(at, "min, max and integer apply to NUMBER and NUMBERENUM settings only")
	}
	if numeric && (rules.MinLength != nil || rules.MaxLength != nil || rules.Pattern != "") {
		v.add(at, "minLength, maxLength and pattern apply to string settings only")
	}
	if rules.Min != nil && rules.Max != nil && *rules.Min > *rules.Max {
		v.add(at, "min must not exceed max")
	}
	if (rules.MinLength != nil && *rules.MinLength < 0) || (rules.MaxLength != nil && *rules.MaxLength < 0) {
		v.add(at, "lengths must not be negative")
	}
	if rules.MinLength != nil && rules.MaxLength != nil && *rules.MinLength > *rules.MaxLength {
		v.add(at, "minLength must not exceed maxLength")
	}
	if rules.Pattern != "" {
		if _, err := regexp.Compile(anchored(rules.Pattern)); err != nil {
			v.add(at+"/pattern", "invalid regular expression: %v", err)
		}
	}
}

// checkGenerate checks a generated-secret declaration.
func (v *validator) checkGenerate(at string, setting Setting) {
	g := setting.Generate
	if g == nil {
		return
	}
	if setting.Type != SettingString && setting.Type != SettingHiddenString {
		v.add(at, "only STRING and HIDDENSTRING settings can be generated")
	}
	if setting.Default != nil && *setting.Default != "" {
		v.add(at, "a generated setting must not have a default")
	}
	if g.Length < 4 || g.Length > 256 {
		v.add(at+"/length", "must be between 4 and 256")
	}
	if g.Charset != CharsetAlnum && g.Charset != CharsetHex {
		v.add(at+"/charset", "must be %s or %s", CharsetAlnum, CharsetHex)
	}
	if rules := setting.Validation; rules != nil {
		if (rules.MinLength != nil && g.Length < *rules.MinLength) || (rules.MaxLength != nil && g.Length > *rules.MaxLength) {
			v.add(at+"/length", "violates the setting's minLength/maxLength")
		}
	}
}

// settingKeyPattern matches setting keys.
var settingKeyPattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_.-]{0,127}$`)

var settingTypes = []string{SettingString, SettingHiddenString, SettingMultilineString, SettingNumber, SettingBoolean, SettingStringEnum, SettingNumberEnum, SettingStringList}

func (v *validator) checkSettings() {
	keys := map[string]string{}
	for i, setting := range v.definition.Settings {
		at := fmt.Sprintf("/settings/%d", i)
		if !settingKeyPattern.MatchString(setting.Key) {
			v.add(at+"/key", "invalid setting key %q", setting.Key)
		}
		// Setting keys are compared case-insensitively by the platform database, so keys
		// that differ only in case would collide.
		lower := strings.ToLower(setting.Key)
		if other, ok := keys[lower]; ok {
			v.add(at, "setting key %q collides with %q (keys must be unique case-insensitively)", setting.Key, other)
		}
		keys[lower] = setting.Key
		v.requireText(at+"/name", setting.Name)
		v.oneOf(at+"/type", setting.Type, settingTypes...)
		if !isCategory(setting.Category) {
			v.add(at+"/category", "unknown category %q", setting.Category)
		}

		switch {
		case setting.IsEnum() && len(setting.Values) == 0 && setting.ValuesSource == "":
			v.add(at, "%s setting %q requires values or valuesSource", setting.Type, setting.Key)
		case !setting.IsEnum() && (len(setting.Values) > 0 || setting.ValuesSource != ""):
			v.add(at, "values and valuesSource apply to enum settings only")
		case len(setting.Values) > 0 && setting.ValuesSource != "":
			v.add(at, "values and valuesSource are mutually exclusive")
		}
		seen := map[string]bool{}
		for j, value := range setting.Values {
			if seen[value.Value] {
				v.add(fmt.Sprintf("%s/values/%d", at, j), "duplicate value %q", value.Value)
			}
			seen[value.Value] = true
			if setting.Type == SettingNumberEnum {
				if !numberPattern.MatchString(value.Value) {
					v.add(fmt.Sprintf("%s/values/%d", at, j), "%q is not a number", value.Value)
				}
			}
		}

		v.checkValidation(at+"/validation", setting)
		v.checkGenerate(at+"/generate", setting)
		if setting.VisibleWhen != nil {
			v.checkCondition(at+"/visibleWhen", *setting.VisibleWhen, 0)
		}
		if setting.Default != nil && *setting.Default != "" {
			if err := ValidateValue(setting, *setting.Default); err != nil {
				v.add(at+"/default", "invalid default: %v", err)
			}
		}
	}
}
