package services

import (
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"time"

	"cloud-storage-go/internal/models"
	"cloud-storage-go/internal/repository"

	"github.com/google/uuid"
)

var allowedMimeTypes = map[string]bool{
	"image/jpeg":         true,
	"image/png":          true,
	"image/gif":          true,
	"image/webp":         true,
	"application/pdf":    true,
	"text/plain":         true,
	"application/msword": true,
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document": true,
}

type FileService struct {
	fileRepo       *repository.FileRepository
	storageService *StorageService
	auditService   *AuditService
	userRepo       *repository.UserRepository
}

func NewFileService(fileRepo *repository.FileRepository, storageService *StorageService, auditService *AuditService, userRepo *repository.UserRepository) *FileService {
	return &FileService{
		fileRepo:       fileRepo,
		storageService: storageService,
		auditService:   auditService,
		userRepo:       userRepo,
	}
}

// getUserDir - возвращает путь к папке пользователя
func (s *FileService) getUserDir(userID uuid.UUID) string {
	return filepath.Join(s.storageService.basePath, "users", userID.String())
}
func ValidateFile(filename string, size int64, mimeType string, maxSize int64) error {
	// Проверка размера
	if size > maxSize {
		return errors.New("file size exceeds limit")
	}

	// Проверка MIME типа
	if !allowedMimeTypes[mimeType] {
		return errors.New("file type not allowed")
	}

	// Проверка расширения
	ext := strings.ToLower(filepath.Ext(filename))
	allowedExts := []string{".jpg", ".jpeg", ".png", ".gif", ".webp", ".pdf", ".txt", ".doc", ".docx"}
	allowed := false
	for _, allowedExt := range allowedExts {
		if ext == allowedExt {
			allowed = true
			break
		}
	}
	if !allowed {
		return errors.New("file extension not allowed")
	}

	// Проверка на опасные имена файлов
	if strings.Contains(filename, "..") || strings.Contains(filename, "/") || strings.Contains(filename, "\\") {
		return errors.New("invalid filename")
	}

	return nil
}

// ==================== MOVE OPERATIONS ====================

// MoveFile - перемещение файла в другую папку
func (s *FileService) MoveFile(userID uuid.UUID, fileID uuid.UUID, newParentID *uuid.UUID) error {
	file, err := s.fileRepo.GetByID(fileID)
	if err != nil {
		return err
	}

	if file.UserID != userID {
		return errors.New("access denied")
	}

	// Проверяем целевую папку
	if newParentID != nil {
		parent, err := s.fileRepo.GetByID(*newParentID)
		if err != nil {
			return err
		}
		if parent.UserID != userID {
			return errors.New("access denied to target folder")
		}
		if !parent.IsFolder {
			return errors.New("target is not a folder")
		}
		// Проверяем, что не пытаемся переместить папку в саму себя
		if file.IsFolder && fileID == *newParentID {
			return errors.New("cannot move folder into itself")
		}
	}

	oldParentID := file.ParentFolderID
	file.ParentFolderID = newParentID
	file.UpdatedAt = time.Now()

	if err := s.fileRepo.Update(file); err != nil {
		return err
	}

	// Обновляем размеры папок
	if oldParentID != nil {
		go s.updateFolderSizeRecursive(*oldParentID)
	}
	if newParentID != nil {
		go s.updateFolderSizeRecursive(*newParentID)
	}

	return nil
}

// MoveFolder - перемещение папки
func (s *FileService) MoveFolder(userID uuid.UUID, folderID uuid.UUID, newParentID *uuid.UUID) error {
	folder, err := s.fileRepo.GetByID(folderID)
	if err != nil {
		return err
	}

	if folder.UserID != userID {
		return errors.New("access denied")
	}

	if !folder.IsFolder {
		return errors.New("not a folder")
	}

	// Проверяем целевую папку
	if newParentID != nil {
		parent, err := s.fileRepo.GetByID(*newParentID)
		if err != nil {
			return err
		}
		if parent.UserID != userID {
			return errors.New("access denied to target folder")
		}
		if !parent.IsFolder {
			return errors.New("target is not a folder")
		}
		// Проверяем, что не пытаемся переместить папку в саму себя или в дочернюю
		if folderID == *newParentID {
			return errors.New("cannot move folder into itself")
		}
		// Проверяем циклическую ссылку
		if s.isChildFolder(folderID, *newParentID) {
			return errors.New("cannot move folder into its own subfolder")
		}
	}

	oldParentID := folder.ParentFolderID
	folder.ParentFolderID = newParentID
	folder.UpdatedAt = time.Now()

	if err := s.fileRepo.Update(folder); err != nil {
		return err
	}

	// Обновляем размеры папок
	if oldParentID != nil {
		go s.updateFolderSizeRecursive(*oldParentID)
	}
	if newParentID != nil {
		go s.updateFolderSizeRecursive(*newParentID)
	}

	return nil
}

// isChildFolder - проверяет, является ли folderID дочерним по отношению к targetID
func (s *FileService) isChildFolder(folderID, targetID uuid.UUID) bool {
	current, err := s.fileRepo.GetByID(targetID)
	if err != nil {
		return false
	}

	for current.ParentFolderID != nil {
		if *current.ParentFolderID == folderID {
			return true
		}
		current, err = s.fileRepo.GetByID(*current.ParentFolderID)
		if err != nil {
			return false
		}
	}
	return false
}

// UploadFile - загрузка файла
func (s *FileService) UploadFile(userID uuid.UUID, parentFolderID *uuid.UUID,
	filename string, fileData io.Reader, size int64, ip, userAgent string) (*models.FileUploadResponse, error) {

	userDir := s.getUserDir(userID)

	var targetPath string
	if parentFolderID != nil {
		parent, err := s.fileRepo.GetByID(*parentFolderID)
		if err != nil {
			return nil, fmt.Errorf("parent folder not found: %v", err)
		}
		// Проверяем, что это папка
		if !parent.IsFolder {
			return nil, errors.New("parent is not a folder")
		}
		// Проверяем права доступа
		if parent.UserID != userID {
			return nil, errors.New("access denied to parent folder")
		}
		targetPath = parent.Path
	} else {
		targetPath = userDir
	}

	// Создаем директорию если её нет
	if err := os.MkdirAll(targetPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %v", err)
	}

	ext := filepath.Ext(filename)
	storedName := uuid.New().String() + ext
	filePath := filepath.Join(targetPath, storedName)

	// Создаем и сохраняем файл
	dst, err := os.Create(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create file: %v", err)
	}
	defer dst.Close()

	written, err := io.Copy(dst, fileData)
	if err != nil {
		os.Remove(filePath)
		return nil, fmt.Errorf("failed to write file: %v", err)
	}

	mimeType := mime.TypeByExtension(ext)
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	now := time.Now()
	file := &models.File{
		ID:             uuid.New(),
		UserID:         userID,
		Name:           storedName,
		OriginalName:   filename,
		Path:           filePath,
		Size:           written,
		MimeType:       mimeType,
		IsFolder:       false,
		ParentFolderID: parentFolderID,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if err := s.fileRepo.Create(file); err != nil {
		os.Remove(filePath)
		return nil, fmt.Errorf("failed to save to database: %v", err)
	}

	// Обновляем размер родительской папки
	if parentFolderID != nil {
		go func() {
			if err := s.updateFolderSizeRecursive(*parentFolderID); err != nil {
				log.Printf("Error updating folder size: %v", err)
			}
		}()
	}

	// Аудит
	if s.auditService != nil {
		user, _ := s.userRepo.GetByID(userID)
		if user != nil {
			go s.auditService.Log(userID, user.Email, "upload", "file", &file.ID, filename,
				fmt.Sprintf("Файл загружен в папку %v", parentFolderID), ip, userAgent)
		}
	}

	return &models.FileUploadResponse{
		ID:        file.ID,
		Name:      filename,
		Size:      written,
		MimeType:  mimeType,
		CreatedAt: now,
	}, nil
}

// CreateFolder - создание папки
func (s *FileService) CreateFolder(userID uuid.UUID, name string, parentFolderID *uuid.UUID, ip, userAgent string) (*models.File, error) {
	now := time.Now()

	userDir := s.getUserDir(userID)

	var folderPath string
	if parentFolderID != nil {
		parent, err := s.fileRepo.GetByID(*parentFolderID)
		if err != nil {
			return nil, err
		}
		folderPath = filepath.Join(parent.Path, name+"_"+uuid.New().String())
	} else {
		folderPath = filepath.Join(userDir, name+"_"+uuid.New().String())
	}

	if err := os.MkdirAll(folderPath, 0755); err != nil {
		return nil, err
	}

	folder := &models.File{
		ID:             uuid.New(),
		UserID:         userID,
		Name:           name,
		OriginalName:   name,
		Path:           folderPath,
		Size:           0,
		FolderSize:     0,
		MimeType:       "inode/directory",
		IsFolder:       true,
		ParentFolderID: parentFolderID,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if err := s.fileRepo.Create(folder); err != nil {
		os.RemoveAll(folderPath)
		return nil, err
	}

	// Обновляем размер родительской папки
	if parentFolderID != nil {
		go s.updateFolderSizeRecursive(*parentFolderID)
	}

	// Аудит
	if s.auditService != nil {
		user, _ := s.userRepo.GetByID(userID)
		if user != nil {
			go s.auditService.Log(userID, user.Email, "create_folder", "folder", &folder.ID, name,
				"Папка создана", ip, userAgent)
		}
	}

	return folder, nil
}

// ==================== GET / LIST ====================

// GetUserFiles - получение файлов пользователя
func (s *FileService) GetUserFiles(userID uuid.UUID, parentFolderID *uuid.UUID, page, limit int) ([]models.File, int64, error) {
	files, total, err := s.fileRepo.GetByUserID(userID, parentFolderID, page, limit)
	if err != nil {
		return nil, 0, err
	}

	// Для папок подставляем folder_size вместо size
	for i := range files {
		if files[i].IsFolder {
			if files[i].FolderSize == 0 && files[i].ID != uuid.Nil {
				// Получаем размер из базы
				size, err := s.fileRepo.GetFolderSize(files[i].ID)
				if err == nil && size > 0 {
					files[i].FolderSize = size
				}
			}
			files[i].Size = files[i].FolderSize
		}
	}

	return files, total, nil
}

// GetFileByID - получение файла по ID
func (s *FileService) GetFileByID(fileID uuid.UUID) (*models.File, error) {
	return s.fileRepo.GetByID(fileID)
}

// GetFileContent - получение содержимого файла
func (s *FileService) GetFileContent(filePath string) (*os.File, error) {
	return os.Open(filePath)
}

// GetUserStorageStats - получение статистики хранилища (ОПТИМИЗИРОВАННАЯ ВЕРСИЯ)
func (s *FileService) GetUserStorageStats(userID uuid.UUID) (map[string]interface{}, error) {
	totalSize, fileCount, folderCount, err := s.fileRepo.GetUserStorageStats(userID)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"total_size":   totalSize,
		"file_count":   fileCount,
		"folder_count": folderCount,
		"total_items":  fileCount + folderCount,
	}, nil
}

// GetFolderContentsWithSize - получение содержимого папки с размерами
func (s *FileService) GetFolderContentsWithSize(userID uuid.UUID, folderID *uuid.UUID) ([]models.File, error) {
	files, _, err := s.fileRepo.GetByUserID(userID, folderID, 1, 1000)
	if err != nil {
		return nil, err
	}

	for i := range files {
		if files[i].IsFolder {
			size, err := s.fileRepo.GetFolderSize(files[i].ID)
			if err == nil {
				files[i].Size = size
			}
		}
	}

	return files, nil
}

// ==================== UPDATE / RENAME ====================

// RenameFile - переименование файла
func (s *FileService) RenameFile(userID uuid.UUID, fileID uuid.UUID, newName string) error {
	file, err := s.fileRepo.GetByID(fileID)
	if err != nil {
		return err
	}

	if file.UserID != userID {
		return errors.New("access denied")
	}

	file.Name = newName
	file.OriginalName = newName
	file.UpdatedAt = time.Now()

	return s.fileRepo.Update(file)
}

// RenameFolder - переименование папки
func (s *FileService) RenameFolder(userID uuid.UUID, folderID uuid.UUID, newName string) error {
	folder, err := s.fileRepo.GetByID(folderID)
	if err != nil {
		return err
	}

	if folder.UserID != userID {
		return errors.New("access denied")
	}

	if !folder.IsFolder {
		return errors.New("not a folder")
	}

	folder.Name = newName
	folder.OriginalName = newName
	folder.UpdatedAt = time.Now()

	return s.fileRepo.Update(folder)
}

// ==================== DELETE ====================

// DeleteFile - удаление файла
func (s *FileService) DeleteFile(userID uuid.UUID, fileID uuid.UUID, ip, userAgent string) error {
	file, err := s.fileRepo.GetByID(fileID)
	if err != nil {
		return err
	}

	if file.UserID != userID {
		return errors.New("access denied")
	}

	parentID := file.ParentFolderID

	if !file.IsFolder && file.Path != "" {
		if err := s.storageService.DeleteFile(file.Path); err != nil && !os.IsNotExist(err) {
			return err
		}
	}

	if err := s.fileRepo.Delete(fileID); err != nil {
		return err
	}

	// Обновляем размер родительской папки
	if parentID != nil {
		go s.updateFolderSizeRecursive(*parentID)
	}

	// Аудит
	if s.auditService != nil {
		user, _ := s.userRepo.GetByID(userID)
		if user != nil {
			go s.auditService.Log(userID, user.Email, "delete", "file", &fileID, file.Name,
				"Файл удален", ip, userAgent)
		}
	}

	return nil
}

// DeleteFolder - удаление папки
func (s *FileService) DeleteFolder(userID uuid.UUID, folderID uuid.UUID, ip, userAgent string) error {
	folder, err := s.fileRepo.GetByID(folderID)
	if err != nil {
		return err
	}

	if folder.UserID != userID {
		return errors.New("access denied")
	}

	if !folder.IsFolder {
		return errors.New("not a folder")
	}

	parentID := folder.ParentFolderID

	if folder.Path != "" {
		if err := os.RemoveAll(folder.Path); err != nil && !os.IsNotExist(err) {
			return err
		}
	}

	if err := s.fileRepo.Delete(folderID); err != nil {
		return err
	}

	// Обновляем размер родительской папки
	if parentID != nil {
		go s.updateFolderSizeRecursive(*parentID)
	}

	// Аудит
	if s.auditService != nil {
		user, _ := s.userRepo.GetByID(userID)
		if user != nil {
			go s.auditService.Log(userID, user.Email, "delete_folder", "folder", &folderID, folder.Name,
				"Папка удалена", ip, userAgent)
		}
	}

	return nil
}

// ==================== SIZE CALCULATION ====================

// updateFolderSizeRecursive - обновляет размер папки и всех родителей
func (s *FileService) updateFolderSizeRecursive(folderID uuid.UUID) error {
	// Используем оптимизированный метод репозитория
	err := s.fileRepo.UpdateFolderSizeRecursive(folderID)
	if err != nil {
		log.Printf("Error updating folder size for %s: %v", folderID, err)
	}
	return err
}
