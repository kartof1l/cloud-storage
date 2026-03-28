package handlers

import (
	"cloud-storage-go/internal/middleware"
	"cloud-storage-go/internal/models"
	"cloud-storage-go/internal/services"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type FolderHandler struct {
	fileService *services.FileService
}

func NewFolderHandler(fileService *services.FileService) *FolderHandler {
	return &FolderHandler{
		fileService: fileService,
	}
}

// MoveFolder - перемещение папки
func (h *FolderHandler) MoveFolder(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	folderID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid folder id"})
		return
	}

	var req struct {
		ParentFolderID *uuid.UUID `json:"parent_folder_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.fileService.MoveFolder(userID, folderID, req.ParentFolderID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "folder moved successfully"})
}
func (h *FolderHandler) CreateFolder(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req models.FolderCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ip := c.ClientIP()
	userAgent := c.GetHeader("User-Agent")

	folder, err := h.fileService.CreateFolder(userID, req.Name, req.ParentFolderID, ip, userAgent)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, folder)
}

func (h *FolderHandler) GetFolderContents(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	folderIDStr := c.Param("id")
	if folderIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "folder id is required"})
		return
	}

	folderID, err := uuid.Parse(folderIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid folder id"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	files, total, err := h.fileService.GetUserFiles(userID, &folderID, page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, models.FileListResponse{
		Files: files,
		Total: total,
		Page:  page,
		Limit: limit,
	})
}

func (h *FolderHandler) RenameFolder(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	folderID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid folder id"})
		return
	}

	var req struct {
		Name string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.fileService.RenameFolder(userID, folderID, req.Name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "folder renamed successfully"})
}

func (h *FolderHandler) DeleteFolder(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	folderIDStr := c.Param("id")
	if folderIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "folder id is required"})
		return
	}

	folderID, err := uuid.Parse(folderIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid folder id"})
		return
	}

	ip := c.ClientIP()
	userAgent := c.GetHeader("User-Agent")

	log.Printf(" Deleting folder %s for user %s", folderID, userID)

	if err := h.fileService.DeleteFolder(userID, folderID, ip, userAgent); err != nil {
		log.Printf(" DeleteFolder error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "folder deleted successfully"})
}
