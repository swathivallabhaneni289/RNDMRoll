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
