package server

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	appconfig "github.com/kastoras/images-api/internal/config"
)

type ObjectStorage struct {
	client  *s3.Client
	presign *s3.PresignClient
	bucket  string
}

func initObjectStorage(ctx context.Context, cfg *appconfig.Config) (*ObjectStorage, error) {
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(cfg.S3Region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.S3AccessKey, cfg.S3SecretKey, "",
		)),
	)
	if err != nil {
		return nil, err
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if cfg.S3Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.S3Endpoint)
		}
		o.UsePathStyle = cfg.S3UsePathStyle
	})

	return &ObjectStorage{
		client:  client,
		presign: s3.NewPresignClient(client),
		bucket:  cfg.S3Bucket,
	}, nil
}

func (o *ObjectStorage) Ping(ctx context.Context) error {
	_, err := o.client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(o.bucket),
	})
	return err
}

func (o *ObjectStorage) Upload(ctx context.Context, key string, body io.Reader, size int64) error {
	_, err := o.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(o.bucket),
		Key:           aws.String(key),
		Body:          body,
		ContentLength: aws.Int64(size),
	})
	return err
}

func (o *ObjectStorage) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	result, err := o.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(o.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, err
	}
	return result.Body, nil
}

func (o *ObjectStorage) Delete(ctx context.Context, key string) error {
	_, err := o.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(o.bucket),
		Key:    aws.String(key),
	})
	return err
}

func (o *ObjectStorage) Presign(ctx context.Context, key string, expiry time.Duration) (string, error) {
	req, err := o.presign.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(o.bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(expiry))
	if err != nil {
		return "", err
	}
	return req.URL, nil
}

// UploadWithMetadata is Upload plus S3 object user-metadata (x-amz-meta-*),
// used to attach master dimensions/format without a separate database.
func (o *ObjectStorage) UploadWithMetadata(ctx context.Context, key string, body io.Reader, size int64, metadata map[string]string) error {
	_, err := o.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(o.bucket),
		Key:           aws.String(key),
		Body:          body,
		ContentLength: aws.Int64(size),
		Metadata:      metadata,
	})
	return err
}

// ErrNotFound is returned by HeadObject when the key does not exist.
var ErrNotFound = errors.New("object not found")

// HeadObject returns an existing object's user-metadata and size without
// downloading its body. Returns ErrNotFound if the key does not exist.
func (o *ObjectStorage) HeadObject(ctx context.Context, key string) (metadata map[string]string, size int64, err error) {
	out, err := o.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(o.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		var notFound *types.NotFound
		if errors.As(err, &notFound) {
			return nil, 0, ErrNotFound
		}
		var noSuchKey *types.NoSuchKey
		if errors.As(err, &noSuchKey) {
			return nil, 0, ErrNotFound
		}
		return nil, 0, err
	}
	return out.Metadata, aws.ToInt64(out.ContentLength), nil
}

// ObjectInfo is a single entry from ListKeys.
type ObjectInfo struct {
	Key          string
	Size         int64
	LastModified time.Time
}

// ListKeys lists every object under prefix, paginating as needed.
func (o *ObjectStorage) ListKeys(ctx context.Context, prefix string) ([]ObjectInfo, error) {
	var objects []ObjectInfo
	var token *string

	for {
		out, err := o.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
			Bucket:            aws.String(o.bucket),
			Prefix:            aws.String(prefix),
			ContinuationToken: token,
		})
		if err != nil {
			return nil, err
		}

		for _, obj := range out.Contents {
			info := ObjectInfo{Key: aws.ToString(obj.Key), Size: aws.ToInt64(obj.Size)}
			if obj.LastModified != nil {
				info.LastModified = *obj.LastModified
			}
			objects = append(objects, info)
		}

		if !aws.ToBool(out.IsTruncated) {
			break
		}
		token = out.NextContinuationToken
	}

	return objects, nil
}

// DeletePrefix deletes every object under prefix. Used to remove all of a
// master's cached derivatives alongside the master itself.
func (o *ObjectStorage) DeletePrefix(ctx context.Context, prefix string) error {
	objects, err := o.ListKeys(ctx, prefix)
	if err != nil {
		return err
	}

	for _, obj := range objects {
		if err := o.Delete(ctx, obj.Key); err != nil {
			return err
		}
	}

	return nil
}
