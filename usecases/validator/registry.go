package validator

import (
	"fmt"
	"sync"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

// Registry stores ValidatorFunc by name, with aliases and a thread-safe Clone.
type Registry struct {
	mu      sync.RWMutex
	fns     map[string]domain.ValidatorFunc
	aliases map[string]string

	// StrictUnknown, if true, unregistered rules yield ValidationError
	// (Rule: "unknown"). Default true in NewRegistry / NewValidator.
	StrictUnknown bool
}

// NewRegistry creates an empty registry with StrictUnknown=true.
func NewRegistry() *Registry {
	return &Registry{
		fns:           make(map[string]domain.ValidatorFunc),
		aliases:       make(map[string]string),
		StrictUnknown: true,
	}
}

// Register binds a rule name to a function.
// Names are normalized to lowercase.
func (r *Registry) Register(name string, fn domain.ValidatorFunc) {
	if name == "" || fn == nil {
		return
	}
	key := normalizeRuleName(name)
	r.mu.Lock()
	r.fns[key] = fn
	delete(r.aliases, key) // concrete registration drops a same-name alias
	r.mu.Unlock()
}

// RegisterAlias creates an alias for an existing (or future) rule.
// Ex.: RegisterAlias("same", "eqfield").
func (r *Registry) RegisterAlias(alias, target string) {
	if alias == "" || target == "" {
		return
	}
	a, t := normalizeRuleName(alias), normalizeRuleName(target)
	if a == t {
		return
	}
	r.mu.Lock()
	r.aliases[a] = t
	r.mu.Unlock()
}

// Lookup resolves aliases and returns the ValidatorFunc.
func (r *Registry) Lookup(name string) (domain.ValidatorFunc, bool) {
	key := normalizeRuleName(name)
	r.mu.RLock()
	defer r.mu.RUnlock()

	seen := map[string]struct{}{}
	for {
		if _, loop := seen[key]; loop {
			return nil, false
		}
		seen[key] = struct{}{}
		if target, ok := r.aliases[key]; ok {
			key = target
			continue
		}
		fn, ok := r.fns[key]
		return fn, ok
	}
}

// Has reports whether the name (or alias) resolves to a rule.
func (r *Registry) Has(name string) bool {
	_, ok := r.Lookup(name)
	return ok
}

// Call runs rule name with the given context and param.
func (r *Registry) Call(ctx domain.FieldContext, name, param string) error {
	fn, ok := r.Lookup(name)
	if !ok {
		return fmt.Errorf("unknown validation rule: %s", name)
	}
	return fn(ctx, param)
}

// Clone copies functions and aliases into a new instance (snapshot).
func (r *Registry) Clone() *Registry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := NewRegistry()
	out.StrictUnknown = r.StrictUnknown
	for k, v := range r.fns {
		out.fns[k] = v
	}
	for k, v := range r.aliases {
		out.aliases[k] = v
	}
	return out
}

// Names returns concrete registered names (no aliases); order is not guaranteed.
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.fns))
	for k := range r.fns {
		names = append(names, k)
	}
	return names
}

func normalizeRuleName(name string) string {
	// Fast path: tags are already ASCII lowercase → zero alloc.
	needs := false
	for i := 0; i < len(name); i++ {
		c := name[i]
		if c >= 'A' && c <= 'Z' {
			needs = true
			break
		}
	}
	if !needs {
		return name
	}
	b := make([]byte, len(name))
	for i := 0; i < len(name); i++ {
		c := name[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		b[i] = c
	}
	return string(b)
}

// --- default singleton ---

var (
	defaultRegistryMu sync.RWMutex
	defaultRegistry   = NewRegistry()
)

// DefaultRegistry returns the process-wide registry.
func DefaultRegistry() *Registry {
	defaultRegistryMu.RLock()
	defer defaultRegistryMu.RUnlock()
	return defaultRegistry
}

// SetDefaultRegistry replaces the global registry (tests / advanced boot).
func SetDefaultRegistry(r *Registry) {
	if r == nil {
		r = NewRegistry()
	}
	defaultRegistryMu.Lock()
	defaultRegistry = r
	defaultRegistryMu.Unlock()
}

// RegisterValidator registers on the global registry.
func RegisterValidator(name string, fn domain.ValidatorFunc) {
	DefaultRegistry().Register(name, fn)
}

// RegisterValidatorAlias registers an alias on the global registry.
func RegisterValidatorAlias(alias, target string) {
	DefaultRegistry().RegisterAlias(alias, target)
}
