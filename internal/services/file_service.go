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
	libraryService *LibraryService
}

func NewFileService(
	fileRepo *repository.FileRepository,
	storageService *StorageService,
	auditService *AuditService,
	userRepo *repository.UserRepository,
	libraryService *LibraryService,
) *FileService {
	return &FileService{
		fileRepo:       fileRepo,
		storageService: storageService,
		auditService:   auditService,
		userRepo:       userRepo,
		libraryService: libraryService,
	}
}

func (s *FileService) getUserDir(userID uuid.UUID) string {
	return filepath.Join(s.storageService.basePath, "users", userID.String())
}

func ValidateFile(filename string, size int64, mimeType string, maxSize int64) error {
	if size > maxSize {
		return errors.New("file size exceeds limit")
	}
	if !allowedMimeTypes[mimeType] {
		return errors.New("file type not allowed")
	}
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
	if strings.Contains(filename, "..") || strings.Contains(filename, "/") || strings.Contains(filename, "\\") {
		return errors.New("invalid filename")
	}
	return nil
}

// ==================== MOVE OPERATIONS ====================

func (s *FileService) MoveFile(userID uuid.UUID, fileID uuid.UUID, newParentID *uuid.UUID) error {
	file, err := s.fileRepo.GetByID(fileID)
	if err != nil {
		return err
	}
	if file.UserID != userID {
		return errors.New("access denied")
	}
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
	if oldParentID != nil {
		go s.updateFolderSizeRecursive(*oldParentID)
	}
	if newParentID != nil {
		go s.updateFolderSizeRecursive(*newParentID)
	}
	return nil
}

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
		if folderID == *newParentID {
			return errors.New("cannot move folder into itself")
		}
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
	if oldParentID != nil {
		go s.updateFolderSizeRecursive(*oldParentID)
	}
	if newParentID != nil {
		go s.updateFolderSizeRecursive(*newParentID)
	}
	return nil
}

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

// ==================== UPLOAD / CREATE ====================

func (s *FileService) UploadFile(userID uuid.UUID, parentFolderID *uuid.UUID,
	filename string, fileData io.Reader, size int64, ip, userAgent string) (*models.FileUploadResponse, error) {

	userDir := s.getUserDir(userID)
	var targetPath string
	if parentFolderID != nil {
		parent, err := s.fileRepo.GetByID(*parentFolderID)
		if err != nil {
			return nil, fmt.Errorf("parent folder not found: %v", err)
		}
		if !parent.IsFolder {
			return nil, errors.New("parent is not a folder")
		}
		if parent.UserID != userID {
			return nil, errors.New("access denied to parent folder")
		}
		targetPath = parent.Path
	} else {
		targetPath = userDir
	}
	if err := os.MkdirAll(targetPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %v", err)
	}
	ext := filepath.Ext(filename)
	storedName := uuid.New().String() + ext
	filePath := filepath.Join(targetPath, storedName)
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
	if parentFolderID != nil {
		go func() {
			if err := s.updateFolderSizeRecursive(*parentFolderID); err != nil {
				log.Printf("Error updating folder size: %v", err)
			}
		}()
	}
	if s.auditService != nil {
		user, _ := s.userRepo.GetByID(userID)
		if user != nil {
			go s.auditService.Log(userID, user.Email, "upload", "file", &file.ID, filename,
				fmt.Sprintf("file uploaded to %v", parentFolderID), ip, userAgent)
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
	if parentFolderID != nil {
		go s.updateFolderSizeRecursive(*parentFolderID)
	}
	if s.auditService != nil {
		user, _ := s.userRepo.GetByID(userID)
		if user != nil {
			go s.auditService.Log(userID, user.Email, "create_folder", "folder", &folder.ID, name,
				"folder created", ip, userAgent)
		}
	}
	return folder, nil
}

// ==================== GET / LIST ====================

func (s *FileService) GetUserFiles(userID uuid.UUID, parentFolderID *uuid.UUID, page, limit int) ([]models.File, int64, error) {
	files, total, err := s.fileRepo.GetByUserID(userID, parentFolderID, page, limit)
	if err != nil {
		return nil, 0, err
	}
	for i := range files {
		if files[i].IsFolder {
			if files[i].FolderSize == 0 && files[i].ID != uuid.Nil {
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

func (s *FileService) GetFileByID(fileID uuid.UUID) (*models.File, error) {
	return s.fileRepo.GetByID(fileID)
}

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

// ==================== UPDATE / RENAME ====================

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
	if parentID != nil {
		go s.updateFolderSizeRecursive(*parentID)
	}
	if s.auditService != nil {
		user, _ := s.userRepo.GetByID(userID)
		if user != nil {
			go s.auditService.Log(userID, user.Email, "delete", "file", &fileID, file.Name,
				"file deleted", ip, userAgent)
		}
	}
	return nil
}

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
	if parentID != nil {
		go s.updateFolderSizeRecursive(*parentID)
	}
	if s.auditService != nil {
		user, _ := s.userRepo.GetByID(userID)
		if user != nil {
			go s.auditService.Log(userID, user.Email, "delete_folder", "folder", &folderID, folder.Name,
				"folder deleted", ip, userAgent)
		}
	}
	return nil
}

// ==================== SIZE CALCULATION ====================

func (s *FileService) updateFolderSizeRecursive(folderID uuid.UUID) error {
	err := s.fileRepo.UpdateFolderSizeRecursive(folderID)
	if err != nil {
		log.Printf("Error updating folder size for %s: %v", folderID, err)
	}
	return err
}

// ==================== MOVE TO LIBRARY ====================

func (s *FileService) MoveToLibrary(userID uuid.UUID, itemID uuid.UUID, libraryParentID *uuid.UUID) (*repository.LibraryItem, error) {
	item, err := s.fileRepo.GetByID(itemID)
	if err != nil {
		return nil, fmt.Errorf("item not found: %v", err)
	}

	if item.UserID != userID {
		return nil, errors.New("access denied")
	}

	if err := s.libraryService.CheckAdmin(userID); err != nil {
		return nil, fmt.Errorf("admin access required: %v", err)
	}

	if item.IsFolder {
		return s.moveFolderToLibrary(userID, item, libraryParentID)
	}
	return s.moveFileToLibrary(userID, item, libraryParentID)
}

func (s *FileService) moveFileToLibrary(userID uuid.UUID, file *models.File, libraryParentID *uuid.UUID) (*repository.LibraryItem, error) {
	srcFile, err := os.Open(file.Path)
	if err != nil {
		return nil, err
	}
	defer srcFile.Close()

	item, err := s.libraryService.UploadFile(userID, libraryParentID, file.OriginalName, srcFile, file.Size, "", "")
	if err != nil {
		return nil, err
	}

	s.DeleteFile(userID, file.ID, "", "")
	return item, nil
}

func (s *FileService) moveFolderToLibrary(userID uuid.UUID, folder *models.File, libraryParentID *uuid.UUID) (*repository.LibraryItem, error) {
	libFolder, err := s.libraryService.CreateFolder(userID, folder.OriginalName, libraryParentID, "", "")
	if err != nil {
		return nil, err
	}

	children, err := s.fileRepo.GetFolderChildren(folder.ID)
	if err != nil {
		s.libraryService.DeleteItem(userID, libFolder.ID, "", "")
		return nil, err
	}

	for _, child := range children {
		if child.IsFolder {
			s.moveFolderToLibrary(userID, &child, &libFolder.ID)
		} else {
			s.moveFileToLibrary(userID, &child, &libFolder.ID)
		}
	}

	s.DeleteFolder(userID, folder.ID, "", "")
	return libFolder, nil
}
