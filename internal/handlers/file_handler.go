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

type FileHandler struct {
	fileService *services.FileService
}

func NewFileHandler(fileService *services.FileService) *FileHandler {
	return &FileHandler{
		fileService: fileService,
	}
}

func (h *FileHandler) UploadFile(c *gin.Context) {
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

	var parentFolderID *uuid.UUID
	if folderIDStr := c.PostForm("folder_id"); folderIDStr != "" && folderIDStr != "null" && folderIDStr != "undefined" {
		fid, err := uuid.Parse(folderIDStr)
		if err == nil {
			parentFolderID = &fid
			log.Printf("Uploading to folder: %s", fid)
		}
	}

	log.Printf("Uploading file: %s, user: %s, parentFolder: %v", file.Filename, userID, parentFolderID)

	src, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to open file"})
		return
	}
	defer src.Close()

	ip := c.ClientIP()
	userAgent := c.GetHeader("User-Agent")

	resp, err := h.fileService.UploadFile(userID, parentFolderID, file.Filename, src, file.Size, ip, userAgent)
	if err != nil {
		log.Printf("Upload error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, resp)
}

func (h *FileHandler) ListFiles(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	var parentFolderID *uuid.UUID
	if folderIDStr := c.Query("folder_id"); folderIDStr != "" {
		fid, err := uuid.Parse(folderIDStr)
		if err == nil {
			parentFolderID = &fid
		}
	}

	files, total, err := h.fileService.GetUserFiles(userID, parentFolderID, page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if files == nil {
		files = []models.File{}
	}

	c.JSON(http.StatusOK, models.FileListResponse{
		Files: files,
		Total: total,
		Page:  page,
		Limit: limit,
	})
}

func (h *FileHandler) RenameFile(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	fileID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid file id"})
		return
	}

	var req struct {
		Name string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.fileService.RenameFile(userID, fileID, req.Name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "file renamed successfully"})
}

func (h *FileHandler) GetFile(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	fileID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid file id"})
		return
	}

	file, err := h.fileService.GetFileByID(fileID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
		return
	}

	if file.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
		return
	}

	c.JSON(http.StatusOK, file)
}

func (h *FileHandler) GetStorageStats(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	stats, err := h.fileService.GetUserStorageStats(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, stats)
}

func (h *FileHandler) DownloadFile(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	fileID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid file id"})
		return
	}

	file, err := h.fileService.GetFileByID(fileID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
		return
	}

	if file.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
		return
	}

	if file.IsFolder {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot download a folder"})
		return
	}

	if file.Path == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "file path is empty"})
		return
	}

	c.FileAttachment(file.Path, file.OriginalName)
}

func (h *FileHandler) DeleteFile(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	fileID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid file id"})
		return
	}

	ip := c.ClientIP()
	userAgent := c.GetHeader("User-Agent")

	if err := h.fileService.DeleteFile(userID, fileID, ip, userAgent); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "file deleted successfully"})
}

// MoveFile - перемещение файла
func (h *FileHandler) MoveFile(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	fileID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid file id"})
		return
	}

	var req struct {
		ParentFolderID *uuid.UUID `json:"parent_folder_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.fileService.MoveFile(userID, fileID, req.ParentFolderID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "file moved successfully"})
}
