package utils

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/go-playground/validator/v10"
	"github.com/spf13/viper"
)

type GCSOptions struct {
	Enable           string `json:"enable" mapstructure:"enable" validate:"required,eq=true"`
	AccessKeyID      string `json:"accessKeyId" mapstructure:"accessKeyId" validate:"required"`
	SecretAccessKey  string `json:"secretAccessKey" mapstructure:"secretAccessKey" validate:"required"`
	Endpoint         string `json:"endpoint" mapstructure:"endpoint" validate:"omitempty,url"`
	S3ForcePathStyle string `json:"s3ForcePathStyle" mapstructure:"s3ForcePathStyle"`
	SignatureVersion string `json:"signatureVersion" mapstructure:"signatureVersion"`
}

type GCSConfig struct {
	AccessKeyID     string
	SecretAccessKey string
	Endpoint        string
	Bucket          string
	IsProd          bool
	ProdBaseURL     string
}

type GCSUploader struct {
	config   *GCSConfig
	validate *validator.Validate
}

type S3UploadRequest struct {
	FileName string `validate:"required"` // Can be a full path like "abc/ac/filename.json" or just "filename.json"
	File     multipart.File
	Header   *multipart.FileHeader
}

// Package-level uploader instance for direct usage
var defaultUploader *GCSUploader

// InitDefaultUploader initializes the default package-level uploader
// Call this once at application startup
func InitDefaultUploader(options GCSOptions, bucket string, isProd bool) error {
	uploader, err := NewGCSUploader(options, bucket, isProd)
	if err != nil {
		return err
	}
	defaultUploader = uploader
	return nil
}

// InitDefaultUploaderFromViper initializes the default uploader from viper config
func InitDefaultUploaderFromViper(v *viper.Viper, bucket string) error {
	uploader, err := NewGCSUploaderFromViper(v, bucket)
	if err != nil {
		return err
	}
	defaultUploader = uploader
	return nil
}

// InitDefaultUploaderFromEnv initializes the default uploader from environment variables
func InitDefaultUploaderFromEnv(bucket string) error {
	uploader, err := NewGCSUploaderFromEnv(bucket)
	if err != nil {
		return err
	}
	defaultUploader = uploader
	return nil
}

// UploadFileToGCS uploads a file using the default package-level uploader
// You must call InitDefaultUploader* first
func UploadFileToGCS(ctx context.Context, req *S3UploadRequest) (*UploadResponse, error) {
	if defaultUploader == nil {
		return nil, fmt.Errorf("uploader not initialized: call InitDefaultUploader first")
	}
	return defaultUploader.UploadFile(ctx, req)
}

// UploadFileToGCSWithConflictCheck uploads a file using the default package-level uploader
// Only adds timestamp if file already exists in the bucket
// You must call InitDefaultUploader* first
func UploadFileToGCSWithConflictCheck(ctx context.Context, req *S3UploadRequest) (*UploadResponse, error) {
	if defaultUploader == nil {
		return nil, fmt.Errorf("uploader not initialized: call InitDefaultUploader first")
	}
	return defaultUploader.UploadFileWithConflictCheck(ctx, req)
}

// validateBucketName validates GCS/S3 bucket name according to naming rules
func validateBucketName(bucket string) error {
	if bucket == "" {
		return fmt.Errorf("bucket name is required")
	}

	// Bucket name length must be between 3 and 63 characters
	if len(bucket) < 3 || len(bucket) > 63 {
		return fmt.Errorf("bucket name must be between 3 and 63 characters, got %d", len(bucket))
	}

	// Bucket name can only contain lowercase letters, numbers, hyphens, and dots
	for i, char := range bucket {
		if !((char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '-' || char == '.') {
			return fmt.Errorf("bucket name can only contain lowercase letters, numbers, hyphens, and dots. Invalid character '%c' at position %d", char, i)
		}
	}

	// Bucket name must start and end with a letter or number
	first := bucket[0]
	last := bucket[len(bucket)-1]
	if !((first >= 'a' && first <= 'z') || (first >= '0' && first <= '9')) {
		return fmt.Errorf("bucket name must start with a lowercase letter or number")
	}
	if !((last >= 'a' && last <= 'z') || (last >= '0' && last <= '9')) {
		return fmt.Errorf("bucket name must end with a lowercase letter or number")
	}

	return nil
}

// NewGCSUploader creates a new GCS uploader with the given configuration
// Validates all required fields before creating the uploader
func NewGCSUploader(options GCSOptions, bucket string, isProd bool) (*GCSUploader, error) {
	// Initialize validator
	validate := validator.New()

	// Validate GCSOptions struct
	if err := validate.Struct(options); err != nil {
		return nil, fmt.Errorf("validation failed: %v", err)
	}

	// Validate bucket name
	if err := validateBucketName(bucket); err != nil {
		return nil, fmt.Errorf("invalid bucket name: %v", err)
	}

	config, err := initGCSConfig(options, bucket, isProd)
	if err != nil {
		return nil, err
	}

	return &GCSUploader{
		config:   config,
		validate: validate,
	}, nil
}

// initGCSConfig initializes GCS configuration from GCSOptions struct (internal)
func initGCSConfig(options GCSOptions, bucket string, isProd bool) (*GCSConfig, error) {
	endpoint := options.Endpoint
	if endpoint == "" {
		endpoint = "https://storage.googleapis.com"
	}

	// Get production base URL from env if not provided
	prodBaseURL := os.Getenv("GCS_PROD_BASE_URL")
	if prodBaseURL == "" {
		prodBaseURL = "http://media.fancraze.com"
	}

	return &GCSConfig{
		AccessKeyID:     options.AccessKeyID,
		SecretAccessKey: options.SecretAccessKey,
		Endpoint:        endpoint,
		Bucket:          bucket,
		IsProd:          isProd,
		ProdBaseURL:     prodBaseURL,
	}, nil
}

// NewGCSUploaderFromViper creates a GCS uploader from viper config
// It reads from the "gcs" key in your config file
func NewGCSUploaderFromViper(v *viper.Viper, bucket string) (*GCSUploader, error) {
	var options GCSOptions
	if err := v.UnmarshalKey("gcs.options", &options); err != nil {
		return nil, fmt.Errorf("failed to unmarshal GCS options: %v", err)
	}

	options.Enable = v.GetString("gcs.enable")

	// Determine if running in production
	env := strings.ToLower(os.Getenv("ENV"))
	isProd := env == "prod" || env == "production"

	return NewGCSUploader(options, bucket, isProd)
}

// NewGCSUploaderFromEnv creates a GCS uploader from environment variables
// Validates all required environment variables before creating the uploader
func NewGCSUploaderFromEnv(bucket string) (*GCSUploader, error) {
	accessKeyID := os.Getenv("GCS_ACCESS_KEY_ID")
	if accessKeyID == "" {
		return nil, fmt.Errorf("GCS_ACCESS_KEY_ID environment variable is required")
	}

	secretAccessKey := os.Getenv("GCS_SECRET_ACCESS_KEY")
	if secretAccessKey == "" {
		return nil, fmt.Errorf("GCS_SECRET_ACCESS_KEY environment variable is required")
	}

	// Validate bucket name
	if err := validateBucketName(bucket); err != nil {
		return nil, fmt.Errorf("invalid bucket name: %v", err)
	}

	endpoint := os.Getenv("GCS_ENDPOINT")
	if endpoint == "" {
		endpoint = "https://storage.googleapis.com"
	}

	env := strings.ToLower(os.Getenv("ENV"))
	isProd := env == "prod" || env == "production"

	prodBaseURL := os.Getenv("GCS_PROD_BASE_URL")

	config := &GCSConfig{
		AccessKeyID:     accessKeyID,
		SecretAccessKey: secretAccessKey,
		Endpoint:        endpoint,
		Bucket:          bucket,
		IsProd:          isProd,
		ProdBaseURL:     prodBaseURL,
	}

	return &GCSUploader{
		config:   config,
		validate: validator.New(),
	}, nil
}

// UploadFile uploads a file to GCS using S3-compatible API
// Validates the request before uploading
// Returns URL based on environment:
//   - IsProd = true:  ProdBaseURL/fullPath
//   - IsProd = false: https://storage.googleapis.com/{bucket}/{fullPath}
func (u *GCSUploader) UploadFile(ctx context.Context, req *S3UploadRequest) (*UploadResponse, error) {
	// Validate request
	if err := u.validate.Struct(req); err != nil {
		return nil, fmt.Errorf("request validation failed: %v", err)
	}

	// Validate file and header are provided
	if req.File == nil {
		return nil, fmt.Errorf("file is required")
	}
	if req.Header == nil {
		return nil, fmt.Errorf("file header is required")
	}

	sess, err := session.NewSession(&aws.Config{
		Region:           aws.String("auto"),
		Endpoint:         aws.String(u.config.Endpoint),
		Credentials:      credentials.NewStaticCredentials(u.config.AccessKeyID, u.config.SecretAccessKey, ""),
		S3ForcePathStyle: aws.Bool(true),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create AWS session: %v", err)
	}

	svc := s3.New(sess)

	// Clean the path and extract directory and base filename
	cleanPath := strings.TrimPrefix(req.FileName, "/")
	dir := filepath.Dir(cleanPath)
	baseFileName := filepath.Base(cleanPath)

	// Get file extension from the uploaded file
	fileExt := filepath.Ext(req.Header.Filename)
	if fileExt == "" {
		fileExt = filepath.Ext(baseFileName)
	}

	// Remove extension from base filename if it exists
	if ext := filepath.Ext(baseFileName); ext != "" {
		baseFileName = strings.TrimSuffix(baseFileName, ext)
	}

	// Create unique filename with timestamp
	uniqueFileName := fmt.Sprintf("%s_%d%s", baseFileName, time.Now().UnixNano(), fileExt)

	// Construct full path with directory structure
	var fullPath string
	if dir != "" && dir != "." {
		fullPath = fmt.Sprintf("%s/%s", dir, uniqueFileName)
	} else {
		fullPath = uniqueFileName
	}

	fileBytes, err := io.ReadAll(req.File)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %v", err)
	}

	contentType := req.Header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	// Upload with full path - S3/GCS automatically creates folder structure
	_, err = svc.PutObjectWithContext(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(u.config.Bucket),
		Key:         aws.String(fullPath),
		Body:        bytes.NewReader(fileBytes),
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to upload file: %v", err)
	}

	var fileURL string
	if u.config.IsProd {
		if u.config.ProdBaseURL != "" {
			fileURL = fmt.Sprintf("%s/%s", u.config.ProdBaseURL, fullPath)
		} else {
			fileURL = fmt.Sprintf("http://media.fancraze.com/%s", fullPath)
		}
	} else {
		fileURL = fmt.Sprintf("https://storage.googleapis.com/%s/%s", u.config.Bucket, fullPath)
	}

	return &UploadResponse{
		Success: true,
		Message: "File uploaded successfully",
		FileURL: fileURL,
	}, nil
}

// UploadFileWithConflictCheck uploads a file to GCS using S3-compatible API
// Only adds timestamp to filename if a file with the same name already exists
// Returns URL based on environment:
//   - IsProd = true:  ProdBaseURL/fullPath
//   - IsProd = false: https://storage.googleapis.com/{bucket}/{fullPath}
func (u *GCSUploader) UploadFileWithConflictCheck(ctx context.Context, req *S3UploadRequest) (*UploadResponse, error) {
	// Validate request
	if err := u.validate.Struct(req); err != nil {
		return nil, fmt.Errorf("request validation failed: %v", err)
	}

	// Validate file and header are provided
	if req.File == nil {
		return nil, fmt.Errorf("file is required")
	}
	if req.Header == nil {
		return nil, fmt.Errorf("file header is required")
	}

	sess, err := session.NewSession(&aws.Config{
		Region:           aws.String("auto"),
		Endpoint:         aws.String(u.config.Endpoint),
		Credentials:      credentials.NewStaticCredentials(u.config.AccessKeyID, u.config.SecretAccessKey, ""),
		S3ForcePathStyle: aws.Bool(true),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create AWS session: %v", err)
	}

	svc := s3.New(sess)

	// Clean the path and extract directory and base filename
	cleanPath := strings.TrimPrefix(req.FileName, "/")
	dir := filepath.Dir(cleanPath)
	baseFileName := filepath.Base(cleanPath)

	// Get file extension from the uploaded file
	fileExt := filepath.Ext(req.Header.Filename)
	if fileExt == "" {
		fileExt = filepath.Ext(baseFileName)
	}

	// Remove extension from base filename if it exists
	if ext := filepath.Ext(baseFileName); ext != "" {
		baseFileName = strings.TrimSuffix(baseFileName, ext)
	}

	// Construct initial full path
	var fullPath string
	fileName := fmt.Sprintf("%s%s", baseFileName, fileExt)
	if dir != "" && dir != "." {
		fullPath = fmt.Sprintf("%s/%s", dir, fileName)
	} else {
		fullPath = fileName
	}

	// Check if file already exists
	_, err = svc.HeadObjectWithContext(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(u.config.Bucket),
		Key:    aws.String(fullPath),
	})

	// If file exists (no error), add timestamp to make it unique
	if err == nil {
		uniqueFileName := fmt.Sprintf("%s_%d%s", baseFileName, time.Now().UnixNano(), fileExt)
		if dir != "" && dir != "." {
			fullPath = fmt.Sprintf("%s/%s", dir, uniqueFileName)
		} else {
			fullPath = uniqueFileName
		}
	}
	// If error is not "NotFound", we continue with the original filename

	fileBytes, err := io.ReadAll(req.File)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %v", err)
	}

	contentType := req.Header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	// Upload with full path - S3/GCS automatically creates folder structure
	_, err = svc.PutObjectWithContext(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(u.config.Bucket),
		Key:         aws.String(fullPath),
		Body:        bytes.NewReader(fileBytes),
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to upload file: %v", err)
	}

	var fileURL string
	if u.config.IsProd {
		if u.config.ProdBaseURL != "" {
			fileURL = fmt.Sprintf("%s/%s", u.config.ProdBaseURL, fullPath)
		} else {
			fileURL = fmt.Sprintf("http://media.fancraze.com/%s", fullPath)
		}
	} else {
		fileURL = fmt.Sprintf("https://storage.googleapis.com/%s/%s", u.config.Bucket, fullPath)
	}

	return &UploadResponse{
		Success: true,
		Message: "File uploaded successfully",
		FileURL: fileURL,
	}, nil
}
