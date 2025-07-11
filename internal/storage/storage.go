package storage

import (
	"context"
	"io"
)

type Client interface {
	UploadFile(ctx context.Context, key string, file io.Reader, contentType string, size int64) (*UploadResult, error)
	DeleteFile(ctx context.Context, key string) error
	GetFileURL(key string) string
}

type UploadResult struct {
	Key string `json:"key"`
	URL string `json:"url"`
}