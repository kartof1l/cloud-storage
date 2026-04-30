package handlers

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"cloud-storage-go/internal/middleware"
	"cloud-storage-go/internal/repository"
	"cloud-storage-go/internal/services"
)

type AdminHandler struct {
	userRepo     *repository.UserRepository
	auditService *services.AuditService
	libService   *services.LibraryService
	db           *sql.DB
}

func NewAdminHandler(userRepo *repository.UserRepository, auditService *services.AuditService, libService *services.LibraryService, db *sql.DB) *AdminHandler {
	return &AdminHandler{
		userRepo:     userRepo,
		auditService: auditService,
		libService:   libService,
		db:           db,
	}
}

// GetAllUsers - получить всех пользователей
func (h *AdminHandler) GetAllUsers(c *gin.Context) {
	adminID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	isAdmin, _ := h.libService.IsAdmin(adminID)
	if !isAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin access required"})
		return
	}

	rows, err := h.db.Query(`
        SELECT id, email, first_name, last_name, is_email_verified, created_at,
               COALESCE((SELECT SUM(size) FROM files WHERE user_id = users.id AND is_folder = false), 0) as storage_used
        FROM users
        ORDER BY created_at DESC
    `)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	var users []map[string]interface{}
	for rows.Next() {
		var id uuid.UUID
		var email, firstName, lastName string
		var isVerified bool
		var createdAt time.Time
		var storageUsed int64

		rows.Scan(&id, &email, &firstName, &lastName, &isVerified, &createdAt, &storageUsed)

		isAdmin, _ := h.libService.IsAdmin(id)

		users = append(users, map[string]interface{}{
			"id":            id,
			"email":         email,
			"first_name":    firstName,
			"last_name":     lastName,
			"is_verified":   isVerified,
			"is_admin":      isAdmin,
			"storage_used":  storageUsed,
			"storage_limit": 100 * 1024 * 1024, // TODO: из настроек
			"created_at":    createdAt,
		})
	}

	c.JSON(http.StatusOK, users)
}

// ToggleUserBlock - блокировка/разблокировка пользователя
func (h *AdminHandler) ToggleUserBlock(c *gin.Context) {
	adminID, _ := middleware.GetUserID(c)
	isAdmin, _ := h.libService.IsAdmin(adminID)
	if !isAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin access required"})
		return
	}

	_, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	var req struct {
		Blocked bool `json:"blocked"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// TODO: добавить поле is_blocked в таблицу users
	// UPDATE users SET is_blocked = $1 WHERE id = $2

	c.JSON(http.StatusOK, gin.H{"message": "user status updated"})
}

// GetUserByEmail - получить пользователя по email
func (h *AdminHandler) GetUserByEmail(c *gin.Context) {
	adminID, _ := middleware.GetUserID(c)
	isAdmin, _ := h.libService.IsAdmin(adminID)
	if !isAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin access required"})
		return
	}

	email := c.Param("email")

	var id uuid.UUID
	var userEmail, firstName, lastName string
	var isVerified bool
	var createdAt time.Time
	var storageUsed int64

	err := h.db.QueryRow(`
        SELECT id, email, first_name, last_name, is_email_verified, created_at,
               COALESCE((SELECT SUM(size) FROM files WHERE user_id = users.id AND is_folder = false), 0) as storage_used
        FROM users
        WHERE email = $1
    `, email).Scan(&id, &userEmail, &firstName, &lastName, &isVerified, &createdAt, &storageUsed)

	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	isAdminUser, _ := h.libService.IsAdmin(id)

	c.JSON(http.StatusOK, map[string]interface{}{
		"id":            id,
		"email":         userEmail,
		"first_name":    firstName,
		"last_name":     lastName,
		"is_verified":   isVerified,
		"is_admin":      isAdminUser,
		"storage_used":  storageUsed,
		"storage_limit": 100 * 1024 * 1024,
		"created_at":    createdAt,
	})
}

// SetUserStorageLimit - установить лимит хранилища для пользователя
func (h *AdminHandler) SetUserStorageLimit(c *gin.Context) {
	adminID, _ := middleware.GetUserID(c)
	isAdmin, _ := h.libService.IsAdmin(adminID)
	if !isAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin access required"})
		return
	}

	userID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	var req struct {
		StorageLimit int64 `json:"storage_limit"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// TODO: добавить поле storage_limit в таблицу users
	_ = userID
	_ = req.StorageLimit

	c.JSON(http.StatusOK, gin.H{"message": "storage limit updated"})
}

// CleanInactiveUsers - очистка неактивных пользователей
func (h *AdminHandler) CleanInactiveUsers(c *gin.Context) {
	adminID, _ := middleware.GetUserID(c)
	isAdmin, _ := h.libService.IsAdmin(adminID)
	if !isAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin access required"})
		return
	}

	var req struct {
		Days int `json:"days"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// TODO: удалить пользователей, неактивных более X дней
	// DELETE FROM users WHERE last_login_at < NOW() - INTERVAL '1 day' * $1 AND is_email_verified = false

	c.JSON(http.StatusOK, gin.H{"deleted_count": 0})
}

// SetDefaultLimit - установить лимит по умолчанию
func (h *AdminHandler) SetDefaultLimit(c *gin.Context) {
	adminID, _ := middleware.GetUserID(c)
	isAdmin, _ := h.libService.IsAdmin(adminID)
	if !isAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin access required"})
		return
	}

	var req struct {
		DefaultLimit int64 `json:"default_limit"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// TODO: сохранить в настройки
	_ = req.DefaultLimit

	c.JSON(http.StatusOK, gin.H{"message": "default limit updated"})
}

// SetMaxFileSize - установить максимальный размер файла
func (h *AdminHandler) SetMaxFileSize(c *gin.Context) {
	adminID, _ := middleware.GetUserID(c)
	isAdmin, _ := h.libService.IsAdmin(adminID)
	if !isAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin access required"})
		return
	}

	var req struct {
		MaxFileSize int64 `json:"max_file_size"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// TODO: сохранить в настройки
	_ = req.MaxFileSize

	c.JSON(http.StatusOK, gin.H{"message": "max file size updated"})
}

// GetAllLogs - получить все логи
func (h *AdminHandler) GetAllLogs(c *gin.Context) {
	adminID, _ := middleware.GetUserID(c)
	isAdmin, _ := h.libService.IsAdmin(adminID)
	if !isAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin access required"})
		return
	}

	logs, _, err := h.auditService.GetLogs(1, 1000)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, logs)
}

// GetUserLogs - получить логи конкретного пользователя
func (h *AdminHandler) GetUserLogs(c *gin.Context) {
	adminID, _ := middleware.GetUserID(c)
	isAdmin, _ := h.libService.IsAdmin(adminID)
	if !isAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin access required"})
		return
	}

	email := c.Param("email")

	rows, err := h.db.Query(`
        SELECT id, user_id, user_email, action, entity_type, entity_id, entity_name, details, ip_address, user_agent, created_at
        FROM audit_logs
        WHERE user_email = $1
        ORDER BY created_at DESC
        LIMIT 500
    `, email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	var logs []map[string]interface{}
	for rows.Next() {
		var id uuid.UUID
		var userID uuid.UUID
		var userEmail, action, entityType string
		var entityID sql.NullString
		var entityName, details, ipAddress, userAgent sql.NullString
		var createdAt time.Time

		rows.Scan(&id, &userID, &userEmail, &action, &entityType, &entityID, &entityName, &details, &ipAddress, &userAgent, &createdAt)

		logs = append(logs, map[string]interface{}{
			"id":          id,
			"user_id":     userID,
			"user_email":  userEmail,
			"action":      action,
			"entity_type": entityType,
			"entity_id":   entityID.String,
			"entity_name": entityName.String,
			"details":     details.String,
			"ip_address":  ipAddress.String,
			"user_agent":  userAgent.String,
			"created_at":  createdAt,
		})
	}

	c.JSON(http.StatusOK, logs)
}
