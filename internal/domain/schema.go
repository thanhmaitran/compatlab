package domain

import (
	"slices"
	"sort"
)

// SchemaKind is the primitive or structural type of a schema.
type SchemaKind string

const (
	KindUnknown SchemaKind = ""
	KindObject  SchemaKind = "object"
	KindArray   SchemaKind = "array"
	KindString  SchemaKind = "string"
	KindInteger SchemaKind = "integer"
	KindNumber  SchemaKind = "number"
	KindBoolean SchemaKind = "boolean"
	KindNull    SchemaKind = "null"
)

// SchemaDefinition is the normalized, reference-resolved form of an OpenAPI
// schema. Enum values are stored in canonical string form. Circular references
// are represented by pointer cycles, so traversals must track visited nodes.
type SchemaDefinition struct {
	Name       string                       `json:"name,omitempty"`
	Type       SchemaKind                   `json:"type,omitempty"`
	Format     string                       `json:"format,omitempty"`
	Nullable   bool                         `json:"nullable,omitempty"`
	Enum       []string                     `json:"enum,omitempty"`
	Required   []string                     `json:"required,omitempty"`
	Properties map[string]*SchemaDefinition `json:"properties,omitempty"`
	Items      *SchemaDefinition            `json:"items,omitempty"`
	OneOf      []*SchemaDefinition          `json:"oneOf,omitempty"`
	AnyOf      []*SchemaDefinition          `json:"anyOf,omitempty"`
	AllOf      []*SchemaDefinition          `json:"allOf,omitempty"`
	Minimum    *float64                     `json:"minimum,omitempty"`
	Maximum    *float64                     `json:"maximum,omitempty"`
	MinLength  *int                         `json:"minLength,omitempty"`
	MaxLength  *int                         `json:"maxLength,omitempty"`
	Pattern    string                       `json:"pattern,omitempty"`
	Ref        string                       `json:"ref,omitempty"`
}

// PropertyNames returns the property names in sorted order so that iteration
// never depends on map ordering.
func (s *SchemaDefinition) PropertyNames() []string {
	if s == nil {
		return nil
	}
	names := make([]string, 0, len(s.Properties))
	for name := range s.Properties {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// IsRequired reports whether the named property is listed as required.
func (s *SchemaDefinition) IsRequired(name string) bool {
	return s != nil && slices.Contains(s.Required, name)
}

// Normalize sorts order-insensitive lists (enum, required) throughout the
// schema graph. It is safe on cyclic graphs.
func (s *SchemaDefinition) Normalize() { s.normalize(map[*SchemaDefinition]bool{}) }

func (s *SchemaDefinition) normalize(seen map[*SchemaDefinition]bool) {
	if s == nil || seen[s] {
		return
	}
	seen[s] = true
	slices.Sort(s.Enum)
	slices.Sort(s.Required)
	for _, p := range s.Properties {
		p.normalize(seen)
	}
	s.Items.normalize(seen)
	for _, group := range [][]*SchemaDefinition{s.OneOf, s.AnyOf, s.AllOf} {
		for _, child := range group {
			child.normalize(seen)
		}
	}
}
