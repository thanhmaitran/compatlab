// Package analyzer detects raw differences between two normalized APIs.
// It does not decide whether a difference is breaking; see package rules.
package analyzer

import (
	"cmp"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/thanhmaitran/compatlab/internal/domain"
)

type differ struct {
	out []domain.ChangeRecord
}

// loc identifies the operation and part of it being compared.
type loc struct {
	path, method string
	scope        domain.ChangeScope
}

// Diff compares two normalized APIs and returns the raw changes in a stable
// order. Both inputs should have been normalized (the parser does this).
func Diff(oldAPI, newAPI *domain.NormalizedAPI) []domain.ChangeRecord {
	d := &differ{}
	oldIdx := indexEndpoints(oldAPI.Endpoints)
	newIdx := indexEndpoints(newAPI.Endpoints)

	newPaths := map[string]bool{}
	for _, e := range newAPI.Endpoints {
		newPaths[domain.CanonicalPath(e.Path)] = true
	}
	keys := map[string]bool{}
	for k := range oldIdx {
		keys[k] = true
	}
	for k := range newIdx {
		keys[k] = true
	}
	for _, key := range sortedSet(keys) {
		o, inOld := oldIdx[key]
		n, inNew := newIdx[key]
		switch {
		case inOld && !inNew:
			if newPaths[domain.CanonicalPath(o.Path)] {
				d.add(o.Path, o.Method, domain.ScopeEndpoint, domain.ChangeMethodRemoved, "endpoint", nil, nil,
					"HTTP method "+o.Method+" was removed from "+o.Path+".", nil)
			} else {
				d.add(o.Path, o.Method, domain.ScopeEndpoint, domain.ChangeEndpointRemoved, "endpoint", nil, nil,
					"Endpoint "+o.Method+" "+o.Path+" was removed.", nil)
			}
		case !inOld && inNew:
			d.add(n.Path, n.Method, domain.ScopeEndpoint, domain.ChangeEndpointAdded, "endpoint", nil, nil,
				"Endpoint "+n.Method+" "+n.Path+" was added.", nil)
		default:
			d.endpoint(o, n)
		}
	}
	slices.SortFunc(d.out, compareChanges)
	return d.out
}

func compareChanges(a, b domain.ChangeRecord) int {
	if c := cmp.Compare(a.Path, b.Path); c != 0 {
		return c
	}
	if c := cmp.Compare(a.Method, b.Method); c != 0 {
		return c
	}
	if c := cmp.Compare(a.Location, b.Location); c != 0 {
		return c
	}
	if c := cmp.Compare(a.ChangeType, b.ChangeType); c != 0 {
		return c
	}
	return cmp.Compare(a.Attributes["constraint"], b.Attributes["constraint"])
}

func (d *differ) add(path, method string, scope domain.ChangeScope, t domain.ChangeType, location string, oldV, newV any, desc string, attrs map[string]string) {
	idLoc := location
	if c := attrs["constraint"]; c != "" {
		idLoc += "#" + c
	}
	d.out = append(d.out, domain.ChangeRecord{
		ID:          domain.NewChangeID(path, method, t, idLoc),
		Path:        path,
		Method:      method,
		ChangeType:  t,
		Scope:       scope,
		Location:    location,
		OldValue:    oldV,
		NewValue:    newV,
		Attributes:  attrs,
		Description: desc,
	})
}

func indexEndpoints(eps []domain.APIEndpoint) map[string]*domain.APIEndpoint {
	idx := make(map[string]*domain.APIEndpoint, len(eps))
	for i := range eps {
		idx[eps[i].Key()] = &eps[i]
	}
	return idx
}

func (d *differ) endpoint(o, n *domain.APIEndpoint) {
	d.parameters(o, n)
	d.requestBody(o, n)
	d.responses(o, n)
	d.security(o, n)
}

// ---------- parameters ----------

type paramKey struct {
	loc  domain.ParameterLocation
	name string
}

// pathParamNames lists template variable names in order of appearance.
func pathParamNames(path string) []string {
	var names []string
	for {
		i := strings.Index(path, "{")
		if i < 0 {
			return names
		}
		j := strings.Index(path[i:], "}")
		if j < 0 {
			return names
		}
		names = append(names, path[i+1:i+j])
		path = path[i+j+1:]
	}
}

func (d *differ) parameters(o, n *domain.APIEndpoint) {
	// Path parameters are matched by position, so renaming {id} to {userId}
	// is not a change.
	rename := map[string]string{}
	on, nn := pathParamNames(o.Path), pathParamNames(n.Path)
	if len(on) == len(nn) {
		for i := range on {
			rename[on[i]] = nn[i]
		}
	}
	oldIdx := map[paramKey]domain.Parameter{}
	for _, p := range o.Parameters {
		name := p.Name
		if p.Location == domain.LocationPath {
			if r, ok := rename[name]; ok {
				name = r
			}
		}
		oldIdx[paramKey{p.Location, name}] = p
	}
	newIdx := map[paramKey]domain.Parameter{}
	for _, p := range n.Parameters {
		newIdx[paramKey{p.Location, p.Name}] = p
	}
	seen := map[paramKey]bool{}
	var keys []paramKey
	for k := range oldIdx {
		if !seen[k] {
			seen[k] = true
			keys = append(keys, k)
		}
	}
	for k := range newIdx {
		if !seen[k] {
			seen[k] = true
			keys = append(keys, k)
		}
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].loc != keys[j].loc {
			return keys[i].loc < keys[j].loc
		}
		return keys[i].name < keys[j].name
	})
	for _, k := range keys {
		op, inOld := oldIdx[k]
		np, inNew := newIdx[k]
		where := "parameter." + string(k.loc) + "." + k.name
		switch {
		case inOld && !inNew:
			d.add(o.Path, o.Method, domain.ScopeParameter, domain.ChangeParameterRemoved, where, nil, nil,
				"Parameter "+k.name+" ("+string(k.loc)+") was removed.",
				map[string]string{"required": boolStr(op.Required)})
		case !inOld && inNew:
			d.add(o.Path, o.Method, domain.ScopeParameter, domain.ChangeParameterAdded, where, nil, nil,
				"Parameter "+k.name+" ("+string(k.loc)+") was added.",
				map[string]string{"required": boolStr(np.Required)})
		default:
			if op.Required != np.Required {
				d.add(o.Path, o.Method, domain.ScopeParameter, domain.ChangeRequiredChanged, where, op.Required, np.Required,
					"Parameter "+k.name+" changed required from "+boolStr(op.Required)+" to "+boolStr(np.Required)+".",
					map[string]string{"from": boolStr(op.Required), "to": boolStr(np.Required)})
			}
			d.schema(loc{o.Path, o.Method, domain.ScopeParameter}, where, op.Schema, np.Schema,
				map[[2]*domain.SchemaDefinition]bool{})
		}
	}
}

// ---------- request body ----------

func (d *differ) requestBody(o, n *domain.APIEndpoint) {
	ob, nb := o.RequestBody, n.RequestBody
	c := loc{o.Path, o.Method, domain.ScopeRequest}
	switch {
	case ob == nil && nb == nil:
	case ob == nil:
		d.add(c.path, c.method, c.scope, domain.ChangeFieldAdded, "request.body", nil, nil,
			"A request body was added.", map[string]string{"kind": "body", "required": boolStr(nb.Required)})
	case nb == nil:
		d.add(c.path, c.method, c.scope, domain.ChangeFieldRemoved, "request.body", nil, nil,
			"The request body was removed.", map[string]string{"kind": "body", "required": boolStr(ob.Required)})
	default:
		if ob.Required != nb.Required {
			d.add(c.path, c.method, c.scope, domain.ChangeRequiredChanged, "request.body", ob.Required, nb.Required,
				"Request body changed required from "+boolStr(ob.Required)+" to "+boolStr(nb.Required)+".",
				map[string]string{"from": boolStr(ob.Required), "to": boolStr(nb.Required)})
		}
		if ob.ContentType != nb.ContentType {
			d.add(c.path, c.method, c.scope, domain.ChangeTypeChanged, "request.body.content_type", ob.ContentType, nb.ContentType,
				"Request content type changed from "+ob.ContentType+" to "+nb.ContentType+".",
				map[string]string{"kind": "content_type", "from": ob.ContentType, "to": nb.ContentType})
			return
		}
		d.schema(c, "request.body", ob.Schema, nb.Schema, map[[2]*domain.SchemaDefinition]bool{})
	}
}

// ---------- responses ----------

func (d *differ) responses(o, n *domain.APIEndpoint) {
	oldIdx := map[string]domain.Response{}
	for _, r := range o.Responses {
		oldIdx[r.StatusCode] = r
	}
	newIdx := map[string]domain.Response{}
	for _, r := range n.Responses {
		newIdx[r.StatusCode] = r
	}
	all := map[string]bool{}
	for k := range oldIdx {
		all[k] = true
	}
	for k := range newIdx {
		all[k] = true
	}
	for _, code := range sortedSet(all) {
		or, inOld := oldIdx[code]
		nr, inNew := newIdx[code]
		where := "response." + code
		c := loc{o.Path, o.Method, domain.ScopeResponse}
		switch {
		case inOld && !inNew:
			d.add(c.path, c.method, c.scope, domain.ChangeStatusCodeRemoved, where, code, nil,
				"Response status "+code+" was removed.", map[string]string{"status": code})
		case !inOld && inNew:
			d.add(c.path, c.method, c.scope, domain.ChangeStatusCodeAdded, where, nil, code,
				"Response status "+code+" was added.", map[string]string{"status": code})
		default:
			if or.ContentType != nr.ContentType {
				d.add(c.path, c.method, c.scope, domain.ChangeTypeChanged, where+".content_type", or.ContentType, nr.ContentType,
					"Response "+code+" content type changed from "+or.ContentType+" to "+nr.ContentType+".",
					map[string]string{"kind": "content_type", "from": or.ContentType, "to": nr.ContentType, "status": code})
				continue
			}
			d.schema(c, where+".body", or.Schema, nr.Schema, map[[2]*domain.SchemaDefinition]bool{})
		}
	}
}

// ---------- security ----------

func securitySets(reqs []domain.SecurityRequirement) (schemes, scopes map[string]bool) {
	schemes, scopes = map[string]bool{}, map[string]bool{}
	for _, r := range reqs {
		schemes[r.Name] = true
		for _, s := range r.Scopes {
			scopes[r.Name+":"+s] = true
		}
	}
	return schemes, scopes
}

func diffSets(a, b map[string]bool) (added, removed []string) {
	for k := range b {
		if !a[k] {
			added = append(added, k)
		}
	}
	for k := range a {
		if !b[k] {
			removed = append(removed, k)
		}
	}
	sort.Strings(added)
	sort.Strings(removed)
	return added, removed
}

func (d *differ) security(o, n *domain.APIEndpoint) {
	oldSchemes, oldScopes := securitySets(o.Security)
	newSchemes, newScopes := securitySets(n.Security)
	addedSchemes, removedSchemes := diffSets(oldSchemes, newSchemes)
	addedScopes, removedScopes := diffSets(oldScopes, newScopes)
	if len(addedSchemes)+len(removedSchemes)+len(addedScopes)+len(removedScopes) == 0 {
		return
	}
	kind := "modified"
	switch {
	case len(oldSchemes) == 0 && len(newSchemes) > 0:
		kind = "added"
	case len(oldSchemes) > 0 && len(newSchemes) == 0:
		kind = "removed"
	}
	d.add(o.Path, o.Method, domain.ScopeSecurity, domain.ChangeAuthChanged, "security", nil, nil,
		"Authentication requirements changed ("+kind+").",
		map[string]string{
			"kind":            kind,
			"added_schemes":   strings.Join(addedSchemes, ","),
			"removed_schemes": strings.Join(removedSchemes, ","),
			"added_scopes":    strings.Join(addedScopes, ","),
			"removed_scopes":  strings.Join(removedScopes, ","),
		})
}

// ---------- schemas ----------

func (d *differ) schema(c loc, prefix string, o, n *domain.SchemaDefinition, seen map[[2]*domain.SchemaDefinition]bool) {
	switch {
	case o == nil && n == nil:
		return
	case o == nil:
		d.add(c.path, c.method, c.scope, domain.ChangeFieldAdded, prefix, nil, string(n.Type),
			"A schema was added at "+prefix+".", map[string]string{"kind": "schema", "required": "false"})
		return
	case n == nil:
		d.add(c.path, c.method, c.scope, domain.ChangeFieldRemoved, prefix, string(o.Type), nil,
			"The schema at "+prefix+" was removed.", map[string]string{"kind": "schema", "required": "false"})
		return
	}
	key := [2]*domain.SchemaDefinition{o, n}
	if seen[key] {
		return
	}
	seen[key] = true

	if o.Type != n.Type {
		d.add(c.path, c.method, c.scope, domain.ChangeTypeChanged, prefix, string(o.Type), string(n.Type),
			"Type at "+prefix+" changed from "+typeName(o.Type)+" to "+typeName(n.Type)+".",
			map[string]string{"from": string(o.Type), "to": string(n.Type)})
		return
	}
	if o.Format != n.Format {
		d.constraint(c, prefix, "format", "changed", o.Format, n.Format)
	}
	if o.Nullable != n.Nullable {
		dir := "tightened"
		if n.Nullable {
			dir = "relaxed"
		}
		d.constraint(c, prefix, "nullable", dir, boolStr(o.Nullable), boolStr(n.Nullable))
	}
	if o.Pattern != n.Pattern {
		d.constraint(c, prefix, "pattern", "changed", o.Pattern, n.Pattern)
	}
	d.bound(c, prefix, "minimum", o.Minimum, n.Minimum, true)
	d.bound(c, prefix, "maximum", o.Maximum, n.Maximum, false)
	d.bound(c, prefix, "minLength", intPtr(o.MinLength), intPtr(n.MinLength), true)
	d.bound(c, prefix, "maxLength", intPtr(o.MaxLength), intPtr(n.MaxLength), false)
	d.enum(c, prefix, o, n)

	names := unionSorted(o.PropertyNames(), n.PropertyNames())
	for _, name := range names {
		op, inOld := o.Properties[name]
		np, inNew := n.Properties[name]
		where := prefix + "." + name
		switch {
		case inOld && !inNew:
			d.add(c.path, c.method, c.scope, domain.ChangeFieldRemoved, where, string(op.Type), nil,
				"Field "+where+" was removed.", map[string]string{"required": boolStr(o.IsRequired(name))})
		case !inOld && inNew:
			d.add(c.path, c.method, c.scope, domain.ChangeFieldAdded, where, nil, string(np.Type),
				"Field "+where+" was added.", map[string]string{"required": boolStr(n.IsRequired(name))})
		default:
			if o.IsRequired(name) != n.IsRequired(name) {
				d.add(c.path, c.method, c.scope, domain.ChangeRequiredChanged, where, o.IsRequired(name), n.IsRequired(name),
					"Field "+where+" changed required from "+boolStr(o.IsRequired(name))+" to "+boolStr(n.IsRequired(name))+".",
					map[string]string{"from": boolStr(o.IsRequired(name)), "to": boolStr(n.IsRequired(name))})
			}
			d.schema(c, where, op, np, seen)
		}
	}
	if o.Items != nil || n.Items != nil {
		d.schema(c, prefix+"[]", o.Items, n.Items, seen)
	}
	d.combinator(c, prefix, "oneOf", o.OneOf, n.OneOf, seen)
	d.combinator(c, prefix, "anyOf", o.AnyOf, n.AnyOf, seen)
	d.combinator(c, prefix, "allOf", o.AllOf, n.AllOf, seen)
}

func (d *differ) combinator(c loc, prefix, name string, o, n []*domain.SchemaDefinition, seen map[[2]*domain.SchemaDefinition]bool) {
	if len(o) != len(n) {
		d.constraint(c, prefix, name, "changed", strconv.Itoa(len(o))+" variants", strconv.Itoa(len(n))+" variants")
		return
	}
	for i := range o {
		d.schema(c, prefix+"{"+name+"["+strconv.Itoa(i)+"]}", o[i], n[i], seen)
	}
}

func (d *differ) constraint(c loc, prefix, name, direction, oldV, newV string) {
	d.add(c.path, c.method, c.scope, domain.ChangeConstraintChanged, prefix, oldV, newV,
		"Constraint "+name+" at "+prefix+" changed from "+show(oldV)+" to "+show(newV)+" ("+direction+").",
		map[string]string{"constraint": name, "direction": direction})
}

func (d *differ) bound(c loc, prefix, name string, o, n *float64, lower bool) {
	if o == nil && n == nil {
		return
	}
	if o != nil && n != nil && *o == *n {
		return
	}
	dir := "relaxed"
	switch {
	case o == nil:
		dir = "tightened"
	case n == nil:
		dir = "relaxed"
	case lower && *n > *o, !lower && *n < *o:
		dir = "tightened"
	}
	d.constraint(c, prefix, name, dir, fmtFloat(o), fmtFloat(n))
}

// enum records added and removed values. A missing enum means "any value", so
// adding an enum removes "*" and dropping one adds "*".
func (d *differ) enum(c loc, prefix string, o, n *domain.SchemaDefinition) {
	if len(o.Enum) == 0 && len(n.Enum) == 0 {
		return
	}
	var added, removed []string
	switch {
	case len(o.Enum) == 0:
		removed = []string{"*"}
	case len(n.Enum) == 0:
		added = []string{"*"}
	default:
		oldSet, newSet := toSet(o.Enum), toSet(n.Enum)
		added, removed = diffSets(oldSet, newSet)
	}
	if len(added) == 0 && len(removed) == 0 {
		return
	}
	d.add(c.path, c.method, c.scope, domain.ChangeEnumChanged, prefix, strings.Join(o.Enum, ","), strings.Join(n.Enum, ","),
		"Enum at "+prefix+" changed (added: "+show(strings.Join(added, ","))+"; removed: "+show(strings.Join(removed, ","))+").",
		map[string]string{"added": strings.Join(added, ","), "removed": strings.Join(removed, ",")})
}

// ---------- helpers ----------

func boolStr(b bool) string { return strconv.FormatBool(b) }

func typeName(k domain.SchemaKind) string {
	if k == "" {
		return "unspecified"
	}
	return string(k)
}

func show(s string) string {
	if s == "" {
		return "none"
	}
	return s
}

func fmtFloat(p *float64) string {
	if p == nil {
		return ""
	}
	return strconv.FormatFloat(*p, 'g', -1, 64)
}

func intPtr(p *int) *float64 {
	if p == nil {
		return nil
	}
	f := float64(*p)
	return &f
}

func toSet(list []string) map[string]bool {
	s := make(map[string]bool, len(list))
	for _, v := range list {
		s[v] = true
	}
	return s
}

func sortedSet(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func unionSorted(a, b []string) []string {
	set := map[string]bool{}
	for _, v := range a {
		set[v] = true
	}
	for _, v := range b {
		set[v] = true
	}
	return sortedSet(set)
}
