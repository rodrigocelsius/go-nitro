package nitro

import (
	"fmt"
	"strconv"
	"strings"
)

// Attrs is the canonical JSONB map for per-tenant custom fields.
// Prefer this over JSON[map[string]any] when modeling extensible entities.
//
//	Attrs nitro.Attrs `json:"attrs" db:"attrs,jsonb" blinq:"gin"`
type Attrs = JSON[map[string]any]

// FieldDef describes one custom attribute in a tenant field schema.
type FieldDef struct {
	Key      string   `json:"key"`
	Label    string   `json:"label,omitempty"`
	Type     string   `json:"type"` // string|number|bool|select
	Required bool     `json:"required,omitempty"`
	Options  []string `json:"options,omitempty"` // for select
}

// ValidateAttrs checks attrs against FieldDef list (required + type + select options).
// Unknown keys are allowed (forward-compatible); missing required keys fail.
func ValidateAttrs(defs []FieldDef, attrs map[string]any) error {
	if attrs == nil {
		attrs = map[string]any{}
	}
	byKey := make(map[string]FieldDef, len(defs))
	for _, d := range defs {
		key := strings.TrimSpace(d.Key)
		if key == "" {
			continue
		}
		d.Key = key
		byKey[key] = d
	}
	for _, d := range byKey {
		v, ok := attrs[d.Key]
		if !ok || v == nil || v == "" {
			if d.Required {
				return fmt.Errorf("attrs: field %q is required", d.Key)
			}
			continue
		}
		if err := validateAttrValue(d, v); err != nil {
			return err
		}
	}
	return nil
}

func validateAttrValue(d FieldDef, v any) error {
	t := strings.ToLower(strings.TrimSpace(d.Type))
	if t == "" {
		t = "string"
	}
	switch t {
	case "string", "text":
		switch v.(type) {
		case string:
			return nil
		default:
			return fmt.Errorf("attrs: field %q must be a string", d.Key)
		}
	case "number", "int", "float":
		switch n := v.(type) {
		case float64, float32, int, int64, int32:
			return nil
		case string:
			if _, err := strconv.ParseFloat(n, 64); err != nil {
				return fmt.Errorf("attrs: field %q must be a number", d.Key)
			}
			return nil
		default:
			return fmt.Errorf("attrs: field %q must be a number", d.Key)
		}
	case "bool", "boolean":
		switch v.(type) {
		case bool:
			return nil
		default:
			return fmt.Errorf("attrs: field %q must be a boolean", d.Key)
		}
	case "select":
		s, ok := v.(string)
		if !ok {
			return fmt.Errorf("attrs: field %q must be a string (select)", d.Key)
		}
		if len(d.Options) == 0 {
			return nil
		}
		for _, opt := range d.Options {
			if opt == s {
				return nil
			}
		}
		return fmt.Errorf("attrs: field %q value %q is not in options", d.Key, s)
	default:
		return nil
	}
}
