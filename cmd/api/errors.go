package main

import (
	"net/http"
)

func (app *application) internalServerError(writer http.ResponseWriter, request *http.Request, err error) {
	app.logger.Errorw("internal server error", "method", request.Method, "path", request.URL.Path, "error", err.Error())
	app.slackNotifier.NotifyServerError(err, request)
	writeJSONError(writer, http.StatusInternalServerError, "the server encountered a problem and could not process your request", nil)
}

func (app *application) badRequestResponse(writer http.ResponseWriter, request *http.Request, err error, errorsMap map[string]string) {
	app.logger.Errorw("bad request error", "method", request.Method, "path", request.URL.Path, "error", err.Error(), "errors", errorsMap)
	writeJSONError(writer, http.StatusBadRequest, err.Error(), errorsMap)
}
func (app *application) methodNotAllowedResponse(writer http.ResponseWriter, request *http.Request, err error) {
	app.logger.Errorf("method not allowed error", "method", request.Method, "path", request.URL.Path, "error", err.Error())
	_ = writeJSONError(writer, http.StatusMethodNotAllowed, "method not allowed", nil)
}

func (app *application) notFoundResponse(writer http.ResponseWriter, request *http.Request, err error) {
	app.logger.Errorf("not found error", "method", request.Method, "path", request.URL.Path, "error", err.Error())
	if app.isCriticalResource(request.URL.Path) {
		app.slackNotifier.NotifyNotFound(err, request)
	}

	writeJSONError(writer, http.StatusNotFound, "not found", nil)
}

func (app *application) conflictResponse(writer http.ResponseWriter, request *http.Request, err error) {
	app.logger.Errorf("conflict error", "method", request.Method, "path", request.URL.Path, "error", err.Error())
	writeJSONError(writer, http.StatusConflict, "resource already exists", nil)
}

func (app *application) forbiddenResponseError(writer http.ResponseWriter, request *http.Request) {
	app.logger.Warnw("forbidden error", "method", request.Method, "path", request.URL.Path)
	app.slackNotifier.NotifyForbidden(request)
	writeJSONError(writer, http.StatusForbidden, "request is forbidden", nil)
}

func (app *application) unauthorizedErrorResponse(writer http.ResponseWriter, request *http.Request, err error) {
	app.logger.Errorf("unauthorized error", "method", request.Method, "path", request.URL.Path, "error", err.Error())
	writeJSONError(writer, http.StatusUnauthorized, "unauthorized", nil)
}

func (app *application) unauthorizedPwdErrorResponse(writer http.ResponseWriter, request *http.Request, err error) {
	app.logger.Errorf("unauthorized password error", "method", request.Method, "path", request.URL.Path, "error", err.Error())
	writeJSONError(writer, http.StatusUnauthorized, "unauthorized, password is incorrect", nil)
}

func (app *application) unauthorizedBasicErrorResponse(writer http.ResponseWriter, request *http.Request, err error) {
	app.logger.Errorf("unauthorized basic error", "method", request.Method, "path", request.URL.Path, "error", err.Error())
	writer.Header().Set("WWW-Authenticate", `Basic realm="restricted", charset="UTF-8"`)
	writeJSONError(writer, http.StatusUnauthorized, "unauthorized", nil)
}

func (app *application) rateLimitExceededResponse(writer http.ResponseWriter, request *http.Request, retryAfter string) {
	app.logger.Warnw("rate limit error", "method", request.Method, "path", request.URL.Path, "error", retryAfter)
	writer.Header().Set("Retry-After", retryAfter)
	writeJSONError(writer, http.StatusTooManyRequests, "rate limit exceeded", nil)
}

func (app *application) isCriticalResource(path string) bool {
	criticalUrls := []string{
		"/v1/health",
		"/v1/payments",
	}

	for _, url := range criticalUrls {
		if url == path {
			return true
		}
	}
	return false
}
