package validator

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

// ResolveFieldMessage applies §3.4 priority:
//  1. msg_<rule> tag
//  2. message tag
//  3. rule default message (already formatted by the builtin)
//
// Placeholders in custom messages: {field}, {param}, {value}, {rule}.
// path is used as {field} when non-empty (e.g. "address.street").
func ResolveFieldMessage(field reflect.StructField, path, rule, defaultMsg, param string, value any) string {
	if field.Name == "" && len(field.Tag) == 0 {
		return defaultMsg
	}
	label := path
	if label == "" {
		label = jsonFieldName(field)
	}
	if msg := field.Tag.Get("msg_" + rule); msg != "" {
		return ExpandMessage(msg, label, rule, param, value)
	}
	if msg := field.Tag.Get("message"); msg != "" {
		return ExpandMessage(msg, label, rule, param, value)
	}
	return defaultMsg
}

// ExpandMessage substitui placeholders em templates de mensagem.
func ExpandMessage(tmpl, field, rule, param string, value any) string {
	if tmpl == "" || !strings.Contains(tmpl, "{") {
		return tmpl
	}
	repl := strings.NewReplacer(
		"{field}", field,
		"{param}", param,
		"{rule}", rule,
		"{value}", formatMessageValue(value),
	)
	return repl.Replace(tmpl)
}

func formatMessageValue(v any) string {
	if v == nil {
		return ""
	}
	s := fmt.Sprint(v)
	const max = 64
	if len(s) > max {
		return s[:max] + "…"
	}
	return s
}

func withResolvedMessage(err *domain.ValidationError, field reflect.StructField, path, rule, param string, value any) *domain.ValidationError {
	if err == nil {
		return nil
	}
	err.Message = ResolveFieldMessage(field, path, rule, err.Message, param, value)
	return err
}
