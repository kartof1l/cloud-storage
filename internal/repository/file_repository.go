package repository

import (
	"cloud-storage-go/internal/models"
	"database/sql"
	"errors"
	"strconv"
	"time"

	"github.com/google/uuid"
)

type FileRepository struct {
	db *sql.DB
}

func NewFileRepository(db *sql.DB) *FileRepository {
	return &FileRepository{db: db}
}

// GetUserStorageStats - получает статистику пользователя (оптимизированный запрос)
func (r *FileRepository) GetUserStorageStats(userID uuid.UUID) (totalSize int64, fileCount int, folderCount int, err error) {
	query := `
		SELECT 
			COALESCE(SUM(CASE WHEN is_folder = false THEN size ELSE 0 END), 0) as total_size,
			COUNT(CASE WHEN is_folder = false THEN 1 END) as file_count,
			COUNT(CASE WHEN is_folder = true THEN 1 END) as folder_count
		FROM files
		WHERE user_id = $1
	`
	err = r.db.QueryRow(query, userID).Scan(&totalSize, &fileCount, &folderCount)
	return
}

// GetUserFolders - получает все папки пользователя
func (r *FileRepository) GetUserFolders(userID uuid.UUID) ([]models.File, error) {
	query := `
		SELECT id, user_id, name, original_name, path, size, folder_size, mime_type, 
			   is_folder, parent_folder_id, created_at, updated_at
		FROM files
		WHERE user_id = $1 AND is_folder = true
		ORDER BY created_at ASC
	`

	rows, err := r.db.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var folders []models.File
	for rows.Next() {
		var folder models.File
		err := rows.Scan(
			&folder.ID, &folder.UserID, &folder.Name, &folder.OriginalName,
			&folder.Path, &folder.Size, &folder.FolderSize, &folder.MimeType, &folder.IsFolder,
			&folder.ParentFolderID, &folder.CreatedAt, &folder.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		folders = append(folders, folder)
	}

	return folders, nil
}
func (r *FileRepository) GetFolderByID(folderID uuid.UUID) (*models.File, error) {
	file, err := r.GetByID(folderID)
	if err != nil {
		return nil, err
	}
	if !file.IsFolder {
		return nil, errors.New("not a folder")
	}
	return file, nil
}

// GetFolderChildren - получает все дочерние элементы папки
func (r *FileRepository) GetFolderChildren(folderID uuid.UUID) ([]models.File, error) {
	query := `
		SELECT id, user_id, name, original_name, path, size, folder_size, mime_type, 
			   is_folder, parent_folder_id, created_at, updated_at
		FROM files
		WHERE parent_folder_id = $1
		ORDER BY is_folder DESC, name ASC
	`

	rows, err := r.db.Query(query, folderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []models.File
	for rows.Next() {
		var file models.File
		err := rows.Scan(
			&file.ID, &file.UserID, &file.Name, &file.OriginalName,
			&file.Path, &file.Size, &file.FolderSize, &file.MimeType, &file.IsFolder,
			&file.ParentFolderID, &file.CreatedAt, &file.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		files = append(files, file)
	}

	return files, nil
}

// UpdateFolderSizeRecursive - обновляет размер папки рекурсивно (оптимизированная версия)
func (r *FileRepository) UpdateFolderSizeRecursive(folderID uuid.UUID) error {
	// Рекурсивный CTE для подсчета размера всех файлов в папке и подпапках
	query := `
		WITH RECURSIVE folder_tree AS (
			-- Начальный уровень: прямая папка
			SELECT id, is_folder, size, parent_folder_id, 1 as level
			FROM files
			WHERE id = $1
			
			UNION ALL
			
			-- Рекурсивно добавляем все дочерние элементы
			SELECT f.id, f.is_folder, f.size, f.parent_folder_id, ft.level + 1
			FROM files f
			INNER JOIN folder_tree ft ON f.parent_folder_id = ft.id
		)
		SELECT COALESCE(SUM(CASE WHEN is_folder = false THEN size ELSE 0 END), 0) as total_size
		FROM folder_tree
		WHERE id != $1  -- Исключаем саму папку из суммы
	`

	var folderSize int64
	err := r.db.QueryRow(query, folderID).Scan(&folderSize)
	if err != nil {
		return err
	}

	// Обновляем размер папки
	_, err = r.db.Exec(`
		UPDATE files 
		SET folder_size = $1, updated_at = $2 
		WHERE id = $3
	`, folderSize, time.Now(), folderID)

	if err != nil {
		return err
	}

	// Обновляем родительскую папку (если есть)
	var parentID *uuid.UUID
	err = r.db.QueryRow("SELECT parent_folder_id FROM files WHERE id = $1", folderID).Scan(&parentID)
	if err != nil && err != sql.ErrNoRows {
		return err
	}

	if parentID != nil {
		return r.UpdateFolderSizeRecursive(*parentID)
	}

	return nil
}

// GetAllUserFiles - получает все файлы пользователя
func (r *FileRepository) GetAllUserFiles(userID uuid.UUID) ([]models.File, error) {
	query := `
		SELECT id, user_id, name, original_name, path, size, folder_size, mime_type, 
		is_folder, parent_folder_id, created_at, updated_at
		FROM files
		WHERE user_id = $1
		ORDER BY is_folder DESC, name ASC
	`

	rows, err := r.db.Query(query, userID)
	if err != nil {
		return []models.File{}, err
	}
	defer rows.Close()

	var files []models.File
	for rows.Next() {
		var file models.File
		err := rows.Scan(
			&file.ID, &file.UserID, &file.Name, &file.OriginalName,
			&file.Path, &file.Size, &file.FolderSize, &file.MimeType, &file.IsFolder,
			&file.ParentFolderID, &file.CreatedAt, &file.UpdatedAt,
		)
		if err != nil {
			return []models.File{}, err
		}
		files = append(files, file)
	}

	return files, nil
}

// UpdateFolderSize - обновляет размер папки
func (r *FileRepository) UpdateFolderSize(folderID uuid.UUID, size int64) error {
	query := `UPDATE files SET folder_size = $1, updated_at = $2 WHERE id = $3`
	_, err := r.db.Exec(query, size, time.Now(), folderID)
	return err
}

// GetFolderSize - получает размер папки
func (r *FileRepository) GetFolderSize(folderID uuid.UUID) (int64, error) {
	var size int64
	query := `SELECT COALESCE(folder_size, 0) FROM files WHERE id = $1`
	err := r.db.QueryRow(query, folderID).Scan(&size)
	return size, err
}

// Update - обновляет файл/папку
func (r *FileRepository) Update(file *models.File) error {
	query := `
		UPDATE files 
		SET name = $1, original_name = $2, path = $3, size = $4, folder_size = $5,
			mime_type = $6, is_folder = $7, parent_folder_id = $8, updated_at = $9
		WHERE id = $10
	`
	_, err := r.db.Exec(query,
		file.Name, file.OriginalName, file.Path, file.Size, file.FolderSize,
		file.MimeType, file.IsFolder, file.ParentFolderID, file.UpdatedAt,
		file.ID,
	)
	return err
}

// Create - создает файл/папку
func (r *FileRepository) Create(file *models.File) error {
	query := `
		INSERT INTO files (id, user_id, name, original_name, path, size, folder_size, 
							mime_type, is_folder, parent_folder_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`
	_, err := r.db.Exec(query, file.ID, file.UserID, file.Name, file.OriginalName,
		file.Path, file.Size, file.FolderSize, file.MimeType, file.IsFolder, file.ParentFolderID,
		file.CreatedAt, file.UpdatedAt)
	return err
}

// GetByUserID - получает файлы пользователя с пагинацией
func (r *FileRepository) GetByUserID(userID uuid.UUID, parentFolderID *uuid.UUID, page, limit int) ([]models.File, int64, error) {
	var files []models.File
	var total int64

	offset := (page - 1) * limit

	countQuery := `SELECT COUNT(*) FROM files WHERE user_id = $1`
	countArgs := []interface{}{userID}

	if parentFolderID != nil {
		countQuery += ` AND parent_folder_id = $2`
		countArgs = append(countArgs, *parentFolderID)
	} else {
		countQuery += ` AND parent_folder_id IS NULL`
	}

	err := r.db.QueryRow(countQuery, countArgs...).Scan(&total)
	if err != nil {
		return []models.File{}, 0, err
	}

	query := `
		SELECT id, user_id, name, original_name, path, size, folder_size, mime_type, 
			   is_folder, parent_folder_id, created_at, updated_at
		FROM files
		WHERE user_id = $1
	`
	queryArgs := []interface{}{userID}
	argPos := 2

	if parentFolderID != nil {
		query += ` AND parent_folder_id = $` + strconv.Itoa(argPos)
		queryArgs = append(queryArgs, *parentFolderID)
		argPos++
	} else {
		query += ` AND parent_folder_id IS NULL`
	}

	query += ` ORDER BY is_folder DESC, name ASC LIMIT $` + strconv.Itoa(argPos) + ` OFFSET $` + strconv.Itoa(argPos+1)
	queryArgs = append(queryArgs, limit, offset)

	rows, err := r.db.Query(query, queryArgs...)
	if err != nil {
		return []models.File{}, 0, err
	}
	defer rows.Close()

	for rows.Next() {
		var file models.File
		err := rows.Scan(
			&file.ID, &file.UserID, &file.Name, &file.OriginalName,
			&file.Path, &file.Size, &file.FolderSize, &file.MimeType, &file.IsFolder,
			&file.ParentFolderID, &file.CreatedAt, &file.UpdatedAt,
		)
		if err != nil {
			return []models.File{}, 0, err
		}
		files = append(files, file)
	}

	if files == nil {
		files = []models.File{}
	}

	return files, total, nil
}

// GetByID - получает файл/папку по ID
func (r *FileRepository) GetByID(id uuid.UUID) (*models.File, error) {
	file := &models.File{}
	query := `
		SELECT id, user_id, name, original_name, path, size, folder_size, mime_type,
			   is_folder, parent_folder_id, created_at, updated_at
		FROM files WHERE id = $1
	`

	err := r.db.QueryRow(query, id).Scan(
		&file.ID, &file.UserID, &file.Name, &file.OriginalName,
		&file.Path, &file.Size, &file.FolderSize, &file.MimeType, &file.IsFolder,
		&file.ParentFolderID, &file.CreatedAt, &file.UpdatedAt,
	)

	return file, err
}

// Delete - удаляет файл/папку
func (r *FileRepository) Delete(id uuid.UUID) error {
	query := `DELETE FROM files WHERE id = $1`
	_, err := r.db.Exec(query, id)
	return err
}
