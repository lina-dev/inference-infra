package s3

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// Client wraps the AWS S3 SDK with domain-level helpers.
type Client struct {
	s3 *s3.Client
}

func New(cfg aws.Config) *Client {
	return &Client{s3: s3.NewFromConfig(cfg)}
}

// Download streams an S3 object to a local file.
func (c *Client) Download(ctx context.Context, bucket, key, destPath string) error {
	resp, err := c.s3.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &key,
	})
	if err != nil {
		return fmt.Errorf("get object s3://%s/%s: %w", bucket, key, err)
	}
	defer resp.Body.Close()

	f, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("create file %s: %w", destPath, err)
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	return nil
}

// UploadJSON marshals v as JSON and puts it at s3://bucket/key.
func (c *Client) UploadJSON(ctx context.Context, bucket, key string, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal json: %w", err)
	}
	_, err = c.s3.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        &bucket,
		Key:           &key,
		Body:          strings.NewReader(string(data)),
		ContentType:   aws.String("application/json"),
		ContentLength: aws.Int64(int64(len(data))),
	})
	return err
}
