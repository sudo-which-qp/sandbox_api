package mailer

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/imroc/req/v3"
	"godsendjoseph.dev/sandbox-api/internal/env"
)

// HttpMailer implements the Client interface using HTTP API calls
type HttpMailer struct {
	apiKey          string
	apiURL          string
	mailFromAddress string
	mailFromName    string
	maxRetries      int
	retryDelay      time.Duration
	httpClient      *http.Client
}

type PlunkRequest struct {
	To          interface{}  `json:"to"`
	Subject     string       `json:"subject"`
	Body        string       `json:"body"`
	Name        string       `json:"name"`
	From        string       `json:"from"`
	Attachments []Attachment `json:"attachments,omitempty"`
}

var PlunkResponse struct {
	Success   bool    `json:"success"`
	Timestamp string  `json:"timestamp"`
	Emails    []Email `json:"emails"`
}

type Email struct {
	Contact struct {
		ID    string `json:"id"`
		Email string `json:"email"`
	} `json:"contact"`
	EmailID string `json:"email"`
}

// NewHttpMailer creates a new HTTP-based mailer using Plunk API
func NewHttpMailer(
	apiKey,
	mailFromAddress,
	mailFromName string) *HttpMailer {

	return &HttpMailer{
		apiKey:          apiKey,
		apiURL:          env.GetString("PLUNK_API_SEND_URL", "https://next-api.useplunk.com/v1/send"),
		mailFromAddress: mailFromAddress,
		mailFromName:    mailFromName,
		maxRetries:      3,
		retryDelay:      5 * time.Second,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (httpMailer *HttpMailer) Send(templateFile, username, email,
	subject string, data any, isSandBox bool, attachments []Attachment) error {
	return httpMailer.SendWithOptions(templateFile, username, email,
		subject, data, SyncDelivery, isSandBox, attachments)
}

func (httpMailer *HttpMailer) SendWithOptions(templateFile, username, email,
	subject string, data any, deliveryMode string, isSandBox bool, attachments []Attachment) error {
	templatePath := filepath.Join("templates", templateFile)

	t, err := template.ParseFS(FS, templatePath)
	if err != nil {
		return fmt.Errorf("error parsing template from FS: %w", err)
	}

	var body bytes.Buffer
	if err := t.ExecuteTemplate(&body, "body", data); err != nil {
		return fmt.Errorf("error executing template: %w", err)
	}

	if subject == "" {
		var subjectBuf bytes.Buffer
		if err := t.ExecuteTemplate(&subjectBuf, "subject", data); err == nil {
			subject = strings.TrimSpace(subjectBuf.String())
		} else {
			subject = fmt.Sprintf("Message for %s", username)
		}
	}

	if isSandBox {
		log.Printf("SANDBOX MODE: Would send email to %s with template %s", email, templateFile)
		log.Printf("Subject: %s", subject)
		log.Printf("Content: %s", body.String())
		return nil
	}

	request := PlunkRequest{
		To:          email,
		Subject:     subject,
		Body:        body.String(),
		Name:        httpMailer.mailFromName,
		From:        httpMailer.mailFromAddress,
		Attachments: attachments,
	}

	var lastErr error
	for attempt := 1; attempt <= httpMailer.maxRetries; attempt++ {
		log.Printf("Attempt %d/%d to send email to %s via HTTP", attempt, httpMailer.maxRetries, email)

		err := httpMailer.sendHTTPRequest(request)
		if err == nil {
			log.Printf("Email sent successfully to %s via HTTP", email)
			return nil
		}

		lastErr = err
		log.Printf("HTTP send attempt %d failed: %v", attempt, err)

		if attempt < httpMailer.maxRetries {
			log.Printf("Retrying in %v...", httpMailer.retryDelay)
			time.Sleep(httpMailer.retryDelay)
		}
	}

	return fmt.Errorf("failed to send email via HTTP after %d attempts: %w", httpMailer.maxRetries, lastErr)
}

func (httpMailer *HttpMailer) sendHTTPRequest(request PlunkRequest) error {
	client := req.C().SetTimeout(30 * time.Second)

	resp, err := client.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		SetBearerAuthToken(httpMailer.apiKey).
		SetBody(&request).
		SetSuccessResult(&PlunkResponse).
		Post(httpMailer.apiURL)
	if err != nil {
		return fmt.Errorf("failed to send HTTP request: %w", err)
	}

	if resp.IsSuccessState() && PlunkResponse.Success {
		return nil
	}

	errorMsg := fmt.Sprintf("HTTP %d response, success=false", resp.StatusCode)
	log.Printf("Plunk Response Body: %s", resp.String())
	return fmt.Errorf("API request failed: %s", errorMsg)
}
