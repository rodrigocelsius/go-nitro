package nitro

import (
	"bufio"
	"fmt"
	"net/url"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
	"github.com/rodrigocelsius/go-nitro/usecases/v2"
)

// LoadDotEnvOption configures LoadDotEnv.
type LoadDotEnvOption func(*loadDotEnvOpts)

type loadDotEnvOpts struct {
	overwrite bool
}

// WithOverwrite makes .env values replace existing process environment variables.
func WithOverwrite() LoadDotEnvOption {
	return func(o *loadDotEnvOpts) { o.overwrite = true }
}

// LoadDotEnv loads KEY=VALUE pairs from .env and .env.local if present into the
// process environment (os.Setenv). ConfigOf does not call this — use it when
// libraries must see values via os.Getenv.
// By default existing environment variables are not overwritten (use WithOverwrite).
func LoadDotEnv(pathsAndOpts ...any) {
	paths, opts := parseLoadDotEnvArgs(pathsAndOpts...)
	dotenv := readDotEnvFiles(paths...)
	applyDotEnv(dotenv, opts.overwrite)
}

func parseLoadDotEnvArgs(args ...any) ([]string, loadDotEnvOpts) {
	var paths []string
	opts := loadDotEnvOpts{}
	for _, a := range args {
		switch v := a.(type) {
		case string:
			paths = append(paths, v)
		case LoadDotEnvOption:
			v(&opts)
		}
	}
	if len(paths) == 0 {
		paths = []string{".env", ".env.local"}
	}
	return paths, opts
}

func readDotEnvFiles(paths ...string) map[string]string {
	out := map[string]string{}
	for _, path := range paths {
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			key, val, ok := strings.Cut(line, "=")
			if !ok {
				continue
			}
			key = strings.TrimSpace(key)
			val = strings.TrimSpace(val)
			val = strings.Trim(val, `"'`)
			if key == "" {
				continue
			}
			out[key] = val // later files win (.env.local)
		}
		_ = f.Close()
	}
	return out
}

func applyDotEnv(dotenv map[string]string, overwrite bool) {
	for key, val := range dotenv {
		if overwrite {
			_ = os.Setenv(key, val)
			continue
		}
		if _, exists := os.LookupEnv(key); !exists {
			_ = os.Setenv(key, val)
		}
	}
}

// MustConfig loads T from environment (and optional .env files).
// Tags: env:"NAME" | env:"A,B" (aliases), env:"NAME,overwrite", default:"value", validate:"required".
// Nested structs are filled recursively. []string is CSV. time.Duration and url.URL supported.
// Struct field env:"DB_" (trailing _) prefixes child env names.
//
//	cfg := nitro.MustConfig[AppConfig]()
func MustConfig[T any]() T {
	v, err := ConfigOf[T]()
	if err != nil {
		panic(err)
	}
	return v
}

// ConfigOf loads T from environment and optional .env files without panicking.
// .env values fill missing fields but are NOT applied to os.Environ (no process
// side effect). Call LoadDotEnv explicitly when you need os.Setenv.
func ConfigOf[T any]() (T, error) {
	paths, _ := parseLoadDotEnvArgs()
	dotenv := readDotEnvFiles(paths...)
	var zero T
	rv := reflect.ValueOf(&zero).Elem()
	if rv.Kind() != reflect.Struct {
		return zero, fmt.Errorf("nitro: ConfigOf requires a struct type")
	}
	var missing []string
	if err := fillConfigStruct(rv, "", dotenv, &missing); err != nil {
		return zero, err
	}
	if len(missing) > 0 {
		return zero, fmt.Errorf("nitro: missing required env: %s", strings.Join(uniqueStrings(missing), ", "))
	}

	// Map NITRO_ENV into domain.Env if struct has Env field of domain.Env / Env type
	if ef := rv.FieldByName("Env"); ef.IsValid() && ef.CanSet() {
		if ef.Type() == reflect.TypeOf(domain.Env(0)) || ef.Type() == reflect.TypeOf(Env(0)) {
			raw := lookupEnvOrDot(dotenv, "NITRO_ENV")
			if raw != "" {
				ef.Set(reflect.ValueOf(parseNitroEnv(raw)))
			}
		}
	}

	if errs := usecases.ValidateStruct(zero); errs.HasErrors() {
		return zero, fmt.Errorf("nitro: config validation: %w", errs)
	}
	return zero, nil
}

func lookupEnvOrDot(dotenv map[string]string, key string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return dotenv[key]
}

func fillConfigStruct(rv reflect.Value, prefix string, dotenv map[string]string, missing *[]string) error {
	rt := rv.Type()
	for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)
		if !field.IsExported() {
			continue
		}
		fv := rv.Field(i)

		// Embedded struct: flatten into current prefix.
		if field.Anonymous && fv.Kind() == reflect.Struct && !isTimeType(fv.Type()) && !isURLType(fv.Type()) {
			if err := fillConfigStruct(fv, prefix, dotenv, missing); err != nil {
				return err
			}
			continue
		}

		envTag := field.Tag.Get("env")
		def := field.Tag.Get("default")
		required := strings.Contains(field.Tag.Get("validate"), "required")

		if fv.Kind() == reflect.Struct && !isTimeType(fv.Type()) && !isURLType(fv.Type()) {
			childPrefix := prefix
			if envTag != "" {
				// env:"DB_" → prefix children; env:"DB" → prefix "DB_"
				p := envTag
				if !strings.HasSuffix(p, "_") {
					p += "_"
				}
				childPrefix = prefix + p
			}
			if err := fillConfigStruct(fv, childPrefix, dotenv, missing); err != nil {
				return fmt.Errorf("nitro: config field %s: %w", field.Name, err)
			}
			continue
		}

		names, overwrite := parseEnvTag(field.Name, envTag, prefix)
		raw := ""
		if overwrite {
			raw = lookupDotEnvAlias(dotenv, names)
		}
		if raw == "" {
			raw = lookupEnvAlias(names)
		}
		if raw == "" {
			// Prefer .env file values without mutating the process environment.
			raw = lookupDotEnvAlias(dotenv, names)
		}
		if raw == "" {
			raw = def
		}
		if raw == "" {
			if required {
				*missing = append(*missing, names[0])
			}
			continue
		}
		if err := setFieldFromString(fv, raw); err != nil {
			return fmt.Errorf("nitro: config field %s (%s): %w", field.Name, strings.Join(names, "|"), err)
		}
	}
	return nil
}

// parseEnvTag returns env key names. Option "overwrite" prefers .env file over process env.
func parseEnvTag(fieldName, envTag, prefix string) (names []string, overwrite bool) {
	if envTag == "" {
		return []string{prefix + toEnvName(fieldName)}, false
	}
	parts := strings.Split(envTag, ",")
	keys := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if strings.EqualFold(p, "overwrite") {
			overwrite = true
			continue
		}
		keys = append(keys, p)
	}
	aliased := len(keys) > 1
	out := make([]string, 0, len(keys))
	for _, p := range keys {
		switch {
		case prefix == "" || aliased:
			out = append(out, p)
		case strings.HasPrefix(p, prefix):
			out = append(out, p)
		default:
			out = append(out, prefix+p)
		}
	}
	if len(out) == 0 {
		return []string{prefix + toEnvName(fieldName)}, overwrite
	}
	return out, overwrite
}

func lookupEnvAlias(names []string) string {
	for _, name := range names {
		if v, ok := os.LookupEnv(name); ok && v != "" {
			return v
		}
	}
	return ""
}

func lookupDotEnvAlias(dotenv map[string]string, names []string) string {
	for _, name := range names {
		if v, ok := dotenv[name]; ok && v != "" {
			return v
		}
	}
	return ""
}

func isTimeType(t reflect.Type) bool {
	return t == reflect.TypeOf(time.Time{}) || t == reflect.TypeOf(time.Duration(0))
}

func isURLType(t reflect.Type) bool {
	return t == reflect.TypeOf(url.URL{})
}

func uniqueStrings(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

// ParseEnv converts an environment name (dev|development|local → development;
// prod|production|empty → production) into an Env. Unknown names fall back to
// production with a warning on stderr.
func ParseEnv(s string) Env { return parseNitroEnv(s) }

func parseNitroEnv(s string) Env {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "dev", "development", "local":
		return EnvDevelopment
	case "prod", "production", "":
		return EnvProduction
	default:
		fmt.Fprintf(os.Stderr, "nitro: warning: unknown NITRO_ENV %q — treating as production\n", s)
		return EnvProduction
	}
}

func toEnvName(field string) string {
	var b strings.Builder
	for i, r := range field {
		if i > 0 && r >= 'A' && r <= 'Z' {
			b.WriteByte('_')
		}
		b.WriteRune(r)
	}
	return strings.ToUpper(b.String())
}

func setFieldFromString(fv reflect.Value, raw string) error {
	if fv.Type() == reflect.TypeOf(time.Duration(0)) {
		d, err := time.ParseDuration(raw)
		if err != nil {
			return err
		}
		fv.SetInt(int64(d))
		return nil
	}
	if fv.Type() == reflect.TypeOf(url.URL{}) {
		u, err := url.Parse(raw)
		if err != nil {
			return err
		}
		fv.Set(reflect.ValueOf(*u))
		return nil
	}
	if fv.Kind() == reflect.Ptr {
		if fv.Type().Elem() == reflect.TypeOf(url.URL{}) {
			u, err := url.Parse(raw)
			if err != nil {
				return err
			}
			fv.Set(reflect.ValueOf(u))
			return nil
		}
	}
	switch fv.Kind() {
	case reflect.String:
		fv.SetString(raw)
	case reflect.Bool:
		v, err := strconv.ParseBool(raw)
		if err != nil {
			return err
		}
		fv.SetBool(v)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return err
		}
		fv.SetInt(v)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		v, err := strconv.ParseUint(raw, 10, 64)
		if err != nil {
			return err
		}
		fv.SetUint(v)
	case reflect.Float32, reflect.Float64:
		v, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return err
		}
		fv.SetFloat(v)
	case reflect.Slice:
		if fv.Type().Elem().Kind() != reflect.String {
			return fmt.Errorf("unsupported slice elem %s", fv.Type().Elem().Kind())
		}
		fv.Set(reflect.ValueOf(splitCSV(raw)))
	default:
		return fmt.Errorf("unsupported kind %s", fv.Kind())
	}
	return nil
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
