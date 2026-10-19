package embeds

import nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"

// ScopedModel is a local base declared in a sibling file.
type ScopedModel struct {
	nitro.HardModel
	ScopeID nitro.UUID `json:"scope_id" blinq:"index"`
}
