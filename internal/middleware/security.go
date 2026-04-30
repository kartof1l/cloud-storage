package middleware

import (
"crypto/rand"
"encoding/base64"
"fmt"

"github.com/gin-gonic/gin"
)

func generateNonce() string {
b := make([]byte, 16)
rand.Read(b)
return base64.StdEncoding.EncodeToString(b)
}

func SecurityHeadersMiddleware() gin.HandlerFunc {
return func(c *gin.Context) {
nonce := generateNonce()

c.Header("Content-Security-Policy", 
fmt.Sprintf("default-src 'self'; script-src 'self' 'nonce-%s'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:;", nonce))
c.Header("X-XSS-Protection", "1; mode=block")
c.Header("X-Content-Type-Options", "nosniff")
c.Header("X-Frame-Options", "DENY")
c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
c.Header("X-Permitted-Cross-Domain-Policies", "none")
c.Header("Referrer-Policy", "strict-origin-when-cross-origin")

c.Set("nonce", nonce)
c.Next()
}
}

func SanitizeInputMiddleware() gin.HandlerFunc {
return func(c *gin.Context) {
contentType := c.GetHeader("Content-Type")
if contentType != "" && 
contentType != "application/json" &&
contentType != "multipart/form-data" &&
contentType != "application/x-www-form-urlencoded" {
c.JSON(415, gin.H{"error": "Unsupported Media Type"})
c.Abort()
return
}
c.Next()
}
}
