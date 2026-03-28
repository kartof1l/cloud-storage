package utils

import (
	"log"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type JWTUtil struct {
	secretKey []byte
	expiresIn time.Duration
}

func NewJWTUtil(secretKey string, expiresIn time.Duration) *JWTUtil {
	log.Printf("JWTUtil initialized with secret length: %d", len(secretKey))
	return &JWTUtil{
		secretKey: []byte(secretKey),
		expiresIn: expiresIn,
	}
}

func (j *JWTUtil) GenerateToken(userID uuid.UUID, email string) (string, error) {
	now := time.Now()

	// Используем MapClaims вместо кастомной структуры
	claims := jwt.MapClaims{
		"user_id": userID.String(),
		"email":   email,
		"exp":     now.Add(j.expiresIn).Unix(),
		"iat":     now.Unix(),
		"nbf":     now.Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString(j.secretKey)

	if err != nil {
		log.Printf("Error generating token: %v", err)
		return "", err
	}

	log.Printf("Generated token for user %s", userID)
	return signedToken, nil
}

func (j *JWTUtil) ValidateToken(tokenString string) (*jwt.Token, error) {
	log.Printf("Validating token: %s", tokenString)

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Проверяем метод подписи
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			log.Printf("Unexpected signing method: %v", token.Header["alg"])
			return nil, jwt.ErrSignatureInvalid
		}
		return j.secretKey, nil
	})

	if err != nil {
		log.Printf("Token parse error: %v", err)
		return nil, err
	}

	log.Printf("Token validated successfully")
	return token, nil
}
