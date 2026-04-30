package services

import (
	"database/sql"
	"errors"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"

	"cloud-storage-go/internal/crypto"
	"cloud-storage-go/internal/repository"
)

type LibraryService struct {
	libRepo      *repository.LibraryRepository
	userRepo     *repository.UserRepository
	crypto       *crypto.EncryptionService
	uploadPath   string
	auditService *AuditService
}

func NewLibraryService(libRepo *repository.LibraryRepository, userRepo *repository.UserRepository,
	crypto *crypto.EncryptionService, uploadPath string, auditService *AuditService) *LibraryService {
	return &LibraryService{
		libRepo:      libRepo,
		userRepo:     userRepo,
		crypto:       crypto,
		uploadPath:   filepath.Join(uploadPath, "library"),
		auditService: auditService,
	}
}

func (s *LibraryService) CheckAdmin(userID uuid.UUID) error {
	isAdmin, err := s.libRepo.IsAdmin(userID)
	if err != nil {
		return err
	}
	if !isAdmin {
		return errors.New("admin access required")
	}
	return nil
}

func (s *LibraryService) IsAdmin(userID uuid.UUID) (bool, error) {
	return s.libRepo.IsAdmin(userID)
}

// InitAdminsFromConfig - инициализирует администраторов из конфига
func (s *LibraryService) InitAdminsFromConfig(adminEmails []string, initiatorID uuid.UUID) error {
	return s.libRepo.InitAdminsFromConfig(adminEmails, initiatorID)
}

// GetLibraryStorageStats - получает статистику всей библиотеки
func (s *LibraryService) GetLibraryStorageStats() (map[string]interface{}, error) {
	totalSize, fileCount, folderCount, err := s.libRepo.GetLibraryStorageStats()
	if err != nil {
		return nil, err
	}

	log.Printf("Library stats: totalSize=%d, files=%d, folders=%d", totalSize, fileCount, folderCount)

	return map[string]interface{}{
		"total_size":   totalSize,
		"file_count":   fileCount,
		"folder_count": folderCount,
		"total_items":  fileCount + folderCount,
	}, nil
}

// GetItems - получение элементов с подсчетом размера папок
func (s *LibraryService) GetItems(userID uuid.UUID, parentID *uuid.UUID) ([]repository.LibraryItem, error) {
	items, err := s.libRepo.GetItems(parentID)
	if err != nil {
		return nil, err
	}

	// Для каждой папки вычисляем размер
	for i := range items {
		if items[i].IsFolder {
			// Вычисляем размер папки рекурсивно
			size, err := s.calculateFolderSize(items[i].ID)
			if err == nil {
				items[i].Size = size
				// Сохраняем в базу для кэширования
				s.libRepo.UpdateFolderSize(items[i].ID, size)
			}
		}
	}

	return items, nil
}

// calculateFolderSize - рекурсивный подсчет размера папки в библиотеке
func (s *LibraryService) calculateFolderSize(folderID uuid.UUID) (int64, error) {
	// Получаем все содержимое папки
	children, err := s.libRepo.GetItems(&folderID)
	if err != nil {
		return 0, err
	}

	var totalSize int64 = 0
	for _, child := range children {
		if child.IsFolder {
			// Рекурсивно считаем размер подпапки
			size, err := s.calculateFolderSize(child.ID)
			if err != nil {
				log.Printf("Error calculating subfolder size: %v", err)
				continue
			}
			totalSize += size
		} else {
			// Добавляем размер файла
			totalSize += child.Size
		}
	}

	return totalSize, nil
}

// UpdateFolderSize - обновляет размер папки и всех родителей
func (s *LibraryService) UpdateFolderSize(folderID uuid.UUID) error {
	size, err := s.calculateFolderSize(folderID)
	if err != nil {
		return err
	}

	if err := s.libRepo.UpdateFolderSize(folderID, size); err != nil {
		return err
	}

	// Обновляем родительскую папку
	item, err := s.libRepo.GetItem(folderID)
	if err != nil {
		return err
	}

	if item.ParentID != nil {
		return s.UpdateFolderSize(*item.ParentID)
	}

	return nil
}

func (s *LibraryService) CreateFolder(userID uuid.UUID, name string, parentID *uuid.UUID, ip, userAgent string) (*repository.LibraryItem, error) {
	if err := s.CheckAdmin(userID); err != nil {
		return nil, err
	}

	var folderPath string
	if parentID != nil {
		parent, err := s.libRepo.GetItem(*parentID)
		if err != nil {
			return nil, err
		}
		if !parent.IsFolder {
			return nil, errors.New("parent is not a folder")
		}
		folderPath = filepath.Join(parent.Path.String, name+"_"+uuid.New().String())
	} else {
		folderPath = filepath.Join(s.uploadPath, name+"_"+uuid.New().String())
	}

	if err := os.MkdirAll(folderPath, 0755); err != nil {
		return nil, err
	}

	item := &repository.LibraryItem{
		ID:        uuid.New(),
		Name:      name,
		IsFolder:  true,
		ParentID:  parentID,
		Path:      sql.NullString{String: folderPath, Valid: true},
		Version:   1,
		CreatedBy: userID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err := s.libRepo.CreateItem(item)
	if err != nil {
		os.RemoveAll(folderPath)
		return nil, err
	}

	// Обновляем размер родительской папки
	if parentID != nil {
		go func() {
			if err := s.UpdateFolderSize(*parentID); err != nil {
				log.Printf("Error updating parent folder size: %v", err)
			}
		}()
	}

	if s.auditService != nil {
		user, _ := s.userRepo.GetByID(userID)
		if user != nil {
			go s.auditService.Log(userID, user.Email, "library_create_folder", "library_item", &item.ID, name,
				"В библиотеке создана папка", ip, userAgent)
		}
	}

	return item, nil
}

func (s *LibraryService) UploadFile(userID uuid.UUID, parentID *uuid.UUID, filename string,
	data io.Reader, size int64, ip, userAgent string) (*repository.LibraryItem, error) {

	if err := s.CheckAdmin(userID); err != nil {
		return nil, err
	}

	var targetPath string
	if parentID != nil {
		parent, err := s.libRepo.GetItem(*parentID)
		if err != nil {
			return nil, err
		}
		if !parent.IsFolder {
			return nil, errors.New("parent is not a folder")
		}
		targetPath = parent.Path.String
	} else {
		targetPath = s.uploadPath
	}

	if err := os.MkdirAll(targetPath, 0755); err != nil {
		return nil, err
	}

	tmpFile, err := os.CreateTemp("", "lib_upload_*")
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmpFile.Name())

	written, err := io.Copy(tmpFile, data)
	tmpFile.Close()
	if err != nil {
		return nil, err
	}

	encryptedPath := filepath.Join(targetPath, uuid.New().String()+".enc")
	if err := s.crypto.EncryptFile(tmpFile.Name(), encryptedPath); err != nil {
		return nil, err
	}

	item := &repository.LibraryItem{
		ID:        uuid.New(),
		Name:      filename,
		MimeType:  s.getMimeType(filename),
		Size:      written,
		Path:      sql.NullString{String: encryptedPath, Valid: true},
		IsFolder:  false,
		ParentID:  parentID,
		Version:   1,
		CreatedBy: userID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := s.libRepo.CreateItem(item); err != nil {
		os.Remove(encryptedPath)
		return nil, err
	}

	// Обновляем размер родительской папки
	if parentID != nil {
		go func() {
			if err := s.UpdateFolderSize(*parentID); err != nil {
				log.Printf("Error updating parent folder size: %v", err)
			}
		}()
	}

	if s.auditService != nil {
		user, _ := s.userRepo.GetByID(userID)
		if user != nil {
			go s.auditService.Log(userID, user.Email, "library_upload", "library_item", &item.ID, filename,
				"В библиотеку загружен файл", ip, userAgent)
		}
	}

	return item, nil
}

func (s *LibraryService) DownloadFile(userID uuid.UUID, itemID uuid.UUID, _ string) ([]byte, string, error) {
	item, err := s.libRepo.GetItem(itemID)
	if err != nil {
		return nil, "", err
	}
	if item.IsFolder {
		return nil, "", errors.New("cannot download folder")
	}
	if !item.Path.Valid || item.Path.String == "" {
		return nil, "", errors.New("file has no stored path")
	}

	tmpFile, err := os.CreateTemp("", "lib_download_*")
	if err != nil {
		return nil, "", err
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	if err := s.crypto.DecryptFile(item.Path.String, tmpPath); err != nil {
		return nil, "", err
	}

	data, err := os.ReadFile(tmpPath)
	if err != nil {
		return nil, "", err
	}
	return data, item.Name, nil
}

func (s *LibraryService) UpdateItem(userID uuid.UUID, itemID uuid.UUID, name, description string) error {
	if err := s.CheckAdmin(userID); err != nil {
		return err
	}
	it, err := s.libRepo.GetItem(itemID)
	if err != nil {
		return err
	}
	it.Name = name
	it.Description = sql.NullString{String: description, Valid: description != ""}
	updatedByStr := userID.String()
	it.UpdatedBy = sql.NullString{String: updatedByStr, Valid: true}
	return s.libRepo.UpdateItem(it)
}

func (s *LibraryService) DeleteItem(userID uuid.UUID, itemID uuid.UUID, ip, userAgent string) error {
	if err := s.CheckAdmin(userID); err != nil {
		return err
	}
	it, err := s.libRepo.GetItem(itemID)
	if err != nil {
		return err
	}

	itemName := it.Name
	parentID := it.ParentID

	if !it.IsFolder && it.Path.Valid && it.Path.String != "" {
		os.Remove(it.Path.String)
	} else if it.IsFolder && it.Path.Valid && it.Path.String != "" {
		os.RemoveAll(it.Path.String)
	}

	if err := s.libRepo.DeleteItem(itemID); err != nil {
		return err
	}

	// Обновляем размер родительской папки
	if parentID != nil {
		go func() {
			if err := s.UpdateFolderSize(*parentID); err != nil {
				log.Printf("Error updating parent folder size after delete: %v", err)
			}
		}()
	}

	if s.auditService != nil {
		user, _ := s.userRepo.GetByID(userID)
		if user != nil {
			go s.auditService.Log(userID, user.Email, "library_delete", "library_item", &itemID, itemName,
				"Удален из библиотеки", ip, userAgent)
		}
	}

	return nil
}

func (s *LibraryService) ListAdmins() ([]repository.LibraryAdmin, error) {
	return s.libRepo.ListAdmins()
}

func (s *LibraryService) AddAdmin(email string, addedBy uuid.UUID) error {
	isAdmin, err := s.libRepo.IsAdmin(addedBy)
	if err != nil || !isAdmin {
		return errors.New("admin access required")
	}
	return s.libRepo.AddAdmin(email, addedBy)
}

func (s *LibraryService) RemoveAdmin(email string, removedBy uuid.UUID) error {
	isAdmin, err := s.libRepo.IsAdmin(removedBy)
	if err != nil || !isAdmin {
		return errors.New("admin access required")
	}
	return s.libRepo.RemoveAdmin(email)
}

func (s *LibraryService) getMimeType(filename string) string {
	ext := filepath.Ext(filename)
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp":
		return "image/" + ext[1:]
	case ".pdf":
		return "application/pdf"
	case ".txt", ".md":
		return "text/plain"
	case ".doc", ".docx":
		return "application/msword"
	default:
		return "application/octet-stream"
	}
}
