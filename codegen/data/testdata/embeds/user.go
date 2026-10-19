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

type Ledger struct {
	nitro.HardModel
	Amount int `json:"amount"`
}

func (Ledger) TableName() string { return "finance_ledger" }

//nitro:table=comment_wins
type Audit struct {
	nitro.HardModel
	Note string `json:"note"`
}

func (*Audit) TableName() string { return "ignored" }
