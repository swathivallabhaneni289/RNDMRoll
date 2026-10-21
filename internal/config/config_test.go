package config

import (
	"strings"
	"testing"
	"time"
)

// setRequiredEnv sets every required environment variable to a valid value
// and explicitly clears every optional/defaulted key, so ambient shell
// environment variables cannot leak into a test's expectations. Individual
// tests override specific keys after calling this to exercise one behavior.
func setRequiredEnv(t *testing.T) {
	t.Helper()
	vars := map[string]string{
		"DATABASE_URL":         "postgres://localhost:5432/rndmroll_dev?sslmode=disable",
		"JWT_SECRET":           strings.Repeat("a", 32),
		"APP_BASE_URL":         "https://rndmroll.example.com",
		"MAIL_DRIVER":          "resend",
		"RESEND_API_KEY":       "re_test_key",
		"MAIL_FROM_ADDRESS":    "onboarding@resend.dev",
		"S3_ENDPOINT":          "https://s3.example.com",
		"S3_BUCKET":            "rndmroll-avatars",
		"S3_ACCESS_KEY_ID":     "test-access-key",
		"S3_SECRET_ACCESS_KEY": "test-secret-key",
		"S3_PUBLIC_BASE_URL":   "https://cdn.example.com",
		// Optional/defaulted keys explicitly cleared.
		"PORT":                 "",
		"TEST_DATABASE_URL":    "",
		"ACCESS_TOKEN_TTL":     "",
		"REFRESH_TOKEN_TTL":    "",
		"EMAIL_TOKEN_TTL":      "",
		"APP_DEEP_LINK_SCHEME": "",
		"MAIL_FROM_NAME":       "",
		"S3_REGION":            "",
		// The spin test-tool settings are cleared so a shell variable cannot
		// turn the tools on inside a test.
		"APP_ENV":               "",
		"SPIN_TEST_TOOLS":       "",
		"SPIN_TEST_USER_EMAILS": "",
	}
	for k, v := range vars {
		t.Setenv(k, v)
	}
}

func TestLoad_SucceedsWhenAllRequiredPresent(t *testing.T) {
	setRequiredEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned unexpected error: %v", err)
	}
	if cfg.DatabaseURL != "postgres://localhost:5432/rndmroll_dev?sslmode=disable" {
		t.Errorf("DatabaseURL = %q, want the value set via t.Setenv", cfg.DatabaseURL)
	}
	if cfg.Port != "8080" {
		t.Errorf("Port default = %q, want 8080", cfg.Port)
	}
	if cfg.MailFromName != "RNDMRoll" {
		t.Errorf("MailFromName default = %q, want RNDMRoll", cfg.MailFromName)
	}
	if cfg.DeepLinkScheme != "rndmroll" {
		t.Errorf("DeepLinkScheme default = %q, want rndmroll", cfg.DeepLinkScheme)
	}
	if cfg.S3Region != "auto" {
		t.Errorf("S3Region default = %q, want auto", cfg.S3Region)
	}
}

func TestLoad_AccumulatesAllMissingRequiredVars(t *testing.T) {
	// Clear every key Load() reads so nothing leaks in from the ambient
	// environment: this proves accumulation across many missing keys at once.
	allKeys := []string{
		"PORT", "DATABASE_URL", "TEST_DATABASE_URL", "JWT_SECRET",
		"ACCESS_TOKEN_TTL", "REFRESH_TOKEN_TTL", "EMAIL_TOKEN_TTL",
		"APP_BASE_URL", "APP_DEEP_LINK_SCHEME", "MAIL_DRIVER", "RESEND_API_KEY",
		"MAIL_FROM_ADDRESS", "MAIL_FROM_NAME", "S3_ENDPOINT", "S3_REGION", "S3_BUCKET",
		"S3_ACCESS_KEY_ID", "S3_SECRET_ACCESS_KEY", "S3_PUBLIC_BASE_URL",
		"APP_ENV", "SPIN_TEST_TOOLS", "SPIN_TEST_USER_EMAILS",
	}
	for _, k := range allKeys {
		t.Setenv(k, "")
	}

	_, err := Load()
	if err == nil {
		t.Fatal("Load() with no required vars set: want error, got nil")
	}

	wantMissing := []string{"DATABASE_URL", "JWT_SECRET", "APP_BASE_URL"}
	found := 0
	for _, key := range wantMissing {
		if strings.Contains(err.Error(), key) {
			found++
		}
	}
	if found < 2 {
		t.Fatalf("Load() error = %q, want at least 2 of %v named (accumulation, not first-failure)", err.Error(), wantMissing)
	}
}

func TestLoad_ParsesTokenTTLDurationsWithDefaults(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("ACCESS_TOKEN_TTL", "")
	t.Setenv("REFRESH_TOKEN_TTL", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned unexpected error: %v", err)
	}
	if cfg.AccessTokenTTL != 15*time.Minute {
		t.Errorf("AccessTokenTTL default = %v, want 15m", cfg.AccessTokenTTL)
	}
	if cfg.RefreshTokenTTL != 720*time.Hour {
		t.Errorf("RefreshTokenTTL default = %v, want 720h", cfg.RefreshTokenTTL)
	}

	t.Setenv("ACCESS_TOKEN_TTL", "30m")
	t.Setenv("REFRESH_TOKEN_TTL", "168h")

	cfg2, err := Load()
	if err != nil {
		t.Fatalf("Load() returned unexpected error: %v", err)
	}
	if cfg2.AccessTokenTTL != 30*time.Minute {
		t.Errorf("AccessTokenTTL = %v, want 30m", cfg2.AccessTokenTTL)
	}
	if cfg2.RefreshTokenTTL != 168*time.Hour {
		t.Errorf("RefreshTokenTTL = %v, want 168h", cfg2.RefreshTokenTTL)
	}
}

func TestLoad_RejectsShortJWTSecret(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("JWT_SECRET", "too-short")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() with a 9-byte JWT_SECRET: want error, got nil")
	}
	if !strings.Contains(err.Error(), "JWT_SECRET") {
		t.Errorf("Load() error = %q, want it to name JWT_SECRET", err.Error())
	}
}

func TestLoad_RejectsInvalidMailDriver(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("MAIL_DRIVER", "sendgrid")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() with MAIL_DRIVER=sendgrid: want error, got nil")
	}
	if !strings.Contains(err.Error(), "MAIL_DRIVER") {
		t.Errorf("Load() error = %q, want it to name MAIL_DRIVER", err.Error())
	}
}

func TestLoad_MailDriverLogDoesNotRequireResendAPIKey(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("MAIL_DRIVER", "log")
	t.Setenv("RESEND_API_KEY", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() with MAIL_DRIVER=log and no RESEND_API_KEY: want success, got error: %v", err)
	}
	if cfg.MailDriver != "log" {
		t.Errorf("MailDriver = %q, want log", cfg.MailDriver)
	}
}

// --- spin test tools ---

func TestLoad_AppEnvDefaultsToProductionAndRejectsOtherValues(t *testing.T) {
	setRequiredEnv(t)
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}
	if cfg.AppEnv != "production" {
		t.Errorf("AppEnv default = %q, want production", cfg.AppEnv)
	}
	if cfg.SpinTestTools {
		t.Error("SpinTestTools is on by default")
	}
	if len(cfg.SpinTestUserEmails) != 0 {
		t.Errorf("SpinTestUserEmails = %v, want none", cfg.SpinTestUserEmails)
	}

	for _, env := range []string{"production", "development", "test"} {
		t.Run(env, func(t *testing.T) {
			setRequiredEnv(t)
			t.Setenv("APP_ENV", env)
			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load() with APP_ENV=%s: %v", env, err)
			}
			if cfg.AppEnv != env {
				t.Errorf("AppEnv = %q, want %q", cfg.AppEnv, env)
			}
		})
	}

	setRequiredEnv(t)
	t.Setenv("APP_ENV", "staging")
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "APP_ENV") {
		t.Fatalf("Load() with APP_ENV=staging: got %v, want an error naming APP_ENV", err)
	}
}

func TestLoad_TestToolsOutsideTestModeFailWithTheToolsMessage(t *testing.T) {
	for _, env := range []string{"", "production", "development"} {
		name := env
		if name == "" {
			name = "unset"
		}
		t.Run("APP_ENV "+name, func(t *testing.T) {
			setRequiredEnv(t)
			t.Setenv("APP_ENV", env)
			t.Setenv("SPIN_TEST_TOOLS", "1")
			t.Setenv("SPIN_TEST_USER_EMAILS", "throwaway@example.com")
			_, err := Load()
			if err == nil {
				t.Fatal("Load() with the tools on outside test mode: want an error, got nil")
			}
			if !strings.Contains(err.Error(), "SPIN_TEST_TOOLS") || !strings.Contains(err.Error(), "APP_ENV=test") {
				t.Errorf("error = %q, want the tools message naming SPIN_TEST_TOOLS and APP_ENV=test", err.Error())
			}
		})
	}
}

func TestLoad_TestToolsLoadWithTestModeAListAndADevDatabase(t *testing.T) {
	setRequiredEnv(t) // DATABASE_URL names rndmroll_dev
	t.Setenv("APP_ENV", "test")
	t.Setenv("SPIN_TEST_TOOLS", "1")
	t.Setenv("SPIN_TEST_USER_EMAILS", " Cutover-P2@Example.com , walkthrough-p2@example.com,, ")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}
	if !cfg.SpinTestTools {
		t.Error("SpinTestTools = false, want true")
	}
	want := []string{"cutover-p2@example.com", "walkthrough-p2@example.com"}
	if strings.Join(cfg.SpinTestUserEmails, "|") != strings.Join(want, "|") {
		t.Errorf("SpinTestUserEmails = %v, want %v (trimmed, lower-cased, no empty items)", cfg.SpinTestUserEmails, want)
	}

	// "true" is the other way to say on, and a _test database is allowed too.
	t.Setenv("SPIN_TEST_TOOLS", "true")
	t.Setenv("DATABASE_URL", "postgres://localhost:5432/rndmroll_test?sslmode=disable")
	if cfg, err := Load(); err != nil || !cfg.SpinTestTools {
		t.Fatalf("Load() with true and a _test database: cfg on = %v, err = %v", cfg != nil && cfg.SpinTestTools, err)
	}
}

func TestLoad_TestToolsNeedAListAndADevOrTestDatabase(t *testing.T) {
	cases := map[string]struct {
		databaseURL string
		emails      string
		wantInError string
	}{
		"empty list":              {"postgres://localhost:5432/rndmroll_dev?sslmode=disable", "", "SPIN_TEST_USER_EMAILS"},
		"only commas in the list": {"postgres://localhost:5432/rndmroll_dev?sslmode=disable", " , ,", "SPIN_TEST_USER_EMAILS"},
		"another database name":   {"postgres://localhost:5432/rndmroll?sslmode=disable", "a@example.com", "_dev or _test"},
		"name only contains dev":  {"postgres://localhost:5432/rndmroll_devices?sslmode=disable", "a@example.com", "_dev or _test"},
		"production name":         {"postgres://localhost:5432/rndmroll_prod?sslmode=disable", "a@example.com", "_dev or _test"},
		"dbname parameter wins":   {"postgres://localhost:5432/rndmroll_dev?dbname=rndmroll_prod", "a@example.com", "_dev or _test"},
		"key value form, bad":     {"host=localhost dbname=rndmroll_prod", "a@example.com", "_dev or _test"},
		"no database name":        {"postgres://localhost:5432", "a@example.com", "_dev or _test"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			setRequiredEnv(t)
			t.Setenv("APP_ENV", "test")
			t.Setenv("SPIN_TEST_TOOLS", "1")
			t.Setenv("DATABASE_URL", c.databaseURL)
			t.Setenv("SPIN_TEST_USER_EMAILS", c.emails)
			_, err := Load()
			if err == nil {
				t.Fatal("Load(): want an error, got nil")
			}
			if !strings.Contains(err.Error(), c.wantInError) {
				t.Errorf("error = %q, want it to contain %q", err.Error(), c.wantInError)
			}
		})
	}

	// The key=value form with a _dev name is read correctly.
	setRequiredEnv(t)
	t.Setenv("APP_ENV", "test")
	t.Setenv("SPIN_TEST_TOOLS", "1")
	t.Setenv("DATABASE_URL", "host=localhost dbname=rndmroll_dev user=me")
	t.Setenv("SPIN_TEST_USER_EMAILS", "a@example.com")
	if _, err := Load(); err != nil {
		t.Fatalf("key=value DATABASE_URL naming a _dev database: %v", err)
	}
}

func TestLoad_TestToolsOffLoadsWithAnyAppEnvAndRejectsOddSwitchValues(t *testing.T) {
	for _, off := range []string{"", "0", "false"} {
		t.Run("off="+off, func(t *testing.T) {
			setRequiredEnv(t)
			t.Setenv("SPIN_TEST_TOOLS", off)
			t.Setenv("DATABASE_URL", "postgres://localhost:5432/rndmroll_prod?sslmode=disable")
			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load() with the tools off: %v", err)
			}
			if cfg.SpinTestTools {
				t.Error("SpinTestTools = true with the switch off")
			}
		})
	}

	for _, odd := range []string{"yes", "on", "TRUE", "2", "enabled"} {
		t.Run("odd="+odd, func(t *testing.T) {
			setRequiredEnv(t)
			t.Setenv("SPIN_TEST_TOOLS", odd)
			_, err := Load()
			if err == nil {
				t.Fatalf("Load() with SPIN_TEST_TOOLS=%s: want an error, got nil", odd)
			}
			if !strings.Contains(err.Error(), "SPIN_TEST_TOOLS") {
				t.Errorf("error = %q, want it to name SPIN_TEST_TOOLS", err.Error())
			}
		})
	}
}

func TestLoad_PushDriverDefaultsToOffAndRejectsUnknown(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("PUSH_DRIVER", "")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}
	if cfg.PushDriver != "off" {
		t.Errorf("PushDriver = %q, want off", cfg.PushDriver)
	}

	t.Setenv("PUSH_DRIVER", "expo")
	t.Setenv("EXPO_ACCESS_TOKEN", "abc")
	cfg, err = Load()
	if err != nil {
		t.Fatalf("Load() with expo: %v", err)
	}
	if cfg.PushDriver != "expo" || cfg.ExpoAccessToken != "abc" {
		t.Errorf("got %q %q", cfg.PushDriver, cfg.ExpoAccessToken)
	}

	t.Setenv("PUSH_DRIVER", "apns")
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "PUSH_DRIVER") {
		t.Errorf("Load() with PUSH_DRIVER=apns: want an error naming PUSH_DRIVER, got %v", err)
	}
}
