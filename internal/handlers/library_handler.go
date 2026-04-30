package handlers

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"cloud-storage-go/internal/middleware"
	"cloud-storage-go/internal/services"
)

type LibraryHandler struct {
	libService *services.LibraryService
}

func NewLibraryHandler(libService *services.LibraryService) *LibraryHandler {
	return &LibraryHandler{libService: libService}
}
func (h *LibraryHandler) GetLibraryStats(c *gin.Context) {
	_, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	stats, err := h.libService.GetLibraryStorageStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// GetItems - получение элементов библиотеки (доступно всем)
func (h *LibraryHandler) GetItems(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var parentID *uuid.UUID
	if parentIDStr := c.Query("parent_id"); parentIDStr != "" {
		pid, err := uuid.Parse(parentIDStr)
		if err == nil {
			parentID = &pid
		}
	}

	items, err := h.libService.GetItems(userID, parentID)
	if err != nil {
		log.Printf("❌ Library GetItems error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, items)
}

// CreateFolder - создание папки в библиотеке (только админ)
func (h *LibraryHandler) CreateFolder(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req struct {
		Name     string     `json:"name" binding:"required"`
		ParentID *uuid.UUID `json:"parent_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ip := c.ClientIP()
	userAgent := c.GetHeader("User-Agent")

	folder, err := h.libService.CreateFolder(userID, req.Name, req.ParentID, ip, userAgent)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, folder)
}

// UploadFile - загрузка файла в библиотеку (только админ)
func (h *LibraryHandler) UploadFile(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file is required"})
		return
	}

	var parentID *uuid.UUID
	if parentIDStr := c.PostForm("parent_id"); parentIDStr != "" {
		pid, err := uuid.Parse(parentIDStr)
		if err == nil {
			parentID = &pid
		}
	}

	src, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to open file"})
		return
	}
	defer src.Close()

	ip := c.ClientIP()
	userAgent := c.GetHeader("User-Agent")

	item, err := h.libService.UploadFile(userID, parentID, file.Filename, src, file.Size, ip, userAgent)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, item)
}

// DownloadFile - скачивание файла из библиотеки (доступно всем)
func (h *LibraryHandler) DownloadFile(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		log.Println("❌ DownloadFile: user not found in context")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	log.Printf("📥 DownloadFile: userID=%s, itemID=%s", userID, c.Param("id"))

	itemID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid item id"})
		return
	}

	data, filename, err := h.libService.DownloadFile(userID, itemID, "")
	if err != nil {
		log.Printf("❌ DownloadFile error: %v", err)
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.Data(http.StatusOK, "application/octet-stream", data)
}

// UpdateItem - обновление элемента (только админ)
func (h *LibraryHandler) UpdateItem(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	itemID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid item id"})
		return
	}

	var req struct {
		Name        string `json:"name" binding:"required"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err = h.libService.UpdateItem(userID, itemID, req.Name, req.Description)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "item updated successfully"})
}

// ListAdmins - список администраторов
func (h *LibraryHandler) ListAdmins(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	// Проверяем, что текущий пользователь админ
	isAdmin, err := h.libService.IsAdmin(userID)
	if err != nil || !isAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin access required"})
		return
	}

	admins, err := h.libService.ListAdmins()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, admins)
}

// AddAdmin - добавление администратора
func (h *LibraryHandler) AddAdmin(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req struct {
		Email string `json:"email" binding:"required,email"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.libService.AddAdmin(req.Email, userID); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "admin added successfully"})
}

// RemoveAdmin - удаление администратора
func (h *LibraryHandler) RemoveAdmin(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	email := c.Param("email")
	if email == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "email required"})
		return
	}

	if err := h.libService.RemoveAdmin(email, userID); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "admin removed successfully"})
}

// DeleteItem - удаление элемента (только админ)
func (h *LibraryHandler) DeleteItem(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	itemID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid item id"})
		return
	}

	ip := c.ClientIP()
	userAgent := c.GetHeader("User-Agent")

	err = h.libService.DeleteItem(userID, itemID, ip, userAgent)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "item deleted successfully"})
}
