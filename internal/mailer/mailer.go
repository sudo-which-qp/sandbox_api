package mailer

import (
	"embed"
	"errors"
)

const (
	UserWelcomeTemplate     = "welcome_mail.tmpl"
	OTPVerificationTemplate = "otp_mail.tmpl"
	GuestInvitationTemplate = "guest_invitation_mail.tmpl"

	// Mail delivery modes
	SyncDelivery    = "sync"
	AsyncInMemory   = "async_memory"
	AsyncPersistent = "async_db"
)

//go:embed "templates"
var FS embed.FS

type Client interface {
	Send(templateFile, username, email, subject string,
		data any, isSandBox bool, attachments []Attachment) error

	SendWithOptions(templateFile, username, email, subject string,
		data any, deliveryMode string, isSandBox bool, attachments []Attachment) error
}

// Error definitions
var (
	ErrQueueNotRunning = errors.New("mail queue is not running")
	ErrQueueFull       = errors.New("mail queue is full")
)

type Attachment struct {
	Filename    string `json:"filename"`
	Content     string `json:"content"`
	ContentType string `json:"contentType"`
}

// MailJob represents a mail to be sent
type MailJob struct {
	ID           string
	TemplateFile string
	Username     string
	Email        string
	Subject      string
	Attachments  []Attachment
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
