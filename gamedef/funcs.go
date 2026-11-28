package gamedef

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	texttemplate "text/template"

	"errors"
)

// functionWhitelist is the complete set of template function names (SPEC §4.2). Functions
// that exist as text/template builtins (eq, ne, lt, le, gt, ge, printf) are shadowed with
// numeric-tolerant implementations in funcMap; everything else the stdlib provides is
// rejected by the static check.
var functionWhitelist = map[string]struct{}{
	// text/template builtins that are safe and needed for conditions.
	"and": {},
	"or":  {},
	"not": {},
	"len": {},

	"contains":  {},
	"hasPrefix": {},
	"hasSuffix": {},

	"default": {},
	"bool":    {},
	"boolInt": {},
	"ternary": {},

	"eq": {},
	"ne": {},
	"lt": {},
	"le": {},
	"gt": {},
	"ge": {},

	"int":  {},
	"intf": {},

	"add": {},
	"sub": {},
	"mul": {},
	"div": {},

	"max": {},
	"min": {},

	"lower":   {},
	"upper":   {},
	"trim":    {},
	"quote":   {},
	"replace": {},
	"printf":  {},
}

// funcMap is the restricted function whitelist for manifest templates (SPEC §4.2). No
// sprig, no OS access. Builtins are shadowed only where the behavior needs to deviate.
var funcMap = texttemplate.FuncMap{
	"default": defaultFunc,
	"bool":    boolFunc,
	"boolInt": boolIntFunc,
	"ternary": ternaryFunc,

	"eq": eqFunc,
	"ne": neFunc,
	"lt": ltFunc,
	"le": leFunc,
	"gt": gtFunc,
	"ge": geFunc,

	"int":  intFunc,
	"intf": intfFunc,

	"add": addFunc,
	"sub": subFunc,
	"mul": mulFunc,
	"div": divFunc,

	"max": maxFunc,
	"min": minFunc,

	"lower":   lowerFunc,
	"upper":   upperFunc,
	"trim":    trimFunc,
	"quote":   quoteFunc,
	"replace": replaceFunc,
	"printf":  printfFunc,

	"contains":  containsFunc,
	"hasPrefix": hasPrefixFunc,
	"hasSuffix": hasSuffixFunc,
}

// truthy implements the manifest truthiness rule (SPEC §4.3): falsy are empty strings,
// "0", "false" (case-insensitive), false, nil and zero numbers. Everything else is truthy.
func truthy(value any) bool {
	switch v := value.(type) {
	case nil:
		return false
	case bool:
		return v
	case string:
		trimmed := strings.TrimSpace(v)
		return trimmed != "" && trimmed != "0" && !strings.EqualFold(trimmed, "false")
	case *string:
		return v != nil && truthy(*v)
	case []string:
		return len(v) > 0
	default:
		number, _, err := numeric(v)
		if err == nil {
			return number != 0
		}
		return true
	}
}

// defaultFunc returns value when it is truthy, otherwise def. Pipe usage:
// {{ .Setting "key" | default "fallback" }}.
func defaultFunc(def any, value any) any {
	if truthy(value) {
		return value
	}
	return def
}

// boolFunc converts a value to a boolean, rendered as "true"/"false" by the template.
func boolFunc(value any) bool {
	return truthy(value)
}

// boolIntFunc converts a value to 1/0, rendered as "1"/"0" by the template.
func boolIntFunc(value any) int {
	if truthy(value) {
		return 1
	}
	return 0
}

// ternaryFunc returns a when cond is truthy, otherwise b.
func ternaryFunc(a any, b any, cond any) any {
	if truthy(cond) {
		return a
	}
	return b
}

// Comparison helpers: unlike the text/template builtins, they accept mixed int/float
// operands by comparing numerically, and compare strings lexicographically.

func eqFunc(a any, b any) (bool, error) { return compareValues("eq", a, b) }
func neFunc(a any, b any) (bool, error) { return compareValues("ne", a, b) }
func ltFunc(a any, b any) (bool, error) { return compareValues("lt", a, b) }
func leFunc(a any, b any) (bool, error) { return compareValues("le", a, b) }
func gtFunc(a any, b any) (bool, error) { return compareValues("gt", a, b) }
func geFunc(a any, b any) (bool, error) { return compareValues("ge", a, b) }

// compareValues compares two template values for the given operator. Numeric-typed
// operands (including mixed int/float) compare as numbers, string operands compare
// lexicographically, bool operands support equality only. Anything else is an error.
func compareValues(op string, a any, b any) (bool, error) {
	aNumber, _, aErr := numeric(a)
	bNumber, _, bErr := numeric(b)
	if aErr == nil && bErr == nil {
		switch op {
		case "eq":
			return aNumber == bNumber, nil
		case "ne":
			return aNumber != bNumber, nil
		case "lt":
			return aNumber < bNumber, nil
		case "le":
			return aNumber <= bNumber, nil
		case "gt":
			return aNumber > bNumber, nil
		case "ge":
			return aNumber >= bNumber, nil
		}
	}

	aString, aIsString := a.(string)
	bString, bIsString := b.(string)
	if aIsString && bIsString {
		switch op {
		case "eq":
			return aString == bString, nil
		case "ne":
			return aString != bString, nil
		case "lt":
			return aString < bString, nil
		case "le":
			return aString <= bString, nil
		case "gt":
			return aString > bString, nil
		case "ge":
			return aString >= bString, nil
		}
	}

	aBool, aIsBool := a.(bool)
	bBool, bIsBool := b.(bool)
	if aIsBool && bIsBool {
		switch op {
		case "eq":
			return aBool == bBool, nil
		case "ne":
			return aBool != bBool, nil
		}
	}

	return false, fmt.Errorf("cannot compare %v (%T) and %v (%T) with %s", a, a, b, b, op)
}

// numeric converts a template value to a float64 and reports whether the value is an
// integer-typed operand. Integer-typed operands keep the arithmetic int-preserving.
func numeric(value any) (float64, bool, error) {
	switch v := value.(type) {
	case int:
		return float64(v), true, nil
	case int8:
		return float64(v), true, nil
	case int16:
		return float64(v), true, nil
	case int32:
		return float64(v), true, nil
	case int64:
		return float64(v), true, nil
	case uint:
		return float64(v), true, nil
	case uint8:
		return float64(v), true, nil
	case uint16:
		return float64(v), true, nil
	case uint32:
		return float64(v), true, nil
	case uint64:
		return float64(v), true, nil
	case float32:
		return float64(v), false, nil
	case float64:
		return v, false, nil
	default:
		return 0, false, fmt.Errorf("value %v (%T) is not a number", value, value)
	}
}

// numericArgs converts all arguments and reports whether every operand is int-typed.
func numericArgs(args []any) ([]float64, bool, error) {
	values := make([]float64, 0, len(args))
	allInt := true
	for _, arg := range args {
		value, isInt, err := numeric(arg)
		if err != nil {
			return nil, false, err
		}
		if !isInt {
			allInt = false
		}
		values = append(values, value)
	}
	return values, allInt, nil
}

// intResult converts a computed float back to an int-typed result for int-preserving
// arithmetic.
func intResult(value float64) any {
	return int(value)
}

// addFunc adds all operands. The result stays an int when all operands are int-typed.
func addFunc(args ...any) (any, error) {
	values, allInt, err := numericArgs(args)
	if err != nil {
		return nil, err
	}
	var sum float64
	for _, value := range values {
		sum += value
	}
	if allInt {
		return intResult(sum), nil
	}
	return sum, nil
}

// subFunc subtracts all operands from the first one.
func subFunc(args ...any) (any, error) {
	if len(args) < 2 {
		return nil, errors.New("sub requires at least 2 arguments")
	}
	values, allInt, err := numericArgs(args)
	if err != nil {
		return nil, err
	}
	result := values[0]
	for _, value := range values[1:] {
		result -= value
	}
	if allInt {
		return intResult(result), nil
	}
	return result, nil
}

// mulFunc multiplies all operands.
func mulFunc(args ...any) (any, error) {
	if len(args) < 2 {
		return nil, errors.New("mul requires at least 2 arguments")
	}
	values, allInt, err := numericArgs(args)
	if err != nil {
		return nil, err
	}
	result := 1.0
	for _, value := range values {
		result *= value
	}
	if allInt {
		return intResult(result), nil
	}
	return result, nil
}

// divFunc divides the first operand by all following operands.
func divFunc(args ...any) (any, error) {
	if len(args) < 2 {
		return nil, errors.New("div requires at least 2 arguments")
	}
	values, allInt, err := numericArgs(args)
	if err != nil {
		return nil, err
	}
	result := values[0]
	for _, value := range values[1:] {
		if value == 0 {
			return nil, errors.New("division by zero")
		}
		result /= value
	}
	if allInt {
		return intResult(result), nil
	}
	return result, nil
}

// maxFunc returns the largest operand.
func maxFunc(args ...any) (any, error) {
	if len(args) < 2 {
		return nil, errors.New("max requires at least 2 arguments")
	}
	values, allInt, err := numericArgs(args)
	if err != nil {
		return nil, err
	}
	result := values[0]
	for _, value := range values[1:] {
		result = math.Max(result, value)
	}
	if allInt {
		return intResult(result), nil
	}
	return result, nil
}

// minFunc returns the smallest operand.
func minFunc(args ...any) (any, error) {
	if len(args) < 2 {
		return nil, errors.New("min requires at least 2 arguments")
	}
	values, allInt, err := numericArgs(args)
	if err != nil {
		return nil, err
	}
	result := values[0]
	for _, value := range values[1:] {
		result = math.Min(result, value)
	}
	if allInt {
		return intResult(result), nil
	}
	return result, nil
}

// intFunc converts a value to an int (floats are truncated).
func intFunc(value any) (int, error) {
	if text, ok := value.(string); ok {
		parsed, parseErr := strconv.ParseFloat(strings.TrimSpace(text), 64)
		if parseErr != nil {
			return 0, fmt.Errorf("int: %q is not a number", text)
		}
		return int(parsed), nil
	}
	number, _, err := numeric(value)
	if err != nil {
		return 0, fmt.Errorf("int: %w", err)
	}
	return int(number), nil
}

// intfFunc converts a string or number to a float.
func intfFunc(value any) (float64, error) {
	switch v := value.(type) {
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		if err != nil {
			return 0, fmt.Errorf("intf: %q is not a number", v)
		}
		return parsed, nil
	case *string:
		if v == nil {
			return 0, errors.New("intf: value is nil")
		}
		return intfFunc(*v)
	default:
		number, isNumber, err := numeric(value)
		if err != nil {
			return 0, fmt.Errorf("intf: %w", err)
		}
		_ = isNumber
		return number, nil
	}
}

// toString converts text function arguments; non-strings keep their template rendering.
func toString(value any) string {
	if text, ok := value.(string); ok {
		return text
	}
	if text, ok := value.(*string); ok && text != nil {
		return *text
	}
	return fmt.Sprint(value)
}

func lowerFunc(value any) string {
	return strings.ToLower(toString(value))
}

func upperFunc(value any) string {
	return strings.ToUpper(toString(value))
}

func trimFunc(value any) string {
	return strings.TrimSpace(toString(value))
}

// quoteFunc wraps the value in double quotes. No escaping is applied: quoting is the
// template's responsibility, the engine never rewrites rendered values (SPEC §4.4).
func quoteFunc(value any) string {
	return "\"" + toString(value) + "\""
}

// replaceFunc replaces all occurrences of old with new. Pipe usage:
// {{ .Setting "key" | replace "old" "new" }}.
func replaceFunc(old any, new any, value any) string {
	return strings.ReplaceAll(toString(value), toString(old), toString(new))
}

// printfFunc formats like the stdlib printf, except that float operands passed to integer
// verbs are truncated to integers. The manifest arithmetic helpers return floats as soon
// as one operand is float-typed (SPEC §4.2), and the manifest examples rely on
// {{ printf "%dM" (div ...) }} style integer formatting of such float results.
func printfFunc(format any, args ...any) (string, error) {
	return fmt.Sprintf(toString(format), coerceIntegerVerbArgs(toString(format), args)...), nil
}

// integerVerbs are the fmt verbs that require integer operands.
const integerVerbs = "bdoxXc"

// coerceIntegerVerbArgs walks the format string and truncates float arguments to int64
// where the corresponding verb is an integer verb. "*" width and precision arguments are
// respected while mapping verbs to arguments.
func coerceIntegerVerbArgs(format string, args []any) []any {
	coerced := make([]any, len(args))
	copy(coerced, args)

	argIndex := 0
	for i := 0; i < len(format); i++ {
		if format[i] != '%' {
			continue
		}
		if i+1 < len(format) && format[i+1] == '%' {
			i++
			continue
		}
		j := i + 1
		// Flags.
		for j < len(format) && strings.ContainsRune("-+ #0", rune(format[j])) {
			j++
		}
		// Width.
		for j < len(format) && format[j] >= '0' && format[j] <= '9' {
			j++
		}
		if j < len(format) && format[j] == '*' {
			argIndex++
			j++
		}
		// Precision.
		if j < len(format) && format[j] == '.' {
			j++
			for j < len(format) && format[j] >= '0' && format[j] <= '9' {
				j++
			}
			if j < len(format) && format[j] == '*' {
				argIndex++
				j++
			}
		}
		if j >= len(format) {
			break
		}
		verb := format[j]
		if strings.ContainsRune(integerVerbs, rune(verb)) && argIndex < len(args) {
			switch v := args[argIndex].(type) {
			case float32:
				coerced[argIndex] = int64(v)
			case float64:
				coerced[argIndex] = int64(v)
			}
		}
		argIndex++
		i = j
	}

	return coerced
}

// containsFunc reports whether value contains substr. Pipe usage:
// {{ .Setting "name" | contains (.Setting "password") }}.
func containsFunc(substr any, value any) bool {
	return strings.Contains(toString(value), toString(substr))
}

func hasPrefixFunc(prefix any, value any) bool {
	return strings.HasPrefix(toString(value), toString(prefix))
}

func hasSuffixFunc(suffix any, value any) bool {
	return strings.HasSuffix(toString(value), toString(suffix))
}
