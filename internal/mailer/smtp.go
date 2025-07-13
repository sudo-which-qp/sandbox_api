package mailer

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"log"
	"net/smtp"
	"path/filepath"
	"strings"
	"text/template"
	"time"
)

type SmtpMailer struct {
	mailHost        string
	mailPort        string
	mailUsername    string
	mailPassword    string
	mailEncryption  string
	mailFromAddress string
	mailFromName    string
	maxRetries      int
	retryDelay      time.Duration
}

func NewSendSMTP(
	mailHost,
	mailPort,
	mailUsername,
	mailPassword,
	mailEncryption,
	mailFromAddress,
	mailFromName string) *SmtpMailer {
	return &SmtpMailer{
		mailHost:        mailHost,
		mailPort:        mailPort,
		mailUsername:    mailUsername,
		mailPassword:    mailPassword,
		mailEncryption:  mailEncryption,
		mailFromAddress: mailFromAddress,
		mailFromName:    mailFromName,
		maxRetries:      3,
		retryDelay:      5 * time.Second,
	}
}

// Send sends an email with retry logic and proper TLS handling
func (s *SmtpMailer) Send(templateFile, username, email, subject string, data any, isSandBox bool) error {
	log.Printf("Sending email to %s with template %s", email, templateFile)

	// Construct the full template path
	templatePath := filepath.Join("templates", templateFile)

	// Parse the template from the embedded filesystem
	t, err := template.ParseFS(FS, templatePath)
	if err != nil {
		return fmt.Errorf("error parsing template from FS: %w", err)
	}

	// Render the template with data
	var body bytes.Buffer
	if err := t.ExecuteTemplate(&body, "body", data); err != nil {
		return fmt.Errorf("error executing template: %w", err)
	}

	// Set up email headers
	from := fmt.Sprintf("%s <%s>", s.mailFromName, s.mailFromAddress)

	// If subject is empty, try to get it from the template
	if subject == "" {
		var subjectBuf bytes.Buffer
		if err := t.ExecuteTemplate(&subjectBuf, "subject", data); err == nil {
			subject = strings.TrimSpace(subjectBuf.String())
		} else {
			// Fallback subject if template doesn't have a subject block
			subject = fmt.Sprintf("Message for %s", username)
		}
	}

	headers := make(map[string]string)
	headers["From"] = from
	headers["To"] = email
	headers["Subject"] = subject
	headers["MIME-Version"] = "1.0"
	headers["Content-Type"] = "text/html; charset=UTF-8"

	// Compose message
	var message bytes.Buffer
	for k, v := range headers {
		message.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
	}
	message.WriteString("\r\n")
	message.Write(body.Bytes())

	// If in sandbox mode, just log the email
	if isSandBox {
		log.Printf("SANDBOX MODE: Would send email to %s with template %s", email, templateFile)
		log.Printf("Content: %s", message.String())
		return nil
	}

	// Server address
	addr := fmt.Sprintf("%s:%s", s.mailHost, s.mailPort)

	// Attempt to send with retries
	var lastErr error
	for attempt := 1; attempt <= s.maxRetries; attempt++ {
		log.Printf("Attempt %d/%d to send email to %s", attempt, s.maxRetries, email)

		err := s.sendMailWithTLS(addr, email, message.Bytes())
		if err == nil {
			log.Printf("Email sent successfully to %s", email)
			return nil
		}

		lastErr = err
		log.Printf("SMTP send attempt %d failed: %v", attempt, err)

		if attempt < s.maxRetries {
			log.Printf("Retrying in %v...", s.retryDelay)
			time.Sleep(s.retryDelay)
		}
	}

	log.Printf("SMTP config - Host: %s, Port: %s, Username: %s", s.mailHost, s.mailPort, s.mailUsername)
	return fmt.Errorf("failed to send email after %d attempts: %w", s.maxRetries, lastErr)
}

func (s *SmtpMailer) sendMailWithTLS(addr, to string, message []byte) error {
	log.Printf("Connecting to SMTP server at %s", addr)

	// Connect to the SMTP server
	client, err := smtp.Dial(addr)
	if err != nil {
		return fmt.Errorf("failed to dial SMTP server: %w", err)
	}
	defer client.Close()

	// Set the hostname for HELO/EHLO
	if err = client.Hello("localhost"); err != nil {
		return fmt.Errorf("failed HELO/EHLO: %w", err)
	}

	// Check if the server supports STARTTLS
	if ok, _ := client.Extension("STARTTLS"); ok {
		// Configure TLS
		tlsConfig := &tls.Config{
			ServerName:         s.mailHost,
			MinVersion:         tls.VersionTLS12,
			InsecureSkipVerify: true,
		}

		// Start TLS
		if err = client.StartTLS(tlsConfig); err != nil {
			return fmt.Errorf("failed STARTTLS: %w", err)
		}
	}

	// Authenticate if username/password are provided
	if s.mailUsername != "" && s.mailPassword != "" {
		auth := smtp.PlainAuth("", s.mailUsername, s.mailPassword, s.mailHost)
		if err = client.Auth(auth); err != nil {
			return fmt.Errorf("failed authentication: %w", err)
		}
	}

	// Set the sender
	if err = client.Mail(s.mailFromAddress); err != nil {
		return fmt.Errorf("failed MAIL FROM: %w", err)
	}

	// Set the recipient
	if err = client.Rcpt(to); err != nil {
		return fmt.Errorf("failed RCPT TO: %w", err)
	}

	// Send the email data
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("failed DATA: %w", err)
	}

	_, err = w.Write(message)
	if err != nil {
		return fmt.Errorf("failed to write message: %w", err)
	}

	err = w.Close()
	if err != nil {
		return fmt.Errorf("failed to close data writer: %w", err)
	}

	return client.Quit()
}
