package contracts

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/thanhmaitran/compatlab/internal/domain"
)

// Verifier implements domain.ContractVerifier by checking each interaction
// against the provider's normalized API specification.
type Verifier struct{}

var _ domain.ContractVerifier = (*Verifier)(nil)

// NewVerifier returns a contract verifier.
func NewVerifier() *Verifier { return &Verifier{} }

// Verify checks that every interaction is still supported by the API: the
// endpoint exists, the status code is documented, and every field in the
// expected body exists in the response schema with a compatible type.
func (v *Verifier) Verify(contract *domain.ConsumerContract, api *domain.NormalizedAPI) (*domain.ContractVerification, error) {
	if contract == nil || api == nil {
		return nil, fmt.Errorf("%w: contract and API are required", domain.ErrInvalid)
	}
	res := &domain.ContractVerification{Consumer: contract.Consumer, Provider: contract.Provider}
	for _, in := range contract.Interactions {
		res.Interactions = append(res.Interactions, verifyInteraction(in, api))
	}
	return res, nil
}

// FindEndpoint returns the most specific endpoint matching a concrete request.
func FindEndpoint(api *domain.NormalizedAPI, method, concretePath string) *domain.APIEndpoint {
	var best *domain.APIEndpoint
	bestVars := -1
	for i := range api.Endpoints {
		e := &api.Endpoints[i]
		if !strings.EqualFold(e.Method, method) {
			continue
		}
		vars, ok := matchTemplate(e.Path, concretePath)
		if !ok {
			continue
		}
		if best == nil || vars < bestVars || (vars == bestVars && e.Path < best.Path) {
			best, bestVars = e, vars
		}
	}
	return best
}

func findResponse(ep *domain.APIEndpoint, status int) *domain.Response {
	exact := strconv.Itoa(status)
	class := fmt.Sprintf("%dXX", status/100)
	var byClass, def *domain.Response
	for i := range ep.Responses {
		r := &ep.Responses[i]
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

func verifyInteraction(in domain.ContractInteraction, api *domain.NormalizedAPI) domain.InteractionResult {
	r := domain.InteractionResult{Description: in.Description}
	ep := FindEndpoint(api, in.Request.Method, in.Request.Path)
	if ep == nil {
		r.Message = fmt.Sprintf("endpoint %s %s is not defined", strings.ToUpper(in.Request.Method), in.Request.Path)
		return r
	}
	resp := findResponse(ep, in.Response.Status)
	if resp == nil {
		r.Message = fmt.Sprintf("status %d is not documented for %s %s", in.Response.Status, ep.Method, ep.Path)
		return r
	}
	if in.Response.Body != nil {
		if resp.Schema == nil {
			r.Message = "the response has no documented schema to check the expected body against"
			return r
		}
		if problems := checkValue(in.Response.Body, resp.Schema, "body", 0); len(problems) > 0 {
			r.Message = strings.Join(problems, "; ")
			return r
		}
	}
	r.Passed = true
	return r
}

func mismatch(path, want string, got any) []string {
	return []string{fmt.Sprintf("%s should be %s but the contract expects %T", path, want, got)}
}

// checkValue compares an expected JSON value with a response schema.
func checkValue(v any, s *domain.SchemaDefinition, path string, depth int) []string {
	if s == nil || depth > 32 {
		return nil
	}
	if v == nil {
		if s.Nullable || s.Type == domain.KindNull || s.Type == domain.KindUnknown {
			return nil
		}
		return []string{path + " is null in the contract but the schema is not nullable"}
	}
	switch s.Type {
	case domain.KindObject:
		obj, ok := v.(map[string]any)
		if !ok {
			return mismatch(path, "an object", v)
		}
		if len(s.Properties) == 0 {
			return nil // free-form object: nothing to check
		}
		keys := make([]string, 0, len(obj))
		for k := range obj {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		var out []string
		for _, k := range keys {
			child, defined := s.Properties[k]
			if !defined {
				out = append(out, fmt.Sprintf("%s.%s is not defined in the response schema", path, k))
				continue
			}
			out = append(out, checkValue(obj[k], child, path+"."+k, depth+1)...)
		}
		return out
	case domain.KindArray:
		arr, ok := v.([]any)
		if !ok {
			return mismatch(path, "an array", v)
		}
		if len(arr) > 0 {
			return checkValue(arr[0], s.Items, path+"[0]", depth+1)
		}
	case domain.KindString:
		if _, ok := v.(string); !ok {
			return mismatch(path, "a string", v)
		}
	case domain.KindInteger:
		if f, ok := v.(float64); !ok || f != math.Trunc(f) {
			return mismatch(path, "an integer", v)
		}
	case domain.KindNumber:
		if _, ok := v.(float64); !ok {
			return mismatch(path, "a number", v)
		}
	case domain.KindBoolean:
		if _, ok := v.(bool); !ok {
			return mismatch(path, "a boolean", v)
		}
	}
	return nil
}
