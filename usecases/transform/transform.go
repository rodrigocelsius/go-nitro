// Package transform applies `transform:"…"` tag rules on string / *string fields.
package transform

import (
	"reflect"
	"strings"
	"unicode"
)

// TitleCaseOptions configure TitleCase exceptions (PT stopwords by default).
type TitleCaseOptions struct {
	// Exceptions are kept lowercase when not the first word (e.g. de, da, do).
	Exceptions map[string]bool
}

var defaultPTExceptions = map[string]bool{
	"de": true, "da": true, "do": true, "das": true, "dos": true, "e": true,
}

// DefaultTitleCasePT returns PT-BR exceptions for titlecase=pt.
func DefaultTitleCasePT() TitleCaseOptions {
	ex := make(map[string]bool, len(defaultPTExceptions))
	for k, v := range defaultPTExceptions {
		ex[k] = v
	}
	return TitleCaseOptions{Exceptions: ex}
}

// Apply walks v and mutates string / *string fields that have transform tags.
// Nested structs and slices/arrays of structs are processed recursively.
func Apply(v any) {
	if v == nil {
		return
	}
	applyValue(reflect.ValueOf(v))
}

func applyValue(rv reflect.Value) {
	for rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return
		}
		rv = rv.Elem()
	}
	if !rv.IsValid() {
		return
	}

	switch rv.Kind() {
	case reflect.Struct:
		applyStruct(rv)
	case reflect.Slice, reflect.Array:
		for i := 0; i < rv.Len(); i++ {
			applyValue(rv.Index(i))
		}
	}
}

func applyStruct(rv reflect.Value) {
	rt := rv.Type()
	for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)
		if !field.IsExported() {
			continue
		}
		fv := rv.Field(i)

		if tag := field.Tag.Get("transform"); tag != "" {
			applyFieldTransform(fv, tag)
		}

		switch fv.Kind() {
		case reflect.Struct:
			applyStruct(fv)
		case reflect.Pointer:
			if !fv.IsNil() && fv.Elem().Kind() == reflect.Struct {
				applyStruct(fv.Elem())
			}
		case reflect.Slice, reflect.Array:
			applyValue(fv)
		}
	}
}

func applyFieldTransform(fv reflect.Value, tag string) {
	// emptyslice applies to nil slices before other rules
	if strings.Contains(tag, "emptyslice") && fv.Kind() == reflect.Slice && fv.IsNil() && fv.CanSet() {
		fv.Set(reflect.MakeSlice(fv.Type(), 0, 0))
	}

	switch fv.Kind() {
	case reflect.String:
		fv.SetString(runPipeline(fv.String(), tag))
	case reflect.Pointer:
		if fv.IsNil() || fv.Type().Elem().Kind() != reflect.String {
			return
		}
		s := runPipeline(fv.Elem().String(), tag)
		fv.Elem().SetString(s)
	}
}

func runPipeline(s, tag string) string {
	for _, part := range strings.Split(tag, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		name, arg, _ := strings.Cut(part, "=")
		name = strings.ToLower(strings.TrimSpace(name))
		arg = strings.TrimSpace(arg)
		s = applyOne(s, name, arg)
	}
	return s
}

func applyOne(s, name, arg string) string {
	switch name {
	case "trim":
		return strings.TrimSpace(s)
	case "upper":
		return strings.ToUpper(s)
	case "lower":
		return strings.ToLower(s)
	case "digits":
		return Digits(s)
	case "squeezespace", "squeeze":
		return SqueezeSpace(s)
	case "titlecase", "title":
		// titlecase=pt → PT stopwords; no arg → capitalize every word
		if strings.EqualFold(arg, "pt") {
			return TitleCase(s, DefaultTitleCasePT())
		}
		return TitleCase(s, TitleCaseOptions{})
	case "camelcase", "camel":
		return CamelCase(s)
	case "pascalcase", "pascal":
		return PascalCase(s)
	case "snakecase", "snake":
		return SnakeCase(s)
	case "slug", "slugify":
		return Slugify(s)
	case "foldaccents", "unaccent":
		return FoldAccents(s)
	case "kebabcase", "kebab":
		return KebabCase(s)
	case "date":
		return transformDate(s, arg)
	case "datetime":
		return transformDateTime(s, arg)
	case "money":
		return transformMoney(s, arg)
	case "measure":
		return transformMeasure(s, arg)
	case "lat", "latitude":
		return transformLat(s)
	case "lng", "lon", "longitude":
		return transformLng(s)
	case "latlng":
		return transformLatLng(s)
	default:
		return s
	}
}

// Digits keeps only ASCII digits.
func Digits(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// SqueezeSpace collapses consecutive whitespace to a single space and trims.
func SqueezeSpace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// TitleCase capitalizes each whitespace-separated word.
// With Exceptions, non-first matching words stay lowercase.
func TitleCase(s string, opts TitleCaseOptions) string {
	words := strings.Fields(s)
	if len(words) == 0 {
		return s
	}
	for i, w := range words {
		lower := strings.ToLower(w)
		if i > 0 && opts.Exceptions != nil && opts.Exceptions[lower] {
			words[i] = lower
			continue
		}
		runes := []rune(lower)
		if len(runes) > 0 {
			runes[0] = unicode.ToUpper(runes[0])
		}
		words[i] = string(runes)
	}
	return strings.Join(words, " ")
}

// CamelCase converts identifiers / phrases to camelCase.
func CamelCase(s string) string {
	parts := splitWords(s)
	if len(parts) == 0 {
		return ""
	}
	var b strings.Builder
	for i, p := range parts {
		lower := strings.ToLower(p)
		if i == 0 {
			b.WriteString(lower)
			continue
		}
		b.WriteString(capitalizeASCII(lower))
	}
	return b.String()
}

// PascalCase converts to PascalCase.
func PascalCase(s string) string {
	parts := splitWords(s)
	var b strings.Builder
	for _, p := range parts {
		b.WriteString(capitalizeASCII(strings.ToLower(p)))
	}
	return b.String()
}

// SnakeCase converts CamelCase / phrases to snake_case.
func SnakeCase(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	var b strings.Builder
	runes := []rune(s)
	for i, r := range runes {
		if r == ' ' || r == '-' || r == '_' {
			if b.Len() > 0 && b.String()[b.Len()-1] != '_' {
				b.WriteByte('_')
			}
			continue
		}
		if unicode.IsUpper(r) {
			if i > 0 {
				prev := runes[i-1]
				if unicode.IsLower(prev) || unicode.IsDigit(prev) ||
					(unicode.IsUpper(prev) && i+1 < len(runes) && unicode.IsLower(runes[i+1])) {
					if b.Len() > 0 && b.String()[b.Len()-1] != '_' {
						b.WriteByte('_')
					}
				}
			}
			b.WriteRune(unicode.ToLower(r))
			continue
		}
		b.WriteRune(unicode.ToLower(r))
	}
	out := b.String()
	for strings.Contains(out, "__") {
		out = strings.ReplaceAll(out, "__", "_")
	}
	return strings.Trim(out, "_")
}

// KebabCase converts to kebab-case.
func KebabCase(s string) string {
	return strings.ReplaceAll(SnakeCase(s), "_", "-")
}

func splitWords(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	// Prefer explicit separators.
	if strings.ContainsAny(s, " _-") {
		return strings.FieldsFunc(s, func(r rune) bool {
			return r == ' ' || r == '_' || r == '-'
		})
	}
	// Split CamelCase / PascalCase.
	var parts []string
	var cur []rune
	runes := []rune(s)
	for i, r := range runes {
		if i > 0 && unicode.IsUpper(r) {
			prev := runes[i-1]
			nextLower := i+1 < len(runes) && unicode.IsLower(runes[i+1])
			if unicode.IsLower(prev) || unicode.IsDigit(prev) || (unicode.IsUpper(prev) && nextLower) {
				if len(cur) > 0 {
					parts = append(parts, string(cur))
					cur = cur[:0]
				}
			}
		}
		cur = append(cur, r)
	}
	if len(cur) > 0 {
		parts = append(parts, string(cur))
	}
	return parts
}

func capitalizeASCII(s string) string {
	if s == "" {
		return s
	}
	runes := []rune(s)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}
