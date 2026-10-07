// Command api is the backend process entrypoint. It loads configuration,
// opens the database pool, constructs every repository and handler, wires
// them into httpapi.NewServer, and serves with graceful shutdown.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/swathivallabhaneni289/RNDMRoll/internal/auth"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/config"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/httpapi"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/storage"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/store/postgres"
)

const (
	// A Gin engine served without explicit timeouts holds a connection open
	// indefinitely on a slow client -- a standing denial-of-service exposure
	// rather than a tuning preference (T-01-SRV-04).
	readHeaderTimeout = 10 * time.Second
	readTimeout       = 30 * time.Second
	writeTimeout      = 30 * time.Second
	// shutdownTimeout bounds how long an in-flight request (for example a
	// mid-write signup) gets to finish before the process exits on SIGINT
	// or SIGTERM (T-01-SRV-06).
	shutdownTimeout = 15 * time.Second
)

func main() {
	// MustLoad panics on missing/invalid configuration, and NewPool below
	// pings once at construction, so a bad DATABASE_URL or missing required
	// variable stops the process at launch rather than at the first request
	// (T-01-SRV-05).
	cfg := config.MustLoad()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	ctx := context.Background()
	pool, err := postgres.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("failed to open database pool", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	users := postgres.NewUserRepo(pool)
	refreshTokens := postgres.NewRefreshTokenRepo(pool)

	refreshSvc := auth.NewRefreshService(refreshTokens, cfg.RefreshTokenTTL)

	googleVerifier := auth.NewGoogleVerifier([]string{
		cfg.GoogleClientIDIOS,
		cfg.GoogleClientIDAndroid,
		cfg.GoogleClientIDWeb,
	})

	// Apple's audience is the bundle ID (iOS/native flow) plus the service
	// ID (web/Android flow) when it differs -- config.Load defaults
	// AppleServiceID to AppleBundleID when unset, so this only appends a
	// second entry when the two are genuinely distinct.
	appleAudiences := []string{cfg.AppleBundleID}
	if cfg.AppleServiceID != "" && cfg.AppleServiceID != cfg.AppleBundleID {
		appleAudiences = append(appleAudiences, cfg.AppleServiceID)
	}
	// NewAppleVerifier fetches Apple's public JWKS at construction. This is
	// a real network call to Apple's own endpoint (not gated by
	// APPLE_BUNDLE_ID being a real value -- the JWKS is public), confirmed
	// empirically to resolve in well under a second before this file was
	// written. Failing fast here matches NewPool's precedent: if Apple's
	// signing keys are unreachable at boot, no Apple sign-in request could
	// ever verify anyway.
	appleVerifier, err := auth.NewAppleVerifier(appleAudiences)
	if err != nil {
		logger.Error("failed to construct Apple verifier (fetching Apple's JWKS)", "error", err)
		os.Exit(1)
	}

	avatarStore, err := storage.NewAvatarStore(storage.Config{
		S3Endpoint:        cfg.S3Endpoint,
		S3Region:          cfg.S3Region,
		S3Bucket:          cfg.S3Bucket,
		S3AccessKeyID:     cfg.S3AccessKeyID,
		S3SecretAccessKey: cfg.S3SecretAccessKey,
		S3PublicBaseURL:   strings.TrimRight(cfg.S3PublicBaseURL, "/"),
	})
	if err != nil {
		logger.Error("failed to construct avatar store", "error", err)
		os.Exit(1)
	}

	authHandler := httpapi.NewAuthHandler(users, refreshSvc, cfg.JWTSecret, cfg.AccessTokenTTL)
	oauthHandler := httpapi.NewOAuthHandler(users, appleVerifier, googleVerifier, refreshSvc, cfg.JWTSecret, cfg.AccessTokenTTL)
	profileHandler := httpapi.NewProfileHandler(users, avatarStore, strings.TrimRight(cfg.S3PublicBaseURL, "/"))
	usernameHandler := httpapi.NewUsernameHandler(users)

	server := httpapi.NewServer(httpapi.Deps{
		Auth:      authHandler,
		OAuth:     oauthHandler,
		Profile:   profileHandler,
		Username:  usernameHandler,
		Users:     users,
		JWTSecret: cfg.JWTSecret,
		Logger:    logger,
	})

	httpServer := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           server.Engine(),
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
	}

	serverErrors := make(chan error, 1)
	go func() {
		logger.Info("server starting", "port", cfg.Port)
		serverErrors <- httpServer.ListenAndServe()
	}()

	shutdownSignal := make(chan os.Signal, 1)
	signal.Notify(shutdownSignal, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server error", "error", err)
			os.Exit(1)
		}
	case sig := <-shutdownSignal:
		logger.Info("shutdown signal received", "signal", sig.String())

		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()

		// An in-flight signup that is mid-write finishes rather than being
		// severed: Shutdown stops accepting new connections and waits for
		// active ones to complete, up to shutdownTimeout.
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			logger.Error("graceful shutdown failed, forcing close", "error", err)
			_ = httpServer.Close()
		}
	}

	logger.Info("server stopped")
}
