package minio

import (
	"context"
	"strings"

	"github.com/go-logr/logr"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

const storageIndentity = "minio"

// minioStorage manages objects storage in minio.
type minioStorage struct {
	// indentity proves its identity.
	indentity string
	// Context carries values across API boundaries.
	context.Context
	// Logger represents the ability to log messages.
	logr.Logger
	// minioClient knowns how to curd minio.
	minioClient *minio.Client
}

// NewMinioStorage creates a new minioStorage.
func NewMinioStorage(endpoint, accessKeyID, secretAccessKey string, useSSL bool, ctx context.Context, logger logr.Logger) (*minioStorage, error) {
	minioClient, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKeyID, secretAccessKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, err
	}
	return &minioStorage{
		Context:     ctx,
		Logger:      logger,
		minioClient: minioClient,
		indentity:   storageIndentity,
	}, nil
}

// StorageIndentity provide proof of identity.
func (ms *minioStorage) StorageIndentity() string {
	return ms.indentity
}

// FileUpload can upload a file.
func (ms *minioStorage) FileUpload(bucketName string, objectName string, filePath string) error {
	// Make a new bucket based on operation name if not exist.
	ctx := context.Background()
	err := ms.minioClient.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{})
	if err != nil {
		exists, errBucketExists := ms.minioClient.BucketExists(ctx, bucketName)
		if errBucketExists == nil && exists {
			ms.Info("bucket already exist.", "bucketName", bucketName)
		} else {
			return err
		}
	} else {
		ms.Info("create bucket successfully.", "bucketName", bucketName)
	}

	// Upload file with FPutObject func.
	info, err := ms.minioClient.FPutObject(ctx, bucketName, objectName, filePath, minio.PutObjectOptions{})
	if err != nil {
		return err
	}
	ms.Info("upload file successfully", "bucketName", bucketName, "objectName", objectName, "size", info.Size)

	return nil
}

// RawUpload stores raw data.
func (ms *minioStorage) RawUpload(bucketName string, objectName string, object string) error {
	// Make a new bucket based on operation name if not exist.
	ctx := context.Background()
	err := ms.minioClient.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{})
	if err != nil {
		exists, errBucketExists := ms.minioClient.BucketExists(ctx, bucketName)
		if errBucketExists == nil && exists {
			ms.Info("bucket already exist.", "bucketName", bucketName)
		} else {
			return err
		}
	} else {
		ms.Info("create bucket successfully.", "bucketName", bucketName)
	}
	rawContent := strings.NewReader(object)
	info, err := ms.minioClient.PutObject(ctx, bucketName, objectName, rawContent, rawContent.Size(), minio.PutObjectOptions{})
	if err != nil {
		return err
	}
	ms.Info("upload object succeed.", "bucketName", bucketName, "objectName", objectName, "size", info.Size)

	return nil
}
