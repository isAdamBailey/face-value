package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/google/uuid"
)

// S3Config holds the settings needed to construct an S3Store.
type S3Config struct {
	Bucket          string
	Region          string
	AccessKeyID     string
	SecretAccessKey string
	// Endpoint overrides the S3 endpoint, for MinIO in local dev. Leave
	// empty in production.
	Endpoint string
	// ForcePathStyle is required for MinIO (bucket-in-path rather than
	// bucket-as-subdomain). Must be false in production.
	ForcePathStyle bool
	PresignTTL     time.Duration
}

// S3Store implements ImageStore against Amazon S3 (or an S3-compatible
// endpoint such as MinIO).
type S3Store struct {
	client        *s3.Client
	presignClient *s3.PresignClient
	bucket        string
	presignTTL    time.Duration
}

// NewS3Store constructs an S3Store from cfg.
func NewS3Store(ctx context.Context, cfg S3Config) (*S3Store, error) {
	awsCfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(cfg.Region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, "")),
	)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if cfg.Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
		}
		o.UsePathStyle = cfg.ForcePathStyle
	})

	return &S3Store{
		client:        client,
		presignClient: s3.NewPresignClient(client),
		bucket:        cfg.Bucket,
		presignTTL:    cfg.PresignTTL,
	}, nil
}

// Put implements ImageStore. Keys are laid out as
// images/<yyyy>/<mm>/<uuid>.jpg — always .jpg, since Preprocess always
// re-encodes to JPEG before storage.
func (s *S3Store) Put(ctx context.Context, r io.Reader, mime string) (string, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return "", fmt.Errorf("read image data: %w", err)
	}

	now := time.Now().UTC()
	key := fmt.Sprintf("images/%04d/%02d/%s.jpg", now.Year(), now.Month(), uuid.NewString())

	_, err = s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:               aws.String(s.bucket),
		Key:                  aws.String(key),
		Body:                 bytes.NewReader(data),
		ContentType:          aws.String(mime),
		ServerSideEncryption: types.ServerSideEncryptionAes256,
	})
	if err != nil {
		return "", fmt.Errorf("put object %q: %w", key, err)
	}

	return key, nil
}

// Get implements ImageStore.
func (s *S3Store) Get(ctx context.Context, key string) (io.ReadCloser, string, error) {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, "", fmt.Errorf("get object %q: %w", key, err)
	}

	contentType := ""
	if out.ContentType != nil {
		contentType = *out.ContentType
	}

	return out.Body, contentType, nil
}

// URL implements ImageStore, returning a presigned GET URL valid for the
// configured PresignTTL.
func (s *S3Store) URL(ctx context.Context, key string) (string, error) {
	req, err := s.presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(s.presignTTL))
	if err != nil {
		return "", fmt.Errorf("presign get object %q: %w", key, err)
	}

	return req.URL, nil
}

// Delete implements ImageStore. A NoSuchKey error is treated as success —
// a missing object should not block deleting the row that referenced it.
func (s *S3Store) Delete(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		var nsk *types.NoSuchKey
		if errors.As(err, &nsk) {
			return nil
		}
		return fmt.Errorf("delete object %q: %w", key, err)
	}

	return nil
}
