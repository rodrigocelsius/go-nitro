package nitro

import (
	"github.com/rodrigocelsius/go-nitro/usecases/v2/transform"
	"github.com/rodrigocelsius/go-nitro/usecases/v2/validator"
)

// Digits, CamelCase, PascalCase, SnakeCase, KebabCase, SqueezeSpace — public helpers.
func Digits(s string) string { return transform.Digits(s) }

func CamelCase(s string) string    { return transform.CamelCase(s) }
func PascalCase(s string) string   { return transform.PascalCase(s) }
func SnakeCase(s string) string    { return transform.SnakeCase(s) }
func KebabCase(s string) string    { return transform.KebabCase(s) }
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
