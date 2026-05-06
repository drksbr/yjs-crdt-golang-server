package objectstore

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
	"github.com/drksbr/yjs-crdt-golang-server/examples/DontPadBR3/apps/backend/internal/common"
)

// S3Config contem as opcoes do backend S3.
type S3Config struct {
	Bucket    string
	Prefix    string
	Region    string
	Endpoint  string
	Profile   string
	PathStyle bool
	TempDir   string
}

// S3Store guarda objetos em um bucket S3 compativel.
type S3Store struct {
	client *s3.Client
	bucket string
	prefix string
	temp   string
}

// NewS3 cria um object store S3 usando a cadeia padrao de credenciais AWS.
func NewS3(ctx context.Context, cfg S3Config) (*S3Store, error) {
	bucket := strings.TrimSpace(cfg.Bucket)
	if bucket == "" {
		return nil, fmt.Errorf("S3 bucket is required")
	}

	options := make([]func(*config.LoadOptions) error, 0, 2)
	if strings.TrimSpace(cfg.Region) != "" {
		options = append(options, config.WithRegion(strings.TrimSpace(cfg.Region)))
	}
	if strings.TrimSpace(cfg.Profile) != "" {
		options = append(options, config.WithSharedConfigProfile(strings.TrimSpace(cfg.Profile)))
	}
	awsCfg, err := config.LoadDefaultConfig(ctx, options...)
	if err != nil {
		return nil, fmt.Errorf("load AWS config: %w", err)
	}

	endpoint := strings.TrimSpace(cfg.Endpoint)
	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.UsePathStyle = cfg.PathStyle
		if endpoint != "" {
			o.BaseEndpoint = aws.String(endpoint)
		}
	})

	return &S3Store{
		client: client,
		bucket: bucket,
		prefix: cleanPrefix(cfg.Prefix),
		temp:   strings.TrimSpace(cfg.TempDir),
	}, nil
}

func (s *S3Store) Put(ctx context.Context, key string, src io.Reader, opts PutOptions) (int64, error) {
	clean, err := cleanKey(key)
	if err != nil {
		return 0, err
	}
	temp, err := os.CreateTemp(s.temp, "dontpad-s3-upload-*")
	if err != nil {
		return 0, err
	}
	tempName := temp.Name()
	defer func() {
		_ = os.Remove(tempName)
	}()

	written, copyErr := copyLimited(ctx, temp, src, opts.MaxBytes)
	closeErr := temp.Close()
	if copyErr != nil {
		return written, copyErr
	}
	if closeErr != nil {
		return written, closeErr
	}

	file, err := os.Open(tempName)
	if err != nil {
		return written, err
	}
	defer func() {
		_ = file.Close()
	}()

	input := &s3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(s.key(clean)),
		Body:          file,
		ContentLength: aws.Int64(written),
	}
	if strings.TrimSpace(opts.ContentType) != "" {
		input.ContentType = aws.String(strings.TrimSpace(opts.ContentType))
	}
	if _, err := s.client.PutObject(ctx, input); err != nil {
		return written, err
	}
	return written, nil
}

func (s *S3Store) Get(ctx context.Context, key string) (*Object, error) {
	clean, err := cleanKey(key)
	if err != nil {
		return nil, err
	}
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.key(clean)),
	})
	if err != nil {
		if isS3NotFound(err) {
			return nil, common.ErrNotFound
		}
		return nil, err
	}
	return &Object{
		Body:        out.Body,
		Size:        aws.ToInt64(out.ContentLength),
		ContentType: aws.ToString(out.ContentType),
	}, nil
}

func (s *S3Store) Exists(ctx context.Context, key string) (bool, error) {
	clean, err := cleanKey(key)
	if err != nil {
		return false, err
	}
	if _, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.key(clean)),
	}); err != nil {
		if isS3NotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (s *S3Store) Delete(ctx context.Context, key string) error {
	clean, err := cleanKey(key)
	if err != nil {
		return err
	}
	_, err = s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.key(clean)),
	})
	return err
}

func (s *S3Store) DeletePrefix(ctx context.Context, prefix string) error {
	clean, err := cleanKey(prefix)
	if err != nil {
		return err
	}
	prefixKey := s.key(clean)
	normalizedPrefix := strings.TrimSpace(strings.ReplaceAll(prefix, "\\", "/"))
	if strings.HasSuffix(normalizedPrefix, "/") && !strings.HasSuffix(prefixKey, "/") {
		prefixKey += "/"
	}
	paginator := s3.NewListObjectsV2Paginator(s.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
		Prefix: aws.String(prefixKey),
	})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return err
		}
		for _, object := range page.Contents {
			if object.Key == nil {
				continue
			}
			if _, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
				Bucket: aws.String(s.bucket),
				Key:    object.Key,
			}); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *S3Store) String() string {
	if s.prefix == "" {
		return "s3://" + s.bucket
	}
	return "s3://" + s.bucket + "/" + s.prefix
}

func (s *S3Store) key(clean string) string {
	if s.prefix == "" {
		return clean
	}
	return path.Join(s.prefix, clean)
}

func cleanPrefix(prefix string) string {
	prefix = strings.TrimSpace(strings.ReplaceAll(prefix, "\\", "/"))
	prefix = strings.Trim(prefix, "/")
	if prefix == "" {
		return ""
	}
	return strings.TrimPrefix(path.Clean("/"+prefix), "/")
}

func isS3NotFound(err error) bool {
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "NoSuchKey", "NotFound", "404":
			return true
		default:
			return false
		}
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "no such key") ||
		strings.Contains(message, "nosuchkey") ||
		strings.Contains(message, "not found") ||
		strings.Contains(message, "notfound")
}
