package mailer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"text/template"
	"time"
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
	To      string `json:"to"`
	Subject string `json:"subject"`
	Body    string `json:"body"`
	Name    string `json:"name"`
	From    string `json:"from"`
}

// PlunkResponse represents the response from Plunk API
type PlunkResponse struct {
	Success   bool   `json:"success"`
	Timestamp string `json:"timestamp"`
	Emails    string `json:"emails"`
	Message   string `json:"message"`
	Error     string `json:"error"`
}

// NewHttpMailer creates a new HTTP-based mailer using Plunk API
func NewHttpMailer(
	apiKey,
	mailFromAddress,
	mailFromName string) *HttpMailer {

	return &HttpMailer{
		apiKey:          apiKey,
		apiURL:          "https://api.useplunk.com/v1/send",
		mailFromAddress: mailFromAddress,
		mailFromName:    mailFromName,
		maxRetries:      3,
		retryDelay:      5 * time.Second,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Send implements the Client interface
func (httpMailer *HttpMailer) Send(templateFile, username, email, subject string, data any, isSandBox bool) error {
	return httpMailer.SendWithOptions(templateFile, username, email, subject, data, SyncDelivery, isSandBox)
}

func (httpMailer *HttpMailer) SendWithOptions(templateFile, username, email, subject string, data any, deliveryMode string, isSandBox bool) error {
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

	// If in sandbox mode, just log the email
	if isSandBox {
		log.Printf("SANDBOX MODE: Would send email to %s with template %s", email, templateFile)
		log.Printf("Subject: %s", subject)
		log.Printf("Content: %s", body.String())
		return nil
	}

	// Prepare the request payload
	request := PlunkRequest{
		To:      email,
		Subject: subject,
		Body:    body.String(),
		Name:    httpMailer.mailFromName,
		From:    httpMailer.mailFromAddress,
	}

	// Attempt to send with retries
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

// sendHTTPRequest sends the email via HTTP API
func (httpMailer *HttpMailer) sendHTTPRequest(request PlunkRequest) error {
	// Marshal the request to JSON
	jsonData, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequest("POST", httpMailer.apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+httpMailer.apiKey)

	// Send the request
	resp, err := httpMailer.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send HTTP request: %w", err)
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	// Parse the response
	var plunkResp PlunkResponse
	if err := json.Unmarshal(body, &plunkResp); err != nil {
		// If we can't parse the response, but got a success status code, consider it successful
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			log.Printf("Email sent successfully but couldn't parse response: %s", string(body))
			return nil
		}
		return fmt.Errorf("failed to parse response (status: %d): %w, body: %s", resp.StatusCode, err, string(body))
	}

	// Check if the request was successful
	if resp.StatusCode >= 200 && resp.StatusCode < 300 && plunkResp.Success {
		return nil
	}

	// Handle error response
	errorMsg := plunkResp.Error
	if errorMsg == "" {
		errorMsg = plunkResp.Message
	}

	if errorMsg == "" {
		errorMsg = fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	log.Printf("Plunk Response Body: %s", string(body))

	return fmt.Errorf("API request failed: %s", errorMsg)
}
