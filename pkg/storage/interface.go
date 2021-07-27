package storage

type Store interface {
	StorageIndentity() string
	FileUpload(bucketName string, objectName string, filePath string) error
	RawUpload(bucketName string, objectName string, content string) error
}
