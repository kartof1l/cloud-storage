package handlers

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"cloud-storage-go/internal/middleware"
	"cloud-storage-go/internal/oauth"
	"cloud-storage-go/internal/services"
)

type OAuthHandler struct {
	providers      map[string]oauth.Provider
	sessionManager *middleware.SessionManager
	authService    *services.AuthService
}

func NewOAuthHandler(
	google, yandex, vk oauth.Provider,
	sessionManager *middleware.SessionManager,
	authService *services.AuthService,
) *OAuthHandler {
	providers := make(map[string]oauth.Provider)
	providers[google.Name()] = google
	providers[yandex.Name()] = yandex
	providers[vk.Name()] = vk

	return &OAuthHandler{
		providers:      providers,
		sessionManager: sessionManager,
		authService:    authService,
	}
}

// generateState создает случайный state для защиты от CSRF
func generateState() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// Login - перенаправляет пользователя на страницу провайдера
// Login - перенаправляет пользователя на страницу провайдера
func (h *OAuthHandler) Login(c *gin.Context) {
	provider := c.Param("provider")
	log.Printf("========== OAuth LOGIN ==========")
	log.Printf("Provider: %s", provider)

	p, exists := h.providers[provider]
	if !exists {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported provider"})
		return
	}

	// Генерируем state
	state, err := generateState()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate state"})
		return
	}

	log.Printf("Generated state: %s", state)

	// Пытаемся сохранить state в сессию
	if err := h.sessionManager.SetOAuthState(c, state, provider); err != nil {
		log.Printf("Warning: failed to save state to session: %v", err)
	}

	// Получаем базовый URL авторизации
	authURL := p.GetAuthURL(state)

	// ВСЕГДА добавляем custom_state в URL для надежности
	// Проверяем, есть ли уже параметры
	if authURL != "" {
		if authURL[len(authURL)-1] == '?' {
			authURL = authURL + "custom_state=" + state
		} else {
			authURL = authURL + "&custom_state=" + state
		}
	}

	log.Printf("Redirecting to: %s", authURL)
	c.Redirect(http.StatusTemporaryRedirect, authURL)
}

// Callback - обрабатывает callback от провайдера
func (h *OAuthHandler) Callback(c *gin.Context) {
	provider := c.Param("provider")

	log.Printf("========== OAuth CALLBACK ==========")
	log.Printf("Provider: %s", provider)
	log.Printf("Full URL: %s", c.Request.URL.String())

	p, exists := h.providers[provider]
	if !exists {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported provider"})
		return
	}

	// Получаем параметры из запроса
	code := c.Query("code")
	state := c.Query("state")
	customState := c.Query("custom_state")

	log.Printf("Code: %s", code[:min(20, len(code))]+"...")
	log.Printf("State from Google: %s", state)
	log.Printf("Custom state from URL: %s", customState)

	if code == "" || state == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing code or state"})
		return
	}

	// Пытаемся получить state из сессии
	savedState, savedProvider, ok := h.sessionManager.GetOAuthState(c)

	// Если в сессии нет, используем customState из URL
	var finalState string
	var finalProvider string

	if ok {
		finalState = savedState
		finalProvider = savedProvider
		log.Printf("Using state from session: %s", finalState)
	} else if customState != "" {
		finalState = customState
		finalProvider = provider
		log.Printf("Using state from URL: %s", finalState)
	} else {
		log.Printf("❌ No state found in session or URL")
		c.JSON(http.StatusForbidden, gin.H{"error": "no oauth state found"})
		return
	}

	// Проверяем state
	if finalState != state {
		log.Printf("❌ State mismatch: saved=%s, received=%s", finalState, state)
		c.JSON(http.StatusForbidden, gin.H{"error": "invalid state parameter"})
		return
	}

	if finalProvider != provider {
		log.Printf("❌ Provider mismatch: saved=%s, received=%s", finalProvider, provider)
		c.JSON(http.StatusForbidden, gin.H{"error": "provider mismatch"})
		return
	}

	log.Printf("✅ State verified successfully")

	// Обмениваем код на токен
	token, err := p.ExchangeCode(c.Request.Context(), code)
	if err != nil {
		log.Printf("❌ Failed to exchange code: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to exchange code"})
		return
	}
	log.Printf("✅ Token obtained")

	// Получаем информацию о пользователе
	userInfo, err := p.GetUserInfo(c.Request.Context(), token)
	if err != nil {
		log.Printf("❌ Failed to get user info: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get user info"})
		return
	}
	log.Printf("✅ User info: email=%s, name=%s", userInfo.Email, userInfo.FullName)

	// Аутентифицируем или создаем пользователя
	authResponse, err := h.authService.OAuthLogin(c.Request.Context(), userInfo, token)
	if err != nil {
		log.Printf("❌ OAuth login failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	log.Printf("✅ User authenticated: %s", authResponse.User.Email)

	// Сохраняем user_id в сессии
	if err := h.sessionManager.SetUserID(c, authResponse.User.ID); err != nil {
		log.Printf("⚠️ Failed to save user session: %v", err)
	}

	// Перенаправляем на дашборд с токеном
	redirectURL := fmt.Sprintf("/dashboard?token=%s", authResponse.Token)
	log.Printf("Redirecting to: %s", redirectURL)
	c.Redirect(http.StatusTemporaryRedirect, redirectURL)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
