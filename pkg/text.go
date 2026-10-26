package nitro

import (
	"github.com/rodrigocelsius/go-nitro/usecases/v2/transform"
	"github.com/rodrigocelsius/go-nitro/usecases/v2/validator"
)

// Digits keeps only the digits of s ("123.456.789-09" → "12345678909").
func Digits(s string) string { return transform.Digits(s) }

// CamelCase converts s to camelCase ("unit_price" → "unitPrice").
func CamelCase(s string) string { return transform.CamelCase(s) }

// PascalCase converts s to PascalCase ("unit_price" → "UnitPrice").
func PascalCase(s string) string { return transform.PascalCase(s) }

// SnakeCase converts s to snake_case ("UnitPrice" → "unit_price").
func SnakeCase(s string) string { return transform.SnakeCase(s) }

// KebabCase converts s to kebab-case ("UnitPrice" → "unit-price").
func KebabCase(s string) string { return transform.KebabCase(s) }

// Slugify returns a URL-safe slug, folding accents: "São Paulo" → "sao-paulo".
func Slugify(s string) string { return transform.Slugify(s) }

// FoldAccents replaces accented Latin letters with ASCII ("Ação" → "Acao").
func FoldAccents(s string) string { return transform.FoldAccents(s) }

// SqueezeSpace trims s and collapses internal whitespace runs to one space.
func SqueezeSpace(s string) string { return transform.SqueezeSpace(s) }

// IsDigits reports whether s is non-empty and only ASCII 0-9.
func IsDigits(s string) bool { return transform.IsDigits(s) }

// SplitName splits "Ana da Silva" into ("Ana", "da Silva").
func SplitName(name string) (first, last string) { return transform.SplitName(name) }

// NormalizeEmail trims and lowercases an email address.
func NormalizeEmail(s string) string { return transform.NormalizeEmail(s) }

// IfBlank returns fallback when s is empty or whitespace.
func IfBlank(s, fallback string) string { return transform.IfBlank(s, fallback) }

// IsEmail uses the same rules as validate:"email".
func IsEmail(s string) bool { return validator.IsEmail(s) }

// RandomDigits returns n cryptographically random ASCII digits (invite codes, OTP).
func RandomDigits(n int) (string, error) { return transform.RandomDigits(n) }

// RandomAlphaNum returns n cryptographically random 0-9A-Za-z characters.
func RandomAlphaNum(n int) (string, error) { return transform.RandomAlphaNum(n) }

// RandomHex returns hex of nBytes from crypto/rand (length 2*nBytes).
func RandomHex(nBytes int) (string, error) { return transform.RandomHex(nBytes) }
