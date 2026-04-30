package repository

import (
	"database/sql"
	"log"
	"time"

	"github.com/google/uuid"
)

type LibraryItem struct {
	ID          uuid.UUID      `json:"id"`
	Name        string         `json:"name"`
	Description sql.NullString `json:"description,omitempty"`
	MimeType    string         `json:"mime_type"`
	Size        int64          `json:"size"`
	Path        sql.NullString `json:"-"`
	IsFolder    bool           `json:"is_folder"`
	ParentID    *uuid.UUID     `json:"parent_id,omitempty"`
	Version     int            `json:"version"`
	CreatedBy   uuid.UUID      `json:"created_by"`
	UpdatedBy   sql.NullString `json:"updated_by,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   *time.Time     `json:"deleted_at,omitempty"`
}

type LibraryAdmin struct {
	ID      uuid.UUID  `json:"id"`
	Email   string     `json:"email"`
	UserID  *uuid.UUID `json:"user_id"`
	AddedBy uuid.UUID  `json:"added_by"`
	AddedAt time.Time  `json:"added_at"`
}

type LibraryRepository struct {
	db *sql.DB
}

func NewLibraryRepository(db *sql.DB) *LibraryRepository {
	return &LibraryRepository{db: db}
}

func (r *LibraryRepository) IsAdmin(userID uuid.UUID) (bool, error) {
	var exists bool
	err := r.db.QueryRow(`
		SELECT EXISTS(
			SELECT 1 FROM library_admins la 
			JOIN users u ON u.email = la.email 
			WHERE u.id = $1
		)`, userID).Scan(&exists)
	return exists, err
}

func (r *LibraryRepository) GetItems(parentID *uuid.UUID) ([]LibraryItem, error) {
	var rows *sql.Rows
	var err error

	if parentID == nil {
		query := `
			SELECT id, name, description, mime_type, size, path, is_folder, parent_id, version,
				   created_by, updated_by, created_at, updated_at, deleted_at
			FROM library_items
			WHERE parent_id IS NULL
			AND deleted_at IS NULL
			ORDER BY is_folder DESC, name ASC
		`
		rows, err = r.db.Query(query)
	} else {
		query := `
			SELECT id, name, description, mime_type, size, path, is_folder, parent_id, version,
				   created_by, updated_by, created_at, updated_at, deleted_at
			FROM library_items
			WHERE parent_id = $1
			AND deleted_at IS NULL
			ORDER BY is_folder DESC, name ASC
		`
		rows, err = r.db.Query(query, *parentID)
	}

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []LibraryItem
	for rows.Next() {
		var it LibraryItem
		var updatedBy sql.NullString
		var description sql.NullString
		var path sql.NullString

		err := rows.Scan(
			&it.ID, &it.Name, &description,
			&it.MimeType, &it.Size, &path,
			&it.IsFolder, &it.ParentID, &it.Version,
			&it.CreatedBy, &updatedBy,
			&it.CreatedAt, &it.UpdatedAt, &it.DeletedAt,
		)
		if err != nil {
			return nil, err
		}
		it.Description = description
		it.Path = path
		it.UpdatedBy = updatedBy
		items = append(items, it)
	}
	return items, nil
}

func (r *LibraryRepository) GetItem(id uuid.UUID) (*LibraryItem, error) {
	var it LibraryItem
	var updatedBy sql.NullString
	var description sql.NullString
	var path sql.NullString

	query := `
		SELECT id, name, description, mime_type, size, path, is_folder, parent_id, version,
			   created_by, updated_by, created_at, updated_at, deleted_at
		FROM library_items
		WHERE id = $1 AND deleted_at IS NULL
	`
	err := r.db.QueryRow(query, id).Scan(
		&it.ID, &it.Name, &description,
		&it.MimeType, &it.Size, &path,
		&it.IsFolder, &it.ParentID, &it.Version,
		&it.CreatedBy, &updatedBy,
		&it.CreatedAt, &it.UpdatedAt, &it.DeletedAt,
	)
	if err != nil {
		return nil, err
	}
	it.Description = description
	it.Path = path
	it.UpdatedBy = updatedBy
	return &it, nil
}

// InitAdminsFromConfig - инициализирует администраторов из конфига
func (r *LibraryRepository) InitAdminsFromConfig(adminEmails []string, addedBy uuid.UUID) error {
	for _, email := range adminEmails {
		if email == "" {
			continue
		}

		// Проверяем, существует ли пользователь с таким email
		var userID *uuid.UUID
		err := r.db.QueryRow("SELECT id FROM users WHERE email = $1", email).Scan(&userID)
		if err != nil && err != sql.ErrNoRows {
			log.Printf("Warning: user not found for email %s", email)
		}

		// Добавляем в администраторы (или обновляем)
		_, err = r.db.Exec(`
			INSERT INTO library_admins (id, email, user_id, added_by)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (email) DO UPDATE SET
				user_id = EXCLUDED.user_id,
				added_by = EXCLUDED.added_by
		`, uuid.New(), email, userID, addedBy)

		if err != nil {
			log.Printf("Warning: failed to add admin %s: %v", email, err)
		}
	}
	return nil
}
func (r *LibraryRepository) GetFolderSize(folderID uuid.UUID) (int64, error) {
	var size int64
	query := `SELECT COALESCE(size, 0) FROM library_items WHERE id = $1 AND is_folder = true`
	err := r.db.QueryRow(query, folderID).Scan(&size)
	return size, err
}

// UpdateFolderSize - обновляет размер папки
func (r *LibraryRepository) UpdateFolderSize(folderID uuid.UUID, size int64) error {
	query := `
        UPDATE library_items 
        SET size = $1, updated_at = $2 
        WHERE id = $3 AND is_folder = true
    `
	_, err := r.db.Exec(query, size, time.Now(), folderID)
	return err
}

// GetLibraryStorageStats - получает статистику всей библиотеки (только не удаленные)
func (r *LibraryRepository) GetLibraryStorageStats() (totalSize int64, fileCount int, folderCount int, err error) {
	query := `
        SELECT 
            COALESCE(SUM(CASE WHEN is_folder = false THEN size ELSE 0 END), 0) as total_size,
            COUNT(CASE WHEN is_folder = false THEN 1 END) as file_count,
            COUNT(CASE WHEN is_folder = true THEN 1 END) as folder_count
        FROM library_items
        WHERE deleted_at IS NULL
    `

	err = r.db.QueryRow(query).Scan(&totalSize, &fileCount, &folderCount)
	if err != nil {
		log.Printf("Error getting library stats: %v", err)
	}
	return
}
func (r *LibraryRepository) CreateItem(item *LibraryItem) error {
	query := `
		INSERT INTO library_items (id, name, description, mime_type, size, path, is_folder, parent_id, version, created_by, updated_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`
	_, err := r.db.Exec(query,
		item.ID, item.Name, item.Description,
		item.MimeType, item.Size, item.Path,
		item.IsFolder, item.ParentID, item.Version,
		item.CreatedBy, item.UpdatedBy,
	)
	return err
}

func (r *LibraryRepository) UpdateItem(item *LibraryItem) error {
	query := `
		UPDATE library_items
		SET name = $1, description = $2, updated_by = $3, updated_at = $4, version = version + 1
		WHERE id = $5
	`
	_, err := r.db.Exec(query,
		item.Name, item.Description,
		item.UpdatedBy, time.Now(),
		item.ID,
	)
	return err
}

func (r *LibraryRepository) DeleteItem(id uuid.UUID) error {
	query := `UPDATE library_items SET deleted_at = $1 WHERE id = $2`
	_, err := r.db.Exec(query, time.Now(), id)
	return err
}

// ListAdmins - список администраторов
func (r *LibraryRepository) ListAdmins() ([]LibraryAdmin, error) {
	rows, err := r.db.Query(`
        SELECT id, email, user_id, added_by, added_at 
        FROM library_admins 
        ORDER BY added_at
    `)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var admins []LibraryAdmin
	for rows.Next() {
		var a LibraryAdmin
		err := rows.Scan(&a.ID, &a.Email, &a.UserID, &a.AddedBy, &a.AddedAt)
		if err != nil {
			return nil, err
		}
		admins = append(admins, a)
	}
	return admins, nil
}

// AddAdmin - добавление администратора
func (r *LibraryRepository) AddAdmin(email string, addedBy uuid.UUID) error {
	var userID *uuid.UUID
	err := r.db.QueryRow("SELECT id FROM users WHERE email = $1", email).Scan(&userID)
	if err != nil && err != sql.ErrNoRows {
		return err
	}

	query := `
        INSERT INTO library_admins (id, email, user_id, added_by)
        VALUES ($1, $2, $3, $4)
        ON CONFLICT (email) DO NOTHING
    `
	_, err = r.db.Exec(query, uuid.New(), email, userID, addedBy)
	return err
}

// RemoveAdmin - удаление администратора
func (r *LibraryRepository) RemoveAdmin(email string) error {
	_, err := r.db.Exec("DELETE FROM library_admins WHERE email = $1", email)
	return err
}
