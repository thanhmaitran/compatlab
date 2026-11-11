package validation

import (
	"encoding/json"
	"testing"

	"github.com/thanhmaitran/compatlab/internal/domain"
)

func decode(t *testing.T, doc string) any {
	t.Helper()
	var v any
	if err := json.Unmarshal([]byte(doc), &v); err != nil {
		t.Fatal(err)
	}
	return v
}

func userSchema() *domain.SchemaDefinition {
	min, max := 2, 5
	age := 18.0
	return &domain.SchemaDefinition{
		Type: domain.KindObject, Required: []string{"name"},
		Properties: map[string]*domain.SchemaDefinition{
			"name": {Type: domain.KindString, MinLength: &min, MaxLength: &max},
			"age":  {Type: domain.KindInteger, Minimum: &age},
			"role": {Type: domain.KindString, Enum: []string{"admin", "member"}},
			"tags": {Type: domain.KindArray, Items: &domain.SchemaDefinition{Type: domain.KindString}},
		},
	}
}

func TestValueAcceptsValidDocument(t *testing.T) {
	doc := decode(t, `{"name":"bob","age":30,"role":"admin","tags":["a","b"]}`)
	if got := Value(doc, userSchema()); len(got) != 0 {
		t.Errorf("unexpected violations: %v", got)
	}
}

func TestValueViolations(t *testing.T) {
	tests := map[string]string{
		"missing required": `{"age":30}`,
		"wrong type":       `{"name":5}`,
		"too short":        `{"name":"a"}`,
		"too long":         `{"name":"abcdef"}`,
		"below minimum":    `{"name":"bob","age":17}`,
		"not integer":      `{"name":"bob","age":18.5}`,
		"bad enum":         `{"name":"bob","role":"root"}`,
		"bad array item":   `{"name":"bob","tags":[1]}`,
		"null value":       `{"name":null}`,
		"not an object":    `[1]`,
	}
	for name, doc := range tests {
		if got := Value(decode(t, doc), userSchema()); len(got) == 0 {
			t.Errorf("%s: expected a violation for %s", name, doc)
		}
	}
}

func TestNullableAndUntypedSchemas(t *testing.T) {
	if got := Value(nil, &domain.SchemaDefinition{Type: domain.KindString, Nullable: true}); len(got) != 0 {
		t.Errorf("nullable must accept null: %v", got)
	}
	if got := Value("anything", &domain.SchemaDefinition{}); len(got) != 0 {
		t.Errorf("untyped schema must accept any value: %v", got)
	}
	if got := Value("x", nil); len(got) != 0 {
		t.Errorf("nil schema must accept any value: %v", got)
	}
}

func TestCombinators(t *testing.T) {
	str := &domain.SchemaDefinition{Type: domain.KindString}
	num := &domain.SchemaDefinition{Type: domain.KindNumber}
	oneOf := &domain.SchemaDefinition{OneOf: []*domain.SchemaDefinition{str, num}}
	if len(Value("x", oneOf)) != 0 || len(Value(1.0, oneOf)) != 0 {
		t.Error("oneOf must accept either variant")
	}
	if len(Value(true, oneOf)) == 0 {
		t.Error("oneOf must reject values matching no variant")
	}
	allOf := &domain.SchemaDefinition{AllOf: []*domain.SchemaDefinition{str, {Type: domain.KindString, Enum: []string{"a"}}}}
	if len(Value("a", allOf)) != 0 || len(Value("b", allOf)) == 0 {
		t.Error("allOf must require every schema to match")
	}
}

func TestParam(t *testing.T) {
	one := 1.0
	page := &domain.SchemaDefinition{Type: domain.KindInteger, Minimum: &one}
	if len(Param("2", page)) != 0 {
		t.Error("2 must be valid")
	}
	for _, raw := range []string{"abc", "0", "1.5", ""} {
		if len(Param(raw, page)) == 0 {
			t.Errorf("%q must be invalid", raw)
		}
	}
	if len(Param("true", &domain.SchemaDefinition{Type: domain.KindBoolean})) != 0 || len(Param("maybe", &domain.SchemaDefinition{Type: domain.KindBoolean})) == 0 {
		t.Error("boolean parsing wrong")
	}
	if len(Param("x", &domain.SchemaDefinition{Type: domain.KindArray})) != 0 {
		t.Error("array parameters are not validated")
	}
	if len(Param("x", nil)) != 0 {
		t.Error("nil schema must accept")
	}
}
