package middleware

import (
	"encoding/gob"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/sessions"
)

// SessionKeys - ключи для хранения данных в сессии
var SessionKeys = struct {
	UserID   string
	State    string
	Provider string
	Redirect string
}{
	UserID:   "user_id",
	State:    "oauth_state",
	Provider: "oauth_provider",
	Redirect: "redirect_url",
}

// SessionConfig - конфигурация сессий
type SessionConfig struct {
	SecretKey string
	MaxAge    int  // секунды
	Secure    bool // true для HTTPS
	HttpOnly  bool
	SameSite  http.SameSite
}

// DefaultSessionConfig - конфигурация по умолчанию
func DefaultSessionConfig() *SessionConfig {
	return &SessionConfig{
		SecretKey: getEnv("SESSION_SECRET", "your-session-secret-key-change-me"),
		MaxAge:    86400 * 7,
		Secure:    false, // Важно: false для HTTP
		HttpOnly:  false, // Временно false для отладки
		SameSite:  http.SameSiteLaxMode,
	}
}

// SessionManager - управление сессиями
type SessionManager struct {
	store  *sessions.CookieStore
	name   string
	config *SessionConfig
}

// NewSessionManager - создает новый менеджер сессий
func NewSessionManager(config *SessionConfig) *SessionManager {
	if config == nil {
		config = DefaultSessionConfig()
	}

	// Регистрируем типы для сериализации
	gob.Register(uuid.UUID{})
	gob.Register(time.Time{})
	gob.Register(map[string]interface{}{})

	store := sessions.NewCookieStore([]byte(config.SecretKey))

	// Настройки хранилища
	store.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   config.MaxAge,
		Secure:   config.Secure,
		HttpOnly: config.HttpOnly,
		SameSite: config.SameSite,
	}

	return &SessionManager{
		store:  store,
		name:   "cloud-storage-session",
		config: config,
	}
}

// GetSession - получает сессию из запроса
func (sm *SessionManager) GetSession(c *gin.Context) (*sessions.Session, error) {
	return sm.store.Get(c.Request, sm.name)
}

// GetSessionWithDefaults - получает сессию или создает новую с дефолтными значениями
func (sm *SessionManager) GetSessionWithDefaults(c *gin.Context) *sessions.Session {
	session, _ := sm.store.Get(c.Request, sm.name)

	// Устанавливаем значения по умолчанию если их нет
	if session.Values["created_at"] == nil {
		session.Values["created_at"] = time.Now()
	}

	return session
}

// SaveSession - сохраняет сессию
func (sm *SessionManager) SaveSession(c *gin.Context, session *sessions.Session) error {
	return session.Save(c.Request, c.Writer)
}

// ClearSession - очищает сессию
func (sm *SessionManager) ClearSession(c *gin.Context) error {
	session, _ := sm.store.Get(c.Request, sm.name)
	session.Options.MaxAge = -1 // удалить
	return session.Save(c.Request, c.Writer)
}

// SetUserID - сохраняет ID пользователя в сессии
func (sm *SessionManager) SetUserID(c *gin.Context, userID uuid.UUID) error {
	session, err := sm.GetSession(c)
	if err != nil {
		session, _ = sm.store.New(c.Request, sm.name)
	}

	session.Values[SessionKeys.UserID] = userID
	return sm.SaveSession(c, session)
}

// GetUserID - получает ID пользователя из сессии
func (sm *SessionManager) GetUserID(c *gin.Context) (uuid.UUID, bool) {
	session, err := sm.GetSession(c)
	if err != nil {
		return uuid.UUID{}, false
	}

	userID, ok := session.Values[SessionKeys.UserID].(uuid.UUID)
	if !ok {
		return uuid.UUID{}, false
	}

	return userID, true
}

// SetOAuthState - сохраняет state для OAuth
func (sm *SessionManager) SetOAuthState(c *gin.Context, state, provider string) error {
	session, err := sm.GetSession(c)
	if err != nil {
		log.Printf("SetOAuthState: error getting session: %v, creating new", err)
		session, _ = sm.store.New(c.Request, sm.name)
	}

	session.Values[SessionKeys.State] = state
	session.Values[SessionKeys.Provider] = provider

	log.Printf("SetOAuthState: saving state=%s, provider=%s", state, provider)

	err = sm.SaveSession(c, session)
	if err != nil {
		log.Printf("SetOAuthState: error saving session: %v", err)
		return err
	}

	// Проверяем, установилась ли кука
	cookieHeader := c.Writer.Header().Get("Set-Cookie")
	log.Printf("SetOAuthState: Set-Cookie header: %s", cookieHeader)

	return nil
}

// GetOAuthState - получает и удаляет state для OAuth
// GetOAuthState - получает и удаляет state для OAuth
func (sm *SessionManager) GetOAuthState(c *gin.Context) (string, string, bool) {
	session, err := sm.GetSession(c)
	if err != nil {
		log.Printf("GetOAuthState: error getting session: %v", err)
		return "", "", false
	}

	state, stateOk := session.Values[SessionKeys.State].(string)
	provider, providerOk := session.Values[SessionKeys.Provider].(string)

	log.Printf("GetOAuthState: session values: state=%s (ok=%v), provider=%s (ok=%v)",
		state, stateOk, provider, providerOk)

	if !stateOk || !providerOk {
		log.Printf("GetOAuthState: missing state or provider in session")
		return "", "", false
	}

	// Удаляем использованный state
	delete(session.Values, SessionKeys.State)
	delete(session.Values, SessionKeys.Provider)
	sm.SaveSession(c, session)

	return state, provider, true
}

// SetRedirectURL - сохраняет URL для редиректа после OAuth
func (sm *SessionManager) SetRedirectURL(c *gin.Context, url string) error {
	session, err := sm.GetSession(c)
	if err != nil {
		session, _ = sm.store.New(c.Request, sm.name)
	}

	session.Values[SessionKeys.Redirect] = url
	return sm.SaveSession(c, session)
}

// GetRedirectURL - получает и удаляет URL для редиректа
func (sm *SessionManager) GetRedirectURL(c *gin.Context) (string, bool) {
	session, err := sm.GetSession(c)
	if err != nil {
		return "", false
	}

	url, _ := session.Values[SessionKeys.Redirect].(string)
	delete(session.Values, SessionKeys.Redirect)
	sm.SaveSession(c, session)

	return url, url != ""
}

// SessionMiddleware - middleware для автоматической загрузки сессии
func (sm *SessionManager) SessionMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Загружаем или создаем сессию
		session := sm.GetSessionWithDefaults(c)

		// Сохраняем сессию в контекст для дальнейшего использования
		c.Set("session", session)
		c.Set("session_manager", sm)

		c.Next()
	}
}

// AuthRequired - middleware для проверки аутентификации
func (sm *SessionManager) AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, ok := sm.GetUserID(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "authentication required",
			})
			c.Abort()
			return
		}

		c.Set("user_id", userID)
		c.Next()
	}
}

// Helper functions
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
