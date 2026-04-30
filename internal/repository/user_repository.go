package repository

import (
	"cloud-storage-go/internal/models"
	"database/sql"
	"errors"
	"log"
	"time"

	"github.com/google/uuid"
)

type UserRepository struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

// Create - создание нового пользователя с полями верификации
func (r *UserRepository) Create(user *models.User) error {
	query := `
        INSERT INTO users (
            id, email, password_hash, first_name, last_name, 
            is_email_verified, verification_code, created_at, updated_at
        ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
    `
	_, err := r.db.Exec(query,
		user.ID, user.Email, user.PasswordHash,
		user.FirstName, user.LastName,
		user.IsEmailVerified, user.VerificationCode,
		user.CreatedAt, user.UpdatedAt,
	)
	return err
}

// GetByEmail - получение пользователя по email (с новыми полями)
func (r *UserRepository) GetByEmail(email string) (*models.User, error) {
	log.Printf("Repository: Looking for user with email: %s", email)

	user := &models.User{}
	query := `SELECT id, email, password_hash, first_name, last_name, created_at, updated_at 
              FROM users WHERE email = $1`

	err := r.db.QueryRow(query, email).Scan(
		&user.ID, &user.Email, &user.PasswordHash,
		&user.FirstName, &user.LastName, &user.CreatedAt, &user.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("Repository: No user found with email: %s", email)
			return nil, errors.New("user not found")
		}
		log.Printf("Repository: Database error: %v", err)
		return nil, err
	}

	log.Printf("Repository: User found: %s", email)
	return user, nil
}
func (r *UserRepository) UpdateAvatar(userID uuid.UUID, avatarURL string) error {
	query := `
        UPDATE users 
        SET avatar_url = $1, updated_at = $2
        WHERE id = $3
    `
	_, err := r.db.Exec(query, avatarURL, time.Now(), userID)
	return err
}

// GetByID - получение пользователя по ID (с новыми полями)
func (r *UserRepository) GetByID(id uuid.UUID) (*models.User, error) {
	user := &models.User{}
	query := `
        SELECT 
            id, email, password_hash, first_name, last_name,
            is_email_verified, verification_code, created_at, updated_at
        FROM users 
        WHERE id = $1
    `

	err := r.db.QueryRow(query, id).Scan(
		&user.ID, &user.Email, &user.PasswordHash,
		&user.FirstName, &user.LastName,
		&user.IsEmailVerified, &user.VerificationCode,
		&user.CreatedAt, &user.UpdatedAt,
	)

	return user, err
}

// Update - полное обновление данных пользователя
func (r *UserRepository) Update(user *models.User) error {
	query := `
        UPDATE users 
        SET 
            email = $1, 
            password_hash = $2, 
            first_name = $3, 
            last_name = $4,
            is_email_verified = $5, 
            verification_code = $6, 
            updated_at = $7
        WHERE id = $8
    `
	result, err := r.db.Exec(query,
		user.Email, user.PasswordHash, user.FirstName, user.LastName,
		user.IsEmailVerified, user.VerificationCode, user.UpdatedAt,
		user.ID,
	)

	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return errors.New("user not found")
	}

	return nil
}

// UpdateVerificationCode - обновление только кода верификации
func (r *UserRepository) UpdateVerificationCode(userID uuid.UUID, code string) error {
	query := `
        UPDATE users 
        SET verification_code = $1, updated_at = $2 
        WHERE id = $3
    `
	_, err := r.db.Exec(query, code, time.Now(), userID)
	return err
}

// VerifyEmail - подтверждение email (сбрасывает код и устанавливает флаг)
func (r *UserRepository) VerifyEmail(userID uuid.UUID) error {
	query := `
        UPDATE users 
        SET 
            is_email_verified = true, 
            verification_code = NULL, 
            updated_at = $1 
        WHERE id = $2
    `
	_, err := r.db.Exec(query, time.Now(), userID)
	return err
}

// EmailExists - проверка существования email
func (r *UserRepository) EmailExists(email string) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)`
	err := r.db.QueryRow(query, email).Scan(&exists)
	return exists, err
}

// GetUnverifiedUsers - получение всех неподтвержденных пользователей (для админки)
func (r *UserRepository) GetUnverifiedUsers() ([]models.User, error) {
	query := `
        SELECT 
            id, email, first_name, last_name,
            is_email_verified, verification_code, created_at
        FROM users 
        WHERE is_email_verified = false
        ORDER BY created_at DESC
    `

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		var user models.User
		err := rows.Scan(
			&user.ID, &user.Email, &user.FirstName, &user.LastName,
			&user.IsEmailVerified, &user.VerificationCode, &user.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}

	return users, nil
}

// DeleteUnverifiedOldUsers - удаление старых неподтвержденных пользователей (очистка)
func (r *UserRepository) DeleteUnverifiedOldUsers(hours int) (int64, error) {
	query := `
        DELETE FROM users 
        WHERE is_email_verified = false 
        AND created_at < NOW() - INTERVAL '1 hour' * $1
    `
	result, err := r.db.Exec(query, hours)
	if err != nil {
		return 0, err
	}

	return result.RowsAffected()
}
