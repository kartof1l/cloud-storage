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

func (h *ScheduleHandler) checkIsAdmin(c *gin.Context) bool {
	userIDStr, exists := c.Get("user_id")
	if !exists {
		return false
	}

	userID, err := uuid.Parse(userIDStr.(string))
	if err != nil {
		return false
	}

	user, err := h.userRepo.GetByID(userID)
	if err != nil {
		return false
	}

	var isAdmin bool
	query := `SELECT EXISTS(SELECT 1 FROM library_admins WHERE email = $1)`
	h.scheduleRepo.DB().QueryRow(query, user.Email).Scan(&isAdmin)
	return isAdmin
}

// GET /api/schedule/tasks
func (h *ScheduleHandler) GetTasks(c *gin.Context) {
	tasks, err := h.scheduleRepo.GetAllTasks()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, tasks)
}

// POST /api/schedule/tasks
func (h *ScheduleHandler) CreateTask(c *gin.Context) {
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

	var task models.ScheduleTask
	if err := c.ShouldBindJSON(&task); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if task.Title == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "title is required"})
		return
	}

	task.CreatedBy = userID.String()
	if err := h.scheduleRepo.CreateTask(&task); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, task)
}

// PUT /api/schedule/tasks/:id
func (h *ScheduleHandler) UpdateTask(c *gin.Context) {
	if !h.checkIsAdmin(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin access required"})
		return
	}

	id := c.Param("id")

	var task models.ScheduleTask
	if err := c.ShouldBindJSON(&task); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	task.ID = id
	if err := h.scheduleRepo.UpdateTask(&task); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, task)
}

// PATCH /api/schedule/tasks/:id/toggle
func (h *ScheduleHandler) ToggleTaskComplete(c *gin.Context) {
	if !h.checkIsAdmin(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin access required"})
		return
	}

	id := c.Param("id")

	var req struct {
		Completed bool `json:"completed"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.scheduleRepo.ToggleTaskComplete(id, req.Completed); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "updated"})
}

// DELETE /api/schedule/tasks/:id
func (h *ScheduleHandler) DeleteTask(c *gin.Context) {
	if !h.checkIsAdmin(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin access required"})
		return
	}

	id := c.Param("id")

	if err := h.scheduleRepo.DeleteTask(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}
