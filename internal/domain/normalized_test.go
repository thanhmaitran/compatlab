package domain

import (
	"reflect"
	"testing"
)

func sampleEndpoints(reverse bool) []APIEndpoint {
	eps := []APIEndpoint{
		{
			Path: "/users/{id}", Method: "GET",
			Parameters: []Parameter{
				{Name: "id", Location: LocationPath, Required: true},
				{Name: "expand", Location: LocationQuery},
			},
			Responses: []Response{{StatusCode: "404"}, {StatusCode: "200"}},
			Security:  []SecurityRequirement{{Name: "oauth", Scopes: []string{"write", "read"}}, {Name: "apiKey"}},
		},
		{Path: "/orders", Method: "POST"},
	}
	if reverse {
		eps[0], eps[1] = eps[1], eps[0]
		eps[1].Parameters[0], eps[1].Parameters[1] = eps[1].Parameters[1], eps[1].Parameters[0]
	}
	return eps
}

func TestNormalizeIsDeterministicRegardlessOfInputOrder(t *testing.T) {
	a := &NormalizedAPI{Endpoints: sampleEndpoints(false), SecuritySchemes: []string{"b", "a"}}
	b := &NormalizedAPI{Endpoints: sampleEndpoints(true), SecuritySchemes: []string{"a", "b"}}
	a.Normalize()
	b.Normalize()
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("normalized APIs differ:\n%+v\n%+v", a, b)
	}
	if a.Endpoints[0].Key() != "GET /users/{}" && a.Endpoints[0].Key() != "POST /orders" {
		t.Errorf("unexpected first endpoint %q", a.Endpoints[0].Key())
	}
	get, ok := a.Endpoint("GET /users/{}")
	if !ok {
		t.Fatal("endpoint lookup failed")
	}
	if get.Parameters[0].Location != LocationPath || get.Responses[0].StatusCode != "200" {
		t.Errorf("parameters/responses not sorted: %+v", get)
	}
	if get.Security[0].Name != "apiKey" || get.Security[1].Scopes[0] != "read" {
		t.Errorf("security not sorted: %+v", get.Security)
	}
}

func TestNormalizeHandlesCircularSchemas(t *testing.T) {
	node := &SchemaDefinition{Type: KindObject, Required: []string{"b", "a"}, Enum: []string{"z", "y"}}
	node.Properties = map[string]*SchemaDefinition{"self": node}
	api := &NormalizedAPI{Schemas: map[string]*SchemaDefinition{"Node": node}}
	api.Normalize() // must terminate
	if node.Required[0] != "a" || node.Enum[0] != "y" {
		t.Errorf("lists not sorted: %v %v", node.Required, node.Enum)
	}
}

func TestSchemaHelpers(t *testing.T) {
	s := &SchemaDefinition{
		Required:   []string{"id"},
		Properties: map[string]*SchemaDefinition{"name": {}, "id": {}},
	}
	names := s.PropertyNames()
	if len(names) != 2 || names[0] != "id" || names[1] != "name" {
		t.Errorf("PropertyNames = %v", names)
	}
	if !s.IsRequired("id") || s.IsRequired("name") {
		t.Error("IsRequired mismatch")
	}
	var nilSchema *SchemaDefinition
	if nilSchema.IsRequired("x") || nilSchema.PropertyNames() != nil {
		t.Error("nil schema must be safe")
	}
}
