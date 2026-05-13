package spork

import (
	"testing"
)

// WithEnvDefaults reads three knobs from the environment in a defined
// priority order. These tests pin that order, the quote/whitespace
// trimming inherited from WithAPIKey, the "empty value is a no-op"
// behaviour, and the option-ordering composition rule. Use t.Setenv so
// each test gets its own isolated environment (Go restores the parent
// process's env automatically on return).

func TestWithEnvDefaults_AllThreeSet(t *testing.T) {
	t.Setenv("SPORK_API_KEY", "sk_from_env")
	t.Setenv("SPORK_ORGANIZATION_ID", "org_from_env")
	t.Setenv("SPORK_BASE_URL", "https://staging.example/v1")

	c := NewClient(WithEnvDefaults())
	if c.token != "sk_from_env" {
		t.Errorf("token = %q, want %q", c.token, "sk_from_env")
	}
	if c.organizationID != "org_from_env" {
		t.Errorf("organizationID = %q, want %q", c.organizationID, "org_from_env")
	}
	if c.baseURL != "https://staging.example/v1" {
		t.Errorf("baseURL = %q, want %q", c.baseURL, "https://staging.example/v1")
	}
}

func TestWithEnvDefaults_OrgFallbackPriority(t *testing.T) {
	// The longer SPORK_ORGANIZATION_ID wins over the legacy SPORK_ORG_ID
	// when both are set, so a user mid-migration sees deterministic
	// behaviour. The CLI uses SPORK_ORG_ID historically; both work.
	t.Setenv("SPORK_API_KEY", "sk_x")
	t.Setenv("SPORK_ORG_ID", "org_legacy")
	t.Setenv("SPORK_ORGANIZATION_ID", "org_new")

	c := NewClient(WithEnvDefaults())
	if c.organizationID != "org_new" {
		t.Errorf("organizationID = %q, want %q (SPORK_ORGANIZATION_ID should win over SPORK_ORG_ID)", c.organizationID, "org_new")
	}
}

func TestWithEnvDefaults_LegacyOrgFallback(t *testing.T) {
	// SPORK_ORG_ID alone (the CLI's historical name) must still be picked up.
	t.Setenv("SPORK_API_KEY", "sk_x")
	t.Setenv("SPORK_ORG_ID", "org_legacy")
	// SPORK_ORGANIZATION_ID intentionally unset.

	c := NewClient(WithEnvDefaults())
	if c.organizationID != "org_legacy" {
		t.Errorf("organizationID = %q, want %q (legacy SPORK_ORG_ID should be used when SPORK_ORGANIZATION_ID is unset)", c.organizationID, "org_legacy")
	}
}

func TestWithEnvDefaults_EmptyEnvIsNoOp(t *testing.T) {
	// An unset (or empty) env var must leave the value at its
	// constructor default — not blank it out. Without this, a partially
	// configured env would silently clobber an earlier explicit option.
	t.Setenv("SPORK_API_KEY", "")
	t.Setenv("SPORK_ORGANIZATION_ID", "")
	t.Setenv("SPORK_BASE_URL", "")

	c := NewClient(WithEnvDefaults())
	if c.token != "" {
		t.Errorf("token = %q, want empty (no env, no key)", c.token)
	}
	if c.organizationID != "" {
		t.Errorf("organizationID = %q, want empty", c.organizationID)
	}
	if c.baseURL != DefaultBaseURL {
		t.Errorf("baseURL = %q, want default %q (empty env must not blank the default)", c.baseURL, DefaultBaseURL)
	}
}

func TestWithEnvDefaults_LaterOptionWins(t *testing.T) {
	// Composition rule: env defaults come first, explicit options after
	// override. This is what lets a user write `WithEnvDefaults(),
	// WithOrganization("org_pinned")` and pin one knob while inheriting
	// the rest from the environment.
	t.Setenv("SPORK_API_KEY", "sk_from_env")
	t.Setenv("SPORK_ORGANIZATION_ID", "org_from_env")

	c := NewClient(
		WithEnvDefaults(),
		WithOrganization("org_explicit"),
		WithAPIKey("sk_explicit"),
	)
	if c.organizationID != "org_explicit" {
		t.Errorf("organizationID = %q, want %q (later option must win)", c.organizationID, "org_explicit")
	}
	if c.token != "sk_explicit" {
		t.Errorf("token = %q, want %q (later option must win)", c.token, "sk_explicit")
	}
}

func TestWithEnvDefaults_APIKeyTrimsQuotes(t *testing.T) {
	// SPORK_API_KEY="\"sk_xxx\"" is the most common env paste-fail in
	// the wild. WithEnvDefaults routes through WithAPIKey so the same
	// trim-quotes hardening applies; pin that here.
	t.Setenv("SPORK_API_KEY", `  "sk_real_key"  `)

	c := NewClient(WithEnvDefaults())
	if c.token != "sk_real_key" {
		t.Errorf("token = %q, want %q (env quotes + whitespace must be trimmed)", c.token, "sk_real_key")
	}
}
