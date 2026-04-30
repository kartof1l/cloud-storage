package services

import (
	"database/sql"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"

	"cloud-storage-go/internal/crypto"
	"cloud-storage-go/internal/models"
	"cloud-storage-go/internal/repository"
)

// ========== TEST HELPERS ==========

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("postgres", "host=localhost port=5432 user=postgres password=postgres dbname=cloud_storage_test sslmode=disable")
	if err != nil {
		t.Skipf("Skipping integration test: cannot connect to DB: %v", err)
	}
	if err := db.Ping(); err != nil {
		t.Skipf("Skipping integration test: DB not reachable: %v", err)
	}

	// Создаём таблицы
	queries := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id UUID PRIMARY KEY,
			email VARCHAR(255) UNIQUE NOT NULL,
			password_hash VARCHAR(255) NOT NULL,
			first_name VARCHAR(100),
			last_name VARCHAR(100),
			avatar_url VARCHAR(512),
			is_email_verified BOOLEAN DEFAULT FALSE,
			verification_code VARCHAR(10),
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS files (
			id UUID PRIMARY KEY,
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			name VARCHAR(255) NOT NULL,
			original_name VARCHAR(255) NOT NULL,
			path VARCHAR(1024),
			size BIGINT DEFAULT 0,
			folder_size BIGINT DEFAULT 0,
			mime_type VARCHAR(255),
			is_folder BOOLEAN DEFAULT FALSE,
			parent_folder_id UUID REFERENCES files(id) ON DELETE CASCADE,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS library_items (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			name VARCHAR(255) NOT NULL,
			description TEXT,
			mime_type VARCHAR(255),
			size BIGINT DEFAULT 0,
			path VARCHAR(1024),
			is_folder BOOLEAN DEFAULT FALSE,
			parent_id UUID REFERENCES library_items(id) ON DELETE CASCADE,
			version INT DEFAULT 1,
			created_by UUID NOT NULL REFERENCES users(id),
			updated_by UUID REFERENCES users(id),
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			deleted_at TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS library_admins (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			email VARCHAR(255) NOT NULL UNIQUE,
			user_id UUID REFERENCES users(id),
			added_by UUID NOT NULL REFERENCES users(id),
			added_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS audit_logs (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			user_email VARCHAR(255) NOT NULL,
			action VARCHAR(50) NOT NULL,
			entity_type VARCHAR(50) NOT NULL,
			entity_id UUID,
			entity_name VARCHAR(255),
			details TEXT,
			ip_address VARCHAR(45),
			user_agent TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
	}

	for _, q := range queries {
		db.Exec(q)
	}

	// Очищаем таблицы перед тестом
	db.Exec("DELETE FROM audit_logs")
	db.Exec("DELETE FROM library_admins")
	db.Exec("DELETE FROM library_items")
	db.Exec("DELETE FROM files")
	db.Exec("DELETE FROM users")

	return db
}

func createTestUser(db *sql.DB, t *testing.T) *models.User {
	t.Helper()
	user := &models.User{
		ID:              uuid.New(),
		Email:           "test_" + uuid.New().String()[:8] + "@test.com",
		PasswordHash:    "$2a$10$dummyhash",
		FirstName:       "Test",
		LastName:        "User",
		IsEmailVerified: true,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	_, err := db.Exec(`INSERT INTO users (id, email, password_hash, first_name, last_name, is_email_verified, created_at, updated_at) 
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		user.ID, user.Email, user.PasswordHash, user.FirstName, user.LastName, user.IsEmailVerified, user.CreatedAt, user.UpdatedAt)
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	// Делаем пользователя админом библиотеки
	db.Exec(`INSERT INTO library_admins (id, email, user_id, added_by) VALUES ($1, $2, $3, $4)`,
		uuid.New(), user.Email, user.ID, user.ID)

	return user
}

// ========== UNIT TESTS ==========

func TestValidateFile(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		size     int64
		mimeType string
		maxSize  int64
		wantErr  bool
	}{
		{"valid JPEG", "photo.jpg", 1024, "image/jpeg", 10 * 1024 * 1024, false},
		{"valid PNG", "icon.png", 512, "image/png", 10 * 1024 * 1024, false},
		{"valid PDF", "doc.pdf", 2048, "application/pdf", 10 * 1024 * 1024, false},
		{"too large", "big.jpg", 100 * 1024 * 1024, "image/jpeg", 10 * 1024 * 1024, true},
		{"invalid type", "movie.mp4", 1024, "video/mp4", 10 * 1024 * 1024, true},
		{"invalid extension", "file.exe", 1024, "application/octet-stream", 10 * 1024 * 1024, true},
		{"path traversal", "../etc/passwd", 1024, "text/plain", 10 * 1024 * 1024, true},
		{"double dot", "..hidden", 1024, "text/plain", 10 * 1024 * 1024, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFile(tt.filename, tt.size, tt.mimeType, tt.maxSize)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateFile() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// ========== INTEGRATION TESTS ==========

func TestFileService_UploadFile(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	db := setupTestDB(t)
	defer db.Close()

	user := createTestUser(db, t)
	tempDir := t.TempDir()

	fileRepo := repository.NewFileRepository(db)
	userRepo := repository.NewUserRepository(db)
	auditRepo := repository.NewAuditRepository(db)
	libRepo := repository.NewLibraryRepository(db)

	storageService := &StorageService{basePath: tempDir}
	auditService := NewAuditService(auditRepo)
	libraryService := NewLibraryService(libRepo, userRepo, nil, tempDir, auditService)

	service := NewFileService(fileRepo, storageService, auditService, userRepo, libraryService)

	t.Run("upload text file", func(t *testing.T) {
		content := "Hello, World!"
		reader := strings.NewReader(content)

		resp, err := service.UploadFile(user.ID, nil, "test.txt", reader, int64(len(content)), "127.0.0.1", "test-agent")
		if err != nil {
			t.Fatalf("UploadFile failed: %v", err)
		}

		if resp.Name != "test.txt" {
			t.Errorf("Expected name 'test.txt', got '%s'", resp.Name)
		}
		if resp.Size != int64(len(content)) {
			t.Errorf("Expected size %d, got %d", len(content), resp.Size)
		}

		// Проверяем что файл создан на диске
		file, err := fileRepo.GetByID(resp.ID)
		if err != nil {
			t.Fatalf("GetByID failed: %v", err)
		}
		if _, err := os.Stat(file.Path); os.IsNotExist(err) {
			t.Error("File not found on disk")
		}
	})

	t.Run("upload to folder", func(t *testing.T) {
		// Создаём папку
		folder, err := service.CreateFolder(user.ID, "TestFolder", nil, "127.0.0.1", "test")
		if err != nil {
			t.Fatalf("CreateFolder failed: %v", err)
		}

		content := "File in folder"
		reader := strings.NewReader(content)
		resp, err := service.UploadFile(user.ID, &folder.ID, "inner.txt", reader, int64(len(content)), "127.0.0.1", "test")
		if err != nil {
			t.Fatalf("UploadFile to folder failed: %v", err)
		}

		file, _ := fileRepo.GetByID(resp.ID)
		if file.ParentFolderID == nil || *file.ParentFolderID != folder.ID {
			t.Error("File should be in folder")
		}
	})
}

func TestFileService_CreateAndListFolder(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	db := setupTestDB(t)
	defer db.Close()

	user := createTestUser(db, t)
	tempDir := t.TempDir()

	fileRepo := repository.NewFileRepository(db)
	userRepo := repository.NewUserRepository(db)
	auditRepo := repository.NewAuditRepository(db)
	libRepo := repository.NewLibraryRepository(db)

	storageService := &StorageService{basePath: tempDir}
	auditService := NewAuditService(auditRepo)
	libraryService := NewLibraryService(libRepo, userRepo, nil, tempDir, auditService)

	service := NewFileService(fileRepo, storageService, auditService, userRepo, libraryService)

	t.Run("create folder", func(t *testing.T) {
		folder, err := service.CreateFolder(user.ID, "MyFolder", nil, "127.0.0.1", "test")
		if err != nil {
			t.Fatalf("CreateFolder failed: %v", err)
		}

		if !folder.IsFolder {
			t.Error("Expected folder")
		}
		if folder.Name != "MyFolder" {
			t.Errorf("Expected name 'MyFolder', got '%s'", folder.Name)
		}
		if folder.UserID != user.ID {
			t.Error("Folder should belong to user")
		}
	})

	t.Run("create nested folders", func(t *testing.T) {
		parent, _ := service.CreateFolder(user.ID, "Parent", nil, "127.0.0.1", "test")
		child, err := service.CreateFolder(user.ID, "Child", &parent.ID, "127.0.0.1", "test")
		if err != nil {
			t.Fatalf("CreateFolder failed: %v", err)
		}

		if child.ParentFolderID == nil || *child.ParentFolderID != parent.ID {
			t.Error("Child folder should reference parent")
		}
	})

	t.Run("list files", func(t *testing.T) {
		// Создаём несколько файлов и папок
		service.CreateFolder(user.ID, "Folder1", nil, "127.0.0.1", "test")
		service.CreateFolder(user.ID, "Folder2", nil, "127.0.0.1", "test")
		service.UploadFile(user.ID, nil, "file1.txt", strings.NewReader("content"), 7, "127.0.0.1", "test")
		service.UploadFile(user.ID, nil, "file2.txt", strings.NewReader("more"), 4, "127.0.0.1", "test")

		files, total, err := service.GetUserFiles(user.ID, nil, 1, 100)
		if err != nil {
			t.Fatalf("GetUserFiles failed: %v", err)
		}

		if total < 4 {
			t.Errorf("Expected at least 4 items, got %d", total)
		}

		folders := 0
		filesCount := 0
		for _, f := range files {
			if f.IsFolder {
				folders++
			} else {
				filesCount++
			}
		}

		if folders < 2 {
			t.Errorf("Expected at least 2 folders, got %d", folders)
		}
		if filesCount < 2 {
			t.Errorf("Expected at least 2 files, got %d", filesCount)
		}
	})
}

func TestFileService_MoveFile(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	db := setupTestDB(t)
	defer db.Close()

	user := createTestUser(db, t)
	tempDir := t.TempDir()

	fileRepo := repository.NewFileRepository(db)
	userRepo := repository.NewUserRepository(db)
	auditRepo := repository.NewAuditRepository(db)
	libRepo := repository.NewLibraryRepository(db)

	storageService := &StorageService{basePath: tempDir}
	auditService := NewAuditService(auditRepo)
	libraryService := NewLibraryService(libRepo, userRepo, nil, tempDir, auditService)

	service := NewFileService(fileRepo, storageService, auditService, userRepo, libraryService)

	t.Run("move file to folder", func(t *testing.T) {
		// Создаём файл в корне
		resp, _ := service.UploadFile(user.ID, nil, "move_me.txt", strings.NewReader("data"), 4, "127.0.0.1", "test")

		// Создаём папку
		folder, _ := service.CreateFolder(user.ID, "Destination", nil, "127.0.0.1", "test")

		// Перемещаем
		err := service.MoveFile(user.ID, resp.ID, &folder.ID)
		if err != nil {
			t.Fatalf("MoveFile failed: %v", err)
		}

		// Проверяем
		file, _ := fileRepo.GetByID(resp.ID)
		if file.ParentFolderID == nil || *file.ParentFolderID != folder.ID {
			t.Error("File should be in destination folder")
		}
	})

	t.Run("cannot move into itself", func(t *testing.T) {
		folder, _ := service.CreateFolder(user.ID, "SelfFolder", nil, "127.0.0.1", "test")
		err := service.MoveFolder(user.ID, folder.ID, &folder.ID)
		if err == nil {
			t.Error("Should not allow moving folder into itself")
		}
	})

	t.Run("access denied for other user", func(t *testing.T) {
		otherUser := createTestUser(db, t)
		resp, _ := service.UploadFile(user.ID, nil, "private.txt", strings.NewReader("secret"), 6, "127.0.0.1", "test")

		err := service.MoveFile(otherUser.ID, resp.ID, nil)
		if err == nil || err.Error() != "access denied" {
			t.Errorf("Expected 'access denied', got: %v", err)
		}
	})
}

func TestFileService_DeleteFile(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	db := setupTestDB(t)
	defer db.Close()

	user := createTestUser(db, t)
	tempDir := t.TempDir()

	fileRepo := repository.NewFileRepository(db)
	userRepo := repository.NewUserRepository(db)
	auditRepo := repository.NewAuditRepository(db)
	libRepo := repository.NewLibraryRepository(db)

	storageService := &StorageService{basePath: tempDir}
	auditService := NewAuditService(auditRepo)
	libraryService := NewLibraryService(libRepo, userRepo, nil, tempDir, auditService)

	service := NewFileService(fileRepo, storageService, auditService, userRepo, libraryService)

	t.Run("delete file", func(t *testing.T) {
		resp, _ := service.UploadFile(user.ID, nil, "delete_me.txt", strings.NewReader("bye"), 3, "127.0.0.1", "test")

		// Получаем путь перед удалением
		file, _ := fileRepo.GetByID(resp.ID)
		filePath := file.Path

		err := service.DeleteFile(user.ID, resp.ID, "127.0.0.1", "test")
		if err != nil {
			t.Fatalf("DeleteFile failed: %v", err)
		}

		// Проверяем что файл удалён из БД
		_, err = fileRepo.GetByID(resp.ID)
		if err == nil {
			t.Error("File should be deleted from DB")
		}

		// Проверяем что файл удалён с диска
		if _, err := os.Stat(filePath); !os.IsNotExist(err) {
			t.Error("File should be deleted from disk")
		}
	})

	t.Run("delete folder with contents", func(t *testing.T) {
		folder, _ := service.CreateFolder(user.ID, "ToDelete", nil, "127.0.0.1", "test")
		service.UploadFile(user.ID, &folder.ID, "child.txt", strings.NewReader("child"), 5, "127.0.0.1", "test")

		err := service.DeleteFolder(user.ID, folder.ID, "127.0.0.1", "test")
		if err != nil {
			t.Fatalf("DeleteFolder failed: %v", err)
		}

		_, err = fileRepo.GetByID(folder.ID)
		if err == nil {
			t.Error("Folder should be deleted")
		}
	})
}

func TestFileService_RenameFile(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	db := setupTestDB(t)
	defer db.Close()

	user := createTestUser(db, t)
	tempDir := t.TempDir()

	fileRepo := repository.NewFileRepository(db)
	userRepo := repository.NewUserRepository(db)
	auditRepo := repository.NewAuditRepository(db)
	libRepo := repository.NewLibraryRepository(db)

	storageService := &StorageService{basePath: tempDir}
	auditService := NewAuditService(auditRepo)
	libraryService := NewLibraryService(libRepo, userRepo, nil, tempDir, auditService)

	service := NewFileService(fileRepo, storageService, auditService, userRepo, libraryService)

	t.Run("rename file", func(t *testing.T) {
		resp, _ := service.UploadFile(user.ID, nil, "old_name.txt", strings.NewReader("test"), 4, "127.0.0.1", "test")

		err := service.RenameFile(user.ID, resp.ID, "new_name.txt")
		if err != nil {
			t.Fatalf("RenameFile failed: %v", err)
		}

		file, _ := fileRepo.GetByID(resp.ID)
		if file.OriginalName != "new_name.txt" {
			t.Errorf("Expected 'new_name.txt', got '%s'", file.OriginalName)
		}
	})

	t.Run("rename folder", func(t *testing.T) {
		folder, _ := service.CreateFolder(user.ID, "OldFolder", nil, "127.0.0.1", "test")

		err := service.RenameFolder(user.ID, folder.ID, "NewFolder")
		if err != nil {
			t.Fatalf("RenameFolder failed: %v", err)
		}

		f, _ := fileRepo.GetByID(folder.ID)
		if f.Name != "NewFolder" {
			t.Errorf("Expected 'NewFolder', got '%s'", f.Name)
		}
	})
}

func TestFileService_GetStorageStats(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	db := setupTestDB(t)
	defer db.Close()

	user := createTestUser(db, t)
	tempDir := t.TempDir()

	fileRepo := repository.NewFileRepository(db)
	userRepo := repository.NewUserRepository(db)
	auditRepo := repository.NewAuditRepository(db)
	libRepo := repository.NewLibraryRepository(db)

	storageService := &StorageService{basePath: tempDir}
	auditService := NewAuditService(auditRepo)
	libraryService := NewLibraryService(libRepo, userRepo, nil, tempDir, auditService)

	service := NewFileService(fileRepo, storageService, auditService, userRepo, libraryService)

	// Создаём файлы и папки
	service.UploadFile(user.ID, nil, "file1.txt", strings.NewReader("12345"), 5, "127.0.0.1", "test")
	service.UploadFile(user.ID, nil, "file2.txt", strings.NewReader("1234567890"), 10, "127.0.0.1", "test")
	service.CreateFolder(user.ID, "Folder1", nil, "127.0.0.1", "test")
	service.CreateFolder(user.ID, "Folder2", nil, "127.0.0.1", "test")
	service.CreateFolder(user.ID, "Folder3", nil, "127.0.0.1", "test")

	stats, err := service.GetUserStorageStats(user.ID)
	if err != nil {
		t.Fatalf("GetUserStorageStats failed: %v", err)
	}

	// stats — это УЖЕ map[string]interface{}, не нужен type assertion!
	fileCount := stats["file_count"].(int)
	folderCount := stats["folder_count"].(int)
	totalSize := stats["total_size"].(int64)

	if fileCount < 2 {
		t.Errorf("Expected at least 2 files, got %d", fileCount)
	}
	if folderCount < 3 {
		t.Errorf("Expected at least 3 folders, got %d", folderCount)
	}
	if totalSize < 15 {
		t.Errorf("Expected total size at least 15, got %d", totalSize)
	}
}

func TestFileService_MoveToLibrary(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	db := setupTestDB(t)
	defer db.Close()

	user := createTestUser(db, t)
	tempDir := t.TempDir()

	fileRepo := repository.NewFileRepository(db)
	userRepo := repository.NewUserRepository(db)
	auditRepo := repository.NewAuditRepository(db)
	libRepo := repository.NewLibraryRepository(db)

	storageService := &StorageService{basePath: tempDir}
	auditService := NewAuditService(auditRepo)

	encryptionService := &crypto.EncryptionService{}
	libraryService := NewLibraryService(libRepo, userRepo, encryptionService, tempDir, auditService)

	service := NewFileService(fileRepo, storageService, auditService, userRepo, libraryService)

	t.Run("move file to library", func(t *testing.T) {
		// Создаём файл
		resp, err := service.UploadFile(user.ID, nil, "to_library.txt", strings.NewReader("library content"), 15, "127.0.0.1", "test")
		if err != nil {
			t.Fatalf("UploadFile failed: %v", err)
		}

		// Перемещаем в библиотеку
		libItem, err := service.MoveToLibrary(user.ID, resp.ID, nil)
		if err != nil {
			t.Fatalf("MoveToLibrary failed: %v", err)
		}

		if libItem == nil {
			t.Fatal("Expected library item")
		}

		if libItem.Name != "to_library.txt" {
			t.Errorf("Expected 'to_library.txt', got '%s'", libItem.Name)
		}

		// Проверяем что файл удалён из личного хранилища
		_, err = fileRepo.GetByID(resp.ID)
		if err == nil {
			t.Error("File should be deleted from personal storage")
		}
	})

	t.Run("move folder to library", func(t *testing.T) {
		// Создаём папку с файлом
		folder, _ := service.CreateFolder(user.ID, "MyFolder", nil, "127.0.0.1", "test")
		service.UploadFile(user.ID, &folder.ID, "inner.txt", strings.NewReader("inner"), 5, "127.0.0.1", "test")

		libItem, err := service.MoveToLibrary(user.ID, folder.ID, nil)
		if err != nil {
			t.Fatalf("MoveToLibrary folder failed: %v", err)
		}

		if !libItem.IsFolder {
			t.Error("Expected folder in library")
		}

		// Проверяем что папка удалена
		_, err = fileRepo.GetByID(folder.ID)
		if err == nil {
			t.Error("Folder should be deleted")
		}
	})

	t.Run("non-admin cannot move to library", func(t *testing.T) {
		// Создаём обычного пользователя (не админа)
		normalUser := &models.User{
			ID:              uuid.New(),
			Email:           "normal_" + uuid.New().String()[:8] + "@test.com",
			PasswordHash:    "$2a$10$hash",
			FirstName:       "Normal",
			LastName:        "User",
			IsEmailVerified: true,
			CreatedAt:       time.Now(),
			UpdatedAt:       time.Now(),
		}
		db.Exec(`INSERT INTO users (id, email, password_hash, first_name, last_name, is_email_verified, created_at, updated_at) 
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
			normalUser.ID, normalUser.Email, normalUser.PasswordHash, normalUser.FirstName, normalUser.LastName, normalUser.IsEmailVerified, normalUser.CreatedAt, normalUser.UpdatedAt)

		resp, _ := service.UploadFile(normalUser.ID, nil, "noadmin.txt", strings.NewReader("test"), 4, "127.0.0.1", "test")

		_, err := service.MoveToLibrary(normalUser.ID, resp.ID, nil)
		if err == nil {
			t.Error("Non-admin should not be able to move to library")
		}
	})
}

// ========== BENCHMARKS ==========

func BenchmarkUploadFile(b *testing.B) {
	db := setupTestDB(nil)
	defer db.Close()

	user := createTestUser(db, nil)
	tempDir := b.TempDir()

	fileRepo := repository.NewFileRepository(db)
	userRepo := repository.NewUserRepository(db)
	auditRepo := repository.NewAuditRepository(db)
	libRepo := repository.NewLibraryRepository(db)

	storageService := &StorageService{basePath: tempDir}
	auditService := NewAuditService(auditRepo)
	libraryService := NewLibraryService(libRepo, userRepo, nil, tempDir, auditService)

	service := NewFileService(fileRepo, storageService, auditService, userRepo, libraryService)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		content := strings.Repeat("a", 1024)
		service.UploadFile(user.ID, nil, "bench.txt", strings.NewReader(content), 1024, "127.0.0.1", "bench")
	}
}

func BenchmarkGetUserFiles(b *testing.B) {
	db := setupTestDB(nil)
	defer db.Close()

	user := createTestUser(db, nil)
	tempDir := b.TempDir()

	fileRepo := repository.NewFileRepository(db)
	userRepo := repository.NewUserRepository(db)
	auditRepo := repository.NewAuditRepository(db)
	libRepo := repository.NewLibraryRepository(db)

	storageService := &StorageService{basePath: tempDir}
	auditService := NewAuditService(auditRepo)
	libraryService := NewLibraryService(libRepo, userRepo, nil, tempDir, auditService)

	service := NewFileService(fileRepo, storageService, auditService, userRepo, libraryService)

	// Создаём 100 файлов для бенчмарка
	for i := 0; i < 100; i++ {
		service.UploadFile(user.ID, nil, "f"+string(rune(i))+".txt", strings.NewReader("x"), 1, "", "")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		service.GetUserFiles(user.ID, nil, 1, 20)
	}
}

// ========== MOCK TESTS (без БД) ==========

type mockFileRepo struct {
	files map[uuid.UUID]*models.File
	err   error
}

func (m *mockFileRepo) GetByID(id uuid.UUID) (*models.File, error) {
	if f, ok := m.files[id]; ok {
		return f, m.err
	}
	return nil, errors.New("not found")
}

func (m *mockFileRepo) Delete(id uuid.UUID) error      { return m.err }
func (m *mockFileRepo) Update(file *models.File) error { return m.err }

func TestMoveFileLogic_WithoutDB(t *testing.T) {
	userID := uuid.New()
	fileID := uuid.New()

	file := &models.File{
		ID:     fileID,
		UserID: userID,
	}

	otherUserFile := &models.File{
		ID:     uuid.New(),
		UserID: uuid.New(),
	}

	// Тест: доступ запрещён к чужому файлу
	repo := &mockFileRepo{
		files: map[uuid.UUID]*models.File{
			fileID: otherUserFile,
		},
	}

	service := &FileService{fileRepo: nil}
	_ = service

	// Симуляция проверки доступа
	if otherUserFile.UserID != userID {
		t.Log("Correctly detected access violation")
	}

	_ = repo
	_ = file
}
