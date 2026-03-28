package models

import (
	"time"

	"github.com/google/uuid"
)

// User - основная модель пользователя
type User struct {
	ID               uuid.UUID `json:"id" db:"id"`
	Email            string    `json:"email" db:"email"`
	PasswordHash     string    `json:"-" db:"password_hash"`
	FirstName        string    `json:"first_name" db:"first_name"`
	LastName         string    `json:"last_name" db:"last_name"`
	AvatarURL        string    `json:"avatar_url,omitempty" db:"avatar_url"`
	IsEmailVerified  bool      `json:"is_email_verified" db:"is_email_verified"`
	VerificationCode string    `json:"-" db:"verification_code"`
	CreatedAt        time.Time `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time `json:"updated_at" db:"updated_at"`
}

// OAuthAccount - модель для OAuth аккаунтов
type OAuthAccount struct {
	ID             uuid.UUID  `json:"id" db:"id"`
	UserID         uuid.UUID  `json:"user_id" db:"user_id"`
	Provider       string     `json:"provider" db:"provider"`
	ProviderUserID string     `json:"provider_user_id" db:"provider_user_id"`
	AccessToken    string     `json:"-" db:"access_token"`
	RefreshToken   *string    `json:"-" db:"refresh_token"`
	TokenExpiry    *time.Time `json:"-" db:"token_expiry"`
	CreatedAt      time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at" db:"updated_at"`
}

// UserRegisterRequest - запрос на регистрацию
type UserRegisterRequest struct {
	Email     string `json:"email" binding:"required,email"`
	Password  string `json:"password" binding:"required,min=6"`
	FirstName string `json:"first_name" binding:"required"`
	LastName  string `json:"last_name" binding:"required"`
}

// UserLoginRequest - запрос на вход
type UserLoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// AuthResponse - ответ с токеном
type AuthResponse struct {
	Token string `json:"token"`
	User  *User  `json:"user"`
}

// RegisterResponse - ответ после регистрации
type RegisterResponse struct {
	Message string `json:"message"`
	Email   string `json:"email"`
}

// VerifyEmailRequest - запрос на подтверждение email
type VerifyEmailRequest struct {
	Email string `json:"email" binding:"required,email"`
	Code  string `json:"code" binding:"required,len=6"`
}

// ResendCodeRequest - запрос на повторную отправку кода
type ResendCodeRequest struct {
	Email string `json:"email" binding:"required,email"`
}
