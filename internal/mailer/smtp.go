package mailer

import (
	"bytes"
	"fmt"
	"log"
	"net/smtp"
	"path/filepath"
	"strings"
	"text/template"
)

type SmtpMailer struct {
	mailHost        string
	mailPort        string
	mailUsername    string
	mailPassword    string
	mailEncryption  string
	mailFromAddress string
	mailFromName    string
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
	}
}

func (s *SmtpMailer) Send(templateFile, username, email, subject string, data any, isSandBox bool) error {
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

	// Authentication
	auth := smtp.PlainAuth("", s.mailUsername, s.mailPassword, s.mailHost)

	// Server address
	addr := fmt.Sprintf("%s:%s", s.mailHost, s.mailPort)

	// Send the email
	if err := smtp.SendMail(
		addr,
		auth,
		s.mailFromAddress,
		[]string{email},
		message.Bytes(),
	); err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}
