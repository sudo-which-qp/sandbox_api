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

	var jR map[string]any

	if status < 399 {
		jR = map[string]any{
			"status":  status,
			"success": true,
			"message": message,
			"data":    data,
		}
	} else {
		jR = map[string]any{
			"status":  status,
			"success": false,
			"message": message,
			"data":    data,
		}
	}

	return json.NewEncoder(writer).Encode(jR)
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

func writeJSONError(writer http.ResponseWriter, status int, message string) error {
	// type envelope struct {
	// 	Error string `json:"error"`
	// }
	return writeJSON(writer, status, message, nil)
}
