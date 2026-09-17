// Package config loads the backend's typed, fail-fast environment
// configuration. Load reports every missing required variable at once so an
// operator can fix them all in a single pass instead of one at a time.
package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// minJWTSecretBytes is the minimum length required for JWT_SECRET, chosen to
// keep the HS256 signing key from being trivially brute-forceable.
const minJWTSecretBytes = 32

// Config holds every environment-derived setting the backend needs to run.
type Config struct {
	Port            string
	DatabaseURL     string
	TestDatabaseURL string
	JWTSecret       []byte

	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
	EmailTokenTTL   time.Duration

	AppBaseURL     string
	DeepLinkScheme string

	ResendAPIKey    string
	MailDriver      string
	MailFromAddress string
	MailFromName    string

	GoogleClientIDIOS     string
	GoogleClientIDAndroid string
	GoogleClientIDWeb     string

	AppleBundleID  string
	AppleServiceID string

	S3Endpoint        string
	S3Region          string
	S3Bucket          string
	S3AccessKeyID     string
	S3SecretAccessKey string
	S3PublicBaseURL   string
}

// Load reads configuration from the process environment. If any required
// variable is missing, it returns a single error naming all of them, not
// just the first one encountered.
func Load() (*Config, error) {
	var missing []string
	requireVar := func(key, value string) {
		if value == "" {
			missing = append(missing, key)
		}
	}

	port := withDefault(os.Getenv("PORT"), "8080")

	databaseURL := os.Getenv("DATABASE_URL")
	requireVar("DATABASE_URL", databaseURL)

	testDatabaseURL := os.Getenv("TEST_DATABASE_URL")

	jwtSecretRaw := os.Getenv("JWT_SECRET")
	requireVar("JWT_SECRET", jwtSecretRaw)

	appBaseURL := os.Getenv("APP_BASE_URL")
	requireVar("APP_BASE_URL", appBaseURL)

	deepLinkScheme := withDefault(os.Getenv("APP_DEEP_LINK_SCHEME"), "rndmroll")

	mailDriver := withDefault(os.Getenv("MAIL_DRIVER"), "resend")

	resendAPIKey := os.Getenv("RESEND_API_KEY")
	if mailDriver != "log" {
		requireVar("RESEND_API_KEY", resendAPIKey)
	}

	mailFromAddress := os.Getenv("MAIL_FROM_ADDRESS")
	requireVar("MAIL_FROM_ADDRESS", mailFromAddress)

	mailFromName := withDefault(os.Getenv("MAIL_FROM_NAME"), "RNDMRoll")

	googleClientIDIOS := os.Getenv("GOOGLE_CLIENT_ID_IOS")
	requireVar("GOOGLE_CLIENT_ID_IOS", googleClientIDIOS)

	googleClientIDAndroid := os.Getenv("GOOGLE_CLIENT_ID_ANDROID")
	requireVar("GOOGLE_CLIENT_ID_ANDROID", googleClientIDAndroid)

	googleClientIDWeb := os.Getenv("GOOGLE_CLIENT_ID_WEB")
	requireVar("GOOGLE_CLIENT_ID_WEB", googleClientIDWeb)

	appleBundleID := os.Getenv("APPLE_BUNDLE_ID")
	requireVar("APPLE_BUNDLE_ID", appleBundleID)

	appleServiceID := os.Getenv("APPLE_SERVICE_ID")

	s3Endpoint := os.Getenv("S3_ENDPOINT")
	requireVar("S3_ENDPOINT", s3Endpoint)

	s3Region := withDefault(os.Getenv("S3_REGION"), "auto")

	s3Bucket := os.Getenv("S3_BUCKET")
	requireVar("S3_BUCKET", s3Bucket)

	s3AccessKeyID := os.Getenv("S3_ACCESS_KEY_ID")
	requireVar("S3_ACCESS_KEY_ID", s3AccessKeyID)

	s3SecretAccessKey := os.Getenv("S3_SECRET_ACCESS_KEY")
	requireVar("S3_SECRET_ACCESS_KEY", s3SecretAccessKey)

	s3PublicBaseURL := os.Getenv("S3_PUBLIC_BASE_URL")
	requireVar("S3_PUBLIC_BASE_URL", s3PublicBaseURL)

	if len(missing) > 0 {
		return nil, fmt.Errorf("config: missing required environment variables: %s", strings.Join(missing, ", "))
	}

	accessTokenTTL, err := parseDurationDefault("ACCESS_TOKEN_TTL", os.Getenv("ACCESS_TOKEN_TTL"), 15*time.Minute)
	if err != nil {
		return nil, err
	}
	refreshTokenTTL, err := parseDurationDefault("REFRESH_TOKEN_TTL", os.Getenv("REFRESH_TOKEN_TTL"), 720*time.Hour)
	if err != nil {
		return nil, err
	}
	emailTokenTTL, err := parseDurationDefault("EMAIL_TOKEN_TTL", os.Getenv("EMAIL_TOKEN_TTL"), 24*time.Hour)
	if err != nil {
		return nil, err
	}

	if len(jwtSecretRaw) < minJWTSecretBytes {
		return nil, fmt.Errorf("config: JWT_SECRET must be at least %d bytes, got %d", minJWTSecretBytes, len(jwtSecretRaw))
	}

	if mailDriver != "resend" && mailDriver != "log" {
		return nil, fmt.Errorf("config: MAIL_DRIVER must be %q or %q, got %q", "resend", "log", mailDriver)
	}

	// APPLE_SERVICE_ID is only needed for a web/Android Apple flow; an
	// iOS-only setup defaults it to the App ID (APPLE_BUNDLE_ID).
	if appleServiceID == "" {
		appleServiceID = appleBundleID
	}

	return &Config{
		Port:            port,
		DatabaseURL:     databaseURL,
		TestDatabaseURL: testDatabaseURL,
		JWTSecret:       []byte(jwtSecretRaw),

		AccessTokenTTL:  accessTokenTTL,
		RefreshTokenTTL: refreshTokenTTL,
		EmailTokenTTL:   emailTokenTTL,

		AppBaseURL:     appBaseURL,
		DeepLinkScheme: deepLinkScheme,

		ResendAPIKey:    resendAPIKey,
		MailDriver:      mailDriver,
		MailFromAddress: mailFromAddress,
		MailFromName:    mailFromName,

		GoogleClientIDIOS:     googleClientIDIOS,
		GoogleClientIDAndroid: googleClientIDAndroid,
		GoogleClientIDWeb:     googleClientIDWeb,

		AppleBundleID:  appleBundleID,
		AppleServiceID: appleServiceID,

		S3Endpoint:        s3Endpoint,
		S3Region:          s3Region,
		S3Bucket:          s3Bucket,
		S3AccessKeyID:     s3AccessKeyID,
		S3SecretAccessKey: s3SecretAccessKey,
		S3PublicBaseURL:   s3PublicBaseURL,
	}, nil
}

// MustLoad calls Load and panics if the configuration is invalid or
// incomplete. Intended for process startup (cmd/api), where a bad config
// should fail fast rather than surface as a runtime error mid-request.
func MustLoad() *Config {
	cfg, err := Load()
	if err != nil {
		panic(err)
	}
	return cfg
}

// withDefault returns value unless it is empty, in which case it returns def.
func withDefault(value, def string) string {
	if value == "" {
		return def
	}
	return value
}

// parseDurationDefault parses raw as a Go duration, or returns def if raw is
// empty. A parse failure returns an error naming the offending key.
func parseDurationDefault(key, raw string, def time.Duration) (time.Duration, error) {
	if raw == "" {
		return def, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("config: invalid %s: %w", key, err)
	}
	return d, nil
}
