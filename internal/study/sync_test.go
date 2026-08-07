package study

import (
	"os"
	"testing"
)

func TestResolveCloudSyncURL(t *testing.T) {
	os.Unsetenv("CLOUD_SYNC_URL")
	os.Unsetenv("SUPABASE_URL")
	os.Unsetenv("VITE_SUPABASE_URL")

	// 1. Explicit stored URL
	if got := ResolveCloudSyncURL("https://custom.example.com/sync"); got != "https://custom.example.com/sync" {
		t.Errorf("expected stored URL, got %q", got)
	}

	// 2. CLOUD_SYNC_URL env var
	os.Setenv("CLOUD_SYNC_URL", "https://env.example.com/sync")
	if got := ResolveCloudSyncURL(""); got != "https://env.example.com/sync" {
		t.Errorf("expected CLOUD_SYNC_URL env var, got %q", got)
	}
	os.Unsetenv("CLOUD_SYNC_URL")

	// 3. SUPABASE_URL env var
	os.Setenv("SUPABASE_URL", "https://xyz.supabase.co")
	if got := ResolveCloudSyncURL(""); got != "https://xyz.supabase.co/rest/v1/rpc/handle_cloud_sync" {
		t.Errorf("expected SUPABASE_URL with rpc path appended, got %q", got)
	}
	os.Unsetenv("SUPABASE_URL")

	// 4. Default fallback
	if got := ResolveCloudSyncURL(""); got != DefaultProductionSyncURL {
		t.Errorf("expected default fallback, got %q", got)
	}
}

func TestResolveAnonKeyAndToken(t *testing.T) {
	os.Unsetenv("CLOUD_API_TOKEN")
	os.Unsetenv("SUPABASE_ANON_KEY")
	os.Unsetenv("SUPABASE_PUBLISHABLE_KEY")
	os.Unsetenv("VITE_SUPABASE_ANON_KEY")

	// 1. Stored user token
	if got := ResolveCloudAPIToken("user-jwt-token"); got != "user-jwt-token" {
		t.Errorf("expected stored token, got %q", got)
	}

	// 2. SUPABASE_PUBLISHABLE_KEY for ResolveAnonKey
	os.Setenv("SUPABASE_PUBLISHABLE_KEY", "sb_pub_456")
	if got := ResolveAnonKey(); got != "sb_pub_456" {
		t.Errorf("expected SUPABASE_PUBLISHABLE_KEY, got %q", got)
	}

	// 3. Fallback when storedToken is empty uses ResolveAnonKey
	if got := ResolveCloudAPIToken(""); got != "sb_pub_456" {
		t.Errorf("expected anon key as fallback for empty stored token, got %q", got)
	}
	os.Unsetenv("SUPABASE_PUBLISHABLE_KEY")

	// 4. Default fallback
	if got := ResolveAnonKey(); got != DefaultProductionAnonKey {
		t.Errorf("expected default fallback, got %q", got)
	}
}
