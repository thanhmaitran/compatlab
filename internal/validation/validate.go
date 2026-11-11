// Package validation checks JSON values and raw parameter strings against a
// normalized schema. It is used by the mock server to validate requests and by
// the test runner to validate responses.
package validation

import (
	"fmt"
	"math"
	"regexp"
	"slices"
	"strconv"
	"unicode/utf8"

	"github.com/thanhmaitran/compatlab/internal/domain"
)

const (
	maxDepth        = 32
	maxArrayChecked = 100
)

// Value validates a decoded JSON value (as produced by encoding/json) and
// returns one message per violation. An empty result means the value is valid.
func Value(v any, s *domain.SchemaDefinition) []string { return check(v, s, "value", 0) }

// Param validates a raw query, header, path or cookie value. The string is
// converted to the schema type first. Array and object parameters are not
// validated.
func Param(raw string, s *domain.SchemaDefinition) []string {
	if s == nil {
		return nil
	}
	var v any = raw
	switch s.Type {
	case domain.KindInteger:
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return []string{fmt.Sprintf("%q is not an integer", raw)}
		}
		v = float64(n)
	case domain.KindNumber:
		f, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return []string{fmt.Sprintf("%q is not a number", raw)}
		}
		v = f
	case domain.KindBoolean:
		b, err := strconv.ParseBool(raw)
		if err != nil {
			return []string{fmt.Sprintf("%q is not a boolean", raw)}
		}
		v = b
	case domain.KindArray, domain.KindObject:
		return nil
	}
	return check(v, s, "parameter", 0)
}

func anyValid(v any, list []*domain.SchemaDefinition, path string, depth int) bool {
	for _, c := range list {
		if len(check(v, c, path, depth+1)) == 0 {
			return true
		}
	}
	return false
}

func check(v any, s *domain.SchemaDefinition, path string, depth int) []string {
	if s == nil || depth > maxDepth {
		return nil
	}
	if v == nil {
		if s.Nullable || s.Type == domain.KindNull || s.Type == domain.KindUnknown {
			return nil
		}
		return []string{path + " must not be null"}
	}
	var out []string
	for _, c := range s.AllOf {
		out = append(out, check(v, c, path, depth+1)...)
	}
	if len(s.AnyOf) > 0 && !anyValid(v, s.AnyOf, path, depth) {
		out = append(out, path+" does not match any allowed schema (anyOf)")
	}
	if len(s.OneOf) > 0 && !anyValid(v, s.OneOf, path, depth) {
		out = append(out, path+" does not match any allowed schema (oneOf)")
	}
	switch s.Type {
	case domain.KindObject:
		obj, ok := v.(map[string]any)
		if !ok {
			return append(out, path+" must be an object")
		}
		for _, name := range s.Required {
			if _, present := obj[name]; !present {
				out = append(out, fmt.Sprintf("%s.%s is required", path, name))
			}
		}
		for _, name := range s.PropertyNames() {
			if val, present := obj[name]; present {
				out = append(out, check(val, s.Properties[name], path+"."+name, depth+1)...)
			}
		}
	case domain.KindArray:
		arr, ok := v.([]any)
		if !ok {
			return append(out, path+" must be an array")
		}
		for i, el := range arr {
			if i >= maxArrayChecked {
				break
			}
			out = append(out, check(el, s.Items, fmt.Sprintf("%s[%d]", path, i), depth+1)...)
		}
	case domain.KindString:
		str, ok := v.(string)
		if !ok {
			return append(out, path+" must be a string")
		}
		n := utf8.RuneCountInString(str)
		if s.MinLength != nil && n < *s.MinLength {
			out = append(out, fmt.Sprintf("%s is shorter than the minimum length %d", path, *s.MinLength))
		}
		if s.MaxLength != nil && n > *s.MaxLength {
			out = append(out, fmt.Sprintf("%s is longer than the maximum length %d", path, *s.MaxLength))
		}
		if s.Pattern != "" {
			if re, err := regexp.Compile(s.Pattern); err == nil && !re.MatchString(str) {
				out = append(out, path+" does not match the required pattern")
			}
		}
	case domain.KindInteger, domain.KindNumber:
		f, ok := v.(float64)
		if !ok || (s.Type == domain.KindInteger && f != math.Trunc(f)) {
			return append(out, fmt.Sprintf("%s must be %s", path, s.Type))
		}
		if s.Minimum != nil && f < *s.Minimum {
			out = append(out, fmt.Sprintf("%s is below the minimum %v", path, *s.Minimum))
		}
		if s.Maximum != nil && f > *s.Maximum {
			out = append(out, fmt.Sprintf("%s is above the maximum %v", path, *s.Maximum))
		}
	case domain.KindBoolean:
		if _, ok := v.(bool); !ok {
			return append(out, path+" must be a boolean")
		}
	}
	if len(s.Enum) > 0 && !slices.Contains(s.Enum, fmt.Sprint(v)) {
		out = append(out, fmt.Sprintf("%s is not one of the allowed values", path))
	}
	return out
}
