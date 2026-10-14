package domain

// TenantClaimKeys lists, in priority order, the token claims Nitro reads as
// the request tenant (convention over configuration): "tenant_id" is Nitro's
// own, "tenantId" is common in JS-issued tokens, "tid" is the Azure AD tenant
// and "org_id" / "organization_id" are used by Auth0, Clerk and WorkOS.
var TenantClaimKeys = []string{"tenant_id", "tenantId", "tid", "org_id", "organization_id"}

// TenantFromClaims returns the first non-empty tenant claim, looked up through get.
func TenantFromClaims(get func(key string) string) string {
	for _, k := range TenantClaimKeys {
		if v := get(k); v != "" {
			return v
		}
	}
	return ""
}
