package storage

import (
	"context"
	"fmt"
	"io"
	"mime"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/google/uuid"
)

type R2Client struct {
	client     *s3.Client
	bucketName string
	publicURL  string
}

type customEndpointResolver struct {
	endpoint string
}

func (c customEndpointResolver) ResolveEndpoint(service, region string) (aws.Endpoint, error) {
	if service == s3.ServiceID {
		return aws.Endpoint{
			URL:           c.endpoint,
			SigningRegion: "auto",
		}, nil
	}
	return aws.Endpoint{}, fmt.Errorf("unknown endpoint requested for %s", service)
}

func NewR2Client(endpoint, accessKeyID, secretAccessKey, bucketName, publicURL string) (*R2Client, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithEndpointResolver(customEndpointResolver{endpoint: endpoint}),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKeyID, secretAccessKey, "")),
		config.WithRegion("auto"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.UsePathStyle = true
	})

	return &R2Client{
		client:     client,
		bucketName: bucketName,
		publicURL:  publicURL,
	}, nil
}

func (r *R2Client) UploadFile(ctx context.Context, key string, file io.Reader, contentType string, size int64) (*UploadResult, error) {
	uploadInput := &s3.PutObjectInput{
		Bucket:        aws.String(r.bucketName),
		Key:           aws.String(key),
		Body:          file,
		ContentType:   aws.String(contentType),
		ContentLength: aws.Int64(size),
		ACL:           types.ObjectCannedACLPublicRead,
	}

	_, err := r.client.PutObject(ctx, uploadInput)
	if err != nil {
		return nil, fmt.Errorf("failed to upload file to R2: %w", err)
	}

	publicURL := r.GetFileURL(key)

	return &UploadResult{
		Key: key,
		URL: publicURL,
	}, nil
}

func (r *R2Client) GetFileURL(key string) string {
	if r.publicURL != "" {
		return fmt.Sprintf("%s/%s/%s", strings.TrimSuffix(r.publicURL, "/"), r.bucketName, key)
	}
	return fmt.Sprintf("https://pub-%s.r2.dev/%s", r.bucketName, key)
}

// Helper functions
func GenerateFileKey(originalFilename string) string {
	ext := filepath.Ext(originalFilename)
	id := uuid.New().String()
	timestamp := time.Now().Unix()
	return fmt.Sprintf("uploads/%d-%s%s", timestamp, id, ext)
}

func GetContentType(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	contentType := mime.TypeByExtension(ext)
	if contentType == "" {
		return "application/octet-stream"
	}
	return contentType
}

func (r *R2Client) DeleteFile(ctx context.Context, key string) error {
	deleteInput := &s3.DeleteObjectInput{
		Bucket: aws.String(r.bucketName),
		Key:    aws.String(key),
	}

	_, err := r.client.DeleteObject(ctx, deleteInput)
	if err != nil {
		return fmt.Errorf("failed to delete file from R2: %w", err)
	}

	return nil
}
