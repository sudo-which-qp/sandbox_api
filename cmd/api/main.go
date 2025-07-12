package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/mysql"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/joho/godotenv"
	"go.uber.org/zap"

	"godsendjoseph.dev/sandbox-api/internal/auth"
	"godsendjoseph.dev/sandbox-api/internal/cron"
	"godsendjoseph.dev/sandbox-api/internal/db"
	"godsendjoseph.dev/sandbox-api/internal/env"
	"godsendjoseph.dev/sandbox-api/internal/mailer"
	"godsendjoseph.dev/sandbox-api/internal/notification"
	ratelimiter "godsendjoseph.dev/sandbox-api/internal/rateLimiter"
	"godsendjoseph.dev/sandbox-api/internal/storage"
	"godsendjoseph.dev/sandbox-api/internal/store"
	"godsendjoseph.dev/sandbox-api/internal/store/cache"
)

const version = "0.0.1"

func main() {
	err := godotenv.Load("/app/.env")
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	cfg := config{
		addr:        env.GetString("ADDR", ":8080"),
		apiURL:      env.GetString("EXTERNAL_URL", "http://localhost:8080"),
		frontendURL: env.GetString("FRONTEND_URL", "http://localhost:8080"),
		db: dbConfig{
			addr:         fmt.Sprintf("%s:%s", env.GetString("DB_HOST", "127.0.0.1"), env.GetString("DB_PORT", "3306")),
			user:         env.GetString("DB_USER", "root"),
			password:     env.GetString("DB_PASSWORD", "root"),
			dbName:       env.GetString("DB_NAME", "testdb"),
			maxOpenConns: env.GetInt("DB_MAX_OPEN_CONNS", 25),
			maxIdleConns: env.GetInt("DB_MAX_IDLE_CONNS", 25),
			maxIdleTime:  env.GetString("DB_MAX_IDLE_TIME", "15m"),
		},
		redisCfg: redisConfig{
			addr:    env.GetString("REDIS_ADDR", "localhost:6379"),
			pwd:     env.GetString("REDIS_PASSWORD", ""),
			db:      env.GetInt("REDIS_DB", 0),
			enabled: env.GetBool("REDIS_ENABLED", false),
		},
		r2: r2Config{
			endpoint:        env.GetString("R2_ENDPOINT", ""),
			accessKeyID:     env.GetString("R2_ACCESS_KEY_ID", ""),
			secretAccessKey: env.GetString("R2_SECRET_ACCESS_KEY", ""),
			bucketName:      env.GetString("R2_BUCKET_NAME", ""),
			publicURL:       env.GetString("R2_PUBLIC_URL", ""),
			enabled:         env.GetBool("R2_ENABLED", false),
		},
		env: env.GetString("ENV", "development"),
		mail: mailConfig{
			smtpMail: smtpMailConfig{
				mailHost:        env.GetString("MAIL_HOST", "smtp.useplunk.com"),
				mailPort:        env.GetString("MAIL_PORT", "587"),
				mailUsername:    env.GetString("MAIL_USERNAME", "plunk"),
				mailPassword:    env.GetString("MAIL_PASSWORD", "-"),
				mailEncryption:  env.GetString("MAIL_ENCRYPTION", "tls"),
				mailFromAddress: env.GetString("MAIL_FROM_ADDRESS", "demo@godsend.dev"),
				mailFromName:    env.GetString("MAIL_FROM_NAME", "Social Blog"),
			},
			exp: time.Hour * 24 * 3, // user have 3 days to accept invitation
		},
		auth: authConfig{
			basic: basicConfig{
				username: env.GetString("BASIC_AUTH_USERNAME", "admin"),
				password: env.GetString("BASIC_AUTH_PASSWORD", "password"),
			},
			token: tokenConfig{
				secret:   env.GetString("TOKEN_SECRET", "secret"),
				exp:      time.Hour * 24 * 1, // expires in 1 days
				audience: env.GetString("TOKEN_AUDIENCE", "social-api"),
				issuer:   env.GetString("TOKEN_ISSUER", "social-api"),
			},
		},
		rateLimiter: ratelimiter.Config{
			RequestPerTimeForIP: env.GetInt("RATE_LIMITER_REQUEST_COUNT", 20),
			TimeFrame:           time.Minute * 5,
			Enabled:             env.GetBool("RATE_LIMITER_ENABLED", true),
		},
		timezone: env.GetString("TIMEZONE", "UTC"),
		slack: slackConfig{
			webhookURL: env.GetString("SLACK_WEBHOOK_URL", ""),
			channel:    env.GetString("SLACK_CHANNEL", "#notifications"),
			username:   env.GetString("SLACK_USERNAME", "GoApp Bot"),
			iconEmoji:  env.GetString("SLACK_ICON_EMOJI", ":robot_face:"),
			enabled:    env.GetBool("SLACK_ENABLED", false),
		},
	}

	// Logger
	logger := zap.Must(zap.NewProduction()).Sugar()
	defer logger.Sync()

	// connect to the database
	db, err := db.New(
		cfg.db.addr,
		cfg.db.user,
		cfg.db.password,
		cfg.db.dbName,
		cfg.db.maxOpenConns,
		cfg.db.maxIdleConns,
		cfg.db.maxIdleTime,
	)

	if err != nil {
		logger.Panic(err)
	}

	// defer closing the database
	defer db.Close()
	logger.Info("connected to database")

	// Cache instance
	var redisDB *redis.Client
	if cfg.redisCfg.enabled {
		redisDB = cache.NewRedisClient(
			cfg.redisCfg.addr,
			cfg.redisCfg.pwd,
			cfg.redisCfg.db,
		)
		logger.Info("redis connection has been established")
	}

	// R2 instance
	var storageClient storage.Client
	if cfg.r2.enabled {
		r2Client, err := storage.NewR2Client(
			cfg.r2.endpoint,
			cfg.r2.accessKeyID,
			cfg.r2.secretAccessKey,
			cfg.r2.bucketName,
			cfg.r2.publicURL,
		)
		if err != nil {
			logger.Fatal("Failed to initialize R2 client:", err)
		}
		storageClient = r2Client
		logger.Info("R2 storage client initialized")
	}

	// Rate Limiter
	rateLimiter := ratelimiter.NewFixedWindowLimiter(
		cfg.rateLimiter.RequestPerTimeForIP,
		cfg.rateLimiter.TimeFrame,
	)

	if err := handleMigrations(db); err != nil {
		logger.Fatal(err)
	}

	// check for exiting after migrations
	if len(os.Args) > 1 && (os.Args[len(os.Args)-1] == "up" || os.Args[len(os.Args)-1] == "down" || os.Args[len(os.Args)-1] == "force") {
		return
	}

	store := store.NewStorage(db)
	rdb := cache.NewRedisStorage(redisDB)

	// Mailer config
	smtpMailer := mailer.NewSendSMTP(
		cfg.mail.smtpMail.mailHost,
		cfg.mail.smtpMail.mailPort,
		cfg.mail.smtpMail.mailUsername,
		cfg.mail.smtpMail.mailPassword,
		cfg.mail.smtpMail.mailEncryption,
		cfg.mail.smtpMail.mailFromAddress,
		cfg.mail.smtpMail.mailFromName,
	)
	// Create in-memory mailer with 3 workers and a queue size of 100
	inMemoryMailer := mailer.NewInMemoryMailer(smtpMailer, 3, 100)
	// Start the mail processing workers
	inMemoryMailer.Start()
	// Make sure to stop gracefully at shutdown
	defer inMemoryMailer.Stop()
	// stops here

	jwtAuthenticator := auth.NewJWTAuthenticator(
		cfg.auth.token.secret,
		cfg.auth.token.audience,
		cfg.auth.token.issuer,
	)

	scheduler := cron.NewScheduler(logger, cfg.timezone)
	// Create job manager with necessary dependencies
	//jobManager := cron.NewJobManager(logger, inMemoryMailer)

	// Register jobs
	//scheduler.Custom("send-test-email", "*/5 * * * *", jobManager.SendTestEmail(cfg.env)) // Every 5 minutes

	// Start the scheduler
	go scheduler.Start()
	// Ensure the scheduler stops when the app shuts down
	defer scheduler.Stop()

	slackNotifier := notification.NewSlackNotifier(
		cfg.slack.webhookURL,
		cfg.slack.channel,
		cfg.slack.username,
		cfg.slack.iconEmoji,
		cfg.slack.enabled,
	)

	app := &application{
		config:        cfg,
		store:         store,
		cacheStorage:  rdb,
		logger:        logger,
		mailer:        inMemoryMailer,
		authenticator: jwtAuthenticator,
		rateLimiter:   rateLimiter,
		scheduler:     scheduler,
		slackNotifier: slackNotifier,
		storageClient: storageClient,
	}

	mux := app.mount()

	logger.Fatal(app.run(mux))
}

func handleMigrations(db *sql.DB) error {
	driver, err := mysql.WithInstance(db, &mysql.Config{})
	if err != nil {
		return fmt.Errorf("could not create driver instance: %v", err)
	}

	// Use filepath.Abs to get absolute path (this is the key fix)
	migrationsPath := "file://cmd/migrate/migrations"
	if os.Getenv("DOCKER_ENV") == "true" {
		// If in Docker, use the absolute path within the container
		migrationsPath = "file:///app/cmd/migrate/migrations"
	}

	m, err := migrate.NewWithDatabaseInstance(
		migrationsPath,
		"mysql",
		driver,
	)
	if err != nil {
		return fmt.Errorf("could not create migration instance: %v", err)
	}

	cmd := os.Args[len(os.Args)-1]
	switch cmd {
	case "up":
		if err := m.Up(); err != nil && err != migrate.ErrNoChange {
			return fmt.Errorf("could not run up migration: %v", err)
		}
	case "down":
		if err := m.Down(); err != nil && err != migrate.ErrNoChange {
			return fmt.Errorf("could not run down migration: %v", err)
		}
	case "force":
		if len(os.Args) != 3 {
			return fmt.Errorf("force command requires a version number")
		}
		version, err := strconv.ParseInt(os.Args[1], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid version number: %v", err)
		}
		if err := m.Force(int(version)); err != nil {
			return fmt.Errorf("could not force version: %v", err)
		}
	}

	return nil
}
