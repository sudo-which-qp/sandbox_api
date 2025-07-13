package mailer

import (
	"log"
	"sync"
	"time"
)

// InMemoryMailer wraps an SMTP mailer with in-memory queuing
type InMemoryMailer struct {
	baseMailer     *SmtpMailer
	queue          chan MailJob
	workerCount    int
	running        bool
	wg             sync.WaitGroup
	mu             sync.Mutex
	processingTime time.Duration // For testing/monitoring
}

// NewInMemoryMailer creates a new mailer with in-memory queue processing
func NewInMemoryMailer(
	baseMailer *SmtpMailer,
	workerCount int,
	queueSize int) *InMemoryMailer {

	if workerCount <= 0 {
		workerCount = 2
	}

	if queueSize <= 0 {
		queueSize = 100
	}

	return &InMemoryMailer{
		baseMailer:     baseMailer,
		queue:          make(chan MailJob, queueSize),
		workerCount:    workerCount,
		running:        false,
		processingTime: 0,
	}
}

// Send implements the Client interface, but uses in-memory queue
func (m *InMemoryMailer) Send(templateFile, username, email, subject string, data any, isSandBox bool) error {
	job := MailJob{
		TemplateFile: templateFile,
		Username:     username,
		Email:        email,
		Subject:      subject,
		Data:         data,
		IsSandbox:    isSandBox,
	}

	// Enqueue the job instead of sending immediately
	return m.Enqueue(job)
}

// SendWithOptions implements the extended Client interface
func (m *InMemoryMailer) SendWithOptions(templateFile, username, email, subject string, data any, deliveryMode string, isSandBox bool) error {
	// If sync is requested, use the base mailer directly
	if deliveryMode == SyncDelivery {
		return m.baseMailer.Send(templateFile, username, email, subject, data, isSandBox)
	}

	// Otherwise use async in-memory delivery
	return m.Send(templateFile, username, email, subject, data, isSandBox)
}

// Enqueue adds a mail job to the queue
func (m *InMemoryMailer) Enqueue(job MailJob) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Add logging here
	log.Printf("Attempting to enqueue mail job for %s", job.Email)

	if !m.running {
		log.Printf("ERROR: Mail queue is not running")
		return ErrQueueNotRunning
	}

	// Non-blocking send to channel with timeout
	select {
	case m.queue <- job:
		log.Printf("Successfully enqueued mail job for %s", job.Email)
		return nil
	case <-time.After(100 * time.Millisecond):
		log.Printf("ERROR: Mail queue is full")
		return ErrQueueFull
	}
}

// Start begins processing the mail queue
func (m *InMemoryMailer) Start() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return
	}

	m.running = true

	// Start worker goroutines
	for i := 0; i < m.workerCount; i++ {
		m.wg.Add(1)
		go m.worker(i)
	}
}

// Stop halts queue processing and waits for workers to finish
func (m *InMemoryMailer) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return
	}

	m.running = false
	close(m.queue)

	m.wg.Wait()
}

// worker processes mail jobs from the queue
func (m *InMemoryMailer) worker(id int) {
	defer m.wg.Done()
	log.Printf("Mail worker %d started", id)

	for job := range m.queue {
		log.Printf("Worker %d processing mail for %s", id, job.Email)
		startTime := time.Now()

		// Use the base mailer to actually send the email
		err := m.baseMailer.Send(
			job.TemplateFile,
			job.Username,
			job.Email,
			job.Subject,
			job.Data,
			job.IsSandbox,
		)

		processingTime := time.Since(startTime)
		m.mu.Lock()
		m.processingTime = processingTime
		m.mu.Unlock()

		if err != nil {
			log.Printf("ERROR: Worker %d failed to send mail to %s: %v", id, job.Email, err)
			continue
		}

		log.Printf("Worker %d successfully sent mail to %s in %v", id, job.Email, processingTime)
	}

	log.Printf("Mail worker %d stopped", id)
}
