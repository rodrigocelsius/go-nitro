package validator

import "strings"

// Rule is a parsed validate tag rule (public API, aligned with validation.md).
type Rule struct {
	Name  string
	Param string
}

// ParseRules converts a validate tag into []Rule (includes required/omitempty/dive).
func ParseRules(tag string) []Rule {
	var rules []Rule
	for _, part := range strings.Split(tag, ",") {
		part = strings.TrimLeft(part, " \t")
		if part == "" {
			continue
		}
		parts := strings.SplitN(part, "=", 2)
		name := strings.TrimSpace(parts[0])
		param := ""
		if len(parts) > 1 {
			param = parts[1]
		}
		if name == "" {
			continue
		}
		rules = append(rules, Rule{Name: name, Param: param})
	}
	return rules
}

type validateRule struct {
	name string
	arg  string
}

type validateMeta struct {
	required  bool
	omitempty bool
	dive      bool
}

func parseValidateTag(tag string) (validateMeta, []validateRule, []validateRule) {
	var meta validateMeta
	var fieldRules, diveRules []validateRule
	seenDive := false

	for _, part := range strings.Split(tag, ",") {
		// Trim only the left side so args like excludes=␠ keep the space.
		part = strings.TrimLeft(part, " \t")
		if part == "" {
			continue
		}
		parts := strings.SplitN(part, "=", 2)
		name := strings.TrimSpace(parts[0])
		arg := ""
		if len(parts) > 1 {
			arg = parts[1]
		}

		switch name {
		case "required":
			meta.required = true
		case "omitempty":
			meta.omitempty = true
		case "dive":
			meta.dive = true
			seenDive = true
		case "default":
			// meta: applied in UnmarshalJSONDefaults / ApplyDefaults (not a validation rule)
		default:
			r := validateRule{name: name, arg: arg}
			if seenDive {
				diveRules = append(diveRules, r)
			} else {
				fieldRules = append(fieldRules, r)
			}
		}
	}
	return meta, fieldRules, diveRules
}
