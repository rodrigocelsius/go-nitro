package fasthttp

import "testing"

// TestOriginTrusted_RejectsPrefixBypass covers the CSRF origin-check bypass
// fixed in vuln.md (V-07): strings.HasPrefix let an attacker-registered
// domain like "https://app.example.com.evil.com" pass a trusted entry of
// "https://app.example.com", defeating the primary CSRF defense of the
// Web BFF (CWE-346, origin validation error).
func TestOriginTrusted_RejectsPrefixBypass(t *testing.T) {
	trusted := []string{"https://app.example.com"}

	tests := []struct {
		name   string
		origin string
		want   bool
	}{
		{"exact match", "https://app.example.com", true},
		{"suffix-appended attacker domain", "https://app.example.com.evil.com", false},
		{"different scheme", "http://app.example.com", false},
		{"different port", "https://app.example.com:8443", false},
		{"unrelated domain", "https://evil.com", false},
		{"empty origin", "", false},
	}
	for _, tt := range tests {
		if got := originTrusted(tt.origin, trusted); got != tt.want {
			t.Errorf("originTrusted(%q) = %v, want %v", tt.origin, got, tt.want)
		}
	}
}

// TestPathMatchesIgnorePrefix_RequiresSegmentBoundary covers V-16:
// CSRFConfig.IgnorePaths previously used strings.HasPrefix without a
// segment boundary, so an entry of "/webhooks" also exempted an unrelated
// sibling path like "/webhooks-admin/delete-all" from CSRF protection.
func TestPathMatchesIgnorePrefix_RequiresSegmentBoundary(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		ignored string
		want    bool
	}{
		{"exact match", "/webhooks", "/webhooks", true},
		{"nested path under ignored dir", "/webhooks/stripe", "/webhooks", true},
		{"glob suffix nested path", "/webhooks/stripe", "/webhooks/*", true},
		{"sibling path sharing string prefix", "/webhooks-admin/delete-all", "/webhooks", false},
		{"sibling path sharing string prefix with glob", "/webhooks-admin/delete-all", "/webhooks/*", false},
		{"unrelated path", "/other", "/webhooks", false},
	}
	for _, tt := range tests {
		if got := pathMatchesIgnorePrefix(tt.path, tt.ignored); got != tt.want {
			t.Errorf("pathMatchesIgnorePrefix(%q, %q) = %v, want %v", tt.path, tt.ignored, got, tt.want)
		}
	}
}

func TestOriginTrusted_Wildcard(t *testing.T) {
	trusted := []string{"*"}
	if !originTrusted("https://anything.example.com", trusted) {
		t.Fatal("\"*\" should trust any non-empty origin")
	}
	if originTrusted("", trusted) {
		t.Fatal("empty origin must never be trusted, even with \"*\"")
	}
}

func TestOriginTrusted_SubdomainWildcard(t *testing.T) {
	trusted := []string{"*.example.com"}

	tests := []struct {
		name   string
		origin string
		want   bool
	}{
		{"matching subdomain", "https://app.example.com", true},
		{"apex domain also matches", "https://example.com", true},
		{"attacker suffix domain", "https://app.example.com.evil.com", false},
		{"unrelated domain", "https://example.org", false},
	}
	for _, tt := range tests {
		if got := originTrusted(tt.origin, trusted); got != tt.want {
			t.Errorf("originTrusted(%q) = %v, want %v", tt.origin, got, tt.want)
		}
	}
}
