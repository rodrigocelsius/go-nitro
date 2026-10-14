package nitro

// TenantModel embeds Model plus TenantID for shared-schema multi-tenancy.
//
//	type Customer struct {
//	    nitro.TenantModel
//	    Name  string      `json:"name" db:"name"`
//	    Attrs nitro.Attrs `json:"attrs" db:"attrs,jsonb" blinq:"gin"`
//	}
type TenantModel struct {
	Model
	TenantID UUID `json:"tenant_id" db:"tenant_id" blinq:"index" readonly:"true"`
}
