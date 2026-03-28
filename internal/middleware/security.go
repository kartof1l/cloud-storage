package middleware

import (
	"crypto/rand"
	"encoding/base64"

	"github.com/gin-gonic/gin"
)

// generateNonce создает случайный nonce для CSP
func generateNonce() string {
	b := make([]byte, 16)
	rand.Read(b)
	return base64.StdEncoding.EncodeToString(b)
}

// SecurityHeadersMiddleware добавляет заголовки безопасности
func SecurityHeadersMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Временно отключаем CSP для отладки
		// c.Header("Content-Security-Policy", "...")

		// Оставляем остальные заголовки
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")

		c.Next()
	}
}

// SanitizeInputMiddleware очищает входные данные
func SanitizeInputMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Проверяем Content-Type
		contentType := c.GetHeader("Content-Type")
		if contentType != "" && contentType != "application/json" &&
			contentType != "multipart/form-data" &&
			contentType != "application/x-www-form-urlencoded" {
			c.JSON(415, gin.H{"error": "Unsupported Media Type"})
			c.Abort()
			return
		}
		c.Next()
	}
}
