package services

import (
	"maps"
	"slices"
)

// JSON-schema builders for the models' strict structured answers, where every
// property is required and no other is allowed.

const typeKey = "type"

func objectSchema(properties map[string]any) map[string]any {
	return map[string]any{
		typeKey:                "object",
		"additionalProperties": false,
		"required":             slices.Sorted(maps.Keys(properties)),
		"properties":           properties,
	}
}

func arraySchema(items map[string]any) map[string]any {
	return map[string]any{typeKey: "array", "items": items}
}

func typeSchema(name string) map[string]any {
	return map[string]any{typeKey: name}
}

func enumSchema(values ...string) map[string]any {
	return map[string]any{typeKey: "string", "enum": values}
}
