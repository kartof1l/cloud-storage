package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"cloud-storage-go/internal/models"
	"cloud-storage-go/internal/repository"
)

type ScheduleHandler struct {
	scheduleRepo *repository.ScheduleRepository
	userRepo     *repository.UserRepository
}

func NewScheduleHandler(scheduleRepo *repository.ScheduleRepository, userRepo *repository.UserRepository) *ScheduleHandler {
	return &ScheduleHandler{
		scheduleRepo: scheduleRepo,
		userRepo:     userRepo,
	}
}

// checkIsAdmin проверяет, является ли пользователь администратором
func (h *ScheduleHandler) checkIsAdmin(c *gin.Context) bool {
	userIDStr, exists := c.Get("user_id")
	if !exists {
		return false
	}

	// Преобразуем string в UUID
	userID, err := uuid.Parse(userIDStr.(string))
	if err != nil {
		return false
	}

	user, err := h.userRepo.GetByID(userID)
	if err != nil {
		return false
	}

	// Проверяем, есть ли пользователь в таблице администраторов библиотеки
	var isAdmin bool
	query := `SELECT EXISTS(SELECT 1 FROM library_admins WHERE email = $1)`
	err = h.scheduleRepo.DB().QueryRow(query, user.Email).Scan(&isAdmin)
	if err != nil {
		return false
	}
	return isAdmin
}

// GetEvents - доступен всем авторизованным пользователям
func (h *ScheduleHandler) GetEvents(c *gin.Context) {
	events, err := h.scheduleRepo.GetAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, events)
}

// CreateEvent - только для администраторов
func (h *ScheduleHandler) CreateEvent(c *gin.Context) {
	if !h.checkIsAdmin(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin access required"})
		return
	}

	userIDStr, _ := c.Get("user_id")
	userID, err := uuid.Parse(userIDStr.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	var event models.ScheduleEvent
	if err := c.ShouldBindJSON(&event); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if event.Title == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "title is required"})
		return
	}

	if event.EventDate == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "event date is required"})
		return
	}

	event.CreatedBy = userID.String()
	if err := h.scheduleRepo.Create(&event); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, event)
}

// UpdateEvent - только для администраторов
func (h *ScheduleHandler) UpdateEvent(c *gin.Context) {
	if !h.checkIsAdmin(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin access required"})
		return
	}

	id := c.Param("id")

	var event models.ScheduleEvent
	if err := c.ShouldBindJSON(&event); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if event.Title == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "title is required"})
		return
	}

	event.ID = id
	if err := h.scheduleRepo.Update(&event); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, event)
}

// DeleteEvent - только для администраторов
func (h *ScheduleHandler) DeleteEvent(c *gin.Context) {
	if !h.checkIsAdmin(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin access required"})
		return
	}

	id := c.Param("id")

	if err := h.scheduleRepo.Delete(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}
