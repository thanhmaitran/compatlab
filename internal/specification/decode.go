package specification

import (
	"bytes"
	"encoding/json"
	"fmt"

	"gopkg.in/yaml.v3"
)

const maxDepth = 256

// decode reads a JSON or YAML document into a generic tree whose maps all have
// string keys (YAML status codes such as 200 become "200").
func decode(data []byte) (map[string]any, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("%w: empty document", ErrParse)
	}
	var raw any
	if trimmed[0] == '{' {
		if err := json.Unmarshal(trimmed, &raw); err != nil {
			return nil, fmt.Errorf("%w: invalid JSON: %v", ErrParse, err)
		}
	} else if err := yaml.Unmarshal(trimmed, &raw); err != nil {
		return nil, fmt.Errorf("%w: invalid YAML: %v", ErrParse, err)
	}
	tree, err := normalizeTree(raw, 0)
	if err != nil {
		return nil, err
	}
	root, ok := tree.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%w: document root must be a mapping", ErrParse)
	}
	return root, nil
}

func normalizeTree(v any, depth int) (any, error) {
	if depth > maxDepth {
		return nil, fmt.Errorf("%w: document nesting exceeds %d levels", ErrValidation, maxDepth)
	}
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			n, err := normalizeTree(val, depth+1)
			if err != nil {
				return nil, err
			}
			out[k] = n
		}
		return out, nil
	case map[any]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			n, err := normalizeTree(val, depth+1)
			if err != nil {
				return nil, err
			}
			out[fmt.Sprint(k)] = n
		}
		return out, nil
	case []any:
		out := make([]any, len(t))
		for i, val := range t {
			n, err := normalizeTree(val, depth+1)
			if err != nil {
				return nil, err
			}
			out[i] = n
		}
		return out, nil
	default:
		return v, nil
	}
}
