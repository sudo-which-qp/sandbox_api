package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/swaggo/swag/example/basic/docs"
	"go.uber.org/zap"

	"godsendjoseph.dev/sandbox-api/internal/auth"
	"godsendjoseph.dev/sandbox-api/internal/cron"
	"godsendjoseph.dev/sandbox-api/internal/mailer"
	"godsendjoseph.dev/sandbox-api/internal/notification"
	ratelimiter "godsendjoseph.dev/sandbox-api/internal/rateLimiter"
	"godsendjoseph.dev/sandbox-api/internal/storage"
	"godsendjoseph.dev/sandbox-api/internal/store"
	"godsendjoseph.dev/sandbox-api/internal/store/cache"
)

type application struct {
	config        config
	store         store.Storage
	cacheStorage  cache.Storage
	logger        *zap.SugaredLogger
	mailer        mailer.Client
	authenticator auth.Authenticator
	rateLimiter   ratelimiter.Limiter
	scheduler     *cron.Scheduler
	slackNotifier *notification.SlackNotifier
	storageClient storage.Client
}

// testing this

type config struct {
	addr        string
	db          dbConfig
	env         string
	apiURL      string
	mail        mailConfig
	frontendURL string
	auth        authConfig
	redisCfg    redisConfig
	rateLimiter ratelimiter.Config
	timezone    string
	slack       slackConfig
	r2          r2Config
}

type redisConfig struct {
	addr    string
	pwd     string
	db      int
	enabled bool
}

type r2Config struct {
	endpoint        string
	accessKeyID     string
	secretAccessKey string
	bucketName      string
	publicURL       string
	enabled         bool
}

type authConfig struct {
	basic basicConfig
	token tokenConfig
}

type basicConfig struct {
	username string
	password string
}

type tokenConfig struct {
	secret   string
	audience string
	issuer   string
	exp      time.Duration
}

type dbConfig struct {
	addr         string
	user         string
	password     string
	dbName       string
	maxOpenConns int
	maxIdleConns int
	maxIdleTime  string
}

type mailConfig struct {
	smtpMail smtpMailConfig
	exp      time.Duration
}

type smtpMailConfig struct {
	mailHost        string
	mailPort        string
	mailUsername    string
	mailPassword    string
	mailEncryption  string
	mailFromAddress string
	mailFromName    string
}

type slackConfig struct {
	webhookURL string
	channel    string
	username   string
	iconEmoji  string
	enabled    bool
}

type contextKey string

func (app *application) mount() http.Handler {
	router := chi.NewRouter()

	// middleware
	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)
	router.Use(middleware.Logger)
	router.Use(middleware.Recoverer)

	// cors
	router.Use(cors.Handler(cors.Options{
		// AllowedOrigins:   []string{"https://foo.com"}, // Use this to allow specific origin hosts
		AllowedOrigins: []string{"https://*", "http://*", "http://localhost:*"},
		// AllowOriginFunc:  func(r *http.Request, origin string) bool { return true },
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "PATCH"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: false,
		MaxAge:           300, // Maximum value not ignored by any of major browsers
	}))

	router.Use(app.RateLimiterMiddleware)

	router.Use(middleware.Timeout(60 * time.Second))

	router.NotFound(func(w http.ResponseWriter, r *http.Request) {
		app.notFoundResponse(w, r, errors.New("route not found"))
	})

	router.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		app.methodNotAllowedResponse(w, r, errors.New("method not allowed"))
	})

	if app.config.env == "development" {
		workDir, _ := os.Getwd()
		uploadsDir := filepath.Join(workDir, "uploads")

		// Create uploads directory if it doesn't exist
		os.MkdirAll(uploadsDir, 0755)

		// Mount the file server
		fileServer := http.FileServer(http.Dir(uploadsDir))
		router.Handle("/uploads/*", http.StripPrefix("/uploads", fileServer))
	}

	// routes
	app.registerRoutes(router)

	return router
}

func (app *application) run(mux http.Handler) error {
	// Docs
	docs.SwaggerInfo.Version = version
	docs.SwaggerInfo.Host = app.config.apiURL
	docs.SwaggerInfo.BasePath = "/v1"

	server := &http.Server{
		Addr:         app.config.addr,
		Handler:      mux,
		WriteTimeout: time.Second * 30,
		ReadTimeout:  time.Second * 10,
		IdleTimeout:  time.Minute,
	}

	shutdown := make(chan error)

	go func() {
		quit := make(chan os.Signal, 1)

		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

		s := <-quit

		ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)

		defer cancel()

		app.logger.Infow("signals caught", "signal", s.String())

		shutdown <- server.Shutdown(ctx)
	}()

	app.logger.Infow("Server has started", "addr", app.config.addr, "env", app.config.env)

	err := server.ListenAndServe()
	if !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	err = <-shutdown
	if err != nil {
		return err
	}

	app.logger.Infow("Server has stopped", "addr", app.config.addr, "env", app.config.env)

	return nil
}
