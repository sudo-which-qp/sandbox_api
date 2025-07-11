package main

import (
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/go-viper/mapstructure/v2"
)

var Validate *validator.Validate

func init() {
	Validate = validator.New(validator.WithRequiredStructEnabled())
}

func writeJSON(writer http.ResponseWriter, status int, message string, data any) error {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)

	response := map[string]any{
		"status":  status,
		"success": status < 400,
		"message": message,
		"data":    data,
	}

	return json.NewEncoder(writer).Encode(response)
}

func readFormData(writer http.ResponseWriter, request *http.Request, data any) (map[string][]*multipart.FileHeader, error) {
	maxBytes := 1_048_576 // 1mb
	request.Body = http.MaxBytesReader(writer, request.Body, int64(maxBytes))

	files := make(map[string][]*multipart.FileHeader)

	// First, try to parse as a multipart form (for file uploads)
	if err := request.ParseMultipartForm(int64(maxBytes)); err != nil {
		if !errors.Is(err, http.ErrNotMultipart) {
			return nil, err
		}
		// If not multipart, try as a regular form
		if err := request.ParseForm(); err != nil {
			return nil, err
		}
	} else {
		// If it was a multipart form, get the files
		files = request.MultipartForm.File
	}

	// Use a mapstructure decoder to map form values to your struct
	decoderConfig := &mapstructure.DecoderConfig{
		Result:     data,
		TagName:    "form", // or "json" if you prefer
		DecodeHook: mapstructure.StringToTimeHookFunc(time.RFC3339),
	}

	decoder, err := mapstructure.NewDecoder(decoderConfig)
	if err != nil {
		return nil, err
	}

	// Convert form values to a map
	values := make(map[string]interface{})
	for key, val := range request.Form {
		if len(val) == 1 {
			values[key] = val[0]
		} else {
			values[key] = val
		}
	}

	if err := decoder.Decode(values); err != nil {
		return nil, err
	}

	return files, nil
}

func readJSON(writer http.ResponseWriter, request *http.Request, data any) error {
	maxBytes := 1_048_576 // 1mb
	request.Body = http.MaxBytesReader(writer, request.Body, int64(maxBytes))

	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()

	return decoder.Decode(data)
}

func writeJSONError(writer http.ResponseWriter, status int, message string, errorsMap map[string]string) error {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)

	response := map[string]any{
		"status":  status,
		"success": false,
		"message": message,
		"data":    nil,
	}

	if errorsMap != nil {
		response["errors"] = errorsMap
	}
	return json.NewEncoder(writer).Encode(response)
}

func validatePayload(writer http.ResponseWriter, payload any) bool {
	if err := Validate.Struct(payload); err != nil {
		msg, errorsMap := formatValidationErrors(err)
		writeJSONError(writer, http.StatusBadRequest, msg, errorsMap)
		return false
	}
	return true
}

func formatValidationErrors(err error) (string, map[string]string) {
	errorsMap := make(map[string]string)
	var firstError string

	var ve validator.ValidationErrors
	if errors.As(err, &ve) {
		for _, fieldErr := range ve {
			field := fieldErr.Field()

			// Create a more user-friendly error message
			var msg string
			switch fieldErr.Tag() {
			case "required":
				msg = field + " is required"
			case "email":
				msg = field + " must be a valid email address"
			case "min":
				msg = field + " must be at least " + fieldErr.Param() + " characters long"
			case "max":
				msg = field + " must be at most " + fieldErr.Param() + " characters long"
			default:
				msg = field + " is " + fieldErr.Tag()
			}

			errorsMap[field] = msg

			// Store the first error if we haven't set it yet
			if firstError == "" {
				firstError = msg
			}
		}
	}

	// Return the first error message and the complete errors map
	if firstError != "" {
		return firstError, errorsMap
	}

	return "Invalid input", errorsMap
}
