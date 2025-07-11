package main

import (
	"net/http"

	"godsendjoseph.dev/sandbox-api/internal/mailer"
)

func (app *application) sendBulkEmails(writer http.ResponseWriter, request *http.Request) {
	emails := []string{
		"godsendjoseph@gmail.com",
	}
	isProdEnv := app.config.env == "production"

	for _, email := range emails {
		err := app.mailer.SendWithOptions(
			mailer.UserWelcomeTemplate,
			"Geek",
			email,
			"Finish up your Registration",
			nil,
			mailer.AsyncInMemory,
			!isProdEnv,
		)
		if err != nil {
			app.logger.Errorw("error sending welcome email", "error", err)
			app.badRequestResponse(writer, request, err)
			return
		}
	}

	writeJSON(writer, http.StatusOK, "Emails sent", nil)
}
