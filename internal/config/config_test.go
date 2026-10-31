package config

import (
	"strings"
	"testing"
)

func env(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

func TestDefaults(t *testing.T) {
	c, err := Load(env(map[string]string{
		"DATABASE_URL": "postgres://x", "JWT_SECRET": strings.Repeat("s", 32), "OPENAI_API_KEY": "k",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if c.Addr != ":8080" || c.Explainer != "openai" || c.OpenAIModel != "gpt-4o-mini" ||
		c.OpenAIBaseURL != "https://api.openai.com" || c.DailyLimit != 20 || c.DemoEnabled {
		t.Fatalf("unexpected defaults: %+v", c)
	}
}

func TestValidation(t *testing.T) {
	cases := map[string]map[string]string{
		"missing database":   {"JWT_SECRET": strings.Repeat("s", 32)},
		"short secret":       {"DATABASE_URL": "postgres://x", "JWT_SECRET": "short"},
		"bad explainer":      {"DATABASE_URL": "postgres://x", "JWT_SECRET": strings.Repeat("s", 32), "EXPLAINER": "magic"},
		"bad limit":          {"DATABASE_URL": "postgres://x", "JWT_SECRET": strings.Repeat("s", 32), "DAILY_LIMIT": "x"},
		"openai without key": {"DATABASE_URL": "postgres://x", "JWT_SECRET": strings.Repeat("s", 32), "EXPLAINER": "openai"},
		"openai placeholder key": {"DATABASE_URL": "postgres://x", "JWT_SECRET": strings.Repeat("s", 32),
			"EXPLAINER": "openai", "OPENAI_API_KEY": "not-configured"},
	}
	for name, m := range cases {
		if _, err := Load(env(m)); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}

func TestFakeNeedsNoKey(t *testing.T) {
	if _, err := Load(env(map[string]string{"DATABASE_URL": "postgres://x", "JWT_SECRET": strings.Repeat("s", 32),
		"EXPLAINER": "fake", "OPENAI_API_KEY": "not-configured"})); err != nil {
		t.Fatal(err)
	}
}

func TestOverrides(t *testing.T) {
	c, err := Load(env(map[string]string{
		"DATABASE_URL": "postgres://x", "JWT_SECRET": strings.Repeat("s", 32), "EXPLAINER": "fake",
		"DEMO_ENABLED": "true", "DAILY_LIMIT": "5", "ADMIN_EMAIL": "a@b.de", "ADMIN_PASSWORD": "p",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if c.Explainer != "fake" || !c.DemoEnabled || c.DailyLimit != 5 || c.AdminEmail != "a@b.de" {
		t.Fatalf("overrides ignored: %+v", c)
	}
}

func TestAppOrigin(t *testing.T) {
	base := func(origin string) map[string]string {
		return map[string]string{"DATABASE_URL": "postgres://x", "JWT_SECRET": strings.Repeat("s", 32), "EXPLAINER": "fake",
			"APP_ORIGIN": origin}
	}
	for _, ok := range []string{"", "https://app.briefklar.omarfourati.de", "http://app.localhost:8080", "https://app.briefklar.omarfourati.de/"} {
		c, err := Load(env(base(ok)))
		if err != nil {
			t.Errorf("%q: %v", ok, err)
			continue
		}
		if want := strings.TrimSuffix(ok, "/"); c.AppOrigin != want {
			t.Errorf("%q: AppOrigin %q, want %q", ok, c.AppOrigin, want)
		}
	}
	for _, bad := range []string{"app.briefklar.omarfourati.de", "ftp://app.x.de", "https://", "https://app.x.de/app", "https://app.x.de?x=1"} {
		if _, err := Load(env(base(bad))); err == nil {
			t.Errorf("%q: expected error", bad)
		}
	}
}
