package services

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/oauth2"

	"cloud-storage-go/internal/models"
	"cloud-storage-go/internal/oauth" // добавить этот импорт
	"cloud-storage-go/internal/repository"
	"cloud-storage-go/internal/utils"
)

type AuthService struct {
	userRepo     *repository.UserRepository
	oauthRepo    *repository.OAuthRepository // добавить
	jwtUtil      *utils.JWTUtil
	emailService *EmailService
}

func NewAuthService(
	userRepo *repository.UserRepository,
	oauthRepo *repository.OAuthRepository, // Добавлен
	jwtUtil *utils.JWTUtil,
	emailService *EmailService,
) *AuthService {
	return &AuthService{
		userRepo:     userRepo,
		oauthRepo:    oauthRepo, // Добавлен
		jwtUtil:      jwtUtil,
		emailService: emailService,
	}
}

// Register - регистрация с отправкой кода (возвращает RegisterResponse)
func (s *AuthService) Register(req *models.UserRegisterRequest) (*models.RegisterResponse, error) {
	log.Println("========== REGISTER METHOD CALLED ==========")
	log.Printf("Email: %s", req.Email)
	log.Printf("FirstName: %s", req.FirstName)
	log.Printf("LastName: %s", req.LastName)
	// Проверяем, существует ли пользователь
	existingUser, _ := s.userRepo.GetByEmail(req.Email)
	if existingUser != nil {
		return nil, errors.New("user already exists")
	}
	verificationCode, err := s.emailService.GenerateVerificationCode()
	if err != nil {
		return nil, err
	}
	// Хешируем пароль
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	user := &models.User{
		ID:               uuid.New(),
		Email:            req.Email,
		PasswordHash:     string(hashedPassword),
		FirstName:        req.FirstName,
		LastName:         req.LastName,
		IsEmailVerified:  true,
		VerificationCode: verificationCode,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	if err := s.userRepo.Create(user); err != nil {
		return nil, err
	}

	// 🔥 Отправляем письмо с кодом
	go s.emailService.SendVerificationEmail(user.Email, verificationCode, user.FirstName)

	// ✅ Возвращаем RegisterResponse
	return &models.RegisterResponse{
		Message: "Registration successful! Please check your email for verification code.",
		Email:   user.Email,
	}, nil
}

// VerifyEmail - подтверждение email (возвращает AuthResponse с токеном)
func (s *AuthService) VerifyEmail(req *models.VerifyEmailRequest) (*models.AuthResponse, error) {
	user, err := s.userRepo.GetByEmail(req.Email)
	if err != nil {
		return nil, errors.New("user not found")
	}

	if user.VerificationCode != req.Code {
		return nil, errors.New("invalid verification code")
	}

	// Обновляем статус
	user.IsEmailVerified = true
	user.VerificationCode = ""
	user.UpdatedAt = time.Now()

	if err := s.userRepo.Update(user); err != nil {
		return nil, err
	}

	// Генерируем токен
	token, err := s.jwtUtil.GenerateToken(user.ID, user.Email)
	if err != nil {
		return nil, err
	}

	return &models.AuthResponse{
		Token: token,
		User:  user,
	}, nil
}

// ResendCode - повторная отправка кода
func (s *AuthService) ResendCode(req *models.ResendCodeRequest) error {
	user, err := s.userRepo.GetByEmail(req.Email)
	if err != nil {
		return errors.New("user not found")
	}

	if user.IsEmailVerified {
		return errors.New("email already verified")
	}

	// Генерируем новый код
	newCode, err := s.emailService.GenerateVerificationCode()
	if err != nil {
		return err
	}

	user.VerificationCode = newCode
	user.UpdatedAt = time.Now()

	if err := s.userRepo.Update(user); err != nil {
		return err
	}

	// Отправляем новый код
	go s.emailService.SendVerificationEmail(user.Email, newCode, user.FirstName)

	return nil
}

// Login - вход только для подтвержденных email
func (s *AuthService) Login(req *models.UserLoginRequest) (*models.AuthResponse, error) {
	log.Println("========== LOGIN ATTEMPT ==========")
	log.Printf("Email: %s", req.Email)

	// Ищем пользователя
	user, err := s.userRepo.GetByEmail(req.Email)
	if err != nil {
		log.Printf("❌ User not found")
		return nil, errors.New("invalid credentials")
	}

	// Проверяем пароль
	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password))
	if err != nil {
		log.Printf("❌ Password mismatch")
		return nil, errors.New("invalid credentials")
	}

	// ✅ Проверяем подтверждение email
	/*if !user.IsEmailVerified {
		log.Printf("❌ Email not verified")
		return nil, errors.New("Подтвердите ваш адресс почты")
	}*/

	log.Println("✅ Login successful")

	// Генерируем токен
	token, err := s.jwtUtil.GenerateToken(user.ID, user.Email)
	if err != nil {
		return nil, err
	}

	return &models.AuthResponse{
		Token: token,
		User:  user,
	}, nil
}
func (s *AuthService) OAuthLogin(ctx context.Context, userInfo *oauth.UserInfo, token *oauth2.Token) (*models.AuthResponse, error) {
	// Ищем существующий OAuth аккаунт
	oauthAccount, err := s.oauthRepo.FindByProvider(userInfo.Provider, userInfo.ProviderID)

	if err == nil {
		// Аккаунт найден - получаем пользователя
		user, err := s.userRepo.GetByID(oauthAccount.UserID)
		if err != nil {
			return nil, err
		}

		// Обновляем токены
		oauthAccount.AccessToken = token.AccessToken
		if token.RefreshToken != "" {
			oauthAccount.RefreshToken = &token.RefreshToken
		}
		if !token.Expiry.IsZero() {
			oauthAccount.TokenExpiry = &token.Expiry
		}
		s.oauthRepo.Update(oauthAccount)

		// Генерируем JWT
		jwtToken, err := s.jwtUtil.GenerateToken(user.ID, user.Email)
		if err != nil {
			return nil, err
		}

		return &models.AuthResponse{
			Token: jwtToken,
			User:  user,
		}, nil
	}

	// Аккаунт не найден - ищем пользователя по email
	user, err := s.userRepo.GetByEmail(userInfo.Email)
	if err != nil {
		// Создаем нового пользователя
		user = &models.User{
			ID:              uuid.New(),
			Email:           userInfo.Email,
			FirstName:       userInfo.FirstName,
			LastName:        userInfo.LastName,
			AvatarURL:       userInfo.AvatarURL,
			IsEmailVerified: true,
			CreatedAt:       time.Now(),
			UpdatedAt:       time.Now(),
		}
		if err := s.userRepo.Create(user); err != nil {
			return nil, err
		}
	}

	// Создаем OAuth аккаунт
	newOAuthAccount := &models.OAuthAccount{
		ID:             uuid.New(),
		UserID:         user.ID,
		Provider:       userInfo.Provider,
		ProviderUserID: userInfo.ProviderID,
		AccessToken:    token.AccessToken,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	if token.RefreshToken != "" {
		newOAuthAccount.RefreshToken = &token.RefreshToken
	}
	if !token.Expiry.IsZero() {
		newOAuthAccount.TokenExpiry = &token.Expiry
	}

	if err := s.oauthRepo.Create(newOAuthAccount); err != nil {
		return nil, err
	}

	// Генерируем JWT
	jwtToken, err := s.jwtUtil.GenerateToken(user.ID, user.Email)
	if err != nil {
		return nil, err
	}

	return &models.AuthResponse{
		Token: jwtToken,
		User:  user,
	}, nil
}
