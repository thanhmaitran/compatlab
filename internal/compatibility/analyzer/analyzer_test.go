package analyzer

import (
	"reflect"
	"testing"

	"github.com/thanhmaitran/compatlab/internal/domain"
)

func api(eps ...domain.APIEndpoint) *domain.NormalizedAPI {
	a := &domain.NormalizedAPI{Endpoints: eps}
	a.Normalize()
	return a
}

func ep(path, method string) domain.APIEndpoint {
	return domain.APIEndpoint{Path: path, Method: method}
}

func obj(required []string, props map[string]*domain.SchemaDefinition) *domain.SchemaDefinition {
	return &domain.SchemaDefinition{Type: domain.KindObject, Required: required, Properties: props}
}

func prim(kind domain.SchemaKind) *domain.SchemaDefinition {
	return &domain.SchemaDefinition{Type: kind}
}

func withResponse(e domain.APIEndpoint, code string, s *domain.SchemaDefinition) domain.APIEndpoint {
	e.Responses = append(e.Responses, domain.Response{StatusCode: code, ContentType: "application/json", Schema: s})
	return e
}

func find(changes []domain.ChangeRecord, t domain.ChangeType, location string) *domain.ChangeRecord {
	for i := range changes {
		if changes[i].ChangeType == t && changes[i].Location == location {
			return &changes[i]
		}
	}
	return nil
}

func TestEndpointAddedRemovedAndMethodRemoved(t *testing.T) {
	oldA := api(ep("/a", "GET"), ep("/b", "GET"), ep("/b", "POST"))
	newA := api(ep("/b", "GET"), ep("/c", "GET"))
	got := Diff(oldA, newA)
	if find(got, domain.ChangeEndpointRemoved, "endpoint") == nil {
		t.Error("expected ENDPOINT_REMOVED for GET /a")
	}
	if find(got, domain.ChangeMethodRemoved, "endpoint") == nil {
		t.Error("expected METHOD_REMOVED for POST /b")
	}
	if find(got, domain.ChangeEndpointAdded, "endpoint") == nil {
		t.Error("expected ENDPOINT_ADDED for GET /c")
	}
	if len(got) != 3 {
		t.Errorf("got %d changes, want 3: %+v", len(got), got)
	}
}

func TestRenamedPathParameterIsNotAChange(t *testing.T) {
	o := ep("/users/{id}", "GET")
	o.Parameters = []domain.Parameter{{Name: "id", Location: domain.LocationPath, Required: true, Schema: prim(domain.KindString)}}
	n := ep("/users/{userId}", "GET")
	n.Parameters = []domain.Parameter{{Name: "userId", Location: domain.LocationPath, Required: true, Schema: prim(domain.KindString)}}
	if got := Diff(api(o), api(n)); len(got) != 0 {
		t.Errorf("expected no changes, got %+v", got)
	}
}

func TestParameterChanges(t *testing.T) {
	o := ep("/a", "GET")
	o.Parameters = []domain.Parameter{
		{Name: "page", Location: domain.LocationQuery, Schema: prim(domain.KindInteger)},
		{Name: "old", Location: domain.LocationQuery},
	}
	n := ep("/a", "GET")
	n.Parameters = []domain.Parameter{
		{Name: "page", Location: domain.LocationQuery, Required: true, Schema: prim(domain.KindString)},
		{Name: "fresh", Location: domain.LocationHeader, Required: true},
	}
	got := Diff(api(o), api(n))
	if c := find(got, domain.ChangeRequiredChanged, "parameter.query.page"); c == nil || c.Attributes["to"] != "true" {
		t.Errorf("expected REQUIRED_CHANGED to true: %+v", c)
	}
	if find(got, domain.ChangeTypeChanged, "parameter.query.page") == nil {
		t.Error("expected TYPE_CHANGED for page")
	}
	if c := find(got, domain.ChangeParameterRemoved, "parameter.query.old"); c == nil {
		t.Error("expected PARAMETER_REMOVED")
	}
	if c := find(got, domain.ChangeParameterAdded, "parameter.header.fresh"); c == nil || c.Attributes["required"] != "true" {
		t.Errorf("expected required PARAMETER_ADDED: %+v", c)
	}
}

func TestRequestBodyFieldAdded(t *testing.T) {
	o := ep("/a", "POST")
	o.RequestBody = &domain.RequestBody{ContentType: "application/json", Schema: obj(nil, map[string]*domain.SchemaDefinition{"email": prim(domain.KindString)})}
	n := ep("/a", "POST")
	n.RequestBody = &domain.RequestBody{ContentType: "application/json", Schema: obj([]string{"password"}, map[string]*domain.SchemaDefinition{
		"email": prim(domain.KindString), "password": prim(domain.KindString)})}
	got := Diff(api(o), api(n))
	c := find(got, domain.ChangeFieldAdded, "request.body.password")
	if c == nil || c.Attributes["required"] != "true" || c.Scope != domain.ScopeRequest {
		t.Errorf("unexpected change: %+v", c)
	}
}

func TestResponseTypeChangeAndFieldRemoval(t *testing.T) {
	o := withResponse(ep("/u", "GET"), "200", obj([]string{"id"}, map[string]*domain.SchemaDefinition{
		"id": prim(domain.KindString), "email": prim(domain.KindString)}))
	n := withResponse(ep("/u", "GET"), "200", obj([]string{"id"}, map[string]*domain.SchemaDefinition{
		"id": prim(domain.KindInteger)}))
	got := Diff(api(o), api(n))
	if c := find(got, domain.ChangeTypeChanged, "response.200.body.id"); c == nil || c.Attributes["from"] != "string" {
		t.Errorf("expected TYPE_CHANGED on id: %+v", c)
	}
	if find(got, domain.ChangeFieldRemoved, "response.200.body.email") == nil {
		t.Error("expected FIELD_REMOVED on email")
	}
}

func TestStatusCodeAddedAndRemoved(t *testing.T) {
	o := withResponse(ep("/u", "GET"), "404", nil)
	n := withResponse(ep("/u", "GET"), "429", nil)
	got := Diff(api(o), api(n))
	if find(got, domain.ChangeStatusCodeRemoved, "response.404") == nil || find(got, domain.ChangeStatusCodeAdded, "response.429") == nil {
		t.Errorf("status code changes missing: %+v", got)
	}
}

func TestEnumChanges(t *testing.T) {
	mk := func(values ...string) *domain.SchemaDefinition {
		return &domain.SchemaDefinition{Type: domain.KindString, Enum: values}
	}
	o := withResponse(ep("/u", "GET"), "200", obj(nil, map[string]*domain.SchemaDefinition{"role": mk("admin", "guest")}))
	n := withResponse(ep("/u", "GET"), "200", obj(nil, map[string]*domain.SchemaDefinition{"role": mk("admin", "owner")}))
	c := find(Diff(api(o), api(n)), domain.ChangeEnumChanged, "response.200.body.role")
	if c == nil || c.Attributes["added"] != "owner" || c.Attributes["removed"] != "guest" {
		t.Errorf("unexpected enum change: %+v", c)
	}
	closed := withResponse(ep("/u", "GET"), "200", obj(nil, map[string]*domain.SchemaDefinition{"role": mk("admin")}))
	open := withResponse(ep("/u", "GET"), "200", obj(nil, map[string]*domain.SchemaDefinition{"role": prim(domain.KindString)}))
	c = find(Diff(api(open), api(closed)), domain.ChangeEnumChanged, "response.200.body.role")
	if c == nil || c.Attributes["removed"] != "*" {
		t.Errorf("open to closed enum must remove '*': %+v", c)
	}
}

func TestConstraintDirections(t *testing.T) {
	one, five := 1.0, 5.0
	o := withResponse(ep("/u", "GET"), "200", &domain.SchemaDefinition{Type: domain.KindInteger, Minimum: &one})
	n := withResponse(ep("/u", "GET"), "200", &domain.SchemaDefinition{Type: domain.KindInteger, Minimum: &five})
	got := Diff(api(o), api(n))
	c := find(got, domain.ChangeConstraintChanged, "response.200.body")
	if c == nil || c.Attributes["constraint"] != "minimum" || c.Attributes["direction"] != "tightened" {
		t.Errorf("unexpected constraint change: %+v", c)
	}
	got = Diff(api(n), api(o))
	c = find(got, domain.ChangeConstraintChanged, "response.200.body")
	if c == nil || c.Attributes["direction"] != "relaxed" {
		t.Errorf("lowering a minimum must be relaxed: %+v", c)
	}
}

func TestSecurityChanges(t *testing.T) {
	o := ep("/u", "GET")
	n := ep("/u", "GET")
	n.Security = []domain.SecurityRequirement{{Name: "bearerAuth", Scopes: []string{"read"}}}
	c := find(Diff(api(o), api(n)), domain.ChangeAuthChanged, "security")
	if c == nil || c.Attributes["kind"] != "added" || c.Attributes["added_schemes"] != "bearerAuth" {
		t.Errorf("unexpected auth change: %+v", c)
	}
	c = find(Diff(api(n), api(o)), domain.ChangeAuthChanged, "security")
	if c == nil || c.Attributes["kind"] != "removed" {
		t.Errorf("unexpected auth change: %+v", c)
	}
}

func TestDiffIsDeterministicAndHandlesCycles(t *testing.T) {
	node := &domain.SchemaDefinition{Type: domain.KindObject}
	node.Properties = map[string]*domain.SchemaDefinition{"next": node, "v": prim(domain.KindString)}
	node2 := &domain.SchemaDefinition{Type: domain.KindObject}
	node2.Properties = map[string]*domain.SchemaDefinition{"next": node2, "v": prim(domain.KindInteger)}
	o := withResponse(ep("/n", "GET"), "200", node)
	n := withResponse(ep("/n", "GET"), "200", node2)
	first := Diff(api(o), api(n))
	second := Diff(api(o), api(n))
	if len(first) == 0 {
		t.Fatal("expected changes")
	}
	if !reflect.DeepEqual(first, second) {
		t.Error("Diff must be deterministic")
	}
}

func TestIdenticalAPIsHaveNoChanges(t *testing.T) {
	o := withResponse(ep("/u", "GET"), "200", obj([]string{"id"}, map[string]*domain.SchemaDefinition{"id": prim(domain.KindString)}))
	if got := Diff(api(o), api(o)); len(got) != 0 {
		t.Errorf("expected no changes, got %+v", got)
	}
}
