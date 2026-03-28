package handlers

import (
    "net/http"
    "strconv"

    "github.com/gin-gonic/gin"

    "cloud-storage-go/internal/middleware"
    "cloud-storage-go/internal/services"
)

type AuditHandler struct {
    auditService *services.AuditService
    libraryService *services.LibraryService
}

func NewAuditHandler(auditService *services.AuditService, libraryService *services.LibraryService) *AuditHandler {
    return &AuditHandler{
        auditService:   auditService,
        libraryService: libraryService,
    }
}

func (h *AuditHandler) GetLogs(c *gin.Context) {
    userID, exists := middleware.GetUserID(c)
    if !exists {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
        return
    }

    // Проверяем, что пользователь админ
    isAdmin, err := h.libraryService.IsAdmin(userID)
    if err != nil || !isAdmin {
        c.JSON(http.StatusForbidden, gin.H{"error": "admin access required"})
        return
    }

    page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
    limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))

    logs, total, err := h.auditService.GetLogs(page, limit)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "logs":  logs,
        "total": total,
        "page":  page,
        "limit": limit,
    })
}
