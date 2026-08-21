// Package storage wraps the real AWS S3 SDK, pointed at a custom endpoint
// for local dev (MinIO) via config.S3Endpoint. Swapping to real AWS S3 in
// production is an env var change (leave S3_ENDPOINT unset, the SDK falls
// back to AWS's actual endpoints and picks up real credentials the normal
// way) -- no code here is MinIO-specific except EnsureBucket's public-read
// policy, which is called out below as a local-dev shortcut.
package storage

import (
	"bytes"
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"stream-service/internal/config"
)

type Client struct {
	s3             *s3.Client
	bucket         string
	publicEndpoint string
}

func New(ctx context.Context, cfg config.Config) (*Client, error) {
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(cfg.S3Region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.S3AccessKey, cfg.S3SecretKey, "")),
	)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if cfg.S3Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.S3Endpoint)
			o.UsePathStyle = true // MinIO (and most S3-alikes) need path-style; real AWS S3 doesn't care once this is unset in prod
		}
	})

	return &Client{s3: client, bucket: cfg.S3Bucket, publicEndpoint: cfg.S3PublicEndpoint}, nil
}

// EnsureBucket creates the bucket if it doesn't exist and makes it
// public-read. Public-read is a local-dev shortcut so the frontend can
// play a recording straight from a plain URL with zero extra plumbing --
// a real deployment would keep the bucket private and serve recordings
// through CloudFront with signed URLs instead (same shape as fraud-
// detection's demo-grade-in-cluster-vs-managed-services trade-off).
func (c *Client) EnsureBucket(ctx context.Context) error {
	_, err := c.s3.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(c.bucket)})
	if err != nil && !isBucketAlreadyOwned(err) {
		return fmt.Errorf("create bucket: %w", err)
	}

	policy := fmt.Sprintf(`{
		"Version": "2012-10-17",
		"Statement": [{
			"Effect": "Allow",
			"Principal": "*",
			"Action": "s3:GetObject",
			"Resource": "arn:aws:s3:::%s/*"
		}]
	}`, c.bucket)
	_, err = c.s3.PutBucketPolicy(ctx, &s3.PutBucketPolicyInput{
		Bucket: aws.String(c.bucket),
		Policy: aws.String(policy),
	})
	if err != nil {
		return fmt.Errorf("set bucket policy: %w", err)
	}
	return nil
}

// Upload stores body under key and returns a URL a browser can fetch
// directly (works because EnsureBucket made the bucket public-read).
func (c *Client) Upload(ctx context.Context, key string, body []byte, contentType string) (string, error) {
	_, err := c.s3.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(c.bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(body),
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return "", fmt.Errorf("put object: %w", err)
	}
	return fmt.Sprintf("%s/%s/%s", c.publicEndpoint, c.bucket, key), nil
}

func isBucketAlreadyOwned(err error) bool {
	var msg string
	if err != nil {
		msg = err.Error()
	}
	return bytes.Contains([]byte(msg), []byte("BucketAlreadyOwnedByYou")) ||
		bytes.Contains([]byte(msg), []byte("BucketAlreadyExists"))
}
