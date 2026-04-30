package services

import (
	"io"
	"mime"
	"os"
	"path/filepath"

	"github.com/google/uuid"
)

type StorageService struct {
	basePath string
}

func NewStorageService(basePath string) *StorageService {
	return &StorageService{
		basePath: basePath,
	}
}

func (s *StorageService) SaveFile(userID uuid.UUID, filename string, fileData io.Reader) (string, int64, error) {
	// Генерируем уникальное имя файла
	ext := filepath.Ext(filename)
	uniqueID := uuid.New().String()
	storedName := uniqueID + ext

	// Создаем директорию пользователя
	userDir := filepath.Join(s.basePath, userID.String())
	if err := os.MkdirAll(userDir, 0755); err != nil {
		return "", 0, err
	}

	// Полный путь к файлу
	filePath := filepath.Join(userDir, storedName)

	// Создаем и сохраняем файл
	dst, err := os.Create(filePath)
	if err != nil {
		return "", 0, err
	}
	defer dst.Close()

	written, err := io.Copy(dst, fileData)
	if err != nil {
		os.Remove(filePath)
		return "", 0, err
	}

	return filePath, written, nil
}

func (s *StorageService) GetFile(filePath string) (*os.File, error) {
	return os.Open(filePath)
}

func (s *StorageService) DeleteFile(filePath string) error {
	return os.Remove(filePath)
}

func (s *StorageService) GetFileSize(filePath string) (int64, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

func (s *StorageService) GetMimeType(filename string) string {
	ext := filepath.Ext(filename)
	mimeType := mime.TypeByExtension(ext)
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	return mimeType
}

func (s *StorageService) FileExists(filePath string) bool {
	_, err := os.Stat(filePath)
	return !os.IsNotExist(err)
}
