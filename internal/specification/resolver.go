package specification

import (
	"fmt"
	"strings"
)

// pointer resolves an internal JSON reference such as #/components/schemas/User.
// External references are rejected by policy: the engine never reads files or
// URLs named inside an uploaded document.
func (x *extractor) pointer(ref string) (any, error) {
	if !strings.HasPrefix(ref, "#/") {
		return nil, fmt.Errorf("%w: only internal references are supported, got %q", ErrReference, ref)
	}
	var cur any = x.root
	for _, seg := range strings.Split(ref[2:], "/") {
		seg = unescapePointer(seg)
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%w: cannot resolve %q", ErrReference, ref)
		}
		cur, ok = m[seg]
		if !ok {
			return nil, fmt.Errorf("%w: %q does not exist", ErrReference, ref)
		}
	}
	return cur, nil
}

// derefMap follows a chain of $ref objects until it reaches a plain object.
func (x *extractor) derefMap(node any) (map[string]any, error) {
	for hops := 0; hops < 32; hops++ {
		m, ok := node.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%w: expected an object", ErrValidation)
		}
		ref, isRef := m["$ref"].(string)
		if !isRef {
			return m, nil
		}
		target, err := x.pointer(ref)
		if err != nil {
			return nil, err
		}
		node = target
	}
	return nil, fmt.Errorf("%w: reference chain too deep", ErrReference)
}

func escapePointer(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "~", "~0"), "/", "~1")
}

func unescapePointer(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "~1", "/"), "~0", "~")
}

func refName(ref string) string {
	return unescapePointer(ref[strings.LastIndex(ref, "/")+1:])
}
