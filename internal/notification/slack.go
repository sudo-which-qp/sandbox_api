package notification

import (
	"fmt"
	"net/http"

	"github.com/slack-go/slack"
)

// SlackNotifier handles sending notifications to Slack
type SlackNotifier struct {
	webhookURL string
	channel    string
	username   string
	iconEmoji  string
	enabled    bool
}

// NewSlackNotifier creates a new instance of SlackNotifier
func NewSlackNotifier(webhookURL, channel, username, iconEmoji string, enabled bool) *SlackNotifier {
	return &SlackNotifier{
		webhookURL: webhookURL,
		channel:    channel,
		username:   username,
		iconEmoji:  iconEmoji,
		enabled:    enabled,
	}
}

// SendNotification sends a simple text message to Slack
func (s *SlackNotifier) SendNotification(message string) error {
	if !s.enabled {
		return nil // Silently return if notifications are disabled
	}

	msg := &slack.WebhookMessage{
		Text:      message,
		Channel:   s.channel,
		Username:  s.username,
		IconEmoji: s.iconEmoji,
	}

	return slack.PostWebhook(s.webhookURL, msg)
}

// SendRichNotification sends a message with attachments to Slack
func (s *SlackNotifier) SendRichNotification(title, message, color string, fields map[string]string) error {
	if !s.enabled {
		return nil
	}

	// Create attachment fields
	attachmentFields := []slack.AttachmentField{}
	for k, v := range fields {
		attachmentFields = append(attachmentFields, slack.AttachmentField{
			Title: k,
			Value: v,
			Short: len(v) < 20, // Short fields are displayed side-by-side
		})
	}

	attachment := slack.Attachment{
		Title:      title,
		Text:       message,
		Color:      color, // Can be "good" (green), "warning" (yellow), "danger" (red), or any hex color code
		Fields:     attachmentFields,
		MarkdownIn: []string{"text", "fields"},
	}

	msg := &slack.WebhookMessage{
		Attachments: []slack.Attachment{attachment},
		Channel:     s.channel,
		Username:    s.username,
		IconEmoji:   s.iconEmoji,
	}

	return slack.PostWebhook(s.webhookURL, msg)
}

// NotifyHTTPError sends an error notification for HTTP errors
func (s *SlackNotifier) NotifyHTTPError(statusCode int, title string, err error, request *http.Request, context map[string]string) error {
	if !s.enabled || err == nil {
		return nil
	}

	// Add error and request details to context
	if context == nil {
		context = make(map[string]string)
	}

	// Add error information
	context["Error"] = fmt.Sprintf("`%v`", err)

	// Add request details if available
	if request != nil {
		context["Method"] = request.Method
		context["Path"] = request.URL.Path
		context["User-Agent"] = request.UserAgent()
		context["Remote IP"] = request.RemoteAddr
	}

	// Set color based on status code
	var color string
	var emoji string

	switch {
	case statusCode >= 500:
		color = "danger" // Red
		emoji = "🚨"      // Red alert
	case statusCode >= 400:
		color = "warning" // Yellow
		emoji = "⚠️"      // Warning
	default:
		color = "#3AA3E3" // Blue
		emoji = "ℹ️"      // Info
	}

	return s.SendRichNotification(
		fmt.Sprintf("%s %s (HTTP %d)", emoji, title, statusCode),
		"",
		color,
		context,
	)
}

// NotifyServerError for 500-level errors
func (s *SlackNotifier) NotifyServerError(err error, request *http.Request) error {
	return s.NotifyHTTPError(
		http.StatusInternalServerError,
		"Internal Server Error",
		err,
		request,
		nil,
	)
}

// NotifyBadRequest for 400 errors
func (s *SlackNotifier) NotifyBadRequest(err error, request *http.Request) error {
	return s.NotifyHTTPError(
		http.StatusBadRequest,
		"Bad Request",
		err,
		request,
		nil,
	)
}

// NotifyNotFound for 404 errors
func (s *SlackNotifier) NotifyNotFound(err error, request *http.Request) error {
	return s.NotifyHTTPError(
		http.StatusNotFound,
		"Not Found",
		err,
		request,
		nil,
	)
}

// NotifyConflict for 409 errors
func (s *SlackNotifier) NotifyConflict(err error, request *http.Request) error {
	return s.NotifyHTTPError(
		http.StatusConflict,
		"Resource Conflict",
		err,
		request,
		nil,
	)
}

// NotifyForbidden for 403 errors
func (s *SlackNotifier) NotifyForbidden(request *http.Request) error {
	dummyErr := fmt.Errorf("access forbidden")
	return s.NotifyHTTPError(
		http.StatusForbidden,
		"Forbidden",
		dummyErr,
		request,
		nil,
	)
}

// NotifyUnauthorized for 401 errors
func (s *SlackNotifier) NotifyUnauthorized(err error, request *http.Request) error {
	return s.NotifyHTTPError(
		http.StatusUnauthorized,
		"Unauthorized",
		err,
		request,
		nil,
	)
}

// NotifyRateLimitExceeded for 429 errors
func (s *SlackNotifier) NotifyRateLimitExceeded(request *http.Request, retryAfter string) error {
	context := map[string]string{
		"Retry-After": retryAfter,
	}

	rateLimitErr := fmt.Errorf("rate limit exceeded")
	return s.NotifyHTTPError(
		http.StatusTooManyRequests,
		"Rate Limit Exceeded",
		rateLimitErr,
		request,
		context,
	)
}

// NotifySuccess for successful operations worth logging
func (s *SlackNotifier) NotifySuccess(title string, message string, context map[string]string) error {
	return s.SendRichNotification(
		fmt.Sprintf("✅ %s", title),
		message,
		"good",
		context,
	)
}

// NotifyWarning for important warnings not tied to HTTP errors
func (s *SlackNotifier) NotifyWarning(title string, message string, context map[string]string) error {
	return s.SendRichNotification(
		fmt.Sprintf("⚠️ %s", title),
		message,
		"warning",
		context,
	)
}

// NotifyInfo for general informational messages
func (s *SlackNotifier) NotifyInfo(title string, message string, context map[string]string) error {
	return s.SendRichNotification(
		fmt.Sprintf("ℹ️ %s", title),
		message,
		"#3AA3E3", // Blue
		context,
	)
}
