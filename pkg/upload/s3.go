package upload

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/Saadlulu/dumptruckd/internal/utils"
	"github.com/Saadlulu/dumptruckd/pkg/config"
)

// S3Uploader uploads backup files to Amazon S3 or S3-compatible services.
type S3Uploader struct {
	cfg     config.S3Config
	session *session.Session
	client  *s3.S3
}

// NewS3Uploader creates a new S3 uploader.
// Credentials are resolved via the AWS SDK default credential chain, which supports
// (in order): env vars, shared credentials file, EC2 instance roles, ECS task roles,
// and IRSA (EKS). Static credentials via AWS_ACCESS_KEY_ID/AWS_SECRET_ACCESS_KEY
// still work — they're just one option in the chain.
func NewS3Uploader(cfg config.S3Config) (*S3Uploader, error) {
	if cfg.Bucket == "" {
		return nil, fmt.Errorf("s3 bucket is required")
	}

	region := cfg.Region
	if region == "" {
		region = "us-east-1"
	}

	// Use the SDK default credential chain (env vars, instance profile, IRSA, shared credentials, etc.)
	awsCfg := &aws.Config{
		Region: aws.String(region),
	}
	if cfg.Endpoint != "" {
		awsCfg.Endpoint = aws.String(cfg.Endpoint)
		awsCfg.S3ForcePathStyle = aws.Bool(true) // Required for S3-compatible services (MinIO, etc.)
	}

	sess, err := session.NewSession(awsCfg)
	if err != nil {
		return nil, fmt.Errorf("create AWS session: %w", err)
	}

	// Verify credentials are available now, not at 2am
	if _, err := sess.Config.Credentials.Get(); err != nil {
		return nil, fmt.Errorf("no AWS credentials found (set AWS_ACCESS_KEY_ID/AWS_SECRET_ACCESS_KEY, use an instance profile, or configure IRSA): %w", err)
	}

	client := s3.New(sess)

	return &S3Uploader{
		cfg:     cfg,
		session: sess,
		client:  client,
	}, nil
}

func (u *S3Uploader) Upload(ctx context.Context, filePath string, backupName string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("open file: %w", err)
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return "", fmt.Errorf("stat file: %w", err)
	}

	fileName := filepath.Base(filePath)
	key, err := utils.BuildBackupPath(u.cfg.Prefix, backupName, fileName)
	if err != nil {
		return "", fmt.Errorf("build backup path: %w", err)
	}

	input := &s3.PutObjectInput{
		Bucket:        aws.String(u.cfg.Bucket),
		Key:           aws.String(key),
		Body:          file,
		ContentLength: aws.Int64(fileInfo.Size()),
	}
	// Enable SSE for real S3 (not S3-compatible services)
	if u.cfg.Endpoint == "" {
		input.ServerSideEncryption = aws.String("AES256")
	}

	_, err = u.client.PutObjectWithContext(ctx, input)
	if err != nil {
		return "", fmt.Errorf("upload to S3: %w", err)
	}

	remotePath := fmt.Sprintf("s3://%s/%s", u.cfg.Bucket, key)
	return remotePath, nil
}

// parseS3Key extracts the S3 object key from an s3://bucket/key path.
func (u *S3Uploader) parseS3Key(remotePath string) string {
	if strings.HasPrefix(remotePath, "s3://") {
		path := strings.TrimPrefix(remotePath, "s3://")
		if idx := strings.Index(path, "/"); idx >= 0 {
			return path[idx+1:]
		}
		return path
	}
	return remotePath
}

// Verify checks if a file exists in S3.
func (u *S3Uploader) Verify(ctx context.Context, remotePath string) error {
	key := u.parseS3Key(remotePath)

	_, err := u.client.HeadObjectWithContext(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(u.cfg.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("file not found or not accessible: %w", err)
	}

	return nil
}

// Delete removes a file from S3.
func (u *S3Uploader) Delete(ctx context.Context, remotePath string) error {
	key := u.parseS3Key(remotePath)

	_, err := u.client.DeleteObjectWithContext(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(u.cfg.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("delete from S3 failed: %w", err)
	}

	return nil
}
