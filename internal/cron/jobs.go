// internal/cron/jobs.go
package cron

import (
	"go.uber.org/zap"

	"godsendjoseph.dev/sandbox-api/internal/mailer"
)

// JobManager holds all available cron jobs
type JobManager struct {
	logger *zap.SugaredLogger
	mailer mailer.Client
}

// NewJobManager creates a new job manager
func NewJobManager(logger *zap.SugaredLogger, mailer mailer.Client) *JobManager {
	return &JobManager{
		logger: logger,
		mailer: mailer,
	}
}

// SendTestEmail sends a test email
func (j *JobManager) SendTestEmail(isProdEnv string) func() {
	return func() {
		j.logger.Info("Running Test Email job")
		isProdEnv := isProdEnv == "production"
		err := j.mailer.SendWithOptions(
			mailer.UserWelcomeTemplate,
			"Geek", "test@gmail.com",
			"Test Email",
			nil,
			mailer.AsyncInMemory,
			!isProdEnv,
		)

		if err != nil {
			j.logger.Errorw("error sending welcome email", "error", err)
			return
		}

	}
}
