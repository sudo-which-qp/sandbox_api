package main

import (
	"net/http"
)

func (app *application) healthCheckHandler(writer http.ResponseWriter, request *http.Request) {
	data := map[string]any{
		"env":      app.config.env,
		"versions": version,
	}

	if err := writeJSON(writer, http.StatusOK, "API is healthy running in "+app.config.env+" mode", data); err != nil {
		app.internalServerError(writer, request, err)
	}
}
