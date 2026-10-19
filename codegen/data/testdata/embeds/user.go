package embeds

import nitro "github.com/rodrigocelsius/go-nitro/pkg/v2"

type Member struct {
	ScopedModel
	Email string            `json:"email"`
	Meta  nitro.Attrs       `json:"meta"`
	Tags  nitro.StringSlice `json:"tags"`
}

type Order struct {
	nitro.TenantModel
	Title string `json:"title"`
}
