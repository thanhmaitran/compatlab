package specification

import (
	"fmt"
	"sort"
	"strings"

	"github.com/thanhmaitran/compatlab/internal/domain"
)

var httpMethods = []string{"get", "put", "post", "delete", "options", "head", "patch", "trace"}

type extractor struct {
	root  map[string]any
	memo  map[string]*domain.SchemaDefinition
	depth int
}

// extract converts the generic document tree into the normalized model.
func extract(root map[string]any) (*domain.NormalizedAPI, error) {
	x := &extractor{root: root, memo: map[string]*domain.SchemaDefinition{}}
	info := asMap(root["info"])
	api := &domain.NormalizedAPI{
		Title:          asString(info["title"]),
		Version:        fmt.Sprint(info["version"]),
		OpenAPIVersion: fmt.Sprint(root["openapi"]),
		Schemas:        map[string]*domain.SchemaDefinition{},
	}
	components := asMap(root["components"])
	for _, name := range sortedKeys(asMap(components["schemas"])) {
		def, err := x.schema(map[string]any{"$ref": "#/components/schemas/" + escapePointer(name)})
		if err != nil {
			return nil, err
		}
		api.Schemas[name] = def
	}
	api.SecuritySchemes = sortedKeys(asMap(components["securitySchemes"]))

	paths := asMap(root["paths"])
	for _, path := range sortedKeys(paths) {
		item := asMap(paths[path])
		for _, method := range httpMethods {
			node, ok := item[method]
			if !ok {
				continue
			}
			op, isMap := node.(map[string]any)
			if !isMap {
				return nil, fmt.Errorf("%w: operation %s %s must be an object", ErrValidation, strings.ToUpper(method), path)
			}
			ep, err := x.endpoint(path, method, item, op, root["security"])
			if err != nil {
				return nil, err
			}
			api.Endpoints = append(api.Endpoints, *ep)
		}
	}
	return api, nil
}

func (x *extractor) endpoint(path, method string, item, op map[string]any, global any) (*domain.APIEndpoint, error) {
	ctx := strings.ToUpper(method) + " " + path
	ep := &domain.APIEndpoint{
		Path:        path,
		Method:      strings.ToUpper(method),
		OperationID: asString(op["operationId"]),
		Deprecated:  asBool(op["deprecated"]),
	}
	params, err := x.parameters(ctx, item["parameters"], op["parameters"])
	if err != nil {
		return nil, err
	}
	ep.Parameters = params
	if rb, ok := op["requestBody"]; ok {
		body, err := x.requestBody(rb)
		if err != nil {
			return nil, err
		}
		ep.RequestBody = body
	}
	responses, err := x.responses(op["responses"])
	if err != nil {
		return nil, err
	}
	ep.Responses = responses
	secNode := global
	if s, ok := op["security"]; ok {
		secNode = s
	}
	ep.Security = securityFrom(secNode)
	return ep, nil
}

func (x *extractor) parameters(ctx string, lists ...any) ([]domain.Parameter, error) {
	byKey := map[string]domain.Parameter{}
	for _, list := range lists {
		for _, node := range asSlice(list) {
			m, err := x.derefMap(node)
			if err != nil {
				return nil, err
			}
			p := domain.Parameter{
				Name:     asString(m["name"]),
				Location: domain.ParameterLocation(asString(m["in"])),
				Required: asBool(m["required"]),
			}
			switch p.Location {
			case domain.LocationPath, domain.LocationQuery, domain.LocationHeader, domain.LocationCookie:
			default:
				return nil, fmt.Errorf("%w: parameter %q of %s has invalid location %q", ErrValidation, p.Name, ctx, p.Location)
			}
			if p.Name == "" {
				return nil, fmt.Errorf("%w: a parameter of %s has no name", ErrValidation, ctx)
			}
			if p.Location == domain.LocationPath {
				p.Required = true
			}
			if s, ok := m["schema"]; ok {
				sch, err := x.schema(s)
				if err != nil {
					return nil, err
				}
				p.Schema = sch
			}
			byKey[string(p.Location)+":"+p.Name] = p
		}
	}
	keys := make([]string, 0, len(byKey))
	for k := range byKey {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]domain.Parameter, 0, len(keys))
	for _, k := range keys {
		out = append(out, byKey[k])
	}
	return out, nil
}

func (x *extractor) requestBody(node any) (*domain.RequestBody, error) {
	m, err := x.derefMap(node)
	if err != nil {
		return nil, err
	}
	ct, schemaNode := pickContent(asMap(m["content"]))
	body := &domain.RequestBody{Required: asBool(m["required"]), ContentType: ct}
	if schemaNode != nil {
		s, err := x.schema(schemaNode)
		if err != nil {
			return nil, err
		}
		body.Schema = s
	}
	return body, nil
}

func (x *extractor) responses(node any) ([]domain.Response, error) {
	m := asMap(node)
	out := make([]domain.Response, 0, len(m))
	for _, code := range sortedKeys(m) {
		rm, err := x.derefMap(m[code])
		if err != nil {
			return nil, err
		}
		ct, schemaNode := pickContent(asMap(rm["content"]))
		r := domain.Response{StatusCode: code, ContentType: ct}
		if schemaNode != nil {
			s, err := x.schema(schemaNode)
			if err != nil {
				return nil, err
			}
			r.Schema = s
		}
		out = append(out, r)
	}
	return out, nil
}

// pickContent chooses application/json when present, otherwise the first
// content type in alphabetical order. Other content types are not modelled.
func pickContent(content map[string]any) (string, any) {
	if len(content) == 0 {
		return "", nil
	}
	keys := sortedKeys(content)
	chosen := keys[0]
	for _, k := range keys {
		if k == "application/json" {
			chosen = k
		}
	}
	return chosen, asMap(content[chosen])["schema"]
}

// securityFrom flattens security requirement objects into one entry per
// scheme name. AND/OR structure between schemes is not preserved.
func securityFrom(node any) []domain.SecurityRequirement {
	scopes := map[string]map[string]bool{}
	for _, req := range asSlice(node) {
		for name, sc := range asMap(req) {
			if scopes[name] == nil {
				scopes[name] = map[string]bool{}
			}
			for _, s := range asSlice(sc) {
				scopes[name][fmt.Sprint(s)] = true
			}
		}
	}
	names := make([]string, 0, len(scopes))
	for n := range scopes {
		names = append(names, n)
	}
	sort.Strings(names)
	var out []domain.SecurityRequirement
	for _, n := range names {
		var list []string
		for s := range scopes[n] {
			list = append(list, s)
		}
		sort.Strings(list)
		out = append(out, domain.SecurityRequirement{Name: n, Scopes: list})
	}
	return out
}

// schema converts a schema node. References are memoized so that circular
// schemas become pointer cycles instead of infinite recursion.
func (x *extractor) schema(node any) (*domain.SchemaDefinition, error) {
	if node == nil {
		return nil, nil
	}
	m, ok := node.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%w: schema must be an object", ErrValidation)
	}
	if ref, isRef := m["$ref"].(string); isRef {
		if def, found := x.memo[ref]; found {
			return def, nil
		}
		target, err := x.derefMap(m)
		if err != nil {
			return nil, err
		}
		def := &domain.SchemaDefinition{Ref: ref, Name: refName(ref)}
		x.memo[ref] = def
		if err := x.fill(def, target); err != nil {
			return nil, err
		}
		return def, nil
	}
	def := &domain.SchemaDefinition{}
	if err := x.fill(def, m); err != nil {
		return nil, err
	}
	return def, nil
}

func (x *extractor) fill(def *domain.SchemaDefinition, m map[string]any) error {
	x.depth++
	defer func() { x.depth-- }()
	if x.depth > maxDepth {
		return fmt.Errorf("%w: schema nesting too deep", ErrValidation)
	}
	switch t := m["type"].(type) {
	case string:
		def.Type = domain.SchemaKind(t)
	case []any:
		for _, e := range t {
			s := fmt.Sprint(e)
			if s == "null" {
				def.Nullable = true
			} else if def.Type == "" {
				def.Type = domain.SchemaKind(s)
			}
		}
	}
	if asBool(m["nullable"]) {
		def.Nullable = true
	}
	def.Format = asString(m["format"])
	def.Pattern = asString(m["pattern"])
	for _, e := range asSlice(m["enum"]) {
		def.Enum = append(def.Enum, fmt.Sprint(e))
	}
	for _, e := range asSlice(m["required"]) {
		def.Required = append(def.Required, fmt.Sprint(e))
	}
	if raw, ok := m["properties"]; ok {
		pm, isMap := raw.(map[string]any)
		if !isMap {
			return fmt.Errorf("%w: \"properties\" must be an object", ErrValidation)
		}
		def.Properties = make(map[string]*domain.SchemaDefinition, len(pm))
		for _, name := range sortedKeys(pm) {
			child, err := x.schema(pm[name])
			if err != nil {
				return err
			}
			if child == nil {
				child = &domain.SchemaDefinition{}
			}
			def.Properties[name] = child
		}
	}
	if raw, ok := m["items"]; ok {
		it, err := x.schema(raw)
		if err != nil {
			return err
		}
		def.Items = it
	}
	groups := []struct {
		key string
		dst *[]*domain.SchemaDefinition
	}{{"oneOf", &def.OneOf}, {"anyOf", &def.AnyOf}, {"allOf", &def.AllOf}}
	for _, g := range groups {
		for _, n := range asSlice(m[g.key]) {
			child, err := x.schema(n)
			if err != nil {
				return err
			}
			if child != nil {
				*g.dst = append(*g.dst, child)
			}
		}
	}
	if f, ok := toFloat(m["minimum"]); ok {
		def.Minimum = &f
	}
	if f, ok := toFloat(m["maximum"]); ok {
		def.Maximum = &f
	}
	if f, ok := toFloat(m["minLength"]); ok {
		n := int(f)
		def.MinLength = &n
	}
	if f, ok := toFloat(m["maxLength"]); ok {
		n := int(f)
		def.MaxLength = &n
	}
	if def.Type == "" {
		if len(def.Properties) > 0 {
			def.Type = domain.KindObject
		} else if def.Items != nil {
			def.Type = domain.KindArray
		}
	}
	return nil
}

func asMap(v any) map[string]any { m, _ := v.(map[string]any); return m }
func asSlice(v any) []any        { s, _ := v.([]any); return s }
func asString(v any) string      { s, _ := v.(string); return s }
func asBool(v any) bool          { b, _ := v.(bool); return b }

func toFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case uint64:
		return float64(n), true
	}
	return 0, false
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
