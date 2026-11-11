package domain

import (
	"cmp"
	"slices"
)

// NormalizedAPI is the internal representation produced by the specification
// engine and consumed by every analysis module. Call Normalize to guarantee a
// stable ordering independent of the source document layout.
type NormalizedAPI struct {
	Title           string                       `json:"title"`
	Version         string                       `json:"version"`
	OpenAPIVersion  string                       `json:"openapi_version"`
	SourceHash      string                       `json:"source_hash,omitempty"`
	Endpoints       []APIEndpoint                `json:"endpoints"`
	Schemas         map[string]*SchemaDefinition `json:"schemas,omitempty"`
	SecuritySchemes []string                     `json:"security_schemes,omitempty"`
}

// Normalize sorts every order-insensitive collection so that two documents
// with the same meaning produce identical structures.
func (a *NormalizedAPI) Normalize() {
	seen := map[*SchemaDefinition]bool{}
	for i := range a.Endpoints {
		e := &a.Endpoints[i]
		slices.SortFunc(e.Parameters, func(x, y Parameter) int {
			if c := cmp.Compare(x.Location, y.Location); c != 0 {
				return c
			}
			return cmp.Compare(x.Name, y.Name)
		})
		slices.SortFunc(e.Responses, func(x, y Response) int {
			if c := cmp.Compare(x.StatusCode, y.StatusCode); c != 0 {
				return c
			}
			return cmp.Compare(x.ContentType, y.ContentType)
		})
		for j := range e.Security {
			slices.Sort(e.Security[j].Scopes)
		}
		slices.SortFunc(e.Security, func(x, y SecurityRequirement) int { return cmp.Compare(x.Name, y.Name) })
		for _, p := range e.Parameters {
			p.Schema.normalize(seen)
		}
		if e.RequestBody != nil {
			e.RequestBody.Schema.normalize(seen)
		}
		for _, r := range e.Responses {
			r.Schema.normalize(seen)
		}
	}
	for _, s := range a.Schemas {
		s.normalize(seen)
	}
	slices.Sort(a.SecuritySchemes)
	slices.SortFunc(a.Endpoints, func(x, y APIEndpoint) int {
		if c := cmp.Compare(x.Key(), y.Key()); c != 0 {
			return c
		}
		return cmp.Compare(x.Path, y.Path)
	})
}

// Endpoint finds an endpoint by its canonical key (see APIEndpoint.Key).
func (a *NormalizedAPI) Endpoint(key string) (*APIEndpoint, bool) {
	for i := range a.Endpoints {
		if a.Endpoints[i].Key() == key {
			return &a.Endpoints[i], true
		}
	}
	return nil, false
}
