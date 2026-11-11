package domain

import "testing"

func TestCanonicalPath(t *testing.T) {
	tests := []struct{ in, want string }{
		{"/users/{id}", "/users/{}"},
		{"/users/{userId}", "/users/{}"},
		{"/orders/{orderId}/items/{itemId}", "/orders/{}/items/{}"},
		{"/health", "/health"},
		{"", ""},
	}
	for _, tc := range tests {
		if got := CanonicalPath(tc.in); got != tc.want {
			t.Errorf("CanonicalPath(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestEndpointKeyIgnoresParameterNamesAndMethodCase(t *testing.T) {
	a := APIEndpoint{Path: "/users/{id}", Method: "get"}
	b := APIEndpoint{Path: "/users/{userId}", Method: "GET"}
	if a.Key() != b.Key() {
		t.Errorf("keys differ: %q vs %q", a.Key(), b.Key())
	}
	if a.Key() != "GET /users/{}" {
		t.Errorf("unexpected key %q", a.Key())
	}
}

func TestMatchPathAndValues(t *testing.T) {
	if vars, ok := MatchPath("/users/{id}", "/users/u-1"); !ok || vars != 1 {
		t.Errorf("MatchPath = %d,%v", vars, ok)
	}
	if _, ok := MatchPath("/users/{id}", "/users"); ok {
		t.Error("different segment counts must not match")
	}
	if _, ok := MatchPath("/users/search", "/users/x"); ok {
		t.Error("static segments must match exactly")
	}
	v := PathValues("/o/{oid}/items/{iid}", "/o/1/items/2")
	if v["oid"] != "1" || v["iid"] != "2" {
		t.Errorf("PathValues = %v", v)
	}
}

func TestEndpointStatusHelpers(t *testing.T) {
	e := APIEndpoint{Responses: []Response{{StatusCode: "201"}, {StatusCode: "422"}, {StatusCode: "default"}}}
	if e.SuccessStatus() != 201 || e.ClientErrorStatus() != 422 {
		t.Errorf("got %d / %d", e.SuccessStatus(), e.ClientErrorStatus())
	}
	if e.ResponseFor(201) == nil || e.ResponseFor(500) == nil {
		t.Error("exact and default lookups must work")
	}
	if (APIEndpoint{}).SuccessStatus() != 200 || (APIEndpoint{}).ClientErrorStatus() != 400 {
		t.Error("defaults must be 200 and 400")
	}
	if n, ok := StatusNumber("4XX"); !ok || n != 400 {
		t.Errorf("StatusNumber(4XX) = %d,%v", n, ok)
	}
}
