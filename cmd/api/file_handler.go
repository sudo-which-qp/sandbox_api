package main

import (
	"context"
	"errors"
	"fmt"
	"godsendjoseph.dev/sandbox-api/internal/storage"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func (app *application) deleteFile(ctx context.Context, fileKey string) error {
	if fileKey == "" {
		return nil // No file to delete
	}

	if app.config.env == "development" {
		// Delete local file - fileKey is just the filename
		filePath := filepath.Join("./uploads", fileKey)
		if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to delete local file: %w", err)
		}
	} else {
		// Delete from R2 - fileKey is the full R2 key
		if app.storageClient != nil {
			if err := app.storageClient.DeleteFile(ctx, fileKey); err != nil {
				return fmt.Errorf("failed to delete R2 file: %w", err)
			}
		}
	}

	return nil
}

func (app *application) uploadFile(writer http.ResponseWriter, request *http.Request, fileHeaders []*multipart.FileHeader) (error, string, string) {
	fileHeader := fileHeaders[0]

	// Get the original file extension
	originalFilename := fileHeader.Filename
	fileExt := filepath.Ext(originalFilename)

	fileExt = strings.ToLower(filepath.Ext(fileHeader.Filename))

	allowedExtensions := map[string]bool{
		".jpg":  true,
		".jpeg": true,
		".png":  true,
		// Add more if needed (e.g., ".gif", ".webp")
	}

	if !allowedExtensions[fileExt] {
		app.badRequestResponse(writer, request, errors.New("invalid file extension"))
		return errors.New("invalid file extension"), "", ""
	}

	// Generate a new filename (you can customize this)
	newFilename := fmt.Sprintf("upload_%d%s", time.Now().UnixNano(), fileExt)

	file, err := fileHeader.Open()
	if err != nil {
		app.internalServerError(writer, request, err)
		return err, "", ""
	}
	defer file.Close()

	var fileKey, fileURL string

	if app.config.env == "development" {
		// LOCAL STORAGE (your existing code)
		uploadDir := "./uploads"
		if err := os.MkdirAll(uploadDir, 0755); err != nil {
			app.internalServerError(writer, request, err)
			return err, "", ""
		}

		filePath := filepath.Join(uploadDir, newFilename)
		dst, err := os.Create(filePath)
		if err != nil {
			app.internalServerError(writer, request, err)
			return err, "", ""
		}
		defer dst.Close()

		if _, err := io.Copy(dst, file); err != nil {
			app.internalServerError(writer, request, err)
			return err, "", ""
		}

		// For local development
		fileKey = newFilename
		fileURL = fmt.Sprintf("%s/uploads/%s", app.config.apiURL, newFilename)
	} else {
		// PRODUCTION: Upload to R2
		if app.storageClient == nil {
			app.internalServerError(writer, request, errors.New("storage service not available"))
			return errors.New("storage service not available"), "", ""
		}

		// Get content type
		contentType := storage.GetContentType(fileHeader.Filename)

		// Generate R2 key
		r2Key := fmt.Sprintf("categories/%s", newFilename)

		// Upload to R2
		result, err := app.storageClient.UploadFile(request.Context(), r2Key, file, contentType, fileHeader.Size)
		if err != nil {
			app.logger.Errorw("Failed to upload to R2", "error", err)
			app.internalServerError(writer, request, errors.New("failed to upload file"))
			return err, "", ""
		}

		// Store the R2 key and URL
		fileKey = result.Key
		fileURL = result.URL
	}

	return nil, fileKey, fileURL
}
