package locale

import (
	"os"
	"strings"
	"sync"
	"time"
)

var (
	mu       sync.RWMutex
	override string
	tzLoc    *time.Location
)

// SetLocale overrides the process locale (e.g. "pt-BR"). Empty clears override.
func SetLocale(locale string) {
	mu.Lock()
	defer mu.Unlock()
	if strings.TrimSpace(locale) == "" {
		override = "" // normalize("") is "en-US", which would pin it instead
		return
	}
	override = normalize(locale)
}

// Locale returns the active locale: SetLocale override → NITRO_LOCALE →
// LC_ALL/LANG (the POSIX "C"/"POSIX" locales count as unset) → en-US.
func Locale() string {
	mu.RLock()
	o := override
	mu.RUnlock()
	if o != "" {
		return o
	}
	if v := strings.TrimSpace(os.Getenv("NITRO_LOCALE")); v != "" {
		return normalize(v)
	}
	for _, key := range []string{"LC_ALL", "LANG"} {
		if v := strings.TrimSpace(os.Getenv(key)); v != "" {
			return fromEnvLocale(v)
		}
	}
	return "en-US"
}

// WithLocale returns locale if non-empty, otherwise Locale().
func WithLocale(locale string) string {
	if strings.TrimSpace(locale) == "" {
		return Locale()
	}
	return normalize(locale)
}

// SetTimezone sets the default timezone for CombineLocal/Format (IANA name).
func SetTimezone(name string) error {
	if name == "" {
		mu.Lock()
		tzLoc = time.UTC
		mu.Unlock()
		return nil
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return err
	}
	mu.Lock()
	tzLoc = loc
	mu.Unlock()
	return nil
}

// Timezone returns the SetTimezone location, else NITRO_TZ, else time.Local.
func Timezone() *time.Location {
	mu.RLock()
	defer mu.RUnlock()
	if tzLoc != nil {
		return tzLoc
	}
	if v := strings.TrimSpace(os.Getenv("NITRO_TZ")); v != "" {
		if loc, err := time.LoadLocation(v); err == nil {
			return loc
		}
	}
	return time.Local
}

func normalize(locale string) string {
	locale = strings.TrimSpace(locale)
	locale = strings.ReplaceAll(locale, "_", "-")
	if i := strings.IndexByte(locale, '.'); i >= 0 {
		locale = locale[:i]
	}
	if locale == "" {
		return "en-US"
	}
	parts := strings.Split(locale, "-")
	if len(parts) == 1 {
		return strings.ToLower(parts[0])
	}
	return strings.ToLower(parts[0]) + "-" + strings.ToUpper(parts[1])
}

func fromEnvLocale(v string) string {
	// e.g. pt_BR.UTF-8
	v = strings.Split(v, ".")[0]
	if v == "C" || v == "POSIX" || v == "" {
		return "en-US"
	}
	return normalize(v)
}
