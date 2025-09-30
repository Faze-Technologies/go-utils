package utils

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"time"

	"cloud.google.com/go/storage"
)

type UploadResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	FileURL string `json:"file_url,omitempty"`
	Error   string `json:"error,omitempty"`
}

type UploadValidation struct {
	Request *UploadRequest
	File    multipart.File
	Header  *multipart.FileHeader
}

type UploadRequest struct {
	FileName string `json:"file_name" validate:"required"`
	Bucket   string `json:"bucket" validate:"required"`
}

func ValidateAndExtractFormData(r *http.Request) (*UploadValidation, error) {

	file, header, err := r.FormFile("file")
	if err != nil {
		return nil, fmt.Errorf("file is required: %v", err)
	}
	defer file.Close()

	fileName := r.FormValue("file_name")
	if fileName == "" {
		return nil, fmt.Errorf("file_name is required")
	}

	bucketName := r.FormValue("bucket")
	if bucketName == "" {
		return nil, fmt.Errorf("bucket is required")
	}

	uploadRequest := &UploadRequest{
		FileName: fileName,
		Bucket:   bucketName,
	}

	return &UploadValidation{
		Request: uploadRequest,
		File:    file,
		Header:  header,
	}, nil
}

func UploadFileToGCP(ctx context.Context, validation *UploadValidation) (*UploadResponse, error) {
	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize storage client: %v", err)
	}
	defer client.Close()

	fileExt := filepath.Ext(validation.Header.Filename)
	uniqueFileName := fmt.Sprintf("%s_%d%s",
		validation.Request.FileName,
		time.Now().UnixNano(),
		fileExt,
	)

	bucket := client.Bucket(validation.Request.Bucket)

	obj := bucket.Object(uniqueFileName)

	wc := obj.NewWriter(ctx)

	if validation.Header.Header.Get("Content-Type") != "" {
		wc.ContentType = validation.Header.Header.Get("Content-Type")
	}

	if _, err := io.Copy(wc, validation.File); err != nil {
		return nil, fmt.Errorf("failed to copy file data: %v", err)
	}

	if err := wc.Close(); err != nil {
		return nil, fmt.Errorf("failed to close writer: %v", err)
	}

	attrs, err := obj.Attrs(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get object attributes: %v", err)
	}

	fileURL := fmt.Sprintf("https://storage.googleapis.com/%s/%s", attrs.Bucket, attrs.Name)

	return &UploadResponse{
		Success: true,
		Message: "File uploaded successfully",
		FileURL: fileURL,
	}, nil
}
