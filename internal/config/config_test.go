package config_test

import (
	"strings"
	"testing"

	"kanban/internal/config"
)

const hexKey = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"

func env(overrides map[string]string) func(string) string {
	base := map[string]string{
		"BASE_URL":           "https://pano.example.com/",
		"DATABASE_URL":       "postgres://u:p@localhost:5432/kanban",
		"DEFAULT_LOCALE":     "tr",
		"OIDC_ISSUER":        "https://auth.example.com/application/o/kanban/",
		"OIDC_CLIENT_ID":     "kanban",
		"OIDC_CLIENT_SECRET": "secret",
		"OIDC_REDIRECT_URL":  "https://pano.example.com/auth/openid/authentik",
		"ADMIN_EMAILS":       " Alice@Example.com ,,bob@example.com",
		"SESSION_KEY":        hexKey,
		"CSRF_KEY":           hexKey,
		"FLASH_KEY":          hexKey,
	}
	for k, v := range overrides {
		base[k] = v
	}
	return func(k string) string { return base[k] }
}

func TestLoadValid(t *testing.T) {
	cfg, err := config.Load(env(nil))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.BaseURL != "https://pano.example.com" {
		t.Errorf("BaseURL = %q, want trailing slash trimmed", cfg.BaseURL)
	}
	if got := strings.Join(cfg.AdminEmails, ","); got != "alice@example.com,bob@example.com" {
		t.Errorf("AdminEmails = %q", got)
	}
	if got := strings.Join(cfg.OIDC.Scopes, " "); got != "openid email profile" {
		t.Errorf("Scopes = %q, want the default", got)
	}
	if cfg.OIDC.CallbackPath != "/auth/openid/authentik" {
		t.Errorf("CallbackPath = %q", cfg.OIDC.CallbackPath)
	}
	if len(cfg.SessionKey) != 32 {
		t.Errorf("SessionKey is %d bytes, want 32", len(cfg.SessionKey))
	}
}

func TestLoadDefaultsLocaleToTurkish(t *testing.T) {
	cfg, err := config.Load(env(map[string]string{"DEFAULT_LOCALE": ""}))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.DefaultLocale != "tr" {
		t.Errorf("DefaultLocale = %q, want tr", cfg.DefaultLocale)
	}
}

func TestLoadReportsEveryProblemByName(t *testing.T) {
	_, err := config.Load(func(string) string { return "" })
	if err == nil {
		t.Fatal("Load with an empty environment succeeded")
	}
	for _, name := range []string{"BASE_URL", "DATABASE_URL", "OIDC_ISSUER", "OIDC_CLIENT_ID", "OIDC_CLIENT_SECRET", "OIDC_REDIRECT_URL", "SESSION_KEY", "CSRF_KEY", "FLASH_KEY"} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("error does not name %s:\n%v", name, err)
		}
	}
}

func TestLoadRejectsBadValues(t *testing.T) {
	cases := map[string]map[string]string{
		"BASE_URL":                       {"BASE_URL": "pano.example.com"},
		"BASE_URL path":                  {"BASE_URL": "https://example.com/kanban"},
		"DEFAULT_LOCALE":                 {"DEFAULT_LOCALE": "fr"},
		"OIDC_SCOPES":                    {"OIDC_SCOPES": "email profile"},
		"OIDC_ISSUER":                    {"OIDC_ISSUER": "not a url"},
		"SESSION_KEY":                    {"SESSION_KEY": "zz"},
		"CSRF_KEY short":                 {"CSRF_KEY": "00ff"},
		"OIDC_REDIRECT_URL other origin": {"OIDC_REDIRECT_URL": "https://elsewhere.example.com/auth/openid/authentik"},
		"OIDC_REDIRECT_URL root":         {"OIDC_REDIRECT_URL": "https://pano.example.com/"},
	}
	for name, overrides := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := config.Load(env(overrides))
			if err == nil {
				t.Fatal("Load succeeded")
			}
			want := strings.Fields(name)[0]
			if !strings.Contains(err.Error(), want) {
				t.Errorf("error does not name %s: %v", want, err)
			}
		})
	}
}
