package modelcatalog

import (
	"fmt"
)

// ValidatePolicyPins checks the cross-field invariant JSON Schema cannot express.
// The document must be a decoded policy object, typically after schema validation.
func ValidatePolicyPins(document any) error {
	root, ok := document.(map[string]any)
	if !ok {
		return fmt.Errorf("policy must be an object")
	}
	models, _ := root["models"].(map[string]any)
	for modelID, raw := range models {
		entry, _ := raw.(map[string]any)
		pin, hasPin := entry["version_pin"].(string)
		identity, _ := entry["identity"].(map[string]any)
		version, hasVersion := identity["version"].(string)
		if hasPin && hasVersion && pin != version {
			return fmt.Errorf("models.%s.version_pin does not match identity.version %q", modelID, version)
		}
	}
	return nil
}
