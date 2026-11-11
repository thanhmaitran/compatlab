// Package benchmarks holds performance benchmarks and the accuracy evaluation
// of the compatibility engine. Datasets are synthesized deterministically.
package benchmarks

import (
	"encoding/json"
	"fmt"
)

func str() map[string]any { return map[string]any{"type": "string"} }

// BuildSpec synthesizes an OpenAPI 3.0 document (JSON) with the given number
// of resources, each having GET/PUT/DELETE operations, a schema with enum and
// numeric constraints, and a referenced nested schema. variant 0 is the base
// version; variant > 0 introduces type changes, removed and added fields and
// removed operations so that diffs are non-trivial.
func BuildSpec(resources, variant int) []byte {
	paths := map[string]any{}
	schemas := map[string]any{}
	for i := 0; i < resources; i++ {
		name := fmt.Sprintf("Resource%d", i)
		detail := fmt.Sprintf("Details%d", i)
		props := map[string]any{
			"id":      str(),
			"name":    map[string]any{"type": "string", "maxLength": 100},
			"count":   map[string]any{"type": "integer", "minimum": 0, "maximum": 1000},
			"status":  map[string]any{"type": "string", "enum": []any{"new", "active", "closed"}},
			"details": map[string]any{"$ref": "#/components/schemas/" + detail},
		}
		required := []any{"id", "name"}
		if variant > 0 && i%3 == 0 {
			props["count"] = str()
			delete(props, "name")
			required = []any{"id"}
		}
		if variant > 0 && i%5 == 0 {
			props["extra"] = str()
		}
		schemas[name] = map[string]any{"type": "object", "required": required, "properties": props}
		schemas[detail] = map[string]any{"type": "object", "properties": map[string]any{
			"note": str(), "tags": map[string]any{"type": "array", "items": str()}}}

		ref := map[string]any{"$ref": "#/components/schemas/" + name}
		okJSON := func(schema any) map[string]any {
			return map[string]any{"description": "ok", "content": map[string]any{"application/json": map[string]any{"schema": schema}}}
		}
		idParam := []any{map[string]any{"name": "id", "in": "path", "required": true, "schema": str()}}
		item := map[string]any{
			"get": map[string]any{"parameters": idParam, "responses": map[string]any{"200": okJSON(ref), "404": map[string]any{"description": "nf"}}},
			"put": map[string]any{
				"parameters":  idParam,
				"requestBody": map[string]any{"required": true, "content": map[string]any{"application/json": map[string]any{"schema": ref}}},
				"responses":   map[string]any{"200": okJSON(ref), "400": map[string]any{"description": "bad"}},
			},
		}
		if variant == 0 || i%7 != 0 {
			item["delete"] = map[string]any{"parameters": idParam, "responses": map[string]any{"204": map[string]any{"description": "gone"}}}
		}
		paths[fmt.Sprintf("/resources%d/{id}", i)] = item
	}
	doc := map[string]any{
		"openapi":    "3.0.3",
		"info":       map[string]any{"title": "Synthetic API", "version": fmt.Sprintf("1.%d.0", variant)},
		"paths":      paths,
		"components": map[string]any{"schemas": schemas},
	}
	data, err := json.Marshal(doc)
	if err != nil {
		panic(err)
	}
	return data
}
