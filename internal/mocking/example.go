// Package mocking serves a deterministic mock of an API described by a
// normalized specification. It validates requests, so contract and generated
// tests can run against it without touching a real system.
package mocking

import (
	"math"
	"strconv"
	"strings"

	"github.com/thanhmaitran/compatlab/internal/domain"
)

const (
	maxExampleDepth  = 6
	maxExampleString = 10000
)

// Example builds a deterministic value that satisfies the schema. Properties
// that would recurse into an ancestor are omitted when optional.
func Example(s *domain.SchemaDefinition) any {
	return example(s, map[*domain.SchemaDefinition]bool{}, 0)
}

func example(s *domain.SchemaDefinition, onStack map[*domain.SchemaDefinition]bool, depth int) any {
	if s == nil || depth > maxExampleDepth || onStack[s] {
		return nil
	}
	onStack[s] = true
	defer delete(onStack, s)

	if len(s.Enum) > 0 {
		return enumValue(s)
	}
	if len(s.OneOf) > 0 {
		return example(s.OneOf[0], onStack, depth+1)
	}
	if len(s.AnyOf) > 0 {
		return example(s.AnyOf[0], onStack, depth+1)
	}
	if len(s.AllOf) > 0 {
		merged := map[string]any{}
		for _, c := range s.AllOf {
			if m, ok := example(c, onStack, depth+1).(map[string]any); ok {
				for k, v := range m {
					merged[k] = v
				}
			}
		}
		for k, v := range objectExample(s, onStack, depth) {
			merged[k] = v
		}
		return merged
	}
	switch s.Type {
	case domain.KindObject:
		return objectExample(s, onStack, depth)
	case domain.KindArray:
		item := example(s.Items, onStack, depth+1)
		if item == nil {
			return []any{}
		}
		return []any{item}
	case domain.KindString:
		return stringExample(s)
	case domain.KindInteger:
		return boundedNumber(s, true)
	case domain.KindNumber:
		return boundedNumber(s, false)
	case domain.KindBoolean:
		return true
	case domain.KindNull:
		return nil
	}
	if len(s.Properties) > 0 {
		return objectExample(s, onStack, depth)
	}
	return map[string]any{}
}

func objectExample(s *domain.SchemaDefinition, onStack map[*domain.SchemaDefinition]bool, depth int) map[string]any {
	obj := map[string]any{}
	for _, name := range s.PropertyNames() {
		v := example(s.Properties[name], onStack, depth+1)
		if v == nil && !s.IsRequired(name) {
			continue
		}
		obj[name] = v
	}
	return obj
}

func enumValue(s *domain.SchemaDefinition) any {
	first := s.Enum[0]
	switch s.Type {
	case domain.KindInteger, domain.KindNumber:
		if f, err := strconv.ParseFloat(first, 64); err == nil {
			return f
		}
	case domain.KindBoolean:
		if b, err := strconv.ParseBool(first); err == nil {
			return b
		}
	}
	return first
}

func stringExample(s *domain.SchemaDefinition) string {
	v := "string"
	switch s.Format {
	case "email":
		v = "user@example.com"
	case "date-time":
		v = "2024-01-01T00:00:00Z"
	case "date":
		v = "2024-01-01"
	case "uuid":
		v = "00000000-0000-4000-8000-000000000000"
	case "uri", "url":
		v = "https://example.com/"
	}
	if s.MinLength != nil && *s.MinLength > len(v) && *s.MinLength <= maxExampleString {
		v += strings.Repeat("a", *s.MinLength-len(v))
	}
	if s.MaxLength != nil && *s.MaxLength >= 0 && len(v) > *s.MaxLength {
		v = v[:*s.MaxLength]
	}
	return v
}

func boundedNumber(s *domain.SchemaDefinition, integer bool) float64 {
	v := 0.0
	if s.Minimum != nil && v < *s.Minimum {
		v = *s.Minimum
		if integer {
			v = math.Ceil(v)
		}
	}
	if s.Maximum != nil && v > *s.Maximum {
		v = *s.Maximum
		if integer {
			v = math.Floor(v)
		}
	}
	return v
}
