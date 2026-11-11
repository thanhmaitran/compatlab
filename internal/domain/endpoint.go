package domain

import (
	"strconv"
	"strings"
)

// ParameterLocation says where an operation parameter is carried.
type ParameterLocation string

const (
	LocationPath   ParameterLocation = "path"
	LocationQuery  ParameterLocation = "query"
	LocationHeader ParameterLocation = "header"
	LocationCookie ParameterLocation = "cookie"
)

// Parameter is a normalized operation parameter.
type Parameter struct {
	Name     string            `json:"name"`
	Location ParameterLocation `json:"location"`
	Required bool              `json:"required"`
	Schema   *SchemaDefinition `json:"schema,omitempty"`
}

// RequestBody is a normalized request body for one content type.
type RequestBody struct {
	Required    bool              `json:"required"`
	ContentType string            `json:"content_type"`
	Schema      *SchemaDefinition `json:"schema,omitempty"`
}

// Response is a normalized response for one status code and content type.
type Response struct {
	StatusCode  string            `json:"status_code"`
	ContentType string            `json:"content_type,omitempty"`
	Schema      *SchemaDefinition `json:"schema,omitempty"`
}

// SecurityRequirement names a security scheme and the scopes it requires.
type SecurityRequirement struct {
	Name   string   `json:"name"`
	Scopes []string `json:"scopes,omitempty"`
}

// APIEndpoint is one operation (path + method) of an API.
type APIEndpoint struct {
	Path        string                `json:"path"`
	Method      string                `json:"method"`
	OperationID string                `json:"operation_id,omitempty"`
	Deprecated  bool                  `json:"deprecated,omitempty"`
	Parameters  []Parameter           `json:"parameters,omitempty"`
	RequestBody *RequestBody          `json:"request_body,omitempty"`
	Responses   []Response            `json:"responses,omitempty"`
	Security    []SecurityRequirement `json:"security,omitempty"`
}

// CanonicalPath replaces every path template variable name with an empty
// placeholder so routes are compared by structure: /users/{id} and
// /users/{userId} share the canonical form /users/{}.
func CanonicalPath(p string) string {
	var b strings.Builder
	inParam := false
	for _, r := range p {
		switch {
		case r == '{':
			inParam = true
			b.WriteString("{}")
		case r == '}':
			inParam = false
		case !inParam:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// Key identifies the endpoint by upper-case method and canonical path.
func (e APIEndpoint) Key() string {
	return strings.ToUpper(e.Method) + " " + CanonicalPath(e.Path)
}

// StatusNumber converts a response status key such as "200" or "2XX" to a number.
func StatusNumber(code string) (int, bool) {
	if len(code) != 3 {
		return 0, false
	}
	if n, err := strconv.Atoi(code); err == nil {
		return n, true
	}
	if strings.EqualFold(code[1:], "XX") && code[0] >= '1' && code[0] <= '5' {
		return int(code[0]-'0') * 100, true
	}
	return 0, false
}

// SuccessResponse returns the first documented 2xx response, or nil.
func (e APIEndpoint) SuccessResponse() *Response {
	for i := range e.Responses {
		if n, ok := StatusNumber(e.Responses[i].StatusCode); ok && n >= 200 && n < 300 {
			return &e.Responses[i]
		}
	}
	return nil
}

// SuccessStatus is the first documented 2xx status, or 200 if none is documented.
func (e APIEndpoint) SuccessStatus() int {
	if r := e.SuccessResponse(); r != nil {
		n, _ := StatusNumber(r.StatusCode)
		return n
	}
	return 200
}

// ClientErrorStatus is the documented status for invalid requests: 400, else
// 422, else 400.
func (e APIEndpoint) ClientErrorStatus() int {
	has422 := false
	for _, r := range e.Responses {
		switch r.StatusCode {
		case "400":
			return 400
		case "422":
			has422 = true
		}
	}
	if has422 {
		return 422
	}
	return 400
}

// ResponseFor finds the response documented for a status: exact code, then
// class (for example 2XX), then default.
func (e APIEndpoint) ResponseFor(status int) *Response {
	exact := strconv.Itoa(status)
	class := strconv.Itoa(status/100) + "XX"
	var byClass, def *Response
	for i := range e.Responses {
		r := &e.Responses[i]
		switch strings.ToUpper(r.StatusCode) {
		case exact:
			return r
		case class:
			byClass = r
		case "DEFAULT":
			def = r
		}
	}
	if byClass != nil {
		return byClass
	}
	return def
}

// MatchPath reports whether a concrete request path matches an OpenAPI path
// template and how many template segments were variables (fewer = more
// specific). A query string in concrete is ignored.
func MatchPath(template, concrete string) (int, bool) {
	concrete, _, _ = strings.Cut(concrete, "?")
	ts := strings.Split(strings.Trim(template, "/"), "/")
	cs := strings.Split(strings.Trim(concrete, "/"), "/")
	if len(ts) != len(cs) {
		return 0, false
	}
	vars := 0
	for i := range ts {
		if strings.HasPrefix(ts[i], "{") && strings.HasSuffix(ts[i], "}") {
			if cs[i] == "" {
				return 0, false
			}
			vars++
			continue
		}
		if ts[i] != cs[i] {
			return 0, false
		}
	}
	return vars, true
}

// PathValues extracts template variable values from a concrete path.
func PathValues(template, concrete string) map[string]string {
	out := map[string]string{}
	ts := strings.Split(strings.Trim(template, "/"), "/")
	cs := strings.Split(strings.Trim(concrete, "/"), "/")
	if len(ts) != len(cs) {
		return out
	}
	for i := range ts {
		if strings.HasPrefix(ts[i], "{") && strings.HasSuffix(ts[i], "}") {
			out[ts[i][1:len(ts[i])-1]] = cs[i]
		}
	}
	return out
}
