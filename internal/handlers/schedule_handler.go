package handlers

import (
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
	return &ScheduleHandler{scheduleRepo: scheduleRepo, userRepo: userRepo}
}

func (h *ScheduleHandler) checkIsAdmin(c *gin.Context) bool {
	uid, ok := c.Get("user_id")
	if !ok {
		return false
	}
	userID, err := uuid.Parse(uid.(string))
	if err != nil {
		return false
	}
	user, err := h.userRepo.GetByID(userID)
	if err != nil {
		return false
	}
	var isAdmin bool
	h.scheduleRepo.DB().QueryRow("SELECT EXISTS(SELECT 1 FROM library_admins WHERE email=$1)", user.Email).Scan(&isAdmin)
	return isAdmin
}

func (h *ScheduleHandler) GetTasks(c *gin.Context) {
	tasks, err := h.scheduleRepo.GetAllTasks()
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	// Фильтрация по региону (если передан параметр)
	region := c.Query("region")
	if region != "" {
		var filtered []models.ScheduleTask
		for _, t := range tasks {
			if t.Region == region || t.Region == "" {
				filtered = append(filtered, t)
			}
		}
		tasks = filtered
	}

	// Получаем список доступных регионов
	regions := getAvailableRegionsFromTasks(tasks)

	c.JSON(200, gin.H{
		"tasks":             tasks,
		"current_region":    region,
		"available_regions": regions,
	})
}

func (h *ScheduleHandler) GetTasksByRegion(c *gin.Context) {
	region := c.Param("region")
	tasks, err := h.scheduleRepo.GetAllTasks()
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	var filtered []models.ScheduleTask
	for _, t := range tasks {
		if t.Region == region || t.Region == "" {
			filtered = append(filtered, t)
		}
	}

	c.JSON(200, filtered)
}

func (h *ScheduleHandler) GetAvailableRegions(c *gin.Context) {
	tasks, err := h.scheduleRepo.GetAllTasks()
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	regions := getAvailableRegionsFromTasks(tasks)
	c.JSON(200, regions)
}

func (h *ScheduleHandler) CreateTask(c *gin.Context) {
	if !h.checkIsAdmin(c) {
		c.JSON(403, gin.H{"error": "admin only"})
		return
	}
	uid, _ := c.Get("user_id")
	userID, _ := uuid.Parse(uid.(string))

	var task models.ScheduleTask
	if err := c.ShouldBindJSON(&task); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if task.Title == "" {
		c.JSON(400, gin.H{"error": "title is required"})
		return
	}

	task.CreatedBy = userID.String()
	if err := h.scheduleRepo.CreateTask(&task); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, task)
}

func (h *ScheduleHandler) UpdateTask(c *gin.Context) {
	if !h.checkIsAdmin(c) {
		c.JSON(403, gin.H{"error": "admin only"})
		return
	}
	var task models.ScheduleTask
	if err := c.ShouldBindJSON(&task); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	task.ID = c.Param("id")
	if err := h.scheduleRepo.UpdateTask(&task); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, task)
}

func (h *ScheduleHandler) ToggleTaskComplete(c *gin.Context) {
	if !h.checkIsAdmin(c) {
		c.JSON(403, gin.H{"error": "admin only"})
		return
	}
	var req struct {
		Completed bool `json:"completed"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if err := h.scheduleRepo.ToggleTaskComplete(c.Param("id"), req.Completed); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"message": "updated"})
}

func (h *ScheduleHandler) DeleteTask(c *gin.Context) {
	if !h.checkIsAdmin(c) {
		c.JSON(403, gin.H{"error": "admin only"})
		return
	}
	if err := h.scheduleRepo.DeleteTask(c.Param("id")); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"message": "deleted"})
}

// Вспомогательные функции
func getAvailableRegionsFromTasks(tasks []models.ScheduleTask) []string {
	seen := make(map[string]bool)
	var regions []string
	for _, t := range tasks {
		if t.Region != "" && !seen[t.Region] {
			seen[t.Region] = true
			regions = append(regions, t.Region)
		}
	}
	if len(regions) == 0 {
		return []string{}
	}
	return regions
}
