package specification

import (
	"fmt"
	"strings"
)

// validate checks the structural minimum required before extraction.
func validate(root map[string]any) error {
	if _, ok := root["swagger"]; ok {
		return fmt.Errorf("%w: Swagger / OpenAPI 2.x documents are not supported", ErrUnsupported)
	}
	v, ok := root["openapi"]
	if !ok {
		return fmt.Errorf("%w: missing required field \"openapi\"", ErrValidation)
	}
	if ver := fmt.Sprint(v); !strings.HasPrefix(ver, "3.") {
		return fmt.Errorf("%w: OpenAPI version %q (only 3.x is supported)", ErrUnsupported, ver)
	}
	info, ok := root["info"].(map[string]any)
	if !ok {
		return fmt.Errorf("%w: missing required object \"info\"", ErrValidation)
	}
	if asString(info["title"]) == "" {
		return fmt.Errorf("%w: info.title is required", ErrValidation)
	}
	if info["version"] == nil {
		return fmt.Errorf("%w: info.version is required", ErrValidation)
	}
	if p, present := root["paths"]; present && p != nil {
		if _, isMap := p.(map[string]any); !isMap {
			return fmt.Errorf("%w: \"paths\" must be an object", ErrValidation)
		}
	}
	return nil
}
