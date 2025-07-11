// internal/cron/scheduler.go
package cron

import (
	"fmt"
	"time"

	"github.com/go-co-op/gocron/v2"
	"go.uber.org/zap"
)

// Scheduler represents the application's scheduler service
type Scheduler struct {
	scheduler gocron.Scheduler
	logger    *zap.SugaredLogger
	jobs      []Job
}

// Job represents a scheduled job
type Job struct {
	Name     string
	Schedule string
	Task     func()
	JobID    string
}

// NewScheduler creates a new scheduler with the given timezone
func NewScheduler(logger *zap.SugaredLogger, timezone string) *Scheduler {
	location, err := time.LoadLocation(timezone)
	if err != nil {
		logger.Warnf("Failed to load timezone %s, using UTC: %v", timezone, err)
		location = time.UTC
	}

	// Create a new scheduler with the specified location
	s, err := gocron.NewScheduler(
		gocron.WithLocation(location),
		gocron.WithLogger(gocron.NewLogger(gocron.LogLevelInfo)),
	)

	if err != nil {
		logger.Fatalf("Failed to create scheduler: %v", err)
	}

	return &Scheduler{
		scheduler: s,
		logger:    logger,
		jobs:      make([]Job, 0),
	}
}

// Start begins the scheduler
func (s *Scheduler) Start() {
	// Register all jobs first
	s.RegisterJobs()

	// Start the scheduler
	s.scheduler.Start()
	s.logger.Info("Scheduler started")
}

// Stop halts the scheduler
func (s *Scheduler) Stop() {
	// Shutdown the scheduler
	s.scheduler.Shutdown()
	s.logger.Info("Scheduler stopped")
}

// RegisterJobs adds all jobs to the scheduler
func (s *Scheduler) RegisterJobs() {
	for i, job := range s.jobs {
		s.logger.Infof("Registering job: %s with schedule %s", job.Name, job.Schedule)

		// Create a wrapped task that includes logging
		task := func() {
			s.logger.Infof("Executing job: %s", job.Name)
			startTime := time.Now()

			defer func() {
				if r := recover(); r != nil {
					s.logger.Errorf("Job %s panicked: %v", job.Name, r)
				}
			}()

			job.Task()

			s.logger.Infof("Job %s completed in %v", job.Name, time.Since(startTime))
		}

		// Schedule based on the provided cron expression
		j, err := s.scheduler.NewJob(
			gocron.CronJob(
				job.Schedule,
				false, // Don't use seconds field
			),
			gocron.NewTask(
				task,
			),
			gocron.WithName(job.Name),
		)

		if err != nil {
			s.logger.Errorf("Failed to schedule job %s: %v", job.Name, err)
			continue
		}

		// Store the job ID as string
		s.jobs[i].JobID = j.ID().String()
	}
}

// AddJob adds a new job to the scheduler
func (s *Scheduler) AddJob(name string, schedule string, task func()) {
	s.jobs = append(s.jobs, Job{
		Name:     name,
		Schedule: schedule,
		Task:     task,
	})
}

// Daily schedules a job to run daily at a specific time
func (s *Scheduler) Daily(name string, timeStr string, task func()) {
	// Convert time (like "08:00") to cron syntax
	// Parse the timeStr into hours and minutes
	var hours, minutes int
	_, err := fmt.Sscanf(timeStr, "%d:%d", &hours, &minutes)
	if err != nil {
		s.logger.Errorf("Invalid time format for daily job %s: %v", name, err)
		return
	}

	schedule := fmt.Sprintf("%d %d * * *", minutes, hours)
	s.AddJob(name, schedule, task)
}

// Hourly schedules a job to run at the specified minute of every hour
func (s *Scheduler) Hourly(name string, minute int, task func()) {
	// Ensure minute is within valid range
	minute = minute % 60
	schedule := fmt.Sprintf("%d * * * *", minute)
	s.AddJob(name, schedule, task)
}

// Weekly schedules a job to run weekly on a specific day
func (s *Scheduler) Weekly(name string, day int, timeStr string, task func()) {
	// In cron, 0 = Sunday, 1 = Monday, etc.
	// Ensure day is within valid range
	day = day % 7

	// Parse the timeStr into hours and minutes
	var hours, minutes int
	_, err := fmt.Sscanf(timeStr, "%d:%d", &hours, &minutes)
	if err != nil {
		s.logger.Errorf("Invalid time format for weekly job %s: %v", name, err)
		return
	}

	schedule := fmt.Sprintf("%d %d * * %d", minutes, hours, day)
	s.AddJob(name, schedule, task)
}

// Monthly schedules a job to run monthly on a specific day
func (s *Scheduler) Monthly(name string, dayOfMonth int, timeStr string, task func()) {
	// Ensure dayOfMonth is within valid range
	if dayOfMonth < 1 {
		dayOfMonth = 1
	} else if dayOfMonth > 31 {
		dayOfMonth = 31
	}

	// Parse the timeStr into hours and minutes
	var hours, minutes int
	_, err := fmt.Sscanf(timeStr, "%d:%d", &hours, &minutes)
	if err != nil {
		s.logger.Errorf("Invalid time format for monthly job %s: %v", name, err)
		return
	}

	schedule := fmt.Sprintf("%d %d %d * *", minutes, hours, dayOfMonth)
	s.AddJob(name, schedule, task)
}

// Custom allows for advanced scheduling options
func (s *Scheduler) Custom(name string, schedule string, task func()) {
	s.AddJob(name, schedule, task)
}

// GetJobs returns all registered jobs
func (s *Scheduler) GetJobs() []Job {
	return s.jobs
}

// RunJobByName finds and runs a job by name immediately
func (s *Scheduler) RunJobByName(name string) error {
	for _, job := range s.jobs {
		if job.Name == name {
			// Run the job in a goroutine to avoid blocking
			go job.Task()
			return nil
		}
	}
	return fmt.Errorf("job not found: %s", name)
}
