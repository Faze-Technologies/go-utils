package utils

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

// ErrObjectNotFound is wrapped, with the object's path, in ReadFromBucket's error for an object that does not
// exist.
var ErrObjectNotFound = errors.New("object not found")

// ReadFromBucket opens an object for reading, through the same S3-compatible API and gcs.* credentials as
// UploadDataToBucket. The object streams as it is read, so a big file never sits in memory; the caller
// closes it. The bytes come back as stored (an application/gzip object comes back as its gzip bytes). A
// missing object returns an error wrapping ErrObjectNotFound.
func (u *GCSUploader) ReadFromBucket(ctx context.Context, bucketName string, path string) (io.ReadCloser, error) {
	if bucketName == "" {
		return nil, fmt.Errorf("bucket name is required")
	}
	if path == "" {
		return nil, fmt.Errorf("path is required")
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

	key := strings.TrimPrefix(path, "/")
	out, err := s3.New(sess).GetObjectWithContext(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
	})
	if err != nil {
		var reqErr awserr.RequestFailure
		if errors.As(err, &reqErr) && reqErr.StatusCode() == http.StatusNotFound {
			return nil, fmt.Errorf("gs://%s/%s: %w", bucketName, key, ErrObjectNotFound)
		}
		return nil, fmt.Errorf("failed to read gs://%s/%s: %w", bucketName, key, err)
	}
	return out.Body, nil
}
