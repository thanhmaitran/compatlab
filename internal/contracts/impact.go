package contracts

import (
	"cmp"
	"slices"
	"strconv"
	"strings"

	"github.com/thanhmaitran/compatlab/internal/domain"
)

// Impact determines which consumers are affected by the backward-direction
// breaking and conditional changes of an analysis.
//
// A change is attributed to a consumer only when one of its interactions
// touches the changed endpoint (and, for response changes, the same status
// code and a field the consumer actually references). Changes that match no
// contract are not attributed: consumers without contracts remain unknown.
func Impact(analysis *domain.CompatibilityAnalysis, cs []*domain.ConsumerContract) []domain.ConsumerImpact {
	out := []domain.ConsumerImpact{}
	if analysis == nil {
		return out
	}
	for _, ch := range analysis.Changes {
		if ch.Direction != domain.DirectionBackward || ch.Classification == domain.NonBreaking {
			continue
		}
		for _, ct := range cs {
			level, ok := impactFor(ct, ch)
			if !ok {
				continue
			}
			out = append(out, domain.ConsumerImpact{
				Consumer: ct.Consumer, Provider: ct.Provider, ChangeID: ch.ID, RuleID: ch.RuleID,
				Classification: ch.Classification, Level: level, Location: ch.Location,
			})
		}
	}
	slices.SortStableFunc(out, func(a, b domain.ConsumerImpact) int {
		if c := cmp.Compare(a.Consumer, b.Consumer); c != 0 {
			return c
		}
		if c := cmp.Compare(a.Location, b.Location); c != 0 {
			return c
		}
		return cmp.Compare(a.RuleID, b.RuleID)
	})
	return out
}

func impactFor(ct *domain.ConsumerContract, ch domain.ClassifiedChange) (string, bool) {
	for _, in := range ct.Interactions {
		if !strings.EqualFold(in.Request.Method, ch.Method) {
			continue
		}
		if _, ok := matchTemplate(ch.Path, in.Request.Path); !ok {
			continue
		}
		if ch.Scope == domain.ScopeResponse {
			code, field, hasBody := splitResponseLocation(ch.Location)
			if code != "" && code != strconv.Itoa(in.Response.Status) {
				continue
			}
			if hasBody && field != "" && !referencesField(in.Response.Body, field) {
				continue
			}
		}
		if ch.Classification == domain.Breaking {
			return domain.ImpactAffected, true
		}
		return domain.ImpactReview, true
	}
	return "", false
}

// splitResponseLocation parses locations such as response.200.body.email.
func splitResponseLocation(loc string) (code, field string, hasBody bool) {
	rest, ok := strings.CutPrefix(loc, "response.")
	if !ok {
		return "", "", false
	}
	code, rest, _ = strings.Cut(rest, ".")
	body, ok := strings.CutPrefix(rest, "body")
	if !ok {
		return code, "", false
	}
	field = strings.TrimPrefix(strings.ReplaceAll(body, "[]", ""), ".")
	return code, field, true
}

func referencesField(body any, field string) bool {
	var paths []string
	collectPaths(body, "", &paths)
	for _, p := range paths {
		if p == field || strings.HasPrefix(p, field+".") || strings.HasPrefix(field, p+".") {
			return true
		}
	}
	return false
}

func collectPaths(v any, prefix string, out *[]string) {
	switch t := v.(type) {
	case map[string]any:
		for k, val := range t {
			p := k
			if prefix != "" {
				p = prefix + "." + k
			}
			*out = append(*out, p)
			collectPaths(val, p, out)
		}
	case []any:
		for _, el := range t {
			collectPaths(el, prefix, out)
		}
	}
}
