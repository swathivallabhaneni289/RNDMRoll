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
	"github.com/swathivallabhaneni289/RNDMRoll/internal/board"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/clock"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/config"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/diary"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/httpapi"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/notifications"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/social"
	"github.com/swathivallabhaneni289/RNDMRoll/internal/spin"
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

	// The avatar store and the entry photo store share one set of storage
	// settings and the same trimmed public base.
	storageConfig := storage.Config{
		S3Endpoint:        cfg.S3Endpoint,
		S3Region:          cfg.S3Region,
		S3Bucket:          cfg.S3Bucket,
		S3AccessKeyID:     cfg.S3AccessKeyID,
		S3SecretAccessKey: cfg.S3SecretAccessKey,
		S3PublicBaseURL:   strings.TrimRight(cfg.S3PublicBaseURL, "/"),
	}
	avatarStore, err := storage.NewAvatarStore(storageConfig)
	if err != nil {
		logger.Error("failed to construct avatar store", "error", err)
		os.Exit(1)
	}
	entryPhotos, err := storage.NewEntryPhotoStore(storageConfig)
	if err != nil {
		logger.Error("failed to construct entry photo store", "error", err)
		os.Exit(1)
	}

	authHandler := httpapi.NewAuthHandler(users, refreshSvc, cfg.JWTSecret, cfg.AccessTokenTTL)
	profileHandler := httpapi.NewProfileHandler(users, avatarStore, strings.TrimRight(cfg.S3PublicBaseURL, "/"))
	usernameHandler := httpapi.NewUsernameHandler(users)
	// The Postgres user repository is also the PasswordChanger: the password
	// and the sessions change in one transaction.
	accountHandler := httpapi.NewAccountHandler(users, users, refreshSvc, cfg.JWTSecret, cfg.AccessTokenTTL)

	// The spin slice has its own clock. Normally it is real time; in test
	// mode (config.Load has already checked APP_ENV, the allow-list and the
	// database name) it is a per-account clock the test routes can move. It
	// goes to the spin service only: tokens, the rate limiter and refresh
	// expiry always use real time.
	var spinClock clock.Clock = clock.Real{}
	var testClock *clock.PerUser
	if cfg.SpinTestTools {
		testClock = clock.NewPerUser()
		spinClock = testClock
	}
	spinRepo := postgres.NewSpinRepo(pool)
	spinService := spin.NewService(spinRepo, spinClock, spin.NewSystemRand())

	// The entry handler is built over the same spin service, so a post, a skip
	// and Today all run under the same person lock and clock. NewServer gives
	// the spin handler the photo store's links.
	entryHandler := httpapi.NewEntryHandler(spinService, entryPhotos)

	// Follows, the Board and the bell run on the same clock as the spin
	// service. Photo links on the Board are built only for visible posts.
	socialService := social.NewService(postgres.NewSocialRepo(pool), spinClock)
	boardService := board.NewService(postgres.NewBoardRepo(pool), socialService, spinService, spinClock, entryPhotos.PublicURL)
	notificationsService := notifications.NewService(postgres.NewNotificationsRepo(pool), socialService, spinService)

	// The diary and the Profile summary only read, through the pool, on the
	// same clock as the spin service. Followers are counted by follows.
	diaryService := diary.NewService(postgres.NewDiaryReader(pool), spinClock, socialService)

	deps := httpapi.Deps{
		Auth:     authHandler,
		Profile:  profileHandler,
		Username: usernameHandler,
		Account:  accountHandler,
		Spin:     httpapi.NewSpinHandler(spinService),
		Wheel:    httpapi.NewWheelHandler(spinService),
		Entry:    entryHandler,
		Diary:    httpapi.NewDiaryHandler(diaryService, entryPhotos),

		Follow:        httpapi.NewFollowHandler(socialService),
		Board:         httpapi.NewBoardHandler(boardService),
		Notifications: httpapi.NewNotificationsHandler(notificationsService, entryPhotos),

		Users:     users,
		JWTSecret: cfg.JWTSecret,
		Logger:    logger,
	}
	if cfg.SpinTestTools {
		deps.TestTools = httpapi.NewSpinTestTools(true, spinService, testClock, cfg.SpinTestUserEmails)
		logger.Warn("spin test tools are ON: the test clock and pretend-day routes answer for the listed accounts only",
			"app_env", cfg.AppEnv, "accounts", len(cfg.SpinTestUserEmails))
	}
	server := httpapi.NewServer(deps)

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
