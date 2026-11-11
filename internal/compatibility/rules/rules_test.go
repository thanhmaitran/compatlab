package rules

import (
	"testing"

	"github.com/thanhmaitran/compatlab/internal/domain"
)

func rec(t domain.ChangeType, scope domain.ChangeScope, attrs map[string]string) domain.ChangeRecord {
	return domain.ChangeRecord{ChangeType: t, Scope: scope, Attributes: attrs}
}

func TestRuleIDsAreUniqueAndComplete(t *testing.T) {
	seen := map[string]bool{}
	for _, r := range Default() {
		if r.ID == "" || r.Name == "" || r.Evaluate == nil {
			t.Errorf("incomplete rule: %+v", r.ID)
		}
		if seen[r.ID] {
			t.Errorf("duplicate rule ID %s", r.ID)
		}
		seen[r.ID] = true
	}
}

func TestEveryChangeTypeHasARule(t *testing.T) {
	scopes := map[domain.ChangeType]domain.ChangeScope{
		domain.ChangeEndpointAdded: domain.ScopeEndpoint, domain.ChangeEndpointRemoved: domain.ScopeEndpoint,
		domain.ChangeMethodRemoved: domain.ScopeEndpoint, domain.ChangeParameterAdded: domain.ScopeParameter,
		domain.ChangeParameterRemoved: domain.ScopeParameter, domain.ChangeRequiredChanged: domain.ScopeRequest,
		domain.ChangeTypeChanged: domain.ScopeResponse, domain.ChangeFieldRemoved: domain.ScopeResponse,
		domain.ChangeFieldAdded: domain.ScopeResponse, domain.ChangeEnumChanged: domain.ScopeResponse,
		domain.ChangeStatusCodeRemoved: domain.ScopeResponse, domain.ChangeStatusCodeAdded: domain.ScopeResponse,
		domain.ChangeConstraintChanged: domain.ScopeResponse, domain.ChangeAuthChanged: domain.ScopeSecurity,
	}
	set := Default()
	for _, ct := range domain.AllChangeTypes() {
		scope, ok := scopes[ct]
		if !ok {
			t.Errorf("test table is missing change type %s", ct)
			continue
		}
		if _, found := Find(set, rec(ct, scope, nil)); !found {
			t.Errorf("no rule for %s in scope %s", ct, scope)
		}
	}
}

func TestVerdictsCoverBothDirectionsAndAreValid(t *testing.T) {
	samples := []domain.ChangeRecord{
		rec(domain.ChangeEndpointRemoved, domain.ScopeEndpoint, nil),
		rec(domain.ChangeParameterAdded, domain.ScopeParameter, map[string]string{"required": "true"}),
		rec(domain.ChangeFieldAdded, domain.ScopeRequest, map[string]string{"required": "true"}),
		rec(domain.ChangeTypeChanged, domain.ScopeResponse, map[string]string{"from": "string", "to": "integer"}),
		rec(domain.ChangeEnumChanged, domain.ScopeRequest, map[string]string{"removed": "x"}),
		rec(domain.ChangeConstraintChanged, domain.ScopeResponse, map[string]string{"direction": "relaxed", "constraint": "nullable"}),
		rec(domain.ChangeStatusCodeRemoved, domain.ScopeResponse, map[string]string{"status": "200"}),
		rec(domain.ChangeAuthChanged, domain.ScopeSecurity, map[string]string{"kind": "added"}),
	}
	for _, c := range samples {
		rule, ok := Find(Default(), c)
		if !ok {
			t.Fatalf("no rule for %+v", c)
		}
		vs := rule.Evaluate(c)
		if len(vs) != 2 || vs[0].Direction != domain.DirectionBackward || vs[1].Direction != domain.DirectionForward {
			t.Errorf("%s: expected one backward and one forward verdict, got %+v", rule.ID, vs)
			continue
		}
		for _, v := range vs {
			if !v.Classification.Valid() || !v.Severity.Valid() || v.Explanation == "" {
				t.Errorf("%s: invalid verdict %+v", rule.ID, v)
			}
		}
	}
}

// Compatibility test matrix (backward direction) from the design guide.
func TestBackwardClassificationMatrix(t *testing.T) {
	tests := []struct {
		name string
		c    domain.ChangeRecord
		want domain.ChangeClassification
	}{
		{"remove endpoint", rec(domain.ChangeEndpointRemoved, domain.ScopeEndpoint, nil), domain.Breaking},
		{"add endpoint", rec(domain.ChangeEndpointAdded, domain.ScopeEndpoint, nil), domain.NonBreaking},
		{"remove method", rec(domain.ChangeMethodRemoved, domain.ScopeEndpoint, nil), domain.Breaking},
		{"add required request field", rec(domain.ChangeFieldAdded, domain.ScopeRequest, map[string]string{"required": "true"}), domain.Breaking},
		{"add optional request field", rec(domain.ChangeFieldAdded, domain.ScopeRequest, map[string]string{"required": "false"}), domain.NonBreaking},
		{"add required parameter", rec(domain.ChangeParameterAdded, domain.ScopeParameter, map[string]string{"required": "true"}), domain.Breaking},
		{"optional to required parameter", rec(domain.ChangeRequiredChanged, domain.ScopeParameter, map[string]string{"to": "true"}), domain.Breaking},
		{"remove request field", rec(domain.ChangeFieldRemoved, domain.ScopeRequest, nil), domain.Conditional},
		{"change response type", rec(domain.ChangeTypeChanged, domain.ScopeResponse, map[string]string{"from": "string", "to": "integer"}), domain.Breaking},
		{"add response field", rec(domain.ChangeFieldAdded, domain.ScopeResponse, nil), domain.Conditional},
		{"remove response field", rec(domain.ChangeFieldRemoved, domain.ScopeResponse, nil), domain.Breaking},
		{"restrict request enum", rec(domain.ChangeEnumChanged, domain.ScopeRequest, map[string]string{"removed": "guest"}), domain.Breaking},
		{"expand response enum", rec(domain.ChangeEnumChanged, domain.ScopeResponse, map[string]string{"added": "owner"}), domain.Conditional},
		{"tighten request constraint", rec(domain.ChangeConstraintChanged, domain.ScopeRequest, map[string]string{"direction": "tightened"}), domain.Breaking},
		{"authentication added", rec(domain.ChangeAuthChanged, domain.ScopeSecurity, map[string]string{"kind": "added"}), domain.Breaking},
		{"authentication scopes added", rec(domain.ChangeAuthChanged, domain.ScopeSecurity, map[string]string{"kind": "modified", "added_scopes": "x:write"}), domain.Conditional},
		{"remove 2xx status", rec(domain.ChangeStatusCodeRemoved, domain.ScopeResponse, map[string]string{"status": "200"}), domain.Breaking},
		{"remove 404 status", rec(domain.ChangeStatusCodeRemoved, domain.ScopeResponse, map[string]string{"status": "404"}), domain.Conditional},
	}
	for _, tc := range tests {
		rule, ok := Find(Default(), tc.c)
		if !ok {
			t.Errorf("%s: no rule", tc.name)
			continue
		}
		got := rule.Evaluate(tc.c)[0].Classification
		if got != tc.want {
			t.Errorf("%s (%s): backward = %s, want %s", tc.name, rule.ID, got, tc.want)
		}
	}
}

func TestForwardDiffersFromBackward(t *testing.T) {
	c := rec(domain.ChangeEndpointAdded, domain.ScopeEndpoint, nil)
	rule, _ := Find(Default(), c)
	vs := rule.Evaluate(c)
	if vs[0].Classification == vs[1].Classification {
		t.Error("adding an endpoint must classify backward and forward differently")
	}
}
