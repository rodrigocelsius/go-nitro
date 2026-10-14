package domain

import "fmt"

// ParamSource indicates where the parameter comes from.
type ParamSource int

const (
	SourcePath ParamSource = iota
	SourceQuery
	SourceHeader
	SourceCookie
	SourceBody
	SourceForm
)

func (s ParamSource) String() string {
	switch s {
	case SourcePath:
		return "path"
	case SourceQuery:
		return "query"
	case SourceHeader:
		return "header"
	case SourceCookie:
		return "cookie"
	case SourceBody:
		return "body"
	case SourceForm:
		return "form"
	default:
		return "unknown"
	}
}

// BindError represents a binding/conversion error.
type BindError struct {
	Field   string
	Source  ParamSource
	Message string
}

func (e *BindError) Error() string {
	return fmt.Sprintf("bind: field %s from %s: %s", e.Field, e.Source, e.Message)
}

// ParamDef is the definition of a parameter for binding.
type ParamDef struct {
	Name   string
	Source ParamSource
	Type   string // "string", "int", "bool", etc.
}
