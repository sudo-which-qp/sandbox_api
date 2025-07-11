package mailer

import (
	"embed"
	"errors"
)

const (
	UserWelcomeTemplate = "welcome_mail.tmpl"

	// Mail delivery modes
	SyncDelivery    = "sync"
	AsyncInMemory   = "async_memory"
	AsyncPersistent = "async_db"
)

//go:embed "templates"
var FS embed.FS

type Client interface {
	Send(templateFile, username, email, subject string, data any, isSandBox bool) error

	SendWithOptions(templateFile, username, email, subject string, data any, deliveryMode string, isSandBox bool) error
}

// Error definitions
var (
	ErrQueueNotRunning = errors.New("mail queue is not running")
	ErrQueueFull       = errors.New("mail queue is full")
)


// MailJob represents a mail to be sent
type MailJob struct {
    ID           string
    TemplateFile string
    Username     string
    Email        string
    Subject      string
    Data         interface{}
    IsSandbox    bool
    Status       string
    Attempts     int
    CreatedAt    string
    UpdatedAt    string
}

// Queue interface for mail queue operations
type Queue interface {
    Enqueue(job MailJob) error
    ProcessQueue()
    Start()
    Stop()
}
