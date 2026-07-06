package objectstore

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

type r2Client struct {
	bucket  string
	s3      *s3.Client
	presign *s3.PresignClient
}

// NewR2 creates an R2-backed Client.
// endpoint is typically https://<accountID>.r2.cloudflarestorage.com.
func NewR2(
	endpoint string,
	accessKeyID string,
	secretAccessKey string,
	bucket string,
) Client {
	creds := credentials.NewStaticCredentialsProvider(accessKeyID, secretAccessKey, "")

	//nolint:exhaustruct //aws.Config has many optional fields; only relevant ones set
	cfg := aws.Config{
		Region:      "auto",
		Credentials: creds,
	}

	s3c := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
		// R2 uses path-style addressing.
		o.UsePathStyle = true
		// R2 rejects the aws-sdk-go-v2 default request checksums (CRC32) with a
		// 403 AccessDenied; only send checksums when an operation requires them.
		o.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
		o.ResponseChecksumValidation = aws.ResponseChecksumValidationWhenRequired
	})

	return &r2Client{
		bucket:  bucket,
		s3:      s3c,
		presign: s3.NewPresignClient(s3c),
	}
}

func (c *r2Client) Put(
	ctx context.Context,
	key string,
	r io.Reader,
	size int64,
	contentType string,
) error {
	//nolint:exhaustruct //s3.PutObjectInput has many optional fields
	_, err := c.s3.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(c.bucket),
		Key:           aws.String(key),
		Body:          r,
		ContentLength: aws.Int64(size),
		ContentType:   aws.String(contentType),
	})
	return err
}

func (c *r2Client) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	//nolint:exhaustruct //s3.GetObjectInput has many optional fields
	out, err := c.s3.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, err
	}
	return out.Body, nil
}

func (c *r2Client) PresignGet(
	ctx context.Context,
	key string,
	ttl time.Duration,
) (string, error) {
	//nolint:exhaustruct //s3.GetObjectInput has many optional fields
	req, err := c.presign.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(ttl))
	if err != nil {
		return "", fmt.Errorf("presign get %q: %w", key, err)
	}
	return req.URL, nil
}

func (c *r2Client) PresignPut(
	ctx context.Context,
	key string,
	ttl time.Duration,
	contentType string,
) (string, error) {
	//nolint:exhaustruct //s3.PutObjectInput has many optional fields
	req, err := c.presign.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(c.bucket),
		Key:         aws.String(key),
		ContentType: aws.String(contentType),
	}, s3.WithPresignExpires(ttl))
	if err != nil {
		return "", fmt.Errorf("presign put %q: %w", key, err)
	}
	return req.URL, nil
}

func (c *r2Client) Delete(ctx context.Context, key string) error {
	//nolint:exhaustruct //s3.DeleteObjectInput has many optional fields
	_, err := c.s3.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})
	return err
}

func (c *r2Client) Copy(ctx context.Context, srcKey, dstKey string) error {
	copySource := c.bucket + "/" + srcKey
	//nolint:exhaustruct //s3.CopyObjectInput has many optional fields
	_, err := c.s3.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:     aws.String(c.bucket),
		CopySource: aws.String(copySource),
		Key:        aws.String(dstKey),
	})
	return err
}

func (c *r2Client) List(
	ctx context.Context,
	prefix string,
) ([]ObjectInfo, error) {
	//nolint:exhaustruct //s3.ListObjectsV2Input has many optional fields
	input := &s3.ListObjectsV2Input{
		Bucket: aws.String(c.bucket),
		Prefix: aws.String(prefix),
	}

	var objects []ObjectInfo
	paginator := s3.NewListObjectsV2Paginator(c.s3, input)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("list objects %q: %w", prefix, err)
		}

		for _, obj := range page.Contents {
			objects = append(objects, ObjectInfo{
				Key:          aws.ToString(obj.Key),
				Size:         aws.ToInt64(obj.Size),
				LastModified: aws.ToTime(obj.LastModified),
			})
		}
	}

	return objects, nil
}

func (c *r2Client) Exists(ctx context.Context, key string) (bool, error) {
	//nolint:exhaustruct //s3.HeadObjectInput has many optional fields
	_, err := c.s3.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		var notFound *types.NotFound
		if errors.As(err, &notFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
