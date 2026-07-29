package s3x

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/leafney/filedock/pkg/configx"
	"github.com/leafney/filedock/pkg/zlogx"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type S3Svc struct {
	client *minio.Client
	bucket string
	log    *zlogx.ZLogSvc
}

// NewS3Svc 初始化 S3 服务
func NewS3Svc(cfg configx.S3Config, log *zlogx.ZLogSvc) *S3Svc {
	endpoint := cfg.GetS3Endpoint()
	accessKeyID := cfg.GetS3AccessKey()
	secretAccessKey := cfg.GetS3SecretKey()
	useSSL := cfg.GetS3UseSSL()
	bucket := cfg.GetS3Bucket()

	// 去除 protocol (http:// or https://) 如果存在，因为 minio-go 需要 endpoint 不带 protocol
	endpoint = strings.TrimPrefix(endpoint, "http://")
	endpoint = strings.TrimPrefix(endpoint, "https://")

	// 初始化 MinIO 客户端对象
	minioClient, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKeyID, secretAccessKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		log.Error(fmt.Sprintf("[S3] init client error: %v", err))
		return nil // 或者 panic，视策略而定
	}

	// 检查 bucket 是否存在，不存在则创建
	// 注意：这里需要 context，暂使用 Background
	ctx := context.Background()
	exists, err := minioClient.BucketExists(ctx, bucket)
	if err != nil {
		log.Error(fmt.Sprintf("[S3] check bucket exists error: %v", err))
		// 不阻断启动，但在调用时可能会失败
	} else if !exists {
		err = minioClient.MakeBucket(ctx, bucket, minio.MakeBucketOptions{})
		if err != nil {
			log.Error(fmt.Sprintf("[S3] make bucket error: %v", err))
		} else {
			log.Info(fmt.Sprintf("[S3] bucket created: %s", bucket))
		}
	}

	log.Info("[S3] Load successful")
	return &S3Svc{
		client: minioClient,
		bucket: bucket,
		log:    log,
	}
}

// UploadContent 上传文本内容作为文件
// objectName: 文件名 (包含路径)
// content: 文本内容
// contentType: 内容类型 (e.g. "text/plain", "application/javascript")
func (s *S3Svc) UploadContent(ctx context.Context, objectName string, content string, contentType string) error {
	if s.client == nil {
		return fmt.Errorf("s3 client not initialized")
	}

	reader := strings.NewReader(content)
	size := int64(reader.Len())

	_, err := s.client.PutObject(ctx, s.bucket, objectName, reader, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		s.log.Error(fmt.Sprintf("[S3] upload error: %v", err))
		return err
	}

	return nil
}

// UploadFile 上传文件
func (s *S3Svc) UploadFile(ctx context.Context, objectName string, fileData []byte, contentType string) error {
	if s.client == nil {
		return fmt.Errorf("s3 client not initialized")
	}

	reader := bytes.NewReader(fileData)
	size := int64(reader.Len())

	_, err := s.client.PutObject(ctx, s.bucket, objectName, reader, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		s.log.Error(fmt.Sprintf("[S3] upload file error: %v", err))
		return err
	}

	return nil
}

// GetFileURL 获取文件访问链接 (Presigned URL)
// expiry: 过期时间 (秒)
func (s *S3Svc) GetFileURL(ctx context.Context, objectName string, expiry int) (string, error) {
	if s.client == nil {
		return "", fmt.Errorf("s3 client not initialized")
	}

	// Set request parameters for content-disposition.
	reqParams := make(url.Values)
	// reqParams.Set("response-content-disposition", "attachment; filename=\"test.txt\"")

	// Generates a presigned url which expires in a day.
	presignedURL, err := s.client.PresignedGetObject(ctx, s.bucket, objectName, time.Duration(expiry)*time.Second, reqParams)
	if err != nil {
		return "", err
	}

	return presignedURL.String(), nil
}

// DownloadContent 下载文本内容
// objectName: 文件名 (包含路径)
func (s *S3Svc) DownloadContent(ctx context.Context, objectName string) ([]byte, error) {
	if s.client == nil {
		return nil, fmt.Errorf("s3 client not initialized")
	}

	object, err := s.client.GetObject(ctx, s.bucket, objectName, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	defer object.Close()

	data, err := io.ReadAll(object)
	if err != nil {
		return nil, err
	}
	return data, nil
}
