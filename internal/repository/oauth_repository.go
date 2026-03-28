package repository

import (
	"database/sql"
	"errors"
	"time"

	"cloud-storage-go/internal/models"

	"github.com/google/uuid"
)

type OAuthRepository struct {
	db *sql.DB
}

func NewOAuthRepository(db *sql.DB) *OAuthRepository {
	return &OAuthRepository{db: db}
}

func (r *OAuthRepository) Create(account *models.OAuthAccount) error {
	query := `
        INSERT INTO oauth_accounts (
            id, user_id, provider, provider_user_id,
            access_token, refresh_token, token_expiry,
            created_at, updated_at
        ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
    `

	_, err := r.db.Exec(query,
		uuid.New(), account.UserID, account.Provider, account.ProviderUserID,
		account.AccessToken, account.RefreshToken, account.TokenExpiry,
		time.Now(), time.Now(),
	)
	return err
}

func (r *OAuthRepository) Update(account *models.OAuthAccount) error {
	query := `
        UPDATE oauth_accounts 
        SET access_token = $1, refresh_token = $2, token_expiry = $3, updated_at = $4
        WHERE id = $5
    `

	_, err := r.db.Exec(query,
		account.AccessToken, account.RefreshToken, account.TokenExpiry,
		time.Now(), account.ID,
	)
	return err
}

func (r *OAuthRepository) FindByProvider(provider, providerUserID string) (*models.OAuthAccount, error) {
	account := &models.OAuthAccount{}
	query := `
        SELECT id, user_id, provider, provider_user_id,
            access_token, refresh_token, token_expiry,
            created_at, updated_at
        FROM oauth_accounts
        WHERE provider = $1 AND provider_user_id = $2
    `

	err := r.db.QueryRow(query, provider, providerUserID).Scan(
		&account.ID, &account.UserID, &account.Provider, &account.ProviderUserID,
		&account.AccessToken, &account.RefreshToken, &account.TokenExpiry,
		&account.CreatedAt, &account.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, errors.New("account not found")
	}
	return account, err
}

func (r *OAuthRepository) FindByUserIDAndProvider(userID uuid.UUID, provider string) (*models.OAuthAccount, error) {
	account := &models.OAuthAccount{}
	query := `
        SELECT id, user_id, provider, provider_user_id,
            access_token, refresh_token, token_expiry,
            created_at, updated_at
        FROM oauth_accounts
        WHERE user_id = $1 AND provider = $2
    `

	err := r.db.QueryRow(query, userID, provider).Scan(
		&account.ID, &account.UserID, &account.Provider, &account.ProviderUserID,
		&account.AccessToken, &account.RefreshToken, &account.TokenExpiry,
		&account.CreatedAt, &account.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, errors.New("account not found")
	}
	return account, err
}
